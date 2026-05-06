// TcNo Modern UI helpers — Phase 2
// Wraps the existing JS contracts (swapTo / Set*Notes / getCurrentPage) so the new UI can
// trigger account swaps and inline note saves without going through the legacy modals.

(function () {
    "use strict";

    function currentPlatformPage() {
        // getCurrentPage() is defined in WebFuncs.js — returns "Steam", "Basic", etc.
        if (typeof getCurrentPage === "function") {
            const p = getCurrentPage();
            if (p === "Steam" || p === "Basic") return p;
        }
        return null;
    }

    // ---------------- Switch overlay ----------------
    function ensureOverlay() {
        let el = document.getElementById("m-switch-overlay");
        if (el) return el;
        el = document.createElement("div");
        el.id = "m-switch-overlay";
        el.className = "m-switch-overlay";
        el.style.display = "none";
        el.innerHTML =
            '<div class="m-switch-card">' +
              '<div class="m-spinner"></div>' +
              '<div>' +
                '<div class="m-switch-eyebrow" id="m-switch-eyebrow">SWITCHING</div>' +
                '<div class="m-switch-name">Signing in as <span class="m-accent" id="m-switch-name">…</span></div>' +
              '</div>' +
            '</div>';
        document.body.appendChild(el);
        return el;
    }

    window.tcnoShowSwitchOverlay = function (displayName, platformLabel) {
        const el = ensureOverlay();
        const eyebrow = document.getElementById("m-switch-eyebrow");
        const name = document.getElementById("m-switch-name");
        if (eyebrow) eyebrow.textContent = (platformLabel || currentPlatformPage() || "SWITCHING").toUpperCase();
        if (name) name.textContent = displayName || "…";
        el.style.display = "flex";
    };

    window.tcnoHideSwitchOverlay = function () {
        const el = document.getElementById("m-switch-overlay");
        if (el) el.style.display = "none";
    };

    // ---------------- Switch action ----------------
    // Selects the account's hidden radio (so getSelected() in WebFuncs.js sees it),
    // shows the overlay, then defers to the existing swapTo() pipeline.
    window.tcnoSwitchAccount = function (accId, displayName, ev) {
        if (ev && typeof ev.preventDefault === "function") ev.preventDefault();
        if (ev && typeof ev.stopPropagation === "function") ev.stopPropagation();

        const radio = document.getElementById(accId);
        if (radio) {
            radio.checked = true;
            try { if (typeof selectedItemChanged === "function") selectedItemChanged(); } catch (e) { /* noop */ }
        }

        window.tcnoShowSwitchOverlay(displayName);

        // Failsafe — if swapTo never resolves, hide the overlay so the user isn't stuck.
        const failsafe = setTimeout(window.tcnoHideSwitchOverlay, 12000);

        try {
            if (typeof swapTo === "function") {
                Promise.resolve(swapTo(-1, null))
                    .catch(function () { /* noop */ })
                    .finally(function () {
                        clearTimeout(failsafe);
                        // Hold the overlay briefly so the animation reads, then hide.
                        setTimeout(window.tcnoHideSwitchOverlay, 600);
                    });
            } else {
                clearTimeout(failsafe);
                window.tcnoHideSwitchOverlay();
            }
        } catch (e) {
            clearTimeout(failsafe);
            window.tcnoHideSwitchOverlay();
        }
    };

    // Wraps the existing "Login as selected" button so it shows the overlay.
    window.tcnoSwitchSelected = function (ev) {
        if (ev && typeof ev.preventDefault === "function") ev.preventDefault();
        const radio = document.querySelector("input.acc:checked");
        if (!radio) {
            // Fall through to existing swapTo so it can show its own "select an account" toast.
            if (typeof swapTo === "function") swapTo(-1, ev);
            return;
        }
        const label = document.querySelector('label[for="' + radio.id + '"] .displayName, label[for="' + radio.id + '"] h6.displayName');
        const name = (label && label.textContent) ? label.textContent.trim() : "selected account";
        window.tcnoSwitchAccount(radio.id, name, ev);
    };

    // ---------------- Inline note save ----------------
    window.tcnoSaveNote = async function (accId, note) {
        const platform = currentPlatformPage();
        if (!platform || typeof DotNet === "undefined") return;
        try {
            await DotNet.invokeMethodAsync("TcNo-Acc-Switcher-Server", "Set" + platform + "Notes", accId, note || "");
        } catch (e) {
            console.error("tcnoSaveNote failed:", e);
        }
    };

    // ---------------- Settings-pane scroll-to-section ----------------
    // The legacy SharedSettings emits <h2 class="SettingsHeader"> per section. We map our
    // nav ids to the index of that <h2> within the pane and scroll to it.
    const SECTION_INDEX = {
        language: 0, theme: 1, system: 2, program: 3, statssharing: 4
    };
    window.tcnoScrollSettingsTo = function (sectionId) {
        const pane = document.getElementById("set-pane");
        if (!pane) return;
        const headers = pane.querySelectorAll("h2.SettingsHeader");
        const idx = SECTION_INDEX[sectionId];
        if (idx === undefined || idx >= headers.length) return;
        headers[idx].scrollIntoView({ behavior: "smooth", block: "start" });
    };

    // ---------------- Density toggle persistence ----------------
    const DENSITY_KEY = "tcno.modern.density";
    window.tcnoGetDensity = function () {
        try { return localStorage.getItem(DENSITY_KEY) || "tiles"; }
        catch (e) { return "tiles"; }
    };
    window.tcnoSetDensity = function (density) {
        try { localStorage.setItem(DENSITY_KEY, density); } catch (e) { /* noop */ }
    };
})();
