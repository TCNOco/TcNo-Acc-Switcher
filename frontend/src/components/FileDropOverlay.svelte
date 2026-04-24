<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { get } from "svelte/store";
  import { Events } from "@wailsio/runtime";
  import { fileDropAcceptor } from "../stores/fileDropTarget";
  import { t } from "../stores/i18n";

  const FILES_DROPPED = "files-dropped";

  let overlayActive = false;
  let offFilesDropped: (() => void) | undefined;

  $: acc = $fileDropAcceptor;
  $: visible = overlayActive && acc !== null;
  $: syncDropTargetAttribute(acc !== null);

  function syncDropTargetAttribute(enabled: boolean): void {
    if (typeof document === "undefined") {
      return;
    }
    if (enabled) {
      document.documentElement.setAttribute("data-file-drop-target", "true");
    } else {
      document.documentElement.removeAttribute("data-file-drop-target");
    }
  }

  function hasFilesType(e: DragEvent): boolean {
    const types = e.dataTransfer?.types;
    if (!types) {
      return false;
    }
    return Array.from(types as unknown as Iterable<string>).includes("Files");
  }

  function onDragEnter(e: DragEvent): void {
    if (!hasFilesType(e) || !get(fileDropAcceptor)) {
      return;
    }
    e.preventDefault();
    const rel = e.relatedTarget as Node | null;
    if (rel === null || !document.documentElement.contains(rel)) {
      overlayActive = true;
    }
  }

  function onDragLeave(e: DragEvent): void {
    if (!hasFilesType(e)) {
      return;
    }
    e.preventDefault();
    const rel = e.relatedTarget as Node | null;
    if (rel === null || !document.documentElement.contains(rel)) {
      overlayActive = false;
    }
  }

  function onDragOver(e: DragEvent): void {
    if (!hasFilesType(e) || !get(fileDropAcceptor)) {
      return;
    }
    e.preventDefault();
    if (e.dataTransfer) {
      e.dataTransfer.dropEffect = "copy";
    }
  }

  function onDrop(e: DragEvent): void {
    if (hasFilesType(e)) {
      e.preventDefault();
    }
    overlayActive = false;
  }

  function normalizePaths(data: unknown): string[] {
    if (!Array.isArray(data)) {
      return [];
    }
    return data.filter((x): x is string => typeof x === "string" && x.trim() !== "");
  }

  onMount(() => {
    document.documentElement.addEventListener("dragenter", onDragEnter, true);
    document.documentElement.addEventListener("dragleave", onDragLeave, true);
    document.documentElement.addEventListener("dragover", onDragOver, true);
    document.documentElement.addEventListener("drop", onDrop, true);

    offFilesDropped = Events.On(FILES_DROPPED, async (ev) => {
      const paths = normalizePaths(ev.data);
      overlayActive = false;
      const acceptor = get(fileDropAcceptor);
      if (!acceptor || paths.length === 0) {
        return;
      }
      await acceptor.handle(paths);
    });
  });

  onDestroy(() => {
    document.documentElement.removeEventListener("dragenter", onDragEnter, true);
    document.documentElement.removeEventListener("dragleave", onDragLeave, true);
    document.documentElement.removeEventListener("dragover", onDragOver, true);
    document.documentElement.removeEventListener("drop", onDrop, true);
    syncDropTargetAttribute(false);
    offFilesDropped?.();
  });
</script>

{#if visible && acc}
  <div class="fileDropOverlay" aria-hidden="true">
    <div class="fileDropOverlay__inner">
      <div class="fileDropOverlay__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"
          ><path
            fill="currentColor"
            d="M5 20h14v-2H5v2zM19 9h-4V3H9v6H5l7 7 7-7z"
          /></svg
        >
      </div>
      <p class="fileDropOverlay__text">{$t(acc.labelKey)}</p>
    </div>
  </div>
{/if}

<style lang="scss">
  .fileDropOverlay {
    position: fixed;
    inset: 0;
    z-index: 9999;
    box-sizing: border-box;
    margin: 0;
    padding: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(30, 100, 200, 0.22);
    backdrop-filter: blur(2px);
    pointer-events: none;
    animation: fileDropOverlayIn 0.18s ease-out;
  }

  @keyframes fileDropOverlayIn {
    from {
      opacity: 0;
      transform: scale(0.985);
    }
    to {
      opacity: 1;
      transform: scale(1);
    }
  }

  .fileDropOverlay__inner {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1rem;
    padding: 2rem 2.5rem;
    max-width: min(420px, 90vw);
    border: 2px dashed rgba(120, 190, 255, 0.65);
    border-radius: 12px;
    background: rgba(10, 20, 40, 0.45);
    box-shadow: 0 8px 40px rgba(0, 0, 0, 0.35);
  }

  .fileDropOverlay__icon {
    color: rgba(160, 210, 255, 0.95);
    width: 4.5rem;
    height: 4.5rem;
    display: flex;
    align-items: center;
    justify-content: center;

    svg {
      width: 100%;
      height: 100%;
      display: block;
    }
  }

  .fileDropOverlay__text {
    margin: 0;
    text-align: center;
    font-size: 1.15rem;
    font-weight: 600;
    line-height: 1.35;
    color: var(--white, #f8f8f2);
    text-shadow: 0 1px 2px rgba(0, 0, 0, 0.5);
  }
</style>
