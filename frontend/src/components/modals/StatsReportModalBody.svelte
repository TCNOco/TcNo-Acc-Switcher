<script lang="ts">
  import { Browser } from "@wailsio/runtime";
  import { get } from "svelte/store";
  import * as PlatformService from "../../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice";
  import type { StatsReport } from "../../../bindings/TcNo-Acc-Switcher/internal/platform/models";
  import { t } from "../../stores/i18n";
  import { offlineMode } from "../../stores/offlineMode";
  import { pushToast } from "../../stores/toast";
  import { openConfirm } from "../../stores/modal";

  const STATS_URL = "https://tcno.co/Projects/AccSwitcher/stats/";

  export let initialReport: StatsReport;

  let report: StatsReport = initialReport;
  let resetBusy = false;

  $: report = initialReport;

  function formatDur(sec: number): string {
    const s = Math.max(0, Math.floor(sec));
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    const r = s % 60;
    return `${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}:${String(r).padStart(2, "0")}`;
  }

  function openStatsUrl(e: MouseEvent): void {
    e.preventDefault();
    if (get(offlineMode)) {
      pushToast({
        type: "info",
        message: get(t)("Toast_OfflineModeNoLinks"),
        duration: 5000,
      });
      return;
    }
    void Browser.OpenURL(STATS_URL);
  }

  function monoSummary(r: StatsReport, tr: (k: string, v?: Record<string, string | number>) => string): string {
    const lines = [
      `${tr("Stats_OperatingSystem")}${r.osDisplay}`,
      `${tr("Stats_FirstLaunch")}${r.firstLaunch}`,
      `${tr("Stats_LaunchCount")}${r.launchCount}`,
      `${tr("Stats_CrashCount")}${r.crashCount}`,
      `${tr("Stats_MostUsedPlatform")}${r.mostUsedPlatform || ""}`,
      `${tr("Stats_TotalTime")}${formatDur(r.totalTimeInAppSec)}`,
      `${tr("Stats_TotalSwitches")}${r.totalSwitches}`,
      `${tr("Stats_TotalGamesLaunched")}${r.totalGamesLaunched}`,
      `${tr("Stats_UniqueDaysLong")}${r.uniqueDaysSwitched}`,
      `${tr("Stats_TotalTags")}${r.totalTags ?? 0}`,
      `${tr("Stats_TotalTaggedAccounts")}${r.totalTaggedAccounts ?? 0}`,
    ];
    return lines.join("\n");
  }

  function monoDetail(r: StatsReport, tr: (k: string, v?: Record<string, string | number>) => string): string {
    const parts: string[] = [];
    parts.push(`Uuid: ${r.uuid}`);
    parts.push(`${tr("Stats_LastUpload")}${r.lastUpload}`);
    parts.push(`${tr("Stats_SwitcherStats")}`);
    for (const sw of r.switchers) {
      parts.push(`- ${sw.platform}:`);
      parts.push(`   - ${tr("Stats_Accounts")}${sw.accounts}`);
      parts.push(`   - ${tr("Stats_Switches")}${sw.switches}`);
      parts.push(`   - ${tr("Stats_FirstActive")}${sw.firstActive}`);
      parts.push(`   - ${tr("Stats_LastActive")}${sw.lastActive}`);
      parts.push(`   - ${tr("Stats_UniqueDays")}${sw.uniqueDays}`);
      parts.push(`   - ${tr("Stats_GameShortcuts")}${sw.gameShortcuts}`);
      parts.push(`   - ${tr("Stats_GameShortcutsHotbar")}${sw.gameShortcutsHotbar}`);
      parts.push(`   - ${tr("Stats_GamesLaunched")}${sw.gamesLaunched}`);
      if (sw.tags !== undefined) parts.push(`   - ${tr("Stats_Tags")}${sw.tags}`);
      if (sw.taggedAccounts !== undefined) parts.push(`   - ${tr("Stats_TaggedAccounts")}${sw.taggedAccounts}`);
    }
    parts.push(`${tr("Stats_PageStats")}`);
    for (const p of r.pages) {
      parts.push(
        `- ${p.path}: ${tr("Stats_VisitsTotalTime", {
          visits: p.visits,
          hours: formatDur(p.totalTimeSec),
        })}`,
      );
    }
    return parts.join("\n");
  }

  $: summaryBlock = monoSummary(report, $t);
  $: detailBlock = monoDetail(report, $t);

  async function onReset(): Promise<void> {
    const ok = await openConfirm({
      title: get(t)("Settings_ClearStats"),
      style: "okcancel",
      body: get(t)("Prompt_ClearStats"),
    });
    if (!ok) {
      return;
    }
    resetBusy = true;
    try {
      await PlatformService.ResetStatistics();
      report = await PlatformService.GetStatsReport();
      pushToast({
        type: "success",
        message: get(t)("Toast_SavedItem", { item: get(t)("Settings_ClearStats") }),
        duration: 4000,
      });
    } catch {
      pushToast({
        type: "error",
        message: get(t)("Toast_SaveFailed"),
        duration: 6000,
      });
    } finally {
      resetBusy = false;
    }
  }
</script>

<div class="stats-report">
  {#if report.shareEnabled}
    <p class="stats-report__share">{$t("Stats_ModalShareEnabled")}</p>
  {:else}
    <p class="stats-report__share">{$t("Stats_ModalShareDisabled")}</p>
  {/if}
  <p class="stats-report__linkrow">
    <a class="fancyLink" href={STATS_URL} on:click={openStatsUrl}>https://hub.tcno.co/switcher/</a>
  </p>

  <pre class="stats-report__mono">{summaryBlock}</pre>

  <h3 class="stats-report__h3">{$t("Stats_InDepth")}</h3>
  <pre class="stats-report__mono stats-report__mono--detail">{detailBlock}</pre>

  <div class="stats-report__actions">
    <button type="button" class="btnicontext" disabled={resetBusy} on:click={() => void onReset()}>
      {$t("Settings_ClearStats")}
    </button>
  </div>
</div>

<style lang="scss">
  .stats-report {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    min-width: 0;
    max-width: 42rem;
  }

  .stats-report__share {
    margin: 0;
  }

  .stats-report__linkrow {
    margin: 0 0 0.25rem;
  }

  .stats-report__h3 {
    margin: 0.5rem 0 0.25rem;
    font-size: 1rem;
    font-weight: 600;
  }

  .stats-report__mono {
    margin: 0;
    padding: 0.5rem 0.6rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
    font-size: 0.8rem;
    line-height: 1.35;
    white-space: pre-wrap;
    word-break: break-word;
    background: var(--backdrop-dark-25);
    border: 1px solid var(--border-bar-bg);
    border-radius: 4px;
    max-height: 12rem;
    overflow: auto;
  }

  .stats-report__mono--detail {
    max-height: 22rem;
  }

  .stats-report__actions {
    margin-top: 0.35rem;
    display: flex;
    justify-content: flex-end;
  }
</style>
