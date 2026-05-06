<script lang="ts" context="module">
  export type SearchResultRow = {
    key: string;
    title: string;
    badge: string;
    accountIconUrl?: string;
    platformIconName?: string;
    /** Category-style row (e.g. disabled platform, game). */
    isCategory?: boolean;
    categoryChecked?: boolean;
  };
</script>

<script lang="ts">
  import { createEventDispatcher, onMount, tick } from "svelte";
  import { get } from "svelte/store";
  import { platformIconFgHref } from "../lib/platformIcon";
  import { searchOverlayCtrl, searchOverlayPendingAppend } from "../stores/searchOverlay";
  import { t } from "../stores/i18n";
  import "../styles/SearchOverlay.scss";

  const dispatch = createEventDispatcher<{ pick: SearchResultRow; close: void }>();

  export let open = false;
  /** Bump when parent re-opens overlay to apply `initialQuery`. */
  export let syncNonce = 0;
  export let initialQuery = "";
  export let query = "";
  export let primaryRows: SearchResultRow[] = [];
  export let categoryRows: SearchResultRow[] = [];
  export let categoryHint = "";
  export let gameRows: SearchResultRow[] = [];
  export let gameHint = "";

  let inputEl: HTMLInputElement | null = null;
  let selectedIndex = 0;
  let appliedNonce = -1;

  $: flatPrimary = primaryRows;
  $: flatCategory = categoryRows;
  $: flatGame = gameRows;
  $: combined = [...flatPrimary, ...flatCategory, ...flatGame];

  /** `openSearchOverlay` always bumps `nonce`; drive reset off that to avoid ordering bugs with `prevOpen`. */
  $: if (open && syncNonce !== appliedNonce) {
    appliedNonce = syncNonce;
    query = initialQuery;
    selectedIndex = 0;
    void tick().then(() => {
      setTimeout(() => inputEl?.focus(), 50);
    });
  }

  onMount(() => {
    return searchOverlayPendingAppend.subscribe((ch) => {
      if (!ch || !get(searchOverlayCtrl).open) {
        return;
      }
      query += ch;
      searchOverlayPendingAppend.set(null);
    });
  });

  $: if (selectedIndex >= combined.length) {
    selectedIndex = Math.max(0, combined.length - 1);
  }

  function onOverlayClick(): void {
    dispatch("close");
  }

  function pickRow(row: SearchResultRow): void {
    dispatch("pick", row);
  }

  function onInputKeydown(e: KeyboardEvent): void {
    if (e.key === "Escape") {
      dispatch("close");
      e.preventDefault();
    } else if (e.key === "ArrowDown") {
      selectedIndex = Math.min(selectedIndex + 1, Math.max(0, combined.length - 1));
      e.preventDefault();
    } else if (e.key === "ArrowUp") {
      selectedIndex = Math.max(selectedIndex - 1, 0);
      e.preventDefault();
    } else if (e.key === "Enter" && combined.length > 0) {
      const row = combined[selectedIndex];
      if (row) {
        pickRow(row);
      }
      e.preventDefault();
    } else if (e.key === "Backspace" && query.length === 0) {
      dispatch("close");
      e.preventDefault();
    }
  }

  function combinedIndexFor(row: SearchResultRow, i: number, section: "p" | "c" | "g"): number {
    if (section === "p") {
      return i;
    }
    if (section === "c") {
      return flatPrimary.length + i;
    }
    return flatPrimary.length + flatCategory.length + i;
  }
</script>

{#if open}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="searchOverlay_overlay" on:click={onOverlayClick}>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="searchOverlay_content" on:click|stopPropagation>
      <input
        bind:this={inputEl}
        bind:value={query}
        type="text"
        class="searchOverlay_searchInput"
        placeholder={$t("Context_Search")}
        spellcheck="false"
        autocomplete="off"
        on:keydown={onInputKeydown}
      />
      {#if combined.length > 0}
        <div class="searchOverlay_results">
          {#each flatPrimary as row, i (row.key)}
            <button
              type="button"
              class="searchOverlay_resultItem"
              class:searchOverlay_selected={combinedIndexFor(row, i, "p") === selectedIndex}
              on:click={() => pickRow(row)}
              on:mouseenter={() => (selectedIndex = combinedIndexFor(row, i, "p"))}
            >
              <span class="searchOverlay_resultTitleWithIcon">
                {#if row.accountIconUrl}
                  <img src={row.accountIconUrl} alt="" class="searchOverlay_categoryIcon" />
                {:else if row.platformIconName}
                  <svg class="searchOverlay_platformIcon" viewBox="0 0 500 500" aria-hidden="true">
                    <use href={platformIconFgHref(row.platformIconName)} />
                  </svg>
                {/if}
                <span class="searchOverlay_resultTitle">{row.title}</span>
              </span>
              <span class="searchOverlay_resultCategory">{row.badge}</span>
            </button>
          {/each}
          {#if flatCategory.length > 0}
            <div class="searchOverlay_categorySection">
              {#if categoryHint}
                <span class="searchOverlay_categoryHint">{categoryHint}</span>
              {/if}
              {#each flatCategory as row, i (row.key)}
                <button
                  type="button"
                  class="searchOverlay_resultItem searchOverlay_categoryResultItem"
                  class:searchOverlay_categoryChecked={row.categoryChecked}
                  class:searchOverlay_selected={combinedIndexFor(row, i, "c") === selectedIndex}
                  on:click={() => pickRow(row)}
                  on:mouseenter={() => (selectedIndex = combinedIndexFor(row, i, "c"))}
                >
                  <span class="searchOverlay_resultTitleWithIcon">
                    {#if row.accountIconUrl}
                      <img src={row.accountIconUrl} alt="" class="searchOverlay_categoryIcon" />
                    {:else if row.platformIconName}
                      <svg class="searchOverlay_platformIcon" viewBox="0 0 500 500" aria-hidden="true">
                        <use href={platformIconFgHref(row.platformIconName)} />
                      </svg>
                    {/if}
                    <span class="searchOverlay_resultTitle">{row.title}</span>
                  </span>
                  <span class="searchOverlay_resultCategory">{row.badge}</span>
                </button>
              {/each}
            </div>
          {/if}
          {#if flatGame.length > 0}
            <div class="searchOverlay_categorySection">
              {#if gameHint}
                <span class="searchOverlay_categoryHint">{gameHint}</span>
              {/if}
              {#each flatGame as row, i (row.key)}
                <button
                  type="button"
                  class="searchOverlay_resultItem searchOverlay_categoryResultItem"
                  class:searchOverlay_categoryChecked={row.categoryChecked}
                  class:searchOverlay_selected={combinedIndexFor(row, i, "g") === selectedIndex}
                  on:click={() => pickRow(row)}
                  on:mouseenter={() => (selectedIndex = combinedIndexFor(row, i, "g"))}
                >
                  <span class="searchOverlay_resultTitleWithIcon">
                    {#if row.accountIconUrl}
                      <img src={row.accountIconUrl} alt="" class="searchOverlay_categoryIcon" />
                    {/if}
                    <span class="searchOverlay_resultTitle">{row.title}</span>
                  </span>
                  <span class="searchOverlay_resultCategory">{row.badge}</span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </div>
{/if}
