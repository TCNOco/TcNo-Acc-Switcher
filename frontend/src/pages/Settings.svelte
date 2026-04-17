<script lang="ts">
    import { route, previousPage, appBarTitle } from '../stores/nav'
    import { Browser } from "@wailsio/runtime"
    import "../styles/Settings.scss"

    $: appBarTitle.set('TcNo Account Switcher - Settings')
    previousPage.set({ page: 'home' })
    route.set({ page: 'settings' })
    
    import { t, availableLocales, locale, setUserLanguage } from '../stores/i18n'
    let open = false;
    function nameFor(code: string) {
        const dn = new Intl.DisplayNames([$locale.replace(/_/g, "-")], { type: "language" });
        return dn.of(code.replace(/_/g, "-")) ?? code;
    }
    $: currentLabel = nameFor($locale);
    async function pick(code: string) {
        await setUserLanguage(code);
        open = false;
    }
</script>

<div class="main-content">
    <h1 class="SettingsHeader">{$t("Settings_Header_AppWide")}</h1>
    <h2 class="SettingsHeader">{$t("Settings_Header_Language")}</h2>
    <div class="rowDropdown">
        <span>{$t("Header_ChooseLanguage")}</span>
        <div class="dropdown" class:show={open}>
          <button type="button" class="dropdown-toggle" on:click={() => (open = !open)}>
            {currentLabel}
            <span class="caret" aria-hidden="true"></span>
          </button>
          {#if open}
          <ul class="custom-dropdown-menu dropdown-menu" style="position:absolute; top:100%; left:0; z-index:1000; margin:0;">
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
</div>