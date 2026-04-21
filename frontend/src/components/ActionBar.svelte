<script lang="ts">
  import { get } from "svelte/store";
  import { route, type Route } from "../stores/nav";
  import { actionBarStatus } from "../stores/actionBarStatus";
  import { platformExeIconUrl, triggerPlatformAction } from "../stores/platformPage";
  import { tooltip } from "../lib/actions/tooltip";
  import { t } from "../stores/i18n";
  import { openAlertNoButton } from "../stores/modal";
  import HelpAboutModalBody from "./modals/HelpAboutModalBody.svelte";

  $: r = $route;
  $: isPlatformPage = r.page === "platform";
  $: platformName = isPlatformPage ? (r as Extract<Route, { page: "platform" }>).platformName : "";
  $: showSaveCurrent = !!platformName && platformName !== "Steam";

  let iconBroken = false;
  $: if (platformName) {
    iconBroken = false;
  }

  function showHelpModal() {
    void openAlertNoButton({
      title: $t("Modal_Title_Info"),
      bodyComponent: HelpAboutModalBody,
    });
  }
</script>

<footer class="actionbar">
  <span class="actionbar__status" title={$actionBarStatus}>{$actionBarStatus}</span>
  <div class="actionbar__actions">
    {#if isPlatformPage}
        <button
          type="button"
          class="actionbar__launch square"
          aria-label={$t("Button_Launch")}
          use:tooltip={$t("Tooltip_Launch")}
          on:click={() => triggerPlatformAction("launch")}
        >
          {#if $platformExeIconUrl && !iconBroken}
            <img class="actionbar__exeicon" src={$platformExeIconUrl} alt="" on:error={() => (iconBroken = true)} />
          {:else}
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true"
              ><path
                d="M288 32c0-17.7-14.3-32-32-32s-32 14.3-32 32V274.7l-73.4-73.4c-12.5-12.5-32.8-12.5-45.3 0s-12.5 32.8 0 45.3l128 128c12.5 12.5 32.8 12.5 45.3 0l128-128c12.5-12.5 12.5-32.8 0-45.3s-32.8-12.5-45.3 0L288 274.7V32z"
              /></svg
            >
          {/if}
        </button>
        <button class="btnicontext" type="button" aria-label={$t("Button_AddNew")} use:tooltip={$t("Tooltip_AddNew")} on:click={() => triggerPlatformAction("addNew")}
          ><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512" aria-hidden="true"
            ><path
              d="M416 208H272V64c0-17.67-14.33-32-32-32h-32c-17.67 0-32 14.33-32 32v144H32c-17.67 0-32 14.33-32 32v32c0 17.67 14.33 32 32 32h144v144c0 17.67 14.33 32 32 32h32c17.67 0 32-14.33 32-32V304h144c17.67 0 32-14.33 32-32v-32c0-17.67-14.33-32-32-32z"
            /></svg
          >{$t("Button_AddNew")}</button
        >
        {#if showSaveCurrent}
          <button
            class="btnicontext"
            type="button"
            aria-label={$t("Button_SaveCurrent")}
            use:tooltip={$t("Tooltip_SaveCurrent")}
            on:click={() => triggerPlatformAction("saveCurrent")}
            ><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512" aria-hidden="true"
              ><path
                d="M64 32C28.7 32 0 60.7 0 96V416c0 35.3 28.7 64 64 64H384c35.3 0 64-28.7 64-64V173.3c0-17-6.7-33.3-18.7-45.3L352 69.3c-12-12-28.3-18.7-45.3-18.7H64zm32 64H288v64H96V96zm160 384H96V288H320V480zm64 0V288h32V480H320z"
              /></svg
            >{$t("Button_SaveCurrent")}</button
          >
        {/if}
        <button
          class="btnicontext actionbar__login"
          type="button"
          aria-label={$t("Button_Login")}
          use:tooltip={$t("Tooltip_Login")}
          on:click={() => triggerPlatformAction("login")}
          >{$t("Button_Login")}<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true">
            <path d="M217.9 105.9L340.7 228.7c7.2 7.2 11.3 17 11.3 27.3s-4.1 20.1-11.3 27.3L217.9 406.1c-6.4 6.4-15 9.9-24 9.9c-18.7 0-33.9-15.2-33.9-33.9V160c0-18.7 15.2-33.9 33.9-33.9c9 0 17.6 3.5 24 9.9z"/></svg>
          </button
        >
    {:else}
        <button class="btnicontext" aria-label={$t("Button_ManagePlatforms")} use:tooltip={$t("Tooltip_ManagePlatforms")} on:click={() => route.set({ page: 'manage-platforms'})}><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512"><!--!Font Awesome Free v5.15.4 by @fontawesome - https://fontawesome.com License - https://fontawesome.com/license/free Copyright 2026 Fonticons, Inc.--><path d="M416 208H272V64c0-17.67-14.33-32-32-32h-32c-17.67 0-32 14.33-32 32v144H32c-17.67 0-32 14.33-32 32v32c0 17.67 14.33 32 32 32h144v144c0 17.67 14.33 32 32 32h32c17.67 0 32-14.33 32-32V304h144c17.67 0 32-14.33 32-32v-32c0-17.67-14.33-32-32-32z"/></svg>{$t("Button_ManagePlatforms")}</button>
    {/if}
    <button class="square" aria-label="Settings" use:tooltip={$t("Tooltip_Settings")} on:click={() => {
    const r = get(route);
    if (r.page === "platform") {
      route.set({ page: "platform-settings", platformName: r.platformName });
    } else {
      route.set({ page: "settings" });
    }
  }}><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true"><!--!Font Awesome Free v5.15.4 by @fontawesome - https://fontawesome.com License - https://fontawesome.com/license/free Copyright 2026 Fonticons, Inc.--><path d="M487.4 315.7l-42.6-24.6c4.3-23.2 4.3-47 0-70.2l42.6-24.6c4.9-2.8 7.1-8.6 5.5-14-11.1-35.6-30-67.8-54.7-94.6-3.8-4.1-10-5.1-14.8-2.3L380.8 110c-17.9-15.4-38.5-27.3-60.8-35.1V25.8c0-5.6-3.9-10.5-9.4-11.7-36.7-8.2-74.3-7.8-109.2 0-5.5 1.2-9.4 6.1-9.4 11.7V75c-22.2 7.9-42.8 19.8-60.8 35.1L88.7 85.5c-4.9-2.8-11-1.9-14.8 2.3-24.7 26.7-43.6 58.9-54.7 94.6-1.7 5.4.6 11.2 5.5 14L67.3 221c-4.3 23.2-4.3 47 0 70.2l-42.6 24.6c-4.9 2.8-7.1 8.6-5.5 14 11.1 35.6 30 67.8 54.7 94.6 3.8 4.1 10 5.1 14.8 2.3l42.6-24.6c17.9 15.4 38.5 27.3 60.8 35.1v49.2c0 5.6 3.9 10.5 9.4 11.7 36.7 8.2 74.3 7.8 109.2 0 5.5-1.2 9.4-6.1 9.4-11.7v-49.2c22.2-7.9 42.8-19.8 60.8-35.1l42.6 24.6c4.9 2.8 11 1.9 14.8-2.3 24.7-26.7 43.6-58.9 54.7-94.6 1.5-5.5-.7-11.3-5.6-14.1zM256 336c-44.1 0-80-35.9-80-80s35.9-80 80-80 80 35.9 80 80-35.9 80-80 80z"/></svg></button>
  <button type="button" class="square" aria-label="Help" use:tooltip={$t("Tooltip_Info")} on:click={showHelpModal}><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 384 512" aria-hidden="true"><!--!Font Awesome Free v5.15.4 by @fontawesome - https://fontawesome.com License - https://fontawesome.com/license/free Copyright 2026 Fonticons, Inc.--><path d="M202.021 0C122.202 0 70.503 32.703 29.914 91.026c-7.363 10.58-5.093 25.086 5.178 32.874l43.138 32.709c10.373 7.865 25.132 6.026 33.253-4.148 25.049-31.381 43.63-49.449 82.757-49.449 30.764 0 68.816 19.799 68.816 49.631 0 22.552-18.617 34.134-48.993 51.164-35.423 19.86-82.299 44.576-82.299 106.405V320c0 13.255 10.745 24 24 24h72.471c13.255 0 24-10.745 24-24v-5.773c0-42.86 125.268-44.645 125.268-160.627C377.504 66.256 286.902 0 202.021 0zM192 373.459c-38.196 0-69.271 31.075-69.271 69.271 0 38.195 31.075 69.27 69.271 69.27s69.271-31.075 69.271-69.271-31.075-69.27-69.271-69.27z"/></svg></button>
  </div>
</footer>

<style lang="scss">
  .actionbar {
    display: flex;
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    width: 100%;
    height: 3.5em;
    min-height: 3.5em;
    padding: 0.25em 0.25em;
    overflow: hidden;
    background: var(--footer-bg);
    color: #fff;

    button:not(.btnicontext) {
      padding: 4.5px;
    }
  }

  .actionbar__login {
    padding: .25em 3em .25em 3.9em;
  }

  .actionbar__launch {
    min-width: 2.25rem;
    min-height: 2.25rem;
    padding: 4px;
  }

  .actionbar__exeicon {
    width: 1.75rem;
    height: 1.75rem;
    object-fit: contain;
    display: block;
    border-radius: 4px;
  }

  .actionbar__status {
    flex: 1 1 auto;
    min-width: 0;
    text-align: left;
    font-size: 0.88rem;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding-left: 0.35rem;
    color: rgba(255, 255, 255, 0.92);
  }

  .actionbar__actions {
    display: flex;
    flex-direction: row;
    align-items: center;
    flex-shrink: 0;
    gap: 0;
  }
</style>
