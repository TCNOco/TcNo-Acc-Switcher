<script lang="ts">
  import { onDestroy, onMount, setContext, tick } from "svelte";
  import { get, writable, type Writable } from "svelte/store";
  import * as FilesystemService from "../../../bindings/TcNo-Acc-Switcher/filesystemservice";
  import PathPickerNode from "./PathPickerNode.svelte";
  import { normalizeDisplayPath, treeRowKey } from "../../lib/fsPaths";
  import { motionEnabled } from "../../lib/animation";
  import { scrollDeltaIntoView } from "../../lib/scrollIntoContainer";
  import { t } from "../../stores/i18n";

  const PATH_PICKER_TREE_CONTEXT = "path-picker-tree";

  /**
   * Rows are addressed by key, not by path: a shortcut root and the row the
   * tree expands to are the same folder, and telling them apart is what keeps
   * focus and scrolling on the row inside the tree.
   */
  type TreeContext = {
    activeKey: Writable<string>;
    setActiveKey: (key: string) => void;
    focusRelative: (currentKey: string, offset: number) => Promise<boolean>;
    focusBoundary: (position: "first" | "last") => Promise<boolean>;
    focusFirstChild: (parentKey: string) => Promise<boolean>;
    focusParent: (parentKey: string | null | undefined) => Promise<boolean>;
  };

  type FsRoot = { path: string; label: string; kind: string };

  // Drives are where the tree really starts; every other root is a shortcut to
  // a folder that already lives under one of them.
  const driveKinds = new Set(["drive", "network-drive"]);

  export let selectedPath: string;
  export let dirsOnly = true;
  export let soughtFilename = "";
  /** Folder-name pattern to highlight, e.g. `TcNo-Acc-Switcher-SteamGuard*`. */
  export let suggestedFolder = "";
  export let onPick: (p: string, entryIsDir: boolean) => void;

  let roots: FsRoot[] = [];
  let err: string | null = null;
  let treeEl: HTMLDivElement | undefined;
  let scrollEl: HTMLDivElement | undefined;

  let expandEpoch = 0;
  let revealSeq = 0;
  let revealTimer = 0;
  let lastSelForExpand = "\u0000sentinel";
  const activeKey = writable("");

  function visibleTreeitems(): HTMLDivElement[] {
    if (!treeEl) return [];
    return Array.from(treeEl.querySelectorAll<HTMLDivElement>('[role="treeitem"]'));
  }

  function keyFor(el: HTMLElement | null | undefined): string {
    return el?.dataset.key ?? "";
  }

  function findVisibleTreeitem(key: string): HTMLDivElement | undefined {
    if (!key) return undefined;
    return visibleTreeitems().find((item) => keyFor(item) === key);
  }

  function pickFallbackTreeitem(preferredKey = ""): HTMLDivElement | undefined {
    const items = visibleTreeitems();
    if (items.length === 0) return undefined;
    return (
      (preferredKey ? findVisibleTreeitem(preferredKey) : undefined) ??
      findVisibleTreeitem(get(activeKey)) ??
      items[0]
    );
  }

  function setActiveKey(key: string): void {
    if (!key) return;
    activeKey.set(key);
  }

  async function focusElement(el: HTMLDivElement | undefined): Promise<boolean> {
    if (!el) return false;
    const nextKey = keyFor(el);
    if (!nextKey) return false;
    setActiveKey(nextKey);
    await tick();
    el.focus();
    return true;
  }

  async function focusRelative(currentKey: string, offset: number): Promise<boolean> {
    await tick();
    const items = visibleTreeitems();
    if (items.length === 0) return false;
    const currentIndex = items.findIndex((item) => keyFor(item) === currentKey);
    const baseIndex = currentIndex >= 0 ? currentIndex : 0;
    const nextIndex = Math.min(items.length - 1, Math.max(0, baseIndex + offset));
    return focusElement(items[nextIndex]);
  }

  async function focusBoundary(position: "first" | "last"): Promise<boolean> {
    await tick();
    const items = visibleTreeitems();
    if (items.length === 0) return false;
    return focusElement(position === "first" ? items[0] : items[items.length - 1]);
  }

  async function focusFirstChild(parentKey: string): Promise<boolean> {
    await tick();
    const items = visibleTreeitems();
    const parentIndex = items.findIndex((item) => keyFor(item) === parentKey);
    if (parentIndex < 0) return false;
    const parentLevel = Number(items[parentIndex].getAttribute("aria-level") ?? "0");
    const child = items.slice(parentIndex + 1).find((item) => {
      const itemLevel = Number(item.getAttribute("aria-level") ?? "0");
      return itemLevel === parentLevel + 1 && (item.dataset.parentKey ?? "") === parentKey;
    });
    return focusElement(child);
  }

  async function focusParent(parentKey: string | null | undefined): Promise<boolean> {
    if (!parentKey) return false;
    await tick();
    return focusElement(findVisibleTreeitem(parentKey));
  }

  /**
   * Brings the row for `target` into view, waiting for it to appear: selecting
   * a path expands the tree towards it, and each level loads asynchronously, so
   * the row usually does not exist yet when the path changes. The row inside
   * the tree is the one to reveal, never the shortcut that shares its path.
   * Only the tree scrolls, and only when the row is out of sight.
   */
  function revealPath(target: string): void {
    if (!target) return;
    const seq = ++revealSeq;
    if (revealTimer) window.clearTimeout(revealTimer);
    const giveUpAt = Date.now() + 4000;
    const attempt = (): void => {
      if (seq !== revealSeq || !scrollEl) return;
      const row = findVisibleTreeitem(treeRowKey(target))?.querySelector<HTMLElement>(".pp-row");
      if (row) {
        scrollRowIntoView(row, scrollEl);
        return;
      }
      if (Date.now() < giveUpAt) {
        revealTimer = window.setTimeout(attempt, 60);
      }
    };
    void tick().then(attempt);
  }

  function scrollRowIntoView(row: HTMLElement, container: HTMLElement): void {
    const delta = scrollDeltaIntoView(row.getBoundingClientRect(), container.getBoundingClientRect());
    if (delta === 0) return;
    container.scrollBy({ top: delta, behavior: motionEnabled() ? "smooth" : "auto" });
  }

  function syncActiveTreeitem(preferredKey = ""): void {
    const next = pickFallbackTreeitem(preferredKey);
    if (!next) return;
    const nextKey = keyFor(next);
    if (nextKey && get(activeKey) !== nextKey) {
      activeKey.set(nextKey);
    }
  }

  setContext<TreeContext>(PATH_PICKER_TREE_CONTEXT, {
    activeKey,
    setActiveKey,
    focusRelative,
    focusBoundary,
    focusFirstChild,
    focusParent,
  });

  $: {
    const s = selectedPath;
    if (s !== lastSelForExpand) {
      lastSelForExpand = s;
      expandEpoch++;
      setActiveKey(treeRowKey(s));
      revealPath(s);
    }
  }

  $: if (roots.length > 0) {
    void tick().then(() => {
      syncActiveTreeitem(selectedPath ? treeRowKey(selectedPath) : "");
    });
  }

  onMount(() => {
    void (async () => {
      try {
        roots = await FilesystemService.ListRoots();
      } catch (e) {
        err = e instanceof Error ? e.message : String(e);
        roots = [];
      }
      // The tree that a path was selected against did not exist until now.
      revealPath(selectedPath);
    })();
  });

  onDestroy(() => {
    revealSeq++;
    if (revealTimer) window.clearTimeout(revealTimer);
  });
