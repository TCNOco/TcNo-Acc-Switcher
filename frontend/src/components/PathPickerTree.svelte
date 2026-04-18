<script lang="ts">
  import { onMount } from "svelte";
  import * as FilesystemService from "../../bindings/changeme/filesystemservice.js";
  import PathPickerNode from "./PathPickerNode.svelte";
  import { normalizeDisplayPath } from "../lib/fsPaths";

  export let selectedPath: string;
  export let dirsOnly = true;
  export let soughtFilename = "";
  export let onPick: (p: string, entryIsDir: boolean) => void;

  let roots: string[] = [];
  let err: string | null = null;

  let expandEpoch = 0;
  let lastSelForExpand = "\u0000sentinel";

  $: {
    const s = selectedPath;
    if (s !== lastSelForExpand) {
      lastSelForExpand = s;
      expandEpoch++;
    }
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
  <div class="pp-root-list">
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
    color: #ff8a8a;
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

  :global(.pathPicker.pp-svg-tree .pp-node > .pp-node) {
    border-left: 1px solid #00252c;
  }
  :global(.pathPicker.pp-svg-tree .pp-root-list > .pp-node) {
    border-left: none !important;
  }
</style>
