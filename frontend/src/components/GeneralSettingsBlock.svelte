<script lang="ts">
  import "../styles/Settings.scss";
  import { onMount } from "svelte";
  import { route } from "../stores/nav";
  import { t, availableLocales, locale, setUserLanguage } from "../stores/i18n";
  import { pushToast } from "../stores/toast";
  import { formatToastWithError } from "../lib/formatWailsError";
  import { tooltip } from "../lib/actions/tooltip";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

  let open = false;

  let isWindows = false;
  let protocolEnabled = false;
  let protocolLoading = false;

  onMount(() => {
    isWindows = /windows/i.test(navigator.userAgent) || /win32/i.test(navigator.userAgent);
    if (isWindows) {
      void PlatformService.GetProtocolEnabled()
        .then((v) => {
          protocolEnabled = v;
        })
        .catch(() => {
          protocolEnabled = false;
        });
    }
  });

  function nameFor(code: string): string {
    const dn = new Intl.DisplayNames([$locale.replace(/_/g, "-")], { type: "language" });
    return dn.of(code.replace(/_/g, "-")) ?? code;
  }

  $: currentLabel = nameFor($locale);

  async function pick(code: string): Promise<void> {
    await setUserLanguage(code);
    open = false;
  }

  async function toggleProtocol(): Promise<void> {
    if (!isWindows || protocolLoading) {
      return;
    }
    const next = !protocolEnabled;
    protocolLoading = true;
    try {
      await PlatformService.SetProtocolEnabled(next);
      protocolEnabled = next;
      pushToast({
        type: "success",
        message: next ? $t("Toast_ProtocolEnabled") : $t("Toast_ProtocolDisabled"),
        duration: 6000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      protocolLoading = false;
    }
  }
</script>

<h2 class="SettingsHeader">{$t("Preview_Modals")}</h2>
<div class="settingsTestLinkRow">
  <button type="button" class="btnicontext" on:click={() => route.set({ page: "test" })}>
    {$t("Preview_Modals")}…
  </button>
</div>

{#if isWindows}
  <div class="rowSetting">
    <div class="form-check">
      <input
        id="gs-protocol"
        type="checkbox"
        checked={protocolEnabled}
        disabled={protocolLoading}
        on:change={() => void toggleProtocol()}
      />
      <label class="form-check-label" for="gs-protocol"></label>
    </div>
    <label for="gs-protocol" use:tooltip={$t("Settings_EnableProtocol")}
      >{$t("Settings_EnableProtocol")}</label
    >
  </div>
{/if}

<h2 class="SettingsHeader">{$t("Settings_Header_Language")}</h2>
<div class="rowDropdown">
  <span>{$t("Header_ChooseLanguage")}</span>
  <div class="dropdown" class:show={open}>
    <button type="button" class="dropdown-toggle" on:click={() => (open = !open)}>
      {currentLabel}
      <span class="caret" aria-hidden="true"></span>
    </button>
    {#if open}
      <ul
        class="custom-dropdown-menu dropdown-menu"
        style="position:absolute; top:100%; left:0; z-index:1000; margin:0;"
      >
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
</div>

<style lang="scss">
  .settingsTestLinkRow {
    margin-bottom: 1.25rem;
  }
</style>
