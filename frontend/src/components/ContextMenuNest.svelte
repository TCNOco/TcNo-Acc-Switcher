<script lang="ts">
  import type { MenuItemDef } from "../stores/contextMenu";
  import { get } from "svelte/store";
  import { closeContextMenu, submenuOpenPath, submenuExpandEnabled } from "../stores/contextMenu";
  import { ctxMenuLog } from "../lib/contextMenuDebug";
  import { t } from "../stores/i18n";

  /** Matches legacy Blazor ContextMenuItem.razor */
  const ITEMS_PER_PAGE = 5;

  export let items: MenuItemDef[] = [];
  /** submenu depth for CSS class */
  export let depth = 1;
  /** Path of indices from root down to this list's parent (empty at root column). */
  export let pathPrefix: number[] = [];

  /** Pagination/search paging only in flyout columns — not the root menu. */
  $: paginateThisColumn = pathPrefix.length > 0;

  let searchQuery = "";
  let currentPage = 0;

  function norm(s: string): string {
    return s.toLowerCase();
  }

  /**
   * Splits a label into words for acronym-style search. Any run of these separators starts a new
   * word (spaces, hyphen/minus, underscores, en/em dash, colon, semicolon, comma, dot, slash).
   */
  const LABEL_WORD_SPLIT_RE = /[\s\u00A0\-_–—:;,.|/\\\u2026]+/u;

  function splitLabelWords(label: string): string[] {
    return label
      .trim()
      .split(LABEL_WORD_SPLIT_RE)
      .map((w) => w.trim())
      .filter((w) => w.length > 0);
  }

  function firstGrapheme(word: string): string {
    if (!word) {
      return "";
    }
    try {
      const Seg = (Intl as typeof Intl & { Segmenter?: typeof Intl.Segmenter }).Segmenter;
      if (Seg) {
        const seg = new Seg(undefined, { granularity: "grapheme" });
        for (const { segment } of seg.segment(word)) {
          return segment;
        }
      }
    } catch {
      /* Segmenter unsupported or invalid locale */
    }
    return [...word][0] ?? "";
  }

  /** Concatenation of first grapheme per word, e.g. "Counter-Strike: 2" → "CS2". */
  function labelAcronymHaystack(label: string): string {
    return splitLabelWords(label).map(firstGrapheme).join("");
  }

  $: hasSearchMarker = items[0]?.type === "search";
  $: tail = hasSearchMarker ? items.slice(1) : items;
  /**
   * Search row only in flyout columns (same >5 threshold as legacy); root lists are unpaged so
   * no search strip on the root menu.
   */
  $: showSearchRow = paginateThisColumn && tail.length > ITEMS_PER_PAGE;
  $: q = norm(searchQuery.trim());

  /** Stable row visibility for search (indices must match pathPrefix). */
  function rowMatchesSearch(it: MenuItemDef): boolean {
    if (!showSearchRow || q === "") {
      return true;
    }
    if (it.type === "separator") {
      return false;
    }
    if (!it.label) {
      return false;
    }
    const lab = norm(it.label);
    if (lab.includes(q)) {
      return true;
    }
    const ac = norm(labelAcronymHaystack(it.label));
    return ac.length > 0 && ac.includes(q);
  }

  type TailEntry = { item: MenuItemDef; idx: number };

  /** Rows under this column after search filter (pagination slices this list). */
  $: filteredTailEntries = (() => {
    void showSearchRow;
    void q;
    const out: TailEntry[] = [];
    for (let idx = 0; idx < tail.length; idx++) {
      const item = tail[idx]!;
      if (!rowMatchesSearch(item)) {
        continue;
      }
      out.push({ item, idx });
    }
    return out;
  })();

  $: pageCount = Math.max(1, Math.ceil(filteredTailEntries.length / ITEMS_PER_PAGE));
  $: showPagination = paginateThisColumn && pageCount > 1;
  $: pagedTailEntries = paginateThisColumn
    ? filteredTailEntries.slice(
        currentPage * ITEMS_PER_PAGE,
        currentPage * ITEMS_PER_PAGE + ITEMS_PER_PAGE,
      )
    : filteredTailEntries;

  $: if (currentPage >= pageCount) {
    currentPage = Math.max(0, pageCount - 1);
  }

  function onSearchInput(): void {
    currentPage = 0;
  }

  function submenuClass(d: number): string {
    return d <= 1 ? "submenu submenu1" : "submenu submenu2";
  }

  function pathsEqual(a: number[], b: number[]): boolean {
    if (a.length !== b.length) {
      return false;
    }
    return a.every((v, i) => v === b[i]);
  }

  function stop(ev: Event): void {
    ev.stopPropagation();
  }

  function run(item: MenuItemDef): void {
    if (item.disabled) {
      return;
    }
    item.action?.();
    closeContextMenu();
  }

  /**
   * Which row index under this list is expanded — must be a `$:` reactive block so
   * `class:submenu-expanded` updates when `submenuOpenPath` changes (store reads inside plain
   * helper functions called from markup are not always subscribed in Svelte).
   */
  $: expandedIdxAtLevel = (() => {
    const p = $submenuOpenPath;
    if (p.length <= pathPrefix.length) {
      return undefined;
    }
    for (let i = 0; i < pathPrefix.length; i++) {
      if (p[i] !== pathPrefix[i]) {
        return undefined;
      }
    }
    return p[pathPrefix.length];
  })();

  /**
   * Expand via hover / movement. Requires layout to have finished (`submenuExpandEnabled`).
   * Supports both Pointer and Mouse events — WebView2 sometimes delivers `mousemove`/`mouseenter`
   * without meaningful `pointermove`.
   */
  function expandBranch(idx: number, source: string): void {
    if (!get(submenuExpandEnabled)) {
      return;
    }
    const next = [...pathPrefix, idx];
    submenuOpenPath.update((cur) => {
      if (pathsEqual(cur, next)) {
        return cur;
      }
      ctxMenuLog("expandBranch", {
        source,
        depth,
        idx,
        pathPrefix,
        newPath: next,
      });
      return next;
    });
  }

  /**
   * Collapse deeper paths when moving onto a leaf at this column.
   * If `elementFromPoint` is not inside this `li`, a sibling row was stacked above the flyout — ignore.
   */
  function onLeafPointerEnter(ev: PointerEvent): void {
    const li = ev.currentTarget as HTMLElement;
    const hit = document.elementFromPoint(ev.clientX, ev.clientY);
    if (!hit || !li.contains(hit)) {
      ctxMenuLog("leaf enter ignored (top hit not inside this row)", {
        hitNode: hit instanceof Node ? hit.nodeName : hit,
      });
      return;
    }
    const next = [...pathPrefix];
    submenuOpenPath.update((cur) => {
      if (pathsEqual(cur, next)) {
        return cur;
      }
      ctxMenuLog("leaf enter → trim path", next);
      return next;
    });
  }

  /** Allow navigation keys to bubble to `.ctx-menu-root`; stop others so they don't hit the page. */
  function onSearchKeydown(ev: KeyboardEvent): void {
    const k = ev.key;
    if (
      k === "ArrowDown" ||
      k === "ArrowUp" ||
      k === "Home" ||
      k === "End" ||
      k === "Escape"
    ) {
      return;
    }
    ev.stopPropagation();
  }

