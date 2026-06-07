<script lang="ts">
  import { onMount } from "svelte";
  import { tick } from "svelte";
  import { get } from "svelte/store";
  import { Browser } from "@wailsio/runtime";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import { t } from "../stores/i18n";
  import { pushToast } from "../stores/toast";
  import { activeModal } from "../stores/modal";
  import { RunAdvancedClearingAction } from "../../bindings/TcNo-Acc-Switcher/internal/steam/steamservice.js";
  import "../styles/Settings.scss";

  const WIKI_URL =
    "https://github.com/TCNOCo/TcNo-Acc-Switcher/wiki/Platform:-Steam#steam-cleaning";

  type Row = { actions: string[] };

  /** Button rows (left-to-right); IDs match Go `RunAdvancedClearingAction`. */
  const generalRows: Row[] = [
    { actions: ["clear_logs", "clear_dumps"] },
    { actions: ["clear_htmlcache", "clear_ui_logs"] },
    { actions: ["clear_appcache", "clear_httpcache"] },
    { actions: ["clear_depotcache"] },
  ];

  const loginRows: Row[] = [
    { actions: ["delete_loginusers_vdf", "clear_ssfn"] },
    { actions: ["delete_config_vdf", "reg_autologinuser"] },
    { actions: ["reg_lastgamenameused", "reg_pseudouuid"] },
    { actions: ["reg_rememberpassword"] },
  ];

  const labelText: Record<string, string> = {
    clear_logs: "..\\Steam\\logs",
    clear_dumps: "..\\Steam\\dumps",
    clear_htmlcache: "%Local%\\Steam\\htmlcache",
    clear_ui_logs: "..\\Steam\\*.log",
    clear_appcache: "..\\Steam\\appcache",
    clear_httpcache: "..\\Steam\\appcache\\httpcache",
    clear_depotcache: "..\\Steam\\depotcache",
    delete_loginusers_vdf: "..\\Steam\\config\\loginusers.vdf",
    clear_ssfn: "..\\Steam\\ssfn*",
    delete_config_vdf: "..\\Steam\\config\\config.vdf",
    reg_autologinuser: "HKCU\\..\\AutoLoginUser",
    reg_lastgamenameused: "HKCU\\..\\LastGameNameUsed",
    reg_pseudouuid: "HKCU\\..\\PseudoUUID",
    reg_rememberpassword: "HKCU\\..\\RememberPassword",
  };

  function isRegistryAction(id: string): boolean {
    return id.startsWith("reg_");
  }

  let acceptedRisk = false;
  let registrySupported = false;
  let busy = false;
  let logLines: string[] = [];
  let logEl: HTMLDivElement | null = null;

  $: appBarTitle.set($t("Title_Steam_Cleaning"));

  const i18nLogPrefix = "i18n:";
  const i18nLogSep = "\u001f";

  function isWindowsClient(): boolean {
    if (typeof navigator === "undefined") return false;
    const uaData = (navigator as { userAgentData?: { platform?: string } }).userAgentData;
    const platform = (uaData?.platform || navigator.platform || navigator.userAgent || "").toLowerCase();
    return platform.includes("win");
  }

  onMount(() => {
    previousPage.set({ page: "platform-settings", platformName: "Steam" });
    // UI visibility is client-platform based; backend still enforces OS support.
    registrySupported = isWindowsClient();
  });

  function showAction(_id: string): boolean {
    return true;
  }

  async function scrollLogToBottom(): Promise<void> {
    await tick();
    if (logEl) {
      logEl.scrollTop = logEl.scrollHeight;
    }
  }

  function translateLogLine(line: string): string {
    if (!line.startsWith(i18nLogPrefix)) {
      return line;
    }
    const parts = line.slice(i18nLogPrefix.length).split(i18nLogSep);
    const key = parts.shift() ?? "";
    const vars: Record<string, string | number> = {};
    for (let i = 0; i < parts.length; i += 2) {
      const name = parts[i];
      if (!name) continue;
      vars[name] = parts[i + 1] ?? "";
    }
    return get(t)(key, vars);
  }

  async function runAction(id: string): Promise<void> {
    if (!acceptedRisk || busy) return;
    busy = true;
    try {
      const res = await RunAdvancedClearingAction(id);
      const lines = res?.lines?.length
        ? res.lines.map(translateLogLine)
        : [$t("SteamAdvanced_NoOutput")];
      logLines = [...logLines, ...lines, ""];
      await scrollLogToBottom();
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      logLines = [...logLines, $t("SteamAdvanced_LogError", { message: msg }), ""];
      await scrollLogToBottom();
      pushToast({ type: "error", title: "", message: msg, duration: 10000 });
    } finally {
      busy = false;
    }
  }

  function clearLog(): void {
    logLines = [];
  }

  function onWiki(): void {
    void Browser.OpenURL(WIKI_URL);
  }

  function onClose(): void {
    route.set({ page: "platform-settings", platformName: "Steam" });
  }

  function onWindowKeyDown(e: KeyboardEvent): void {
    if (e.key !== "Escape") return;
    if (get(activeModal)) return;
    onClose();
  }
</script>

