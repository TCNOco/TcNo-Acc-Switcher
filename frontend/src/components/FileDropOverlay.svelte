<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { get } from "svelte/store";
  import { Events } from "@wailsio/runtime";
  import { fileDropAcceptor } from "../stores/fileDropTarget";
  import { fileDropInterceptor } from "../stores/fileDropInterceptor";
  import { accountProfileImageDropActive } from "../stores/accountProfileImageDropUi";
  import { shouldUseAccountProfileRowDropCue } from "../lib/profileImageDrop";
  import { t } from "../stores/i18n";

  const FILES_DROPPED = "files-dropped";

  let overlayActive = false;
  let offFilesDropped: (() => void) | undefined;

  $: acc = $fileDropAcceptor;
  $: visible = overlayActive && acc !== null && !$accountProfileImageDropActive;
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

  function dropHandlersActive(): boolean {
    return get(fileDropAcceptor) !== null || get(fileDropInterceptor) !== null;
  }

  function clearAccountProfileDropUi(): void {
    accountProfileImageDropActive.set(false);
    overlayActive = false;
  }

  function onDragEnter(e: DragEvent): void {
    if (!hasFilesType(e) || !dropHandlersActive()) {
      return;
    }
    e.preventDefault();
    const rel = e.relatedTarget as Node | null;
    if (rel === null || !document.documentElement.contains(rel)) {
      const rowCue =
        get(fileDropInterceptor) !== null && shouldUseAccountProfileRowDropCue(e.dataTransfer);
      if (rowCue) {
        accountProfileImageDropActive.set(true);
        overlayActive = false;
      } else {
        accountProfileImageDropActive.set(false);
        overlayActive = true;
      }
    }
  }

  function onDragLeave(e: DragEvent): void {
    if (!hasFilesType(e)) {
      return;
    }
    e.preventDefault();
    const rel = e.relatedTarget as Node | null;
    if (rel === null || !document.documentElement.contains(rel)) {
      clearAccountProfileDropUi();
    }
  }

  function onDragOver(e: DragEvent): void {
    if (!hasFilesType(e) || !dropHandlersActive()) {
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
    clearAccountProfileDropUi();
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
      accountProfileImageDropActive.set(false);
      overlayActive = false;
      if (paths.length === 0) {
        return;
      }
      const intercept = get(fileDropInterceptor);
      if (intercept) {
        try {
          if (await intercept(paths)) {
            return;
          }
        } catch {
          /* interceptor failed — fall through to shortcuts */
        }
      }
      const acceptor = get(fileDropAcceptor);
      if (!acceptor) {
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
