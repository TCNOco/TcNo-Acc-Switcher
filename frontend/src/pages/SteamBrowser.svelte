<script lang="ts">
  /**
   * The toolbar for a Steam Guard session window.
   *
   * The page itself is not here. It lives in a native content view attached
   * beneath this one, so this component draws chrome and issues commands and
   * never has access to the site or its session.
   */
  import { onDestroy, onMount } from "svelte";
  import { t } from "../stores/i18n";
  import { route } from "../stores/nav";
  import { pushToast } from "../stores/toast";
  import * as SteamBrowser from "../../bindings/TcNo-Acc-Switcher/internal/steambrowser/service.js";
  import type { ViewState, Certificate } from "../../bindings/TcNo-Acc-Switcher/internal/steambrowser/models.js";
  import { Events } from "@wailsio/runtime";
  import CertificatePopover from "../components/steambrowser/CertificatePopover.svelte";

  const sessionId = $route.page === "steam-browser" ? $route.sessionId : "";

  let state: ViewState = {
    url: "", title: "", loading: false,
    canGoBack: false, canGoForward: false,
    trusted: false, secure: false, host: "",
  };

  /** The address the user is editing, or null when the bar mirrors the page. */
  let draft: string | null = null;
  let addressInput: HTMLInputElement | undefined;
  let toolbar: HTMLElement | undefined;
  let certificate: Certificate | null = null;
  let certificateError = "";
  let showCertificate = false;
  let dragging = false;

  $: displayed = draft ?? state.url;
  // Off the whitelist the whole window is outlined, not just the bar, so it
  // reads at a glance even when the address is scrolled out of view.
  $: untrusted = !!state.url && !state.trusted;

  function report(next: ViewState): void {
    state = next;
    // A navigation the user did not type discards their half-typed address.
    if (document.activeElement !== addressInput) draft = null;
    showCertificate = false;
    certificate = null;
  }

  async function run(action: () => Promise<unknown>, failureKey: string): Promise<void> {
    try {
      await action();
    } catch (error) {
      pushToast({ type: "error", message: $t(failureKey), duration: 6000 });
    }
  }

  const goBack = () => run(() => SteamBrowser.Back(sessionId), "SteamBrowser_Error_Navigate");
  const goForward = () => run(() => SteamBrowser.Forward(sessionId), "SteamBrowser_Error_Navigate");
  const reload = () => run(() => SteamBrowser.Reload(sessionId), "SteamBrowser_Error_Navigate");
  const stop = () => run(() => SteamBrowser.Stop(sessionId), "SteamBrowser_Error_Navigate");

  function navigate(target: string): void {
    const trimmed = target.trim();
    if (!trimmed) return;
    draft = null;
    addressInput?.blur();
    void run(() => SteamBrowser.Navigate(sessionId, trimmed), "SteamBrowser_Error_Navigate");
  }

  function onAddressKey(event: KeyboardEvent): void {
    if (event.key === "Enter") {
      navigate(displayed);
    } else if (event.key === "Escape") {
      draft = null;
      addressInput?.blur();
    }
  }

  async function toggleCertificate(): Promise<void> {
    if (showCertificate) {
      showCertificate = false;
      return;
    }
    if (!state.secure) return;
    certificate = null;
    certificateError = "";
    showCertificate = true;
    try {
      certificate = await SteamBrowser.Certificate(sessionId);
    } catch (error) {
      certificateError = $t("SteamBrowser_Cert_Unavailable");
    }
  }

  // Dropping a link on the toolbar opens it, the same as typing it.
  function onDrop(event: DragEvent): void {
    event.preventDefault();
    dragging = false;
    const dropped =
      event.dataTransfer?.getData("text/uri-list") ||
      event.dataTransfer?.getData("text/plain") ||
      "";
    // A uri-list may hold several entries and comment lines.
    const first = dropped.split(/\r?\n/).find((line) => line && !line.startsWith("#"));
    if (first) navigate(first);
  }

  /**
   * Tell the host how tall this toolbar is, so the content view sits exactly
   * beneath it. Reported rather than assumed, because the height moves with the
   * theme and the user's font size.
   */
  function reportHeight(): void {
    if (!toolbar) return;
    const height = Math.ceil(toolbar.getBoundingClientRect().height);
    if (height > 0) void SteamBrowser.SetChromeHeight(sessionId, height).catch(() => {});
  }

  let observer: ResizeObserver | undefined;
  let offState: (() => void) | undefined;

  onMount(() => {
    reportHeight();
    observer = new ResizeObserver(reportHeight);
    if (toolbar) observer.observe(toolbar);

    offState = Events.On("steambrowser:state", (event: { data: ViewState }) => {
      if (event?.data) report(event.data);
    });
    // The window may have navigated before this component mounted.
    void SteamBrowser.State(sessionId).then(report).catch(() => {});
  });

  onDestroy(() => {
    observer?.disconnect();
    offState?.();
  });
