<script lang="ts">
  import { onMount } from "svelte";
  import { get } from "svelte/store";
  import { fade } from "svelte/transition";
  import { cubicOut } from "svelte/easing";
  import { Events } from "@wailsio/runtime";
  import { initStreamerMode } from "./stores/streamerMode";
  import { initScreenCovered } from "./stores/screenCovered";
  import { motionEnabled } from "./lib/animation";
  import { applyAnimationClass } from "./lib/animationClass";
  import { installInputModalityTracking } from "./lib/inputModality";
  import { animationsEnabled, loadAnimationsEnabled } from "./stores/animationSettings";
  import TitleBar from './components/TitleBar.svelte'
  import UpdateBar from './components/UpdateBar.svelte'
  import AppModal from './components/AppModal.svelte'
  import Toast from './components/Toast.svelte'
  import StabilityPrompt from './components/StabilityPrompt.svelte'
  import FileDropOverlay from './components/FileDropOverlay.svelte'
  import UserDataMoveOverlay from './components/UserDataMoveOverlay.svelte'
  import AppLockOverlay from './components/AppLockOverlay.svelte'
  import SecurityProgressOverlay from './components/SecurityProgressOverlay.svelte'
  import ContextMenu from './components/ContextMenu.svelte'
  import BackgroundDropZones from './components/BackgroundDropZones.svelte'
  import ActionBar from './components/ActionBar.svelte'
  import { route, applyNavigateJSON, navigateBackLikeButton, navigateForward } from './stores/nav'
  import { installPageStatsTracking } from "./lib/pageStatsTrack";
  import { loadPageModule, prefetchCommonPages } from "./lib/pageLoaders";
  import { actionBarStatus } from './stores/fileDrop'
  import { t } from "./stores/i18n";
  import { NotifyLaunchUpdateCheck } from "../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import * as PlatformService from "../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { pushToast } from "./stores/toast";
  import { formatToastWithError } from "./lib/formatWailsError";
  import { registerSvgRenderBridge } from "./lib/svgRenderBridge";
  import { runCrashReportPromptIfNeeded } from "./lib/crashReportPrompt";
  import { runLegacyInstallPromptIfNeeded } from "./lib/legacyInstallPrompt";
  import { activeModal, openConfirm } from "./stores/modal";
  import { contextMenu } from "./stores/contextMenu";
  import {
    openSearchOverlay,
    searchOverlayCtrl,
    searchOverlayPendingAppend,
  } from "./stores/searchOverlay";
  import { focusSteamGamesSearch } from "./stores/steamGamesSearch";
  import {
    commandPaletteHotkey,
    eventMatchesCommandPaletteHotkey,
    loadCommandPaletteHotkey,
  } from "./stores/commandPalette";
  import { platformActionBusy } from "./stores/platformPage";
  import { pageFrameAlert } from "./stores/pageFrameAlert";
  import {
    listInterruptedRestores,
    loadSecurityStatus,
    repairInterruptedRestore,
    securityStatus,
    securityStatusLoaded,
  } from "./stores/security";
  import { appBgInfo, platformBgInfo, userOverriddenAppBg, UNMEASURED_LUMA } from "./stores/backgroundImage";
  import type { AppBackgroundInfo } from "./stores/backgroundImage";
  import { currentThemeBgUrl } from "./lib/themes";
  import { currentThemeId } from "./lib/theme/stores";
  import { syncBackdropInk, applyBackdropInk } from "./lib/theme/backdropDom";
  import {
    backgroundObjectPosition,
    normalizeBackgroundAlignment,
    normalizeBackgroundFit,
  } from "./lib/backgroundDisplay";
  import { applyUserDataMoveProgress } from "./stores/userDataMove";
  import { createControllerInputController } from "./lib/controllerInput";
  import { controllerSupportEnabled, loadControllerSupportEnabled } from "./stores/controllerSupport";
  import { preventUnmodifiedBrowserContextMenu } from "./lib/actions/contextMenu";
  import { installSteamGuardBridge } from "./lib/steamGuardBridge";
  import { escapeHtml } from "./lib/html";

  function resolveActiveBg(
    r: typeof $route,
    app: AppBackgroundInfo,
    plat: AppBackgroundInfo,
    themeBgUrl: string,
    userOverridden: boolean
  ): AppBackgroundInfo | null {
    const isPlatformPage =
      r.page === "platform" ||
      r.page === "platform-settings" ||
      r.page === "steam-advanced-clearing" ||
      r.page === "steam-confirmations" ||
      r.page === "steam-server-picker";
    if (isPlatformPage && plat.hasImage) return plat;
    if (app.hasImage) return app;
    if (!userOverridden && themeBgUrl) {
      return {
        hasImage: true,
        imageUrl: themeBgUrl,
        opacity: 1.0,
        blur: 0,
        alignment: "center",
        fit: "cover",
        themeBgOverride: false,
        // A theme's bundled background never goes through the backend's measure
        // step, so its brightness is sampled on the client instead.
        luma: UNMEASURED_LUMA,
      };
    }
    return null;
  }

  // The backgrounds arrive async: the app background once at startup, the
  // platform background on every transition. Until both have answered, hold
  // whatever is already showing — recomputing meanwhile fell through to the
  // theme background and flashed it over the user's wallpaper on every
  // transition.
  let activeBg: AppBackgroundInfo | null = null;
  let appBgLoaded = false;
  let platformBgPending = 0;
  $: {
    const next = resolveActiveBg($route, $appBgInfo, $platformBgInfo, $currentThemeBgUrl, $userOverriddenAppBg);
    if (appBgLoaded && platformBgPending === 0) activeBg = next;
  }
  $: showActionBar = $route.page === "home" || $route.page === "platform";

  // Content sitting straight on the wallpaper needs a text colour chosen for that
  // wallpaper, not for the theme. Re-derived whenever the background changes, and
  // again when the theme does, since the theme colour is half of what the image is
  // composited against.
  let bgLayerEl: HTMLImageElement | null = null;
  $: {
    void $currentThemeId;
    if (activeBg?.hasImage) {
      syncBackdropInk(activeBg.luma, activeBg.opacity, bgLayerEl);
    } else {
      applyBackdropInk(null);
    }
  }

  /** Backgrounds the backend never measured get sampled once the image decodes. */
  function onBgLayerLoad(event: Event): void {
    bgLayerEl = event.currentTarget as HTMLImageElement;
    if (activeBg?.hasImage && !activeBg.luma?.measured) {
      syncBackdropInk(activeBg.luma, activeBg.opacity, bgLayerEl);
    }
  }

  let restoreRepairPromptOpen = false;
  let restoreRepairPromptDismissed = false;

  $: if (
    $securityStatusLoaded &&
    !$securityStatus.appLocked &&
    $securityStatus.interruptedRestorePending &&
    !$activeModal &&
    !restoreRepairPromptOpen &&
    !restoreRepairPromptDismissed
  ) {
    void promptInterruptedRestoreRepair();
  }

  /**
   * One shared instance rather than a literal per call. `platformBgInfo` is
   * written from a reactive block and read by another; a fresh object each pass
   * meant the store always looked changed, and under Svelte 5 the two blocks
   * drove each other forever. Identity is the signal that nothing moved.
   */
  const NO_PLATFORM_BG: AppBackgroundInfo = {
    hasImage: false,
    imageUrl: "",
    opacity: 0.6,
    blur: 4.0,
    alignment: "center",
    fit: "cover",
    themeBgOverride: false,
    luma: UNMEASURED_LUMA,
  };

  function clearPlatformBg(): void {
    if (get(platformBgInfo) !== NO_PLATFORM_BG) {
      platformBgInfo.set(NO_PLATFORM_BG);
    }
  }

  /** Load/reload the platform background for the given platform name. */
  async function loadPlatformBg(platformName: string): Promise<void> {
    platformBgPending += 1;
    try {
      const info = await PlatformService.GetPlatformBackground(platformName);
      platformBgInfo.set(info);
    } catch {
      clearPlatformBg();
    } finally {
      platformBgPending -= 1;
    }
  }

  // When the route changes to a platform page, reload the platform background.
  $: {
    const r = $route;
    if (r.page === "platform" || r.page === "platform-settings") {
      void loadPlatformBg(r.platformName);
    } else if (
      r.page === "steam-advanced-clearing" ||
      r.page === "steam-confirmations" ||
      r.page === "steam-server-picker"
    ) {
      void loadPlatformBg("Steam");
    } else {
      clearPlatformBg();
    }
  }

  function isEditableTarget(t: EventTarget | null): boolean {
    if (!t || !(t instanceof HTMLElement)) {
      return false;
    }
    if (t.isContentEditable) {
      return true;
    }
    const tag = t.tagName;
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") {
      return true;
    }
    return t.closest("input, textarea, select, [contenteditable]") !== null;
  }

  async function promptInterruptedRestoreRepair(): Promise<void> {
    restoreRepairPromptOpen = true;
    try {
      const restores = await listInterruptedRestores().catch(() => []);
      const labels = restores
        .map((r) => [r.platformKey, r.accountName || r.uniqueId || r.journalPath].filter(Boolean).join(" / "))
        .filter(Boolean);
      const body = [
        `<p>${escapeHtml($t("Security_InterruptedRestore_Body"))}</p>`,
        labels.length
          ? `<p>${escapeHtml($t("Security_InterruptedRestore_Affected"))}</p><ul>${labels
              .map((label) => `<li>${escapeHtml(label)}</li>`)
              .join("")}</ul>`
          : "",
      ].join("");
      const ok = await openConfirm({
        title: $t("Security_InterruptedRestore_Title"),
        body,
        positiveLabel: $t("Security_InterruptedRestore_Repair"),
        negativeLabel: $t("Security_InterruptedRestore_Later"),
        style: "yesno",
      });
      restoreRepairPromptDismissed = !ok;
      if (!ok) return;
      await repairInterruptedRestore();
      pushToast({ type: "success", message: $t("Security_InterruptedRestore_Repaired"), duration: 5000 });
      await loadSecurityStatus();
    } catch (e) {
      restoreRepairPromptDismissed = true;
      pushToast({
        type: "error",
        message: formatToastWithError($t("Security_InterruptedRestore_RepairFailed"), e),
        duration: 8000,
      });
    } finally {
      restoreRepairPromptOpen = false;
    }
  }

  function onGlobalKeydownCapture(e: KeyboardEvent): void {
    if (get(securityStatus).appLocked) {
      return;
    }
    const r = get(route);
    if (r.page !== "home" && r.page !== "platform") {
      return;
    }
    if (get(activeModal)) {
      return;
    }
    if (get(contextMenu)) {
      return;
    }
    const hotkey = get(commandPaletteHotkey);
    const isConfiguredCommandHotkey = eventMatchesCommandPaletteHotkey(e, hotkey);
    if (isConfiguredCommandHotkey) {
      if (isEditableTarget(e.target)) {
        return;
      }
      e.preventDefault();
      // The Steam games tab has no overlay to open, so the hotkey lands on its
      // inline bar. The overlay it used to raise there listed nothing but games
      // anyway, and never the commands the ">" prefix asks for.
      if (focusSteamGamesSearch()) {
        return;
      }
      openSearchOverlay(">");
      return;
    }
    if (e.ctrlKey || e.metaKey || e.altKey) {
      return;
    }
    if (e.key.length !== 1) {
      return;
    }
    const so = get(searchOverlayCtrl);
    if (so.open) {
      if (isEditableTarget(e.target)) {
        return;
      }
      e.preventDefault();
      searchOverlayPendingAppend.set(e.key);
      return;
    }
    if (isEditableTarget(e.target)) {
      return;
    }
    e.preventDefault();
    if (focusSteamGamesSearch(e.key)) {
      return;
    }
    openSearchOverlay(e.key);
  }

  function canHandleGlobalNavInput(target: EventTarget | null): boolean {
    if (get(securityStatus).appLocked) {
      return false;
    }
    if (get(activeModal) || get(contextMenu)) {
      return false;
    }
    return !isEditableTarget(target);
  }

  function onGlobalHistoryKeydownCapture(e: KeyboardEvent): void {
    if (!canHandleGlobalNavInput(e.target)) {
      return;
    }
    const key = e.key;
    const isBack = key === "BrowserBack" || (e.altKey && key === "ArrowLeft");
    const isForward = key === "BrowserForward" || (e.altKey && key === "ArrowRight");
    if (!isBack && !isForward) {
      return;
    }
    e.preventDefault();
    if (isBack) {
      navigateBackLikeButton();
      return;
    }
    navigateForward();
  }

  function onGlobalHistoryMouseUpCapture(e: MouseEvent): void {
    if (!canHandleGlobalNavInput(e.target)) {
      return;
    }
    if (e.button !== 3 && e.button !== 4) {
      return;
    }
    e.preventDefault();
    if (e.button === 3) {
      navigateBackLikeButton();
      return;
    }
    navigateForward();
  }

  onMount(() => {
    void loadSecurityStatus();
    void loadAnimationsEnabled();
    // Resolves after the first paint; the html class it sets is what censors the
    // account lists, so hydrate it before any of them can mount.
    let offStreamerMode: (() => void) | undefined;
    void initStreamerMode().then((off) => { offStreamerMode = off; });
    // Lets the account list drop its animated avatar frames while a game holds
    // the screen. Hydrates once, then follows the backend's WinEvent hooks.
    initScreenCovered();
    void loadCommandPaletteHotkey();
    // Load initial app background state. Loaded-or-failed both count as
    // settled: on failure there is no user wallpaper to protect, so the theme
    // background may show.
    void PlatformService.GetAppBackground().then((info) => {
      appBgInfo.set(info);
    }).catch(() => {}).finally(() => {
      appBgLoaded = true;
    });

	    const offPageStats = installPageStatsTracking();
	    const offSteamGuard = installSteamGuardBridge();
    const offSvgBridge = registerSvgRenderBridge();
    const offInputModality = installInputModalityTracking();
    let cleanupAnimationsClass = () => {};
    const offAnimationsClass = animationsEnabled.subscribe((enabled) => {
      cleanupAnimationsClass();
      cleanupAnimationsClass = applyAnimationClass(enabled);
    });
    const controllerInput = createControllerInputController();
    const offControllerSupport = controllerSupportEnabled.subscribe((enabled) => {
      controllerInput.setEnabled(enabled);
    });
    void loadControllerSupportEnabled();
    const offNav = Events.On("navigate", (ev) => {
      const raw = typeof ev.data === "string" ? ev.data : "";
      applyNavigateJSON(raw);
    });
    void NotifyLaunchUpdateCheck();
    // Sequenced, not parallel: both open a modal and the store holds one at a time.
    void runCrashReportPromptIfNeeded()
      .catch(() => {})
      .then(() => runLegacyInstallPromptIfNeeded())
      .catch(() => {});

    const schedulePrefetch = (): void => {
      prefetchCommonPages();
    };
    if (typeof requestIdleCallback === "function") {
      requestIdleCallback(schedulePrefetch);
    } else {
      setTimeout(schedulePrefetch, 1500);
    }

    const offUpdateFail = Events.On("update-check-failed", () => {
      pushToast({
        type: "error",
        title: "",
        message: get(t)("Toast_UpdateCheckFail"),
        duration: 15000,
      });
    });

    const offPlatformsFound = Events.On("platforms-json-update-found", (ev) => {
      const version = typeof ev.data === "object" && ev.data && "version" in ev.data
        ? String((ev.data as { version?: string }).version ?? "")
        : "";
      pushToast({
        type: "info",
        title: "",
        message: get(t)("Toast_PlatformsJsonUpdateFound", { version }),
        duration: 8000,
      });
    });

    const offPlatformsUpdated = Events.On("platforms-json-updated", (ev) => {
      const version = typeof ev.data === "object" && ev.data && "version" in ev.data
        ? String((ev.data as { version?: string }).version ?? "")
        : "";
      pushToast({
        type: "success",
        title: "",
        message: get(t)("Toast_PlatformsJsonUpdated", { version }),
        duration: 8000,
      });
    });

    const offUserDataMoveProgress = Events.On("userdata-move-progress", (ev) => {
      const data = ev.data;
      if (typeof data !== "object" || !data) return;
      const payload = data as { phase?: string; done?: number; total?: number };
      applyUserDataMoveProgress({
        phase: payload.phase,
        done: payload.done,
        total: payload.total,
      });
    });

    function parseI18nPayload(raw: string): { key: string; vars?: Record<string, string | number> } {
      const parts = raw.slice(5).split("\u001f");
      const key = parts.shift() ?? "";
      if (parts.length > 1) {
        const vars: Record<string, string | number> = {};
        for (let i = 0; i < parts.length; i += 2) {
          const name = parts[i];
          if (!name) continue;
          vars[name] = parts[i + 1] ?? "";
        }
        return { key, vars };
      }
      if (parts.length === 1) return { key, vars: { platform: parts[0] } };
      return { key };
    }

    const off = Events.On("action-bar-status", (ev) => {
      const raw = typeof ev.data === "string" ? ev.data : "";
      if (raw.startsWith("i18n:")) {
        const { key, vars } = parseI18nPayload(raw);
        actionBarStatus.set(vars ? $t(key, vars) : $t(key));
      } else {
        actionBarStatus.set(raw);
      }
    });


    window.addEventListener("keydown", onGlobalKeydownCapture, true);
    window.addEventListener("keydown", onGlobalHistoryKeydownCapture, true);
    window.addEventListener("mouseup", onGlobalHistoryMouseUpCapture, true);
    window.addEventListener("contextmenu", preventUnmodifiedBrowserContextMenu, true);
    return () => {
      window.removeEventListener("keydown", onGlobalKeydownCapture, true);
      window.removeEventListener("keydown", onGlobalHistoryKeydownCapture, true);
      window.removeEventListener("mouseup", onGlobalHistoryMouseUpCapture, true);
      window.removeEventListener("contextmenu", preventUnmodifiedBrowserContextMenu, true);
	      offPageStats();
	      offSteamGuard();
      off?.();
      offNav?.();
      offUpdateFail?.();
      offPlatformsFound?.();
      offPlatformsUpdated?.();
      offUserDataMoveProgress?.();
      offStreamerMode?.();
      offSvgBridge?.();
      offInputModality();
      offAnimationsClass();
      cleanupAnimationsClass();
      offControllerSupport();
      controllerInput.destroy();
    };
  });
