<script lang="ts">
  import { get } from "svelte/store";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { viewportDropdown } from "../lib/actions/viewportDropdown";
  import type { CrowdinTranslatorsList } from "../lib/crowdinTranslators";
  import { openExternalUrl } from "../lib/openExternalUrl";
  import { t, availableLocales, locale, setUserLanguage } from "../stores/i18n";
  import { offlineMode } from "../stores/offlineMode";
  import { openAlertNoButton } from "../stores/modal";
  import CrowdinTranslatorsModalBody from "./modals/CrowdinTranslatorsModalBody.svelte";

  const CROWDIN_URL = "https://crowdin.com/project/tcno-account-switcher";

  let open = false;

  function nameFor(code: string): string {
    const dn = new Intl.DisplayNames([$locale.replace(/_/g, "-")], { type: "language" });
    return dn.of(code.replace(/_/g, "-")) ?? code;
  }

  $: currentLabel = nameFor($locale);

  async function pick(code: string): Promise<void> {
    await setUserLanguage(code);
    open = false;
  }

  function openHelpTranslate(e: MouseEvent): void {
    e.preventDefault();
    void openExternalUrl(CROWDIN_URL);
  }

  async function openCreditsModal(): Promise<void> {
    let list: CrowdinTranslatorsList = { proofReaders: [], translators: [] };
    let loadError: string | undefined;

    if (get(offlineMode)) {
      loadError = "OFFLINE MODE";
    } else {
      try {
        list = await PlatformService.GetCrowdinTranslators();
      } catch {
        loadError = "Failed to load Crowdin supporters!";
      }
    }

    void openAlertNoButton({
      title: get(t)("Modal_Crowdin_Header"),
      bodyComponent: CrowdinTranslatorsModalBody,
      bodyProps: { list, loadError },
    });
  }
</script>

<h2 class="SettingsHeader">{$t("Settings_Header_Language")}</h2>
<div class="rowDropdown">
  <span>{$t("Header_ChooseLanguage")}</span>
  <div class="dropdown" class:show={open}>
    <button type="button" class="dropdown-toggle" on:click={() => (open = !open)}>
      {currentLabel}
      <span class="caret" aria-hidden="true"></span>
    </button>
    {#if open}
      <ul class="custom-dropdown-menu dropdown-menu" use:viewportDropdown>
        {#each availableLocales as code}
          <li role="none">
            <button type="button" class="dropdown-item" on:click={() => void pick(code)}>
              {code} - {nameFor(code)}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
  <a class="fancyLink" href={CROWDIN_URL} on:click={openHelpTranslate}>{$t("Settings_HelpTranslate")}</a>
  <button type="button" class="fancyLink" on:click={() => void openCreditsModal()}
    >{$t("Settings_ViewTranslators")}</button
  >
</div>

<style lang="scss">
  .dropdown-toggle {
    position: relative;
    height: 38px;
  }
</style>
