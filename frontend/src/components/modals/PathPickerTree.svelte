<script lang="ts">
  import { onMount, setContext, tick } from "svelte";
  import { get, writable, type Writable } from "svelte/store";
  import * as FilesystemService from "../../../bindings/TcNo-Acc-Switcher/filesystemservice";
  import PathPickerNode from "./PathPickerNode.svelte";
  import { normalizeDisplayPath, sameFsPath } from "../../lib/fsPaths";
  import { t } from "../../stores/i18n";

  const PATH_PICKER_TREE_CONTEXT = "path-picker-tree";

  type TreeContext = {
    activePath: Writable<string>;
    setActivePath: (path: string) => void;
    focusRelative: (currentPath: string, offset: number) => Promise<boolean>;
    focusBoundary: (position: "first" | "last") => Promise<boolean>;
    focusFirstChild: (parentPath: string) => Promise<boolean>;
    focusParent: (parentPath: string | null | undefined) => Promise<boolean>;
  };

  export let selectedPath: string;
  export let dirsOnly = true;
  export let soughtFilename = "";
  export let onPick: (p: string, entryIsDir: boolean) => void;

  let roots: string[] = [];
  let err: string | null = null;
  let treeEl: HTMLDivElement | undefined;

  let expandEpoch = 0;
  let lastSelForExpand = "\u0000sentinel";
  const activePath = writable("");

  function visibleTreeitems(): HTMLDivElement[] {
    if (!treeEl) return [];
    return Array.from(treeEl.querySelectorAll<HTMLDivElement>('[role="treeitem"]'));
  }

  function pathFor(el: HTMLElement | null | undefined): string {
    return el?.dataset.path ?? "";
  }

  function findVisibleTreeitem(path: string): HTMLDivElement | undefined {
    return visibleTreeitems().find((item) => sameFsPath(pathFor(item), path));
  }

  function pickFallbackTreeitem(preferredPath = ""): HTMLDivElement | undefined {
    const items = visibleTreeitems();
    if (items.length === 0) return undefined;
    return (
      (preferredPath ? findVisibleTreeitem(preferredPath) : undefined) ??
      findVisibleTreeitem(get(activePath)) ??
      items[0]
    );
  }

  function setActivePath(path: string): void {
    if (!path) return;
    activePath.set(normalizeDisplayPath(path));
  }

  async function focusElement(el: HTMLDivElement | undefined): Promise<boolean> {
    if (!el) return false;
    const nextPath = pathFor(el);
    if (!nextPath) return false;
    setActivePath(nextPath);
    await tick();
    el.focus();
    return true;
  }

  async function focusRelative(currentPath: string, offset: number): Promise<boolean> {
    await tick();
    const items = visibleTreeitems();
    if (items.length === 0) return false;
    const currentIndex = items.findIndex((item) => sameFsPath(pathFor(item), currentPath));
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

  async function focusFirstChild(parentPath: string): Promise<boolean> {
    await tick();
    const parent = findVisibleTreeitem(parentPath);
    if (!parent) return false;
    const parentLevel = Number(parent.getAttribute("aria-level") ?? "0");
    const items = visibleTreeitems();
    const parentIndex = items.findIndex((item) => sameFsPath(pathFor(item), parentPath));
    if (parentIndex < 0) return false;
    const child = items.slice(parentIndex + 1).find((item) => {
      const itemLevel = Number(item.getAttribute("aria-level") ?? "0");
      return itemLevel === parentLevel + 1 && sameFsPath(item.dataset.parentPath ?? "", parentPath);
    });
    return focusElement(child);
  }

  async function focusParent(parentPath: string | null | undefined): Promise<boolean> {
    if (!parentPath) return false;
    await tick();
    return focusElement(findVisibleTreeitem(parentPath));
  }

  function syncActiveTreeitem(preferredPath = ""): void {
    const next = pickFallbackTreeitem(preferredPath);
    if (!next) return;
    const nextPath = pathFor(next);
    if (nextPath && !sameFsPath(get(activePath), nextPath)) {
      activePath.set(nextPath);
    }
  }

  setContext<TreeContext>(PATH_PICKER_TREE_CONTEXT, {
    activePath,
    setActivePath,
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
      setActivePath(s);
    }
  }

  $: if (roots.length > 0) {
    void tick().then(() => {
      syncActiveTreeitem(selectedPath);
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
    })();
  });
</script>

{#if err}
  <div class="pathPicker-err">{err}</div>
{/if}
<div class="pathPicker modal-pathPicker pp-svg-tree">
  <div bind:this={treeEl} class="pp-root-list" role="tree" aria-label={$t("Modal_SetUserdata_ChooseFolder")}>
    {#each roots as r (r)}
      <PathPickerNode
        path={normalizeDisplayPath(r)}
        label={normalizeDisplayPath(r)}
        depth={0}
        {selectedPath}
        {dirsOnly}
        {soughtFilename}
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
