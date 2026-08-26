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
  import { pageFrameAlert } from "../stores/pageFrameAlert";
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
  let chrome: HTMLElement | undefined;
  let certificate: Certificate | null = null;
  let certificateError = "";
  let showCertificate = false;
  let dragging = false;

  $: displayed = draft ?? state.url;
  // Off the whitelist the whole window is framed, not just the bar, so it reads
  // at a glance even when the address is scrolled out of view.
  $: untrusted = !!state.url && !state.trusted;
  // The frame is the application shell's, so the warning is raised rather than
  // drawn: this component owns nothing that reaches the window's edges.
  $: pageFrameAlert.set(untrusted);

  // Names the site's host and nothing more. The rest of an address can carry
  // tokens and identifiers, and a tooltip is the wrong place to put them: it
  // outlives a glance, follows the pointer around, and lands in screenshots.
  $: addressHint = !state.url
    ? ""
    : state.trusted
      ? $t("SteamBrowser_Trusted_Hint", { host: state.host })
      : $t("SteamBrowser_Untrusted_Hint", { host: state.host });

  function report(next: ViewState): void {
    const moved = next.url !== state.url;
    state = next;
    // A navigation the user did not type discards their half-typed address.
    if (document.activeElement !== addressInput) draft = null;
    // Only a new page invalidates the certificate on show. State also arrives
    // when a page changes its own title, and closing the panel under the user
    // for that would look like the lock icon refusing to stay open.
    if (moved) {
      showCertificate = false;
      certificate = null;
    }
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
   * Tell the host where the page should start: the chrome's bottom edge,
   * measured from the top of the window.
   *
   * The toolbar's own height would not be enough, because the application's
   * title bar sits above it. Measuring rather than assuming also survives the
   * theme or font size moving the toolbar, and covers the certificate panel,
   * which adds to the chrome while it is open.
   */
  function reportHeight(): void {
    if (!chrome) return;
    const bottom = Math.ceil(chrome.getBoundingClientRect().bottom);
    if (bottom > 0) void SteamBrowser.SetChromeHeight(sessionId, bottom).catch(() => {});
  }

  let observer: ResizeObserver | undefined;
  let offState: (() => void) | undefined;

  onMount(() => {
    reportHeight();
    observer = new ResizeObserver(reportHeight);
    if (chrome) observer.observe(chrome);

    offState = Events.On("steambrowser:state", (event: { data: ViewState }) => {
      if (event?.data) report(event.data);
    });
    // The window may have navigated before this component mounted.
    void SteamBrowser.State(sessionId).then(report).catch(() => {});
  });

  onDestroy(() => {
    observer?.disconnect();
    offState?.();
    pageFrameAlert.set(false);
  });
</script>

<div class="sb" class:sb--untrusted={untrusted}>
  <!-- Toolbar and certificate panel share a wrapper, and the wrapper is what is
       measured. Everything below it belongs to the page, so anything drawn
       there would be behind the content view and invisible. -->
  <div class="sb__chrome" bind:this={chrome}>
    <header
      class="sb__bar"
      role="toolbar"
      tabindex="-1"
      aria-label={$t("SteamBrowser_Address")}
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

      <div
      class="sb__address"
      class:sb__address--trusted={state.trusted}
      class:sb__address--untrusted={untrusted}
      title={addressHint}
    >
        <button
          type="button"
          class="sb__lock"
          class:sb__lock--secure={state.secure}
          disabled={!state.secure}
          aria-expanded={showCertificate}
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
</div>

<style lang="scss">
  // The toolbar flows in the normal page position, under the application's
  // title bar. It must not be fixed at the top of the window: that put it
  // behind the title bar and left the resize edges unreachable.
  //
  // Nothing below it is drawn. That area belongs to the native content view,
  // which is placed in front of this window's own webview.
  .sb {
    display: flex;
    flex-direction: column;
    // Fills the page area rather than wrapping the toolbar. The area below the
    // toolbar is never drawn - the content view covers it - but the element has
    // to reach the window's edges for the untrusted frame to show in the strip
    // left clear around it.
    flex: 1 1 auto;
    min-height: 0;
  }

  // Backs the red frame the application shell draws, so the two meet with no
  // seam.
  //
  // The shell's border is a themed width in CSS pixels; the strip the content
  // view leaves clear is the system's resize border, in physical ones. Neither
  // knows the other, so whichever is narrower would leave a hairline of ordinary
  // background between the red border and the page.
  //
  // Drawn as a frame around the page area rather than as a fill behind it. A
  // fill covers the seam just as well, but everything it covers is where the
  // content view lands - so until that view's first paint the window was a sheet
  // of red rather than a red border, which is the one thing the border is meant
  // to distinguish itself from. Left transparent inside, the page area shows the
  // same background as every other page until the view arrives to cover it.
  //
  // Width is a fixed margin over the widest seam that can open up: the resize
  // strip is around eight physical pixels and the shell's frame is three to
  // seven CSS pixels of it, and a scaled display only ever closes the gap. No
  // top edge - the content view starts flush against the toolbar, so there is no
  // seam there to back.
  .sb--untrusted::after {
    content: "";
    flex: 1 1 auto;
    min-height: 0;
    border: 8px solid var(--danger, #d9534f);
    border-top: 0;
  }

  // Everything the window draws for itself. Its bottom edge is where the
  // content view starts, so the certificate panel has to live inside it and in
  // normal flow: an overlay hanging below the toolbar would be covered by the
  // content view, which is a native window in front of this one and cannot be
  // drawn over from here. Opening the panel pushes the page down instead.
  .sb__chrome {
    flex: 0 0 auto;
  }

  .sb__bar {
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

  .sb__address--untrusted {
    border-color: var(--danger, #d9534f);
  }

  .sb__lock {
    border: 0; background: transparent; padding: 0;
    font-size: 12px; line-height: 1; cursor: pointer;
    color: var(--text-muted, #91a3ba);
  }
  .sb__lock--secure { color: var(--success, #3fa45b); }
  .sb__lock:disabled { cursor: default; color: var(--warning, #d9a441); }

  .sb__url {
    flex: 1;
    min-width: 0;
    margin: 0;
    padding: 0;
    border: 0;
    outline: none;
    background: transparent;
    color: var(--text, #e6edf5);
    font-size: 13px;
  }

  // The shared text-input styling underlines a focused input and rings it. Both
  // change the input's box, so the address jumped upwards the moment it was
  // clicked. Selector is chained through the parent because those rules carry a
  // :focus of their own and would otherwise win.
  .sb__address .sb__url:focus,
  .sb__address .sb__url:focus-visible {
    border: 0;
    outline: 0;
    box-shadow: none;
  }

  // Focus is shown on the pill instead, as a ring. It is drawn with a shadow so
  // it takes no space and nothing shifts, and it leaves the border colour alone
  // so a focused address still reads as trusted or not.
  .sb__address:focus-within {
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent, #4c8dff) 55%, transparent);
  }
</style>
