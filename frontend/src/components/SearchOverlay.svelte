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

  export type SearchOverlayKeyResult =
    | { type: "close" }
    | { type: "move"; selectedIndex: number }
    | { type: "pick"; selectedIndex: number }
    | { type: "noop" };

  export function resolveSearchOverlayKey(
    key: string,
    queryLength: number,
    selectedIndex: number,
    resultCount: number,
  ): SearchOverlayKeyResult {
    if (key === "Escape") {
      return { type: "close" };
    }
    if (key === "ArrowDown") {
      return {
        type: "move",
        selectedIndex: Math.min(selectedIndex + 1, Math.max(0, resultCount - 1)),
      };
    }
    if (key === "ArrowUp") {
      return {
        type: "move",
        selectedIndex: Math.max(selectedIndex - 1, 0),
      };
    }
    if (key === "Enter" && resultCount > 0) {
      return { type: "pick", selectedIndex };
    }
    if (key === "Backspace" && queryLength === 0) {
      return { type: "close" };
    }
    return { type: "noop" };
  }

  export function searchOverlayOptionId(index: number): string {
    return `searchOverlay_option_${index}`;
  }

  export function resolveSearchOverlayActiveDescendant(
    selectedIndex: number,
    resultCount: number,
  ): string | undefined {
    if (resultCount <= 0 || selectedIndex < 0 || selectedIndex >= resultCount) {
      return undefined;
    }
    return searchOverlayOptionId(selectedIndex);
  }
</script>