</script>

<div class="container" class:busyCursor={$platformActionBusy.busy} class:animations-disabled={!$animationsEnabled}>
  <FileDropOverlay />
  <UserDataMoveOverlay />
  <ContextMenu />
  <a class="skip-link" href="#app-main">Skip to content</a>
  <TitleBar />
  <UpdateBar />
  <div class="page" class:page--alert={$pageFrameAlert} class:page--has-bg={!!activeBg}>
    {#key activeBg?.imageUrl}
      {#if activeBg}
        <img
          class="bg-layer"
          src={activeBg.imageUrl}
          alt=""
          aria-hidden="true"
          in:fade={{ duration: motionEnabled() ? 350 : 0, easing: cubicOut }}
          out:fade={{ duration: motionEnabled() ? 250 : 0, easing: cubicOut }}
          on:load={onBgLayerLoad}
          style="object-fit: {normalizeBackgroundFit(activeBg.fit)}; object-position: {backgroundObjectPosition(normalizeBackgroundAlignment(activeBg.alignment))}; opacity: {activeBg.opacity}; filter: blur({activeBg.blur}px);"
        />
      {/if}
    {/key}
    <div class="page-content-wrapper">
      {#key $route.page + ("platformName" in $route ? $route.platformName : "")}
        <main id="app-main" class="page-content" tabindex="-1">
          {#await loadPageModule($route) then { default: Page }}
            {#if $route.page === "home"}
              <Page />
            {:else if $route.page === "settings"}
              <Page />
            {:else if $route.page === "preview-css"}
              <Page />
            {:else if $route.page === "platform"}
              <Page name={$route.platformName} />
            {:else if $route.page === "platform-settings"}
              <Page name={$route.platformName} />
            {:else if $route.page === "steam-advanced-clearing"}
              <Page />
            {:else if $route.page === "steam-confirmations"}
              <Page />
            {:else if $route.page === "steam-server-picker"}
              <Page />
            {:else if $route.page === "steam-browser"}
              <Page />
            {:else if $route.page === "manage-platforms"}
              <Page />
            {/if}
          {/await}
        </main>
      {/key}
      {#if showActionBar}
        <ActionBar />
      {/if}
    </div>
    <BackgroundDropZones />
    <AppLockOverlay />
    <SecurityProgressOverlay />
    <AppModal />
    <Toast />
    <StabilityPrompt />
  </div>
</div>

<style>
  .container {
    background: var(--program-bg);
    height: 100vh;
    width: 100vw;
    display: flex;
    flex-direction: column;
  }
  .container.busyCursor,
  .container.busyCursor * {
    cursor: progress !important;
  }
  .skip-link {
    position: absolute;
    left: 1rem;
    top: 1rem;
    z-index: 1000000;
    transform: translateY(calc(-100% - 1rem));
    padding: 0.5rem 0.75rem;
    background: var(--mainContentBackground);
    color: var(--whiteSecondary);
    border: 2px solid var(--accent);
    border-radius: 4px;
    text-decoration: none;
  }
  .skip-link:focus-visible {
    transform: translateY(0);
  }
  .page {
    position: relative;
    isolation: isolate;
    border-left: var(--border-bar-size) solid var(--border-bar-bg);
    border-right: var(--border-bar-size) solid var(--border-bar-bg);
    border-bottom: var(--border-bar-size) solid var(--border-bar-bg);
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }
  /* The frame the window already draws, turned into a warning. Raised by the
     Steam Guard session browser when its page leaves the trusted list; see
     pageFrameAlert for why the warning has to live out here. */
  .page--alert {
    border-color: var(--danger, #d9534f);
  }
  .bg-layer {
    position: absolute;
    inset: -24px;
    z-index: -1;
    width: calc(100% + 48px);
    height: calc(100% + 48px);
    pointer-events: none;
    will-change: opacity;
  }
  .page-content-wrapper {
    position: relative;
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .page-content {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
</style>
