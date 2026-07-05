<script lang="ts">
  import { onMount } from "svelte";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import { t } from "../stores/i18n";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import "../styles/Settings.scss";

  let allNames: string[] = [];
  let disabled: string[] = [];
  let loadError: string | null = null;

  $: appBarTitle.set($t("Title_Platforms_Settings"));
  $: disabledSorted = [...disabled].sort((a, b) =>
    a.localeCompare(b, undefined, { sensitivity: "base" }),
  );
  $: enabledSorted = allNames
    .filter((n) => !disabled.includes(n))
    .sort((a, b) => a.localeCompare(b, undefined, { sensitivity: "base" }));
  $: anyEnabled = enabledSorted.length > 0;

  async function refresh(): Promise<void> {
    loadError = null;
    try {
      const s = await PlatformService.GetStartup();
      if (s.platformsFileMissing) {
        allNames = [];
        disabled = [];
        return;
      }
      allNames = s.allPlatformNames ?? [];
      disabled = [...(s.disabledPlatformNames ?? [])];
    } catch (e) {
      loadError = e instanceof Error ? e.message : String(e);
    }
  }

  async function showPlatform(name: string): Promise<void> {
    const next = disabled.filter((x) => x !== name);
    disabled = next;
    await PlatformService.SetDisabledPlatforms(next);
    await refresh();
  }

  async function hidePlatform(name: string): Promise<void> {
    const next = [...disabled, name];
    disabled = next;
    await PlatformService.SetDisabledPlatforms(next);
    await refresh();
  }

  function onDisabledRowChange(e: Event, item: string): void {
    const el = e.currentTarget as HTMLInputElement;
    if (el.checked) void showPlatform(item);
  }

  function onEnabledRowChange(e: Event, item: string): void {
    const el = e.currentTarget as HTMLInputElement;
    if (!el.checked) void hidePlatform(item);
  }

  function saveAndClose(): void {
    route.set({ page: "home" });
  }

  onMount(() => {
    previousPage.set({ page: "home" });
    route.set({ page: "manage-platforms" });
    void refresh();
  });
</script>

<div class="main-content main-spacing">
  <h1 class="SettingsHeader">{$t("Settings_Header_ExtraPlatforms")}</h1>

  {#if loadError}
    <p class="settings-err">{loadError}</p>
  {:else if !allNames.length}
    <span>{$t("Settings_NoPlatforms")}</span>
  {:else}
    {#if disabledSorted.length}
      <h2 class="SettingsHeader">{$t("Settings_ExtraPlatformsDisabled")}</h2>
      {#if !anyEnabled}
        <span>{$t("Settings_NoPlatforms")}</span>
      {/if}
      <div class="rowSetting platformsCheckboxes">
        {#each disabledSorted as item (item)}
          <div class="form-check mb-2">
            <input
              class="form-check-input"
              type="checkbox"
              id={"dis-" + item}
              on:change={(e) => onDisabledRowChange(e, item)}
            />
            <label class="form-check-label" for={"dis-" + item}></label>
            <label for={"dis-" + item}>{item}<br /></label>
          </div>
        {/each}
      </div>
    {/if}

    {#if enabledSorted.length}
      <h2 class="SettingsHeader">{$t("Settings_ExtraPlatformsEnabled")}</h2>
      <div class="rowSetting platformsCheckboxes">
        {#each enabledSorted as item (item)}
          <div class="form-check mb-2">
            <input
              class="form-check-input"
              type="checkbox"
              id={"en-" + item}
              checked
              on:change={(e) => onEnabledRowChange(e, item)}
            />
            <label class="form-check-label" for={"en-" + item}></label>
            <label for={"en-" + item}>{item}<br /></label>
          </div>
        {/each}
      </div>
    {/if}

    <div class="buttoncol col_close">
      <button type="button" class="btn_close" on:click={saveAndClose}>
        <span>{$t("Button_Close")}</span>
      </button>
    </div>
  {/if}
</div>

<style lang="scss">
  .settings-err {
    color: var(--red);
    padding: 0.5rem;
  }
</style>