<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount, tick } from "svelte";
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
  export let placeholder = "";
  export let primaryRows: SearchResultRow[] = [];
  export let categoryRows: SearchResultRow[] = [];
  export let categoryHint = "";
  export let gameRows: SearchResultRow[] = [];
  export let gameHint = "";

  let inputEl: HTMLInputElement | null = null;
  let dialogEl: HTMLDivElement | null = null;
  let selectedIndex = 0;
  let appliedNonce = -1;
  let focusTimeout: ReturnType<typeof setTimeout> | null = null;
  let openerEl: HTMLElement | null = null;

  const searchDialogTitleId = "searchOverlay_dialogTitle";
  const searchInputLabelId = "searchOverlay_inputLabel";
  const searchResultsId = "searchOverlay_results";
  const searchStatusId = "searchOverlay_status";
  const searchCategoryHeadingId = "searchOverlay_categoryHeading";
  const searchGameHeadingId = "searchOverlay_gameHeading";

  $: flatPrimary = primaryRows;
  $: flatCategory = categoryRows;
  $: flatGame = gameRows;
  $: combined = [...flatPrimary, ...flatCategory, ...flatGame];
  $: hasResults = combined.length > 0;
  $: selectedRow = combined[selectedIndex] ?? null;
  $: activeDescendantId = resolveSearchOverlayActiveDescendant(selectedIndex, combined.length);
  $: selectedSectionLabel =
    selectedRow === null
      ? ""
      : selectedIndex < flatPrimary.length
        ? ""
        : selectedIndex < flatPrimary.length + flatCategory.length
          ? categoryHint
          : gameHint;
  $: selectedAnnouncement =
    selectedRow === null
      ? ""
      : [selectedRow.title, selectedRow.badge, selectedSectionLabel, selectedRow.categoryChecked ? "selected" : ""]
          .filter(Boolean)
          .join(", ");

  function restoreOpenerFocus(): void {
    if (!openerEl || typeof openerEl.focus !== "function") {
      openerEl = null;
      return;
    }
    try {
      openerEl.focus({ preventScroll: true });
    } catch {
      openerEl.focus();
    }
    openerEl = null;
  }

  /** `openSearchOverlay` always bumps `nonce`; drive reset off that to avoid ordering bugs with `prevOpen`. */
  $: if (open && syncNonce !== appliedNonce) {
    appliedNonce = syncNonce;
    const active = document.activeElement;
    openerEl = active instanceof HTMLElement && !dialogEl?.contains(active) ? active : openerEl;
    query = initialQuery;
    selectedIndex = 0;
    if (focusTimeout !== null) {
      clearTimeout(focusTimeout);
      focusTimeout = null;
    }
    void tick().then(() => {
      focusTimeout = setTimeout(() => {
        focusTimeout = null;
        inputEl?.focus();
      }, 50);
    });
  }

  $: if (!open && openerEl) {
    void tick().then(() => {
      if (!open) {
        restoreOpenerFocus();
      }
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

  onDestroy(() => {
    if (focusTimeout !== null) {
      clearTimeout(focusTimeout);
    }
  });

  $: if (selectedIndex >= combined.length) {
    selectedIndex = Math.max(0, combined.length - 1);
  }

  $: if (open && activeDescendantId) {
    void tick().then(() => {
      const activeOption = document.getElementById(activeDescendantId);
      activeOption?.scrollIntoView({ block: "nearest" });
    });
  }

  function onOverlayClick(): void {
    dispatch("close");
  }

  function pickRow(row: SearchResultRow): void {
    dispatch("pick", row);
  }

  function onInputKeydown(e: KeyboardEvent): void {
    const result = resolveSearchOverlayKey(e.key, query.length, selectedIndex, combined.length);
    if (result.type === "close") {
      dispatch("close");
      e.preventDefault();
      return;
    }
    if (result.type === "move") {
      selectedIndex = result.selectedIndex;
      e.preventDefault();
      return;
    }
    if (result.type === "pick") {
      const row = combined[result.selectedIndex];
      if (row) {
        pickRow(row);
      }
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
  <div class="searchOverlay_overlay" role="presentation" on:click|self={onOverlayClick}>
    <div
      bind:this={dialogEl}
      class="searchOverlay_content"
      role="dialog"
      aria-modal="true"
      aria-labelledby={searchDialogTitleId}
    >
      <h2 id={searchDialogTitleId} class="searchOverlay_srOnly">{$t("Context_Search")}</h2>
      <label id={searchInputLabelId} class="searchOverlay_srOnly" for="searchOverlay_input">
        {placeholder || $t("Context_Search")}
      </label>
      <input
        id="searchOverlay_input"
        bind:this={inputEl}
        bind:value={query}
        type="text"
        class="searchOverlay_searchInput"
        placeholder={placeholder || $t("Context_Search")}
        role="combobox"
        aria-labelledby={searchInputLabelId}
        aria-controls={searchResultsId}
        aria-expanded={hasResults}
        aria-haspopup="listbox"
        aria-autocomplete="list"
        aria-activedescendant={activeDescendantId}
        spellcheck="false"
        autocomplete="off"
        on:keydown={onInputKeydown}
      />
      <div id={searchStatusId} class="searchOverlay_srOnly" aria-live="polite">
        {selectedAnnouncement}
      </div>
      {#if hasResults}
        <div id={searchResultsId} class="searchOverlay_results" role="listbox" aria-label={$t("Context_Search")}>
          {#each flatPrimary as row, i (row.key)}
            <button
              type="button"
              id={searchOverlayOptionId(combinedIndexFor(row, i, "p"))}
              class="searchOverlay_resultItem"
              role="option"
              tabindex="-1"
              aria-label={[row.title, row.badge].filter(Boolean).join(", ")}
              aria-selected={combinedIndexFor(row, i, "p") === selectedIndex}
              class:searchOverlay_selected={combinedIndexFor(row, i, "p") === selectedIndex}
              on:mousedown|preventDefault
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
            <div class="searchOverlay_categorySection" role="group" aria-labelledby={searchCategoryHeadingId}>
              {#if categoryHint}
                <span id={searchCategoryHeadingId} class="searchOverlay_categoryHint">{categoryHint}</span>
              {/if}
              {#each flatCategory as row, i (row.key)}
                <button
                  type="button"
                  id={searchOverlayOptionId(combinedIndexFor(row, i, "c"))}
                  class="searchOverlay_resultItem searchOverlay_categoryResultItem"
                  role="option"
                  tabindex="-1"
                  aria-label={[row.title, row.badge, categoryHint, row.categoryChecked ? "selected" : ""].filter(Boolean).join(", ")}
                  aria-selected={combinedIndexFor(row, i, "c") === selectedIndex}
                  class:searchOverlay_categoryChecked={row.categoryChecked}
                  class:searchOverlay_selected={combinedIndexFor(row, i, "c") === selectedIndex}
                  on:mousedown|preventDefault
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
            <div class="searchOverlay_categorySection" role="group" aria-labelledby={searchGameHeadingId}>
              {#if gameHint}
                <span id={searchGameHeadingId} class="searchOverlay_categoryHint">{gameHint}</span>
              {/if}
              {#each flatGame as row, i (row.key)}
                <button
                  type="button"
                  id={searchOverlayOptionId(combinedIndexFor(row, i, "g"))}
                  class="searchOverlay_resultItem searchOverlay_categoryResultItem"
                  role="option"
                  tabindex="-1"
                  aria-label={[row.title, row.badge, gameHint, row.categoryChecked ? "selected" : ""].filter(Boolean).join(", ")}
                  aria-selected={combinedIndexFor(row, i, "g") === selectedIndex}
                  class:searchOverlay_categoryChecked={row.categoryChecked}
                  class:searchOverlay_selected={combinedIndexFor(row, i, "g") === selectedIndex}
                  on:mousedown|preventDefault
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
