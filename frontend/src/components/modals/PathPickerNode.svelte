<script lang="ts">
  import * as FilesystemService from "../../../bindings/TcNo-Acc-Switcher/filesystemservice";
  import {
    normalizeDisplayPath,
    sameFsPath,
    isStrictAncestorFolder,
    folderCoversSelected,
    parentDisplayPath,
    shortcutRowKey,
    treeRowKey,
  } from "../../lib/fsPaths";
  import { folderNameMatches } from "../../lib/folderNamePattern";
  import { formatUnknownError } from "../../lib/formatWailsError";
  import { collapse, DUR } from "../../lib/animation";
  import { getContext, onDestroy } from "svelte";
  import { writable, type Writable } from "svelte/store";
  import { t } from "../../stores/i18n";

  type DirEntry = { name: string; path: string; isDir: boolean };
  type TreeContext = {
    activeKey: Writable<string>;
    setActiveKey: (key: string) => void;
    focusRelative: (currentKey: string, offset: number) => Promise<boolean>;
    focusBoundary: (position: "first" | "last") => Promise<boolean>;
    focusFirstChild: (parentKey: string) => Promise<boolean>;
    focusParent: (parentKey: string | null | undefined) => Promise<boolean>;
  };
  const PATH_PICKER_TREE_CONTEXT = "path-picker-tree";
  const inactiveKey = writable("");
  const tree = getContext<TreeContext | undefined>(PATH_PICKER_TREE_CONTEXT);
  const activeTreeKey = tree?.activeKey ?? inactiveKey;

  export let path: string;
  export let label: string;
  export let depth = 0;
  export let parentPath: string | null = null;
  export let parentKey: string | null = null;
  export let selectedPath: string;
  export let dirsOnly = true;
  export let soughtFilename = "";
  /** Folder-name pattern to highlight, e.g. `TcNo-Acc-Switcher-SteamGuard*`. */
  export let suggestedFolder = "";
  export let isDir = true;
  export let onPick: (p: string, entryIsDir: boolean) => void;
  export let expandEpoch = 0;
  /** Root entries only: what this row is, which decides its icon. */
  export let kind = "";
  /**
   * A shortcut to somewhere that already exists under a drive. Selecting it
   * only fills in the path; expanding it here would show the same folder twice,
   * once under the shortcut and again under the drive the tree expands to.
   */
  export let link = false;

  let expanded = false;
  let loading = false;
  let children: DirEntry[] = [];
  let loadError: string | null = null;
  let seenEpoch = -1;

  let listAttempted = false;
  let hasExpandableChildren = true;
  let loadChildrenSeq = 0;

  $: displayLabel = label || normalizeDisplayPath(path);

  // A drive row already reads as its own path; a user folder shows its name, so
  // the full location is worth having on hover.
  $: rowTitle = depth === 0 && !sameFsPath(displayLabel, path) ? normalizeDisplayPath(path) : "";

  const locationIcons: Record<string, string> = {
    "network-drive": "img/icons/loc_network.svg",
    home: "img/icons/loc_home.svg",
    desktop: "img/icons/loc_desktop.svg",
    documents: "img/icons/loc_documents.svg",
    downloads: "img/icons/loc_downloads.svg",
    pictures: "img/icons/loc_pictures.svg",
    music: "img/icons/loc_music.svg",
    videos: "img/icons/loc_videos.svg",
  };

  // A known location leads with what it is; the folder shrinks to a corner
  // badge, where it still carries the open and closed state.
  $: locationIconSrc = locationIcons[kind] ?? "";

  $: showTwisty =
    !link && isDir && (!listAttempted || !!loadError || hasExpandableChildren);

  $: expanderIconSrc = (() => {
    if (!expanded) return "img/icons/folder.svg";
    if (loading || loadError) return "img/icons/folder_open.svg";
    if (listAttempted && children.length === 0) return "img/icons/folder_open_empty.svg";
    return "img/icons/folder_open.svg";
  })();

  $: if (expandEpoch !== seenEpoch) {
    seenEpoch = expandEpoch;
    if (!link && isDir && selectedPath && folderCoversSelected(path, selectedPath)) {
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
    if (link) {
      onPick(pNorm, true);
      return;
    }
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
    tree?.setActiveKey(nodeKey);
  }

  function handleArrowRight(): void {
    if (link || !isDir) return;
    if (!expanded) {
      expanded = true;
      void loadChildren();
      return;
    }
    void tree?.focusFirstChild(childParentKey);
  }

  function handleArrowLeft(): void {
    const pNorm = normalizeDisplayPath(path);
    if (!link && isDir && expanded) {
      handleDirCollapse(pNorm);
      return;
    }
    void tree?.focusParent(parentKey);
  }

  $: nodeKey = link ? shortcutRowKey(path) : treeRowKey(path);
  $: childParentKey = treeRowKey(path);
  $: rowSelected = sameFsPath(selectedPath, path);
  // A shortcut is a way in, not a step along the way: marking it as an ancestor
  // would highlight the home folder for every path inside it.
  $: rowAncestor = !link && isStrictAncestorFolder(path, selectedPath);
  $: soughtNorm = soughtNameNorm(soughtFilename);
  // Folders are suggested by name pattern, files by the exact name being sought.
  // A root is its own drive path, never a name worth matching.
  $: rowSuggested = isDir
    ? depth > 0 && folderNameMatches(label, suggestedFolder)
    : soughtNorm !== "" && label.toLowerCase() === soughtNorm;
  $: rowActive = $activeTreeKey === nodeKey;

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
  aria-expanded={isDir && !link ? expanded : undefined}
  aria-busy={loading ? "true" : undefined}
  data-key={nodeKey}
  data-parent-key={parentKey ?? undefined}
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
        void tree?.focusRelative(nodeKey, 1);
        return;
      case "ArrowUp":
        e.preventDefault();
        void tree?.focusRelative(nodeKey, -1);
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
          src={locationIconSrc || expanderIconSrc}
          alt=""
          width="20"
          height="20"
          draggable="false"
        />
        {#if locationIconSrc}
          <img class="pp-row-badge" src={expanderIconSrc} alt="" width="12" height="12" draggable="false" />
        {/if}
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
          src={locationIconSrc || "img/icons/folder.svg"}
          alt=""
          width="20"
          height="20"
          draggable="false"
        />
        {#if locationIconSrc}
          <img class="pp-row-badge" src="img/icons/folder.svg" alt="" width="12" height="12" draggable="false" />
        {/if}
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
      title={rowTitle}
    >{displayLabel}</span>
  </div>
  {#if expanded}
    <div class="pp-node-group" role="group" transition:collapse={{ duration: DUR.fast }}>
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
            parentKey={childParentKey}
            isDir={c.isDir}
            {selectedPath}
            {dirsOnly}
            {soughtFilename}
            {suggestedFolder}
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
    position: relative;
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
  /* The folder rides along in the corner so the row still reads as a folder,
     and keeps showing whether it is open. */
  .pp-row-badge {
    position: absolute;
    right: 0;
    bottom: 1px;
    width: 12px;
    height: 12px;
    pointer-events: none;
    user-select: none;
    -webkit-user-drag: none;
    filter: drop-shadow(0 0 1.5px var(--program-bg, #000));
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
