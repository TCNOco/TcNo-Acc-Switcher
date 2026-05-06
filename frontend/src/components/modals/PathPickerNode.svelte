<script lang="ts">
  import * as FilesystemService from "../../../bindings/TcNo-Acc-Switcher/filesystemservice";
  import {
    normalizeDisplayPath,
    sameFsPath,
    isStrictAncestorFolder,
    folderCoversSelected,
    parentDisplayPath,
  } from "../../lib/fsPaths";
  import { formatUnknownError } from "../../lib/formatBindingError";
  import { t } from "../../stores/i18n";

  type DirEntry = { name: string; path: string; isDir: boolean };

  export let path: string;
  export let label: string;
  export let depth = 0;
  export let selectedPath: string;
  export let dirsOnly = true;
  export let soughtFilename = "";
  export let isDir = true;
  export let onPick: (p: string, entryIsDir: boolean) => void;
  export let expandEpoch = 0;

  let expanded = false;
  let loading = false;
  let children: DirEntry[] = [];
  let loadError: string | null = null;
  let seenEpoch = -1;

  let listAttempted = false;
  let hasExpandableChildren = true;

  $: displayLabel = depth === 0 ? normalizeDisplayPath(path) : label;

  $: showTwisty =
    isDir && (!listAttempted || !!loadError || hasExpandableChildren);

  $: expanderIconSrc = (() => {
    if (!expanded) return "img/icons/folder.svg";
    if (loading || loadError) return "img/icons/folder_open.svg";
    if (listAttempted && children.length === 0) return "img/icons/folder_open_empty.svg";
    return "img/icons/folder_open.svg";
  })();

  $: if (expandEpoch !== seenEpoch) {
    seenEpoch = expandEpoch;
    if (isDir && selectedPath && folderCoversSelected(path, selectedPath)) {
      if (!expanded) expanded = true;
      void loadChildren();
    }
  }

  async function loadChildren(): Promise<void> {
    if (!isDir || loading) return;
    if (children.length > 0) return;
    loading = true;
    loadError = null;
    try {
      const list = await FilesystemService.ListDir(path);
      children = dirsOnly ? list.filter((c: DirEntry) => c.isDir) : list;
      listAttempted = true;
      if (dirsOnly) {
        hasExpandableChildren = children.some((c) => c.isDir);
      } else {
        hasExpandableChildren =
          children.some((c) => !c.isDir) || children.some((c) => c.isDir);
      }
    } catch (e) {
      loadError = formatUnknownError(e);
      children = [];
      listAttempted = true;
      hasExpandableChildren = true;
    } finally {
      loading = false;
    }
  }

  function soughtNameNorm(raw: string): string {
    const t = raw.trim();
    if (!t) return "";
    const u = t.replace(/\\/g, "/");
    const idx = u.lastIndexOf("/");
    return (idx >= 0 ? u.slice(idx + 1) : t).toLowerCase();
  }

  function onLabelClick(): void {
    const pNorm = normalizeDisplayPath(path);
    if (isDir && expanded) {
      const openedEmpty =
        listAttempted && !loadError && children.length === 0;
      if (openedEmpty) {
        onPick(pNorm, true);
        expanded = false;
        return;
      }
      expanded = false;
      if (
        selectedPath &&
        (sameFsPath(selectedPath, pNorm) || folderCoversSelected(pNorm, selectedPath))
      ) {
        onPick(parentDisplayPath(pNorm), true);
      }
      return;
    }
    onPick(pNorm, isDir);
    if (!isDir) return;
    if (!expanded) {
      expanded = true;
      void loadChildren();
    }
  }

  $: rowSelected = sameFsPath(selectedPath, path);
  $: rowAncestor = isStrictAncestorFolder(path, selectedPath);
  $: soughtNorm = soughtNameNorm(soughtFilename);
  $: rowSuggested =
    !isDir && soughtNorm !== "" && label.toLowerCase() === soughtNorm;
</script>

<div class="pp-node" style:padding-left={depth === 0 ? "0" : "14px"}>
  <div class="pp-row" class:selected-path={rowSelected} class:ancestor-of-selected={rowAncestor} class:suggested={rowSuggested}>
    {#if isDir && showTwisty}
      <button
        type="button"
        class="pp-twisty"
        aria-expanded={expanded}
        aria-label={expanded ? $t("Aria_Collapse") : $t("Aria_Expand")}
        on:click|stopPropagation={onLabelClick}
      >
        <img
          class="pp-row-icon"
          src={expanderIconSrc}
          alt=""
          width="20"
          height="20"
          draggable="false"
        />
      </button>
    {:else if isDir}
      <button
        type="button"
        class="pp-twisty"
        aria-label={displayLabel}
        on:click|stopPropagation={onLabelClick}
      >
        <img
          class="pp-row-icon"
          src="img/icons/folder.svg"
          alt=""
          width="20"
          height="20"
          draggable="false"
        />
      </button>
    {:else}
      <button
        type="button"
        class="pp-twisty"
        aria-label={displayLabel}
        on:click|stopPropagation={onLabelClick}
      >
        <img
          class="pp-row-icon"
          src="img/icons/file.svg"
          alt=""
          width="20"
          height="20"
          draggable="false"
        />
      </button>
    {/if}
    <span
      class="pp-label"
      class:selected-path={rowSelected}
      class:ancestor-of-selected={rowAncestor}
      role="button"
      tabindex="0"
      on:click|stopPropagation={onLabelClick}
      on:keydown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onLabelClick();
        }
      }}
    >{displayLabel}</span>
  </div>
  {#if expanded}
    {#if loading}
      <div class="pp-muted" style:padding-left="14px">…</div>
    {/if}
    {#if loadError}
      <div class="pp-err" style:padding-left="14px">{loadError}</div>
    {/if}
    {#each children as c (c.path)}
      {#if c.isDir || !dirsOnly}
        <svelte:self
          path={c.path}
          label={c.name}
          depth={depth + 1}
          isDir={c.isDir}
          {selectedPath}
          {dirsOnly}
          {soughtFilename}
          {onPick}
          {expandEpoch}
        />
      {/if}
    {/each}
  {/if}
</div>

<style lang="scss">
  .pp-node {
    text-align: left;
  }
  .pp-row {
    display: flex;
    align-items: center;
    gap: 6px;
    min-height: 1.5em;
  }
  .pp-twisty {
    flex: 0 0 auto;
    width: 26px;
    height: 26px;
    padding: 0;
    margin: 0;
    border: 0;
    background: transparent;
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0.9;
    &:hover {
      opacity: 1;
    }
  }
  .pp-row-icon {
    width: 20px;
    height: 20px;
    object-fit: contain;
    display: block;
    user-select: none;
    -webkit-user-drag: none;
  }
  .pp-label {
    cursor: pointer;
    user-select: none;
    flex: 1;
    min-width: 0;
    word-break: break-all;
    padding: 0.3em 0;
    &:hover {
      color: #b1f2ff;
    }
  }
  .pp-label.ancestor-of-selected:not(.selected-path) {
    color: var(--cyan);
  }
  .pp-muted,
  .pp-err {
    font-size: 11px;
    opacity: 0.85;
  }
  .pp-err {
    color: #ff8a8a;
  }
</style>