</script>

<div class="sb" class:sb--untrusted={untrusted}>
  <header
    class="sb__bar"
    role="toolbar"
    tabindex="-1"
    aria-label={$t("SteamBrowser_Address")}
    bind:this={toolbar}
    class:sb__bar--dropping={dragging}
    on:dragover|preventDefault={() => (dragging = true)}
    on:dragleave={() => (dragging = false)}
    on:drop={onDrop}
  >
    <div class="sb__nav">
      <button
        type="button" class="sb__btn" disabled={!state.canGoBack}
        on:click={goBack} title={$t("SteamBrowser_Back")} aria-label={$t("SteamBrowser_Back")}
      >◀</button>
      <button
        type="button" class="sb__btn" disabled={!state.canGoForward}
        on:click={goForward} title={$t("SteamBrowser_Forward")} aria-label={$t("SteamBrowser_Forward")}
      >▶</button>
      {#if state.loading}
        <button
          type="button" class="sb__btn" on:click={stop}
          title={$t("SteamBrowser_Stop")} aria-label={$t("SteamBrowser_Stop")}
        >✕</button>
      {:else}
        <button
          type="button" class="sb__btn" on:click={reload}
          title={$t("SteamBrowser_Reload")} aria-label={$t("SteamBrowser_Reload")}
        >⟳</button>
      {/if}
    </div>

    <div class="sb__address" class:sb__address--trusted={state.trusted}>
      <button
        type="button"
        class="sb__lock"
        class:sb__lock--secure={state.secure}
        disabled={!state.secure}
        on:click={toggleCertificate}
        title={state.secure ? $t("SteamBrowser_Cert_View") : $t("SteamBrowser_NotSecure")}
        aria-label={state.secure ? $t("SteamBrowser_Cert_View") : $t("SteamBrowser_NotSecure")}
      >{state.secure ? "🔒" : "⚠"}</button>
      <input
        bind:this={addressInput}
        class="sb__url"
        type="text"
        spellcheck="false"
        autocomplete="off"
        value={displayed}
        on:input={(e) => (draft = e.currentTarget.value)}
        on:keydown={onAddressKey}
        on:focus={(e) => e.currentTarget.select()}
        aria-label={$t("SteamBrowser_Address")}
      />
    </div>
  </header>

  {#if showCertificate}
    <CertificatePopover
      {certificate}
      error={certificateError}
      on:close={() => (showCertificate = false)}
    />
  {/if}
</div>

<style lang="scss">
  .sb {
    position: fixed;
    inset: 0;
    // Only the toolbar is painted. Everything below is the native content view,
    // which sits in front of this window's webview.
    pointer-events: none;
  }

  // An untrusted page outlines the whole window, not just the address.
  .sb--untrusted::after {
    content: "";
    position: fixed;
    inset: 0;
    border: 2px solid var(--danger, #d9534f);
    pointer-events: none;
    z-index: 5;
  }

  .sb__bar {
    pointer-events: auto;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 10px;
    background: var(--modal-bg, #1b2636);
    border-bottom: 1px solid var(--border, #2c3a4f);
    --wails-draggable: drag;
  }

  .sb__bar--dropping { outline: 2px dashed var(--accent, #4c8dff); outline-offset: -4px; }

  .sb__nav { display: flex; gap: 4px; --wails-draggable: no-drag; }

  .sb__btn {
    min-width: 30px; height: 30px;
    border: 0; border-radius: 6px;
    background: transparent; color: var(--text, #e6edf5);
    cursor: pointer; font-size: 14px; line-height: 1;
  }
  .sb__btn:hover:not(:disabled) { background: var(--hover, #26344a); }
  .sb__btn:disabled { opacity: 0.35; cursor: default; }

  .sb__address {
    flex: 1;
    display: flex; align-items: center; gap: 6px;
    height: 30px; padding: 0 8px;
    border: 1px solid var(--border, #2c3a4f); border-radius: 15px;
    background: var(--input-bg, #101a28);
    --wails-draggable: no-drag;
  }

  // Green only on a whitelisted https origin, which is what the lock claims.
  .sb__address--trusted {
    border-color: var(--success, #3fa45b);
    background: color-mix(in srgb, var(--success, #3fa45b) 12%, var(--input-bg, #101a28));
  }

  .sb__lock {
    border: 0; background: transparent; padding: 0;
    font-size: 12px; line-height: 1; cursor: pointer;
    color: var(--text-muted, #91a3ba);
  }
  .sb__lock--secure { color: var(--success, #3fa45b); }
  .sb__lock:disabled { cursor: default; color: var(--warning, #d9a441); }

  .sb__url {
    flex: 1; min-width: 0;
    border: 0; outline: none; background: transparent;
    color: var(--text, #e6edf5); font-size: 13px;
  }
</style>
