<script lang="ts">
  import "../styles/Settings.scss";
  import { route } from "../stores/nav";
  import { t, availableLocales, locale, setUserLanguage } from "../stores/i18n";

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
</script>

<h2 class="SettingsHeader">{$t("Preview_Modals")}</h2>
<div class="settingsTestLinkRow">
  <button type="button" class="btnicontext" on:click={() => route.set({ page: "test" })}>
    {$t("Preview_Modals")}…
  </button>
</div>

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
