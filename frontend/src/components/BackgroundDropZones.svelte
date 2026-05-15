<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { get } from "svelte/store";
  import { t } from "../stores/i18n";
  import { route } from "../stores/nav";
  import { shouldUseAccountProfileRowDropCue } from "../lib/profileImageDrop";
  import { backgroundZoneInterceptor } from "../stores/backgroundZoneInterceptor";
  import { appBgInfo, platformBgInfo } from "../stores/backgroundImage";
  import { pushToast } from "../stores/toast";
  import { formatToastWithError } from "../lib/formatWailsError";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

  let dragActive = false;

  let hoveredZone: "app" | "platform" | null = null;

  $: showPlatformZone = isPlatformRoute($route);
  $: currentPlatformName = getPlatformName($route);

  function isPlatformRoute(r: typeof $route): boolean {
    return (
      r.page === "platform" ||
      r.page === "platform-settings" ||
      r.page === "steam-advanced-clearing"
    );
  }

  function getPlatformName(r: typeof $route): string {
    if (r.page === "platform" || r.page === "platform-settings") {
      return (r as { page: string; platformName: string }).platformName;
    }
    if (r.page === "steam-advanced-clearing") return "Steam";
    return "";
  }

  function hasFilesType(e: DragEvent): boolean {
    const types = e.dataTransfer?.types;
    if (!types) return false;
    return Array.from(types as unknown as Iterable<string>).includes("Files");
  }

  function startDrag(): void {
    dragActive = true;
    backgroundZoneInterceptor.set(async (paths: string[]) => {
      const zone = hoveredZone;
      const platform = currentPlatformName;
      endDrag();
      if (!zone || paths.length === 0) return false;
      try {
        if (zone === "app") {
          await PlatformService.SetAppBackground(paths[0]);
          const appInfo = await PlatformService.GetAppBackground();
          if (appInfo.imageUrl) appInfo.imageUrl += `?t=${Date.now()}`;
          appBgInfo.set(appInfo); // appInfo.themeBgOverride is true — backend auto-sets it
          pushToast({ type: "success", message: get(t)("Toast_AppBgSet"), duration: 4000 });
        } else if (zone === "platform" && platform) {
          await PlatformService.SetPlatformBackground(platform, paths[0]);
          const platInfo = await PlatformService.GetPlatformBackground(platform);
          if (platInfo.imageUrl) platInfo.imageUrl += `?t=${Date.now()}`;
          platformBgInfo.set(platInfo);
          pushToast({
            type: "success",
            message: get(t)("Toast_PlatformBgSet", { platform }),
            duration: 4000,
          });
        }
      } catch (err) {
        pushToast({
          type: "error",
          message: formatToastWithError(get(t)("Toast_SaveFailed"), err),
          duration: 8000,
        });
      }
      return true;
    });
  }

  function endDrag(): void {
    dragActive = false;
    hoveredZone = null;
    backgroundZoneInterceptor.set(null);
  }

  function onDragEnter(e: DragEvent): void {
    if (!hasFilesType(e)) return;
    const rel = e.relatedTarget as Node | null;
    if (rel !== null && document.documentElement.contains(rel)) return;
    if (!dragActive && shouldUseAccountProfileRowDropCue(e.dataTransfer)) {
      e.preventDefault(); // signal to OS that this window accepts this drop
      startDrag();
    }
  }

  function onDragLeave(e: DragEvent): void {
    if (!hasFilesType(e)) return;
    const rel = e.relatedTarget as Node | null;
    if (rel === null || !document.documentElement.contains(rel)) {
      endDrag();
    }
  }

  function onDragOver(e: DragEvent): void {
    if (!hasFilesType(e)) return;
    if (dragActive) {
      e.preventDefault();
    }
  }

  function onGlobalDrop(): void {
    dragActive = false;
  }

  function onZoneDragEnter(zone: "app" | "platform"): void {
    hoveredZone = zone;
  }

  function onZoneDragLeave(zone: "app" | "platform", e: DragEvent): void {
    const target = e.currentTarget as Element;
    const rel = e.relatedTarget as Node | null;
    if (rel && target.contains(rel)) return;
    if (hoveredZone === zone) hoveredZone = null;
  }

  onMount(() => {
    document.documentElement.addEventListener("dragenter", onDragEnter, true);
    document.documentElement.addEventListener("dragleave", onDragLeave, true);
    document.documentElement.addEventListener("dragover", onDragOver, true);
    document.documentElement.addEventListener("drop", onGlobalDrop, true);
  });

  onDestroy(() => {
    document.documentElement.removeEventListener("dragenter", onDragEnter, true);
    document.documentElement.removeEventListener("dragleave", onDragLeave, true);
    document.documentElement.removeEventListener("dragover", onDragOver, true);
    document.documentElement.removeEventListener("drop", onGlobalDrop, true);
    endDrag();
  });
</script>

{#if dragActive}
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="bg-drop-zones" aria-hidden="true">
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div
      class="bg-drop-zone"
      class:bg-drop-zone--hovered={hoveredZone === "app"}
      on:dragenter={() => onZoneDragEnter("app")}
      on:dragleave={(e) => onZoneDragLeave("app", e)}
      on:dragover={(e) => e.preventDefault()}
    >
      <span class="bg-drop-zone__icon" aria-hidden="true">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true">
          <path fill="currentColor" d="M5 20h14v-2H5v2zM19 9h-4V3H9v6H5l7 7 7-7z" />
        </svg>
      </span>
      <span class="bg-drop-zone__label">{$t("DropZone_SetAppBg")}</span>
    </div>

    {#if showPlatformZone && currentPlatformName}
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <div
        class="bg-drop-zone"
        class:bg-drop-zone--hovered={hoveredZone === "platform"}
        on:dragenter={() => onZoneDragEnter("platform")}
        on:dragleave={(e) => onZoneDragLeave("platform", e)}
        on:dragover={(e) => e.preventDefault()}
      >
        <span class="bg-drop-zone__icon" aria-hidden="true">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true">
            <path fill="currentColor" d="M5 20h14v-2H5v2zM19 9h-4V3H9v6H5l7 7 7-7z" />
          </svg>
        </span>
        <span class="bg-drop-zone__label"
          >{$t("DropZone_SetPlatformBg", { platform: currentPlatformName })}</span
        >
      </div>
    {/if}
  </div>
{/if}
