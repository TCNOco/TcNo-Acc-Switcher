<script lang="ts">
  import * as FilesystemService from "../../../bindings/TcNo-Acc-Switcher/filesystemservice";
  import {
    normalizeDisplayPath,
    sameFsPath,
    isStrictAncestorFolder,
    folderCoversSelected,
    parentDisplayPath,
  } from "../../lib/fsPaths";
  import { formatUnknownError } from "../../lib/formatWailsError";
  import { getContext, onDestroy } from "svelte";
  import { writable, type Writable } from "svelte/store";
  import { t } from "../../stores/i18n";

  type DirEntry = { name: string; path: string; isDir: boolean };
  type TreeContext = {
    activePath: Writable<string>;
    setActivePath: (path: string) => void;
    focusRelative: (currentPath: string, offset: number) => Promise<boolean>;
    focusBoundary: (position: "first" | "last") => Promise<boolean>;
    focusFirstChild: (parentPath: string) => Promise<boolean>;
    focusParent: (parentPath: string | null | undefined) => Promise<boolean>;
  };
  const PATH_PICKER_TREE_CONTEXT = "path-picker-tree";
  const inactivePath = writable("");
  const tree = getContext<TreeContext | undefined>(PATH_PICKER_TREE_CONTEXT);
  const activeTreePath = tree?.activePath ?? inactivePath;

  export let path: string;
  export let label: string;
  export let depth = 0;
  export let parentPath: string | null = null;
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
  let loadChildrenSeq = 0;

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

  function computeExpandableChildren(list: DirEntry[]): boolean {
    if (dirsOnly) return list.some((c) => c.isDir);
    return list.some((c) => !c.isDir) || list.some((c) => c.isDir);
  }

  async function loadChildren(): Promise<void> {
    if (!isDir || loading) return;
    if (children.length > 0) return;
    const seq = ++loadChildrenSeq;
    loading = true;
    loadError = null;
    try {
      const list = await FilesystemService.ListDir(path);
      if (seq !== loadChildrenSeq) return;
      children = dirsOnly ? list.filter((c: DirEntry) => c.isDir) : list;
      listAttempted = true;
      hasExpandableChildren = computeExpandableChildren(list);
    } catch (e) {
      if (seq !== loadChildrenSeq) return;
      loadError = formatUnknownError(e);
      children = [];
      listAttempted = true;
      hasExpandableChildren = true;
    } finally {
      if (seq === loadChildrenSeq) loading = false;
    }
  }

  function soughtNameNorm(raw: string): string {
    const t = raw.trim();
    if (!t) return "";
    const u = t.replace(/\\/g, "/");
    const idx = u.lastIndexOf("/");
    return (idx >= 0 ? u.slice(idx + 1) : t).toLowerCase();
  }

  function handleDirCollapse(pNorm: string): void {
    if (listAttempted && !loadError && children.length === 0) {
      onPick(pNorm, true);
      expanded = false;
      loadChildrenSeq++;
      return;
    }
    expanded = false;
    loadChildrenSeq++;
    if (selectedPath && (sameFsPath(selectedPath, pNorm) || folderCoversSelected(pNorm, selectedPath))) {
      onPick(parentDisplayPath(pNorm), true);
    }
  }

  function onLabelClick(): void {
    const pNorm = normalizeDisplayPath(path);
    if (isDir && expanded) {
      handleDirCollapse(pNorm);
      return;
    }
    onPick(pNorm, isDir);
    if (isDir && !expanded) {
      expanded = true;
      void loadChildren();
    }
  }

  function onTreeitemFocus(): void {
    tree?.setActivePath(path);
  }

  function handleArrowRight(): void {
    if (!isDir) return;
    if (!expanded) {
      expanded = true;
      void loadChildren();
      return;
    }
    void tree?.focusFirstChild(path);
  }

  function handleArrowLeft(): void {
    const pNorm = normalizeDisplayPath(path);
    if (isDir && expanded) {
      handleDirCollapse(pNorm);
      return;
    }
    void tree?.focusParent(parentPath);
  }

  $: rowSelected = sameFsPath(selectedPath, path);
  $: rowAncestor = isStrictAncestorFolder(path, selectedPath);
  $: soughtNorm = soughtNameNorm(soughtFilename);
  $: rowSuggested =
    !isDir && soughtNorm !== "" && label.toLowerCase() === soughtNorm;
  $: rowActive = sameFsPath($activeTreePath, path);

  onDestroy(() => {
    loadChildrenSeq++;
  });
</script>

<div
  class="pp-node"
  style:padding-left={depth === 0 ? "0" : "14px"}
  role="treeitem"
  tabindex={rowActive ? 0 : -1}
  aria-level={depth + 1}
  aria-selected={rowSelected}
  aria-expanded={isDir ? expanded : undefined}
  aria-busy={loading ? "true" : undefined}
  data-path={normalizeDisplayPath(path)}
  data-parent-path={parentPath ?? undefined}
  data-is-dir={isDir ? "true" : "false"}
  data-expanded={expanded ? "true" : "false"}
  on:focus={onTreeitemFocus}
  on:keydown={(e) => {
    switch (e.key) {
      case "Enter":
      case " ":
        e.preventDefault();
        onLabelClick();
        return;
      case "ArrowDown":
        e.preventDefault();
        void tree?.focusRelative(path, 1);
        return;
      case "ArrowUp":
        e.preventDefault();
        void tree?.focusRelative(path, -1);
        return;
      case "ArrowRight":
        e.preventDefault();
        handleArrowRight();
        return;
      case "ArrowLeft":
        e.preventDefault();
        handleArrowLeft();
        return;
      case "Home":
        e.preventDefault();
        void tree?.focusBoundary("first");
        return;
      case "End":
        e.preventDefault();
        void tree?.focusBoundary("last");
        return;
    }
  }}
>
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div
    class="pp-row"
    class:selected-path={rowSelected}
    class:ancestor-of-selected={rowAncestor}
    class:suggested={rowSuggested}
    on:click|stopPropagation={onLabelClick}
  >
    {#if isDir && showTwisty}
      <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
      <span
        class="pp-twisty"
        aria-hidden="true"
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
      </span>
    {:else if isDir}
      <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
      <span
        class="pp-twisty"
        aria-hidden="true"
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
      </span>
    {:else}
      <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
      <span
        class="pp-twisty"
        aria-hidden="true"
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
      </span>
    {/if}
    <span
      class="pp-label"
      class:selected-path={rowSelected}
      class:ancestor-of-selected={rowAncestor}
    >{displayLabel}</span>
  </div>
  {#if expanded}
    <div class="pp-node-group" role="group">
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
            parentPath={path}
            isDir={c.isDir}
            {selectedPath}
            {dirsOnly}
            {soughtFilename}
            {onPick}
            {expandEpoch}
          />
        {/if}
      {/each}
    </div>
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
      color: var(--accent-text-bright);
    }
  }
  .pp-label.ancestor-of-selected:not(.selected-path) {
    color: var(--accent);
  }
  .pp-muted,
  .pp-err {
    font-size: 11px;
    opacity: 0.85;
  }
  .pp-err {
    color: var(--red);
  }
</style>