</script>

{#if err}
  <div class="pathPicker-err">{err}</div>
{/if}
<div bind:this={scrollEl} class="pathPicker modal-pathPicker pp-svg-tree">
  <div bind:this={treeEl} class="pp-root-list" role="tree" aria-label={$t("Modal_SetUserdata_ChooseFolder")}>
    {#each roots as r (r.path)}
      <PathPickerNode
        path={normalizeDisplayPath(r.path)}
        label={r.label || normalizeDisplayPath(r.path)}
        kind={r.kind}
        link={!driveKinds.has(r.kind)}
        depth={0}
        {selectedPath}
        {dirsOnly}
        {soughtFilename}
        {suggestedFolder}
        {onPick}
        {expandEpoch}
      />
    {/each}
  </div>
</div>

<style lang="scss">
  .pathPicker-err {
    color: var(--red);
    margin: 0.5rem 0;
    font-size: 12px;
  }

  :global(.pathPicker.pp-svg-tree .pp-row span.pp-label::before),
  :global(.pathPicker.pp-svg-tree .pp-row span.pp-label.folder::before),
  :global(.pathPicker.pp-svg-tree .pp-row span.pp-label.head::before) {
    content: none !important;
    display: none !important;
    margin: 0 !important;
  }

  :global(.pathPicker.pp-svg-tree div) {
    padding-left: 0;
    border-left: none;
  }

  :global(.pathPicker.pp-svg-tree .pp-node > .pp-node),
  :global(.pathPicker.pp-svg-tree .pp-node > .pp-node-group > .pp-node) {
    border-left: 1px solid var(--accent-border-deep);
  }
  :global(.pathPicker.pp-svg-tree .pp-root-list > .pp-node) {
    border-left: none !important;
  }
</style>
