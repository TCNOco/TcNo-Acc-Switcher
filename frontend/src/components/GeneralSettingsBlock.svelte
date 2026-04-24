<script lang="ts">
  import "../styles/Settings.scss";
  import { onMount } from "svelte";
  import { route } from "../stores/nav";
  import { t, availableLocales, locale, setUserLanguage } from "../stores/i18n";
  import { pushToast } from "../stores/toast";
  import { formatToastWithError } from "../lib/formatWailsError";
  import { tooltip } from "../lib/actions/tooltip";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { listThemes, setUserTheme, currentThemeId, type ThemeOption } from "../lib/themes";

  let open = false;
  let themeOpen = false;
  let themes: ThemeOption[] = [];

  let isWindows = false;
  let protocolEnabled = false;
  let protocolLoading = false;

  onMount(() => {
    themes = listThemes();
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
  $: currentThemeLabel =
    themes.find((th) => th.id === $currentThemeId)?.label ?? themes[0]?.label ?? "";

  async function pickTheme(id: string): Promise<void> {
    await setUserTheme(id);
    themeOpen = false;
  }

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

<h2 class="SettingsHeader">{$t("Settings_Header_Theme")}</h2>
<div class="themeRow">
  <div class="rowDropdown themeRow__dropdown">
    <span>{$t("Settings_CurrentStyle")}</span>
    <div class="dropdown" class:show={themeOpen}>
      <button type="button" class="dropdown-toggle" on:click={() => (themeOpen = !themeOpen)}>
        {currentThemeLabel}
        <span class="caret" aria-hidden="true"></span>
      </button>
      {#if themeOpen}
        <ul
          class="custom-dropdown-menu dropdown-menu"
          style="position:absolute; top:100%; left:0; z-index:1000; margin:0;"
        >
          {#each themes as th}
            <li role="none">
              <button type="button" class="dropdown-item" on:click={() => void pickTheme(th.id)}>
                {th.label}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>
  <button type="button" class="btnicontext themeRow__preview" on:click={() => route.set({ page: "preview-css" })}>
    {$t("PreviewCss")}
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
  .themeRow {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 0.75rem 1rem;
    margin-bottom: 1.25rem;
  }

  .themeRow__dropdown {
    flex: 1 1 12rem;
    min-width: 0;
  }

  .themeRow__preview {
    flex: 0 0 auto;
    align-self: flex-end;
  }
</style>
