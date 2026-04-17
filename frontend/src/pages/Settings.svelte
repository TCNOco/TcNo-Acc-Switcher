<script lang="ts">
    import { route, previousPage, appBarTitle } from '../stores/nav'
    import { Browser } from "@wailsio/runtime"
  
    $: appBarTitle.set('TcNo Account Switcher - Settings')
    previousPage.set({ page: 'home' })
    route.set({ page: 'settings' })
    
    import { t, availableLocales, locale, setUserLanguage } from '../stores/i18n'
    let selected = $locale
</script>

<div class="main-content">
    <h1>{$t("Settings_Header_AppWide")}</h1>
    <div class="rowDropdown">
        <span>{$t("Header_ChooseLanguage")}</span>
        <div class="dropdown">
            <select bind:value={selected} on:change={() => void setUserLanguage(selected)} class="custom-dropdown-menu dropdown-menu">
              {#each availableLocales as code}
                <option value={code} class="custom-dropdown-item dropdown-item">{code}</option>
              {/each}
            </select>
            &nbsp;<a class="fancyLink" href="https://crowdin.com/project/tcno-account-switcher" on:click|preventDefault={async () => { await Browser.OpenURL("https://crowdin.com/project/tcno-account-switcher") }} aria-label={$t("Settings_HelpTranslate")}>{$t("Settings_HelpTranslate")}</a>
            <!--&nbsp;<a class="fancyLink" on:click={showModal("crowdin")}>@Locale["Settings_ViewTranslators"]</a> -->
        </div>
    </div>
</div>