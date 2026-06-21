<script lang="ts">
  import { t } from "../../stores/i18n";
  import type { CrowdinTranslatorsList } from "../../lib/crowdinTranslators";

  export let list: CrowdinTranslatorsList;
  export let loadError: string | undefined = undefined;
</script>

<div class="crowdin">
  <h2 class="crowdin__title">
    {$t("Modal_Crowdin_Header")}
    <svg
      class="heart"
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 512 512"
      aria-hidden="true"
    >
      <path
        d="M462.3 62.6C407.5 15.9 326 24.3 275.7 76.2L256 96.5l-19.7-20.3C186.1 24.3 104.5 15.9 49.7 62.6c-62.8 53.6-66 152.1-6.9 210.8l192 192c12.5 12.5 32.8 12.5 45.3 0l192-192c59.1-58.7 55.9-157.2-6.9-210.8z"
      />
    </svg>
  </h2>
  <p class="crowdin__info">{@html $t("Modal_Crowdin_Info")}</p>
  <ul class="crowdin__list">
    {#if loadError}
      <li><b>{loadError}</b></li>
    {:else}
      {#each list.proofReaders as pr (pr.name)}
        <li>{pr.name} ({pr.languages})</li>
      {/each}
      {#if list.proofReaders.length > 0 && list.translators.length > 0}
        <li class="crowdin__separator">----------</li>
      {/if}
      {#each list.translators as name (name)}
        <li>{name}</li>
      {/each}
    {/if}
  </ul>
</div>

<style lang="scss">
  .crowdin {
    text-align: initial;
    min-width: 0;
    max-width: 36rem;
  }

  .crowdin__title {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    margin: 0 0 0.5rem;
    font-size: 1.1rem;
    font-weight: 600;
  }

  .crowdin__info {
    margin: 0 0 0.75rem;
  }

  .crowdin__list {
    list-style-type: none;
    padding-left: 1rem;
    overflow: auto;
    max-height: 40vh;
    margin: 0;
  }

  .heart {
    color: red;
    height: 1em;
    width: 1.1em;
    fill: currentColor;
    flex-shrink: 0;
  }

  .crowdin__separator {
    user-select: none;
  }
</style>