<div class="main-content main-spacing steam-adv-root">
  <h1 class="SettingsHeader">{$t("Title_Steam_Cleaning")}</h1>

  <h2 class="SettingsHeader">{$t("Cleaning_ImportantInfoHeader")}</h2>
  <div class="steam-adv-info">
    <!-- eslint-disable-next-line svelte/no-at-html-tags -->
    {@html $t("Cleaning_ImportantInfo")}
  </div>

  <div class="rowSetting steam-adv-ack">
    <div class="form-check">
      <input id="steam-adv-understand" type="checkbox" bind:checked={acceptedRisk} />
      <label class="form-check-label" for="steam-adv-understand"></label>
    </div>
    <span class="steam-adv-ack-text">{$t("Cleaning_Understand")}</span>
  </div>

  <button type="button" disabled={!acceptedRisk || busy} on:click={() => void runAction("close_steam")}>
    {$t("Cleaning_Button_KillProcess", { platform: "Steam" })}
  </button>

  <div class="steam-adv-layout">
    <div class="steam-adv-actions">
      <h2 class="SettingsHeader">{$t("Cleaning_Header_General")}</h2>
      {#each generalRows as row}
        <div class="buttoncol">
          {#each row.actions as id}
            {#if showAction(id)}
              <button
                type="button"
                disabled={!acceptedRisk || busy || (isRegistryAction(id) && !registrySupported)}
                on:click={() => void runAction(id)}
              >
                {labelText[id]}
              </button>
            {/if}
          {/each}
        </div>
      {/each}

      <h2 class="SettingsHeader">{$t("Cleaning_Header_LoginHistory")}</h2>
      {#each loginRows as row}
        <div class="buttoncol">
          {#each row.actions as id}
            {#if showAction(id)}
              <button
                type="button"
                disabled={!acceptedRisk || busy || (isRegistryAction(id) && !registrySupported)}
                on:click={() => void runAction(id)}
              >
                {labelText[id]}
              </button>
            {/if}
          {/each}
        </div>
      {/each}
    </div>

    <div class="steam-adv-log-wrap">
      <div class="steam-adv-log" bind:this={logEl} role="log" aria-live="polite">
        {#if logLines.length === 0}
          <p class="steam-adv-log-empty">{$t("SteamAdvanced_LogPlaceholder")}</p>
        {:else}
          {#each logLines as line, i (i)}
            <div class="steam-adv-log-line">{line || "\u00a0"}</div>
          {/each}
        {/if}
      </div>
      <div class="steam-adv-log-actions">
        <button type="button" class="steam-adv-clear-log" on:click={clearLog}>{$t("Button_ClearLog")}</button>
      </div>
    </div>
  </div>

  <div class="buttoncol col_close steam-adv-footer">
    <button type="button" class="fancyLinkBtn" on:click={onWiki}>{$t("Button_WikiInfo")}</button>
    <button type="button" class="btn_close" on:click={onClose}><span>{$t("Button_Close")}</span></button>
  </div>
</div>

<svelte:window on:keydown={onWindowKeyDown} />

<style lang="scss">
  .steam-adv-root {
    overflow-y: auto;
    flex: 1;
    min-height: 0;
    padding-bottom: 1rem;
  }

  .steam-adv-info {
    color: var(--text-white-90);
    font-size: 0.95rem;
    line-height: 1.45;
    margin-bottom: 0.75rem;
  }

  .steam-adv-ack {
    margin: 0.5rem 0 1rem;
    align-items: flex-start;
  }

  .steam-adv-ack-text {
    padding: 0 0.5em;
    line-height: 1.35;
  }

  .steam-adv-layout {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    align-items: stretch;
  }

  @media (min-width: 720px) {
    .steam-adv-layout {
      flex-direction: row;
      align-items: flex-start;
      gap: 1.25rem;
    }

    .steam-adv-actions {
      flex: 1 1 52%;
      min-width: 0;
    }

    .steam-adv-log-wrap {
      flex: 1 1 42%;
      position: sticky;
      top: 0;
      align-self: flex-start;
      max-height: min(70vh, 32rem);
    }
  }

  .steam-adv-log-wrap {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    width: 100%;
    min-height: 12rem;
  }

  .steam-adv-log {
    flex: 1;
    min-height: 12rem;
    max-height: min(50vh, 28rem);
    overflow-y: auto;
    padding: 0.6rem 0.75rem;
    background: var(--backdrop-dark-35);
    border: 1px solid var(--accent);
    border-radius: 4px;
    font-family: ui-monospace, monospace;
    font-size: 0.8rem;
    line-height: 1.35;
    color: var(--text-white-92);
  }

  .steam-adv-log-empty {
    margin: 0;
    opacity: 0.65;
    font-style: italic;
  }

  .steam-adv-log-line {
    white-space: pre-wrap;
    word-break: break-word;
  }

  .steam-adv-log-actions {
    display: flex;
    justify-content: flex-end;
  }

  .steam-adv-clear-log {
    position: relative;
    width: auto;
  }

  .fancyLinkBtn {
    position: relative;
    width: auto !important;
    background: transparent;
    border: none;
    color: var(--accent);
    text-decoration: underline;
    cursor: pointer;
    font: inherit;
    padding: 0.35rem 0.5rem;
  }

  .fancyLinkBtn:hover {
    filter: brightness(1.15);
  }

  .steam-adv-footer {
    margin-top: 1.5rem;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: center;
  }
</style>