</script>

{#if showSearchRow}
  <li class="contextSearch" role="none" on:pointerdown={stop}>
    <input
      type="search"
      class="ctx-menu__search"
      placeholder={hasSearchMarker ? items[0]?.label || $t("Context_Search") : $t("Context_Search")}
      bind:value={searchQuery}
      on:input={onSearchInput}
      on:pointerdown={stop}
      on:keydown={onSearchKeydown}
    />
  </li>
{/if}

{#each pagedTailEntries as { item, idx } (idx)}
  {#if item.type === "separator"}
    <li class="ctx-sep" role="separator"><hr /></li>
  {:else if item.children?.length}
    <li
      class="hasSubmenu"
      class:submenu-expanded={expandedIdxAtLevel === idx}
      role="none"
      data-submenu-path={JSON.stringify([...pathPrefix, idx])}
      on:pointerenter={() => expandBranch(idx, "pointerenter")}
      on:mouseenter={() => expandBranch(idx, "mouseenter")}
    >
      <span
        role="menuitem"
        tabindex="-1"
        aria-haspopup="true"
        aria-expanded={expandedIdxAtLevel === idx}
        class="ctx-menu__label"
        on:pointermove={() => expandBranch(idx, "pointermove")}
        on:mousemove={() => expandBranch(idx, "mousemove")}
      >{item.label}</span>
      <ul class={submenuClass(depth)} role="menu">
        <svelte:self items={item.children ?? []} depth={depth + 1} pathPrefix={[...pathPrefix, idx]} />
      </ul>
    </li>
  {:else}
    <li role="menuitem" on:pointerenter={(e) => onLeafPointerEnter(e)}>
      <button
        type="button"
        class="ctx-menu__btn"
        class:ctx-menu__btn--disabled={item.disabled}
        disabled={item.disabled}
        aria-disabled={item.disabled ? "true" : undefined}
        tabindex={item.disabled ? -1 : undefined}
        on:click={() => run(item)}>{item.label}</button>
    </li>
  {/if}
{/each}

{#if showPagination}
  <li class="ctx-pagination-li" role="none" on:pointerdown={stop}>
    <div class="paginationContainer">
      <div class="pagination">
      {#if currentPage >= 1}
        <button
          type="button"
          class="paginationButton"
          aria-label="Previous page"
          on:pointerdown={stop}
          on:click={() => {
            currentPage--;
          }}
        >
          <i aria-hidden="true">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 320 512" width="12" height="12">
              <path
                fill="currentColor"
                d="M34.52 239.03L228.87 44.69c9.37-9.37 24.57-9.37 33.94 0l22.67 22.67c9.36 9.36 9.37 24.52.04 33.9L131.49 256l154.02 154.75c9.34 9.38 9.32 24.54-.04 33.9l-22.67 22.67c-9.37 9.37-24.57 9.37-33.94 0L34.52 272.97c-9.37-9.37-9.37-24.57 0-33.94z"
              />
            </svg>
          </i>
        </button>
      {:else}
        <button
          type="button"
          class="paginationButton"
          style="visibility: hidden"
          aria-hidden="true"
          tabindex="-1"
          disabled
        >
          <i aria-hidden="true">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 320 512" width="12" height="12">
              <path
                fill="currentColor"
                d="M34.52 239.03L228.87 44.69c9.37-9.37 24.57-9.37 33.94 0l22.67 22.67c9.36 9.36 9.37 24.52.04 33.9L131.49 256l154.02 154.75c9.34 9.38 9.32 24.54-.04 33.9l-22.67 22.67c-9.37 9.37-24.57 9.37-33.94 0L34.52 272.97c-9.37-9.37-9.37-24.57 0-33.94z"
              />
            </svg>
          </i>
        </button>
      {/if}
      <span>{currentPage + 1} / {pageCount}</span>
      {#if currentPage < pageCount - 1}
        <button
          type="button"
          class="paginationButton"
          aria-label="Next page"
          on:pointerdown={stop}
          on:click={() => {
            currentPage++;
          }}
        >
          <i aria-hidden="true">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 320 512"
              width="12"
              height="12"
              style="transform: scaleX(-1)"
            >
              <path
                fill="currentColor"
                d="M34.52 239.03L228.87 44.69c9.37-9.37 24.57-9.37 33.94 0l22.67 22.67c9.36 9.36 9.37 24.52.04 33.9L131.49 256l154.02 154.75c9.34 9.38 9.32 24.54-.04 33.9l-22.67 22.67c-9.37 9.37-24.57 9.37-33.94 0L34.52 272.97c-9.37-9.37-9.37-24.57 0-33.94z"
              />
            </svg>
          </i>
        </button>
      {:else}
        <button
          type="button"
          class="paginationButton"
          style="visibility: hidden"
          aria-hidden="true"
          tabindex="-1"
          disabled
        >
          <i aria-hidden="true">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 320 512"
              width="12"
              height="12"
              style="transform: scaleX(-1)"
            >
              <path
                fill="currentColor"
                d="M34.52 239.03L228.87 44.69c9.37-9.37 24.57-9.37 33.94 0l22.67 22.67c9.36 9.36 9.37 24.52.04 33.9L131.49 256l154.02 154.75c9.34 9.38 9.32 24.54-.04 33.9l-22.67 22.67c-9.37 9.37-24.57 9.37-33.94 0L34.52 272.97c-9.37-9.37-9.37-24.57 0-33.94z"
              />
            </svg>
          </i>
        </button>
      {/if}
      </div>
    </div>
  </li>
{/if}

<style lang="scss">
  .row-hidden {
    display: none !important;
  }
</style>
