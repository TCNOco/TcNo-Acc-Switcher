<script lang="ts">
  import { Browser } from "@wailsio/runtime";
  import { onMount } from "svelte";
  import { get } from "svelte/store";
  import * as PlatformService from "../../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { t } from "../../stores/i18n";
  import { offlineMode } from "../../stores/offlineMode";
  import { pushToast } from "../../stores/toast";
  import { checkForUpdatesManually } from "../../lib/checkForUpdates";

  let currentVersion = "0.0.0";

  onMount(() => {
    void PlatformService.GetAppVersion()
      .then((v) => {
        currentVersion = v || "0.0.0";
      })
      .catch(() => {
        currentVersion = "0.0.0";
      });
  });

  function openExternal(url: string, e: MouseEvent): void {
    e.preventDefault();
    if (get(offlineMode)) {
      pushToast({
        type: "info",
        message: get(t)("Toast_OfflineModeNoLinks"),
        duration: 5000,
      });
      return;
    }
    void Browser.OpenURL(url);
  }

  async function onVersionClick(e: MouseEvent): Promise<void> {
    e.preventDefault();
    await checkForUpdatesManually();
  }
</script>

<div class="about-modal">
  <div class="imgDiv">
    <a
      href="https://tcno.co"
      on:click={(e) => openExternal("https://tcno.co", e)}
    >
      <img width="100" src="img/TcNo500.webp" draggable="false" alt="" />
    </a>
  </div>
  <div class="rightContent">
    <h2>TcNo Account Switcher</h2>
    <p>{$t("Modal_Info_Creator")}</p>
    <div class="linksList">
      <a href="https://patreon.com/TroubleChute" on:click={(e) => openExternal("https://patreon.com/TroubleChute", e)}>
        <svg viewBox="0 0 24 24" aria-hidden="true" class="modalIcoPatreon">
          <use href="img/icons/ico_patreon.svg#icoPatreon"></use>
        </svg>
        {$t("Modal_Info_ViewPatreon")}
      </a>
      <a href="https://ko-fi.com/tcnoco" on:click={(e) => openExternal("https://ko-fi.com/tcnoco", e)}>
        <svg viewBox="0 0 24 24" aria-hidden="true" class="modalIcoKofi">
          <use href="img/icons/ico_kofi.svg#icoKofi"></use>
        </svg>
        {$t("Modal_Info_ViewKofi")}
      </a>
      <a href="https://github.com/TCNOCo/TcNo-Acc-Switcher" on:click={(e) => openExternal("https://github.com/TCNOCo/TcNo-Acc-Switcher", e)}>
        <svg viewBox="0 0 24 24" aria-hidden="true" class="modalIcoGitHub">
          <use href="img/icons/ico_github.svg#icoGitHub"></use>
        </svg>
        {$t("Modal_Info_ViewGitHub")}
      </a>
      <a href="https://s.tcno.co/AccSwitcherDiscord" on:click={(e) => openExternal("https://s.tcno.co/AccSwitcherDiscord", e)}>
        <svg viewBox="0 0 24 24" aria-hidden="true" class="modalIcoDiscord">
          <use href="img/icons/ico_discord.svg#icoDiscord"></use>
        </svg>
        {$t("Modal_Info_BugReport")}
      </a>
      <a href="https://tcno.co" on:click={(e) => openExternal("https://tcno.co", e)}>
        <svg viewBox="0 0 24 24" aria-hidden="true" class="modalIcoNetworking">
          <use href="img/icons/ico_networking.svg#icoNetworking"></use>
        </svg>
        {$t("Modal_Info_VisitSite")}
      </a>
      <a
        href="https://github.com/TCNOCo/TcNo-Acc-Switcher/blob/master/DISCLAIMER.md"
        on:click={(e) =>
          openExternal("https://github.com/TCNOCo/TcNo-Acc-Switcher/blob/master/DISCLAIMER.md", e)}
      >
        <svg viewBox="0 0 2084 2084" aria-hidden="true" class="modalIcoDoc">
          <use href="img/icons/ico_doc.svg#icoDoc"></use>
        </svg>
        {$t("Modal_Info_Disclaimer")}
      </a>
    </div>
  </div>
</div>
<div class="versionIdentifier">
  <span>{$t("Modal_Info_Version")}: <a href="#" class="version-link" on:click={(e) => void onVersionClick(e)}>{currentVersion}</a></span>
</div>

<style lang="scss">
  .about-modal {
    display: flex;
    flex-direction: row;
    flex-wrap: wrap;
    gap: 1rem;
    align-items: flex-start;
  }

  .imgDiv {
    flex: 0 0 auto;
  }

  .imgDiv img {
    display: block;
    border-radius: 4px;
  }

  .rightContent {
    flex: 1 1 200px;
    min-width: 0;
  }

  .rightContent h2 {
    margin: 0 0 0.5rem;
    font-size: 1.25rem;
  }

  .rightContent p {
    margin: 0 0 0.75rem;
  }

  .linksList {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .linksList svg {
    flex-shrink: 0;
    margin-right: 0.25rem;
    width: 1.5rem;
    height: 1.5rem;

    use {
      fill: var(--whiteSecondary);
    }
  }

  .versionIdentifier {
    margin-top: 0.75rem;
    padding-top: 0.5rem;
    border-top: 1px solid var(--border-bar-bg);
    font-size: 0.85rem;
    opacity: 0.85;
  }

  .version-link {
    color: var(--accent, #f90);
    text-decoration: underline;
    cursor: pointer;

    &:hover {
      opacity: 0.85;
    }
  }
</style>
