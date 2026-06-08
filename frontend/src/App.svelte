<script lang="ts">
  import { onMount } from "svelte";
  import { get } from "svelte/store";
  import { fade } from "svelte/transition";
  import { cubicOut } from "svelte/easing";
  import { Events } from "@wailsio/runtime";
  import { motionEnabled } from "./lib/animation";
  import { animationsEnabled, loadAnimationsEnabled } from "./stores/animationSettings";
  import TitleBar from './components/TitleBar.svelte'
  import UpdateBar from './components/UpdateBar.svelte'
  import AppModal from './components/AppModal.svelte'
  import Toast from './components/Toast.svelte'
  import FileDropOverlay from './components/FileDropOverlay.svelte'
  import ContextMenu from './components/ContextMenu.svelte'
  import BackgroundDropZones from './components/BackgroundDropZones.svelte'
  import ActionBar from './components/ActionBar.svelte'
  import { route, applyNavigateJSON, navigateBackLikeButton, navigateForward } from './stores/nav'
  import { installPageStatsTracking } from "./lib/pageStatsTrack";
  import { loadPageModule, prefetchCommonPages } from "./lib/pageLoaders";
  import { actionBarStatus } from './stores/fileDrop'
  import { t } from "./stores/i18n";
  import { NotifyLaunchUpdateCheck } from "../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { pushToast } from "./stores/toast";
  import { registerSvgRenderBridge } from "./lib/svgRenderBridge";
  import { activeModal } from "./stores/modal";
  import { contextMenu } from "./stores/contextMenu";
  import {
    openSearchOverlay,
    searchOverlayCtrl,
    searchOverlayPendingAppend,
  } from "./stores/searchOverlay";
  import { platformActionBusy } from "./stores/platformPage";
  import { appBgInfo, platformBgInfo, userOverriddenAppBg } from "./stores/backgroundImage";
  import type { AppBackgroundInfo } from "./stores/backgroundImage";
  import { currentThemeBgUrl } from "./lib/themes";
  import * as PlatformService from "../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

  function resolveActiveBg(
    r: typeof $route,
    app: AppBackgroundInfo,
    plat: AppBackgroundInfo,
    themeBgUrl: string,
    userOverridden: boolean
  ): AppBackgroundInfo | null {
    const isPlatformPage =
      r.page === "platform" ||
      r.page === "platform-settings" ||
      r.page === "steam-advanced-clearing";
    if (isPlatformPage && plat.hasImage) return plat;
    if (app.hasImage) return app;
    if (!userOverridden && themeBgUrl) {
      return { hasImage: true, imageUrl: themeBgUrl, opacity: 1.0, blur: 0, themeBgOverride: false };
    }
    return null;
  }

  $: activeBg = resolveActiveBg($route, $appBgInfo, $platformBgInfo, $currentThemeBgUrl, $userOverriddenAppBg);
  $: showActionBar = $route.page === "home" || $route.page === "platform";

  /** Load/reload the platform background for the given platform name. */
  async function loadPlatformBg(platformName: string): Promise<void> {
    try {
      const info = await PlatformService.GetPlatformBackground(platformName);
      platformBgInfo.set(info);
    } catch {
      platformBgInfo.set({ hasImage: false, imageUrl: "", opacity: 0.6, blur: 4.0, themeBgOverride: false });
    }
  }

  // When the route changes to a platform page, reload the platform background.
  $: {
    const r = $route;
    if (r.page === "platform" || r.page === "platform-settings") {
      void loadPlatformBg(r.platformName);
    } else if (r.page === "steam-advanced-clearing") {
      void loadPlatformBg("Steam");
    } else {
      platformBgInfo.set({ hasImage: false, imageUrl: "", opacity: 0.6, blur: 4.0, themeBgOverride: false });
    }
  }

  function isEditableTarget(t: EventTarget | null): boolean {
    if (!t || !(t instanceof HTMLElement)) {
      return false;
    }
    if (t.isContentEditable) {
      return true;
    }
    const tag = t.tagName;
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") {
      return true;
    }
    return t.closest("input, textarea, select, [contenteditable]") !== null;
  }

  function onGlobalKeydownCapture(e: KeyboardEvent): void {
    const r = get(route);
    if (r.page !== "home" && r.page !== "platform") {
      return;
    }
    if (get(activeModal)) {
      return;
    }
    if (get(contextMenu)) {
      return;
    }
    if (e.ctrlKey || e.metaKey || e.altKey) {
      return;
    }
    if (e.key.length !== 1) {
      return;
    }
    const so = get(searchOverlayCtrl);
    if (so.open) {
      if (isEditableTarget(e.target)) {
        return;
      }
      e.preventDefault();
      searchOverlayPendingAppend.set(e.key);
      return;
    }
    if (isEditableTarget(e.target)) {
      return;
    }
    e.preventDefault();
    openSearchOverlay(e.key);
  }

  function canHandleGlobalNavInput(target: EventTarget | null): boolean {
    if (get(activeModal) || get(contextMenu)) {
      return false;
    }
    return !isEditableTarget(target);
  }

  function onGlobalHistoryKeydownCapture(e: KeyboardEvent): void {
    if (!canHandleGlobalNavInput(e.target)) {
      return;
    }
    const key = e.key;
    const isBack = key === "BrowserBack" || (e.altKey && key === "ArrowLeft");
    const isForward = key === "BrowserForward" || (e.altKey && key === "ArrowRight");
    if (!isBack && !isForward) {
      return;
    }
    e.preventDefault();
    if (isBack) {
      navigateBackLikeButton();
      return;
    }
    navigateForward();
  }

  function onGlobalHistoryMouseUpCapture(e: MouseEvent): void {
    if (!canHandleGlobalNavInput(e.target)) {
      return;
    }
    if (e.button !== 3 && e.button !== 4) {
      return;
    }
    e.preventDefault();
    if (e.button === 3) {
      navigateBackLikeButton();
      return;
    }
    navigateForward();
  }

  onMount(() => {
    void loadAnimationsEnabled();
    // Load initial app background state.
    void PlatformService.GetAppBackground().then((info) => {
      appBgInfo.set(info);
    }).catch(() => {});

    const offPageStats = installPageStatsTracking();
    const offSvgBridge = registerSvgRenderBridge();
    const offNav = Events.On("navigate", (ev) => {
      const raw = typeof ev.data === "string" ? ev.data : "";
      applyNavigateJSON(raw);
    });
    void NotifyLaunchUpdateCheck();

    const schedulePrefetch = (): void => {
      prefetchCommonPages();
    };
    if (typeof requestIdleCallback === "function") {
      requestIdleCallback(schedulePrefetch);
    } else {
      setTimeout(schedulePrefetch, 1500);
    }

    const offUpdateFail = Events.On("update-check-failed", () => {
      pushToast({
        type: "error",
        title: "",
        message: get(t)("Toast_UpdateCheckFail"),
        duration: 15000,
      });
    });

    const offPlatformsFound = Events.On("platforms-json-update-found", (ev) => {
      const version = typeof ev.data === "object" && ev.data && "version" in ev.data
        ? String((ev.data as { version?: string }).version ?? "")
        : "";
      pushToast({
        type: "info",
        title: "",
        message: get(t)("Toast_PlatformsJsonUpdateFound", { version }),
        duration: 8000,
      });
    });

    const offPlatformsUpdated = Events.On("platforms-json-updated", (ev) => {
      const version = typeof ev.data === "object" && ev.data && "version" in ev.data
        ? String((ev.data as { version?: string }).version ?? "")
        : "";
      pushToast({
        type: "success",
        title: "",
        message: get(t)("Toast_PlatformsJsonUpdated", { version }),
        duration: 8000,
      });
    });

    function parseI18nPayload(raw: string): { key: string; vars?: Record<string, string | number> } {
      const parts = raw.slice(5).split("\u001f");
      const key = parts.shift() ?? "";
      if (parts.length > 1) {
        const vars: Record<string, string | number> = {};
        for (let i = 0; i < parts.length; i += 2) {
          const name = parts[i];
          if (!name) continue;
          vars[name] = parts[i + 1] ?? "";
        }
        return { key, vars };
      }
      if (parts.length === 1) return { key, vars: { platform: parts[0] } };
      return { key };
    }

    const off = Events.On("action-bar-status", (ev) => {
      const raw = typeof ev.data === "string" ? ev.data : "";
      if (raw.startsWith("i18n:")) {
        const { key, vars } = parseI18nPayload(raw);
        actionBarStatus.set(vars ? $t(key, vars) : $t(key));
      } else {
        actionBarStatus.set(raw);
      }
    });


    window.addEventListener("keydown", onGlobalKeydownCapture, true);
    window.addEventListener("keydown", onGlobalHistoryKeydownCapture, true);
    window.addEventListener("mouseup", onGlobalHistoryMouseUpCapture, true);
    return () => {
      window.removeEventListener("keydown", onGlobalKeydownCapture, true);
      window.removeEventListener("keydown", onGlobalHistoryKeydownCapture, true);
      window.removeEventListener("mouseup", onGlobalHistoryMouseUpCapture, true);
      offPageStats();
      off?.();
      offNav?.();
      offUpdateFail?.();
      offPlatformsFound?.();
      offPlatformsUpdated?.();
      offSvgBridge?.();
    };
  });
</script>

<div class="container" class:busyCursor={$platformActionBusy.busy} class:animations-disabled={!$animationsEnabled}>
  <FileDropOverlay />
  <ContextMenu />
  <TitleBar />
  <UpdateBar />
  <div class="page">
    {#key activeBg?.imageUrl}
      {#if activeBg}
        <div
          class="bg-layer"
          in:fade={{ duration: motionEnabled() ? 350 : 0, easing: cubicOut }}
          out:fade={{ duration: motionEnabled() ? 250 : 0, easing: cubicOut }}
          style="background-image: url({JSON.stringify(activeBg.imageUrl)}); opacity: {activeBg.opacity}; filter: blur({activeBg.blur}px);"
        ></div>
      {/if}
    {/key}
    <div class="page-content-wrapper">
      {#key $route.page + ("platformName" in $route ? $route.platformName : "")}
        <div class="page-content">
          {#await loadPageModule($route) then { default: Page }}
            {#if $route.page === "home"}
              <Page />
            {:else if $route.page === "settings"}
              <Page />
            {:else if $route.page === "preview-css"}
              <Page />
            {:else if $route.page === "platform"}
              <Page name={$route.platformName} />
            {:else if $route.page === "platform-settings"}
              <Page name={$route.platformName} />
            {:else if $route.page === "steam-advanced-clearing"}
              <Page />
            {:else if $route.page === "manage-platforms"}
              <Page />
            {/if}
          {/await}
        </div>
      {/key}
      {#if showActionBar}
        <ActionBar />
      {/if}
    </div>
    <BackgroundDropZones />
    <AppModal />
    <Toast />
  </div>
</div>

<style>
  .container {
    background: var(--program-bg);
    height: 100vh;
    width: 100vw;
    display: flex;
    flex-direction: column;
  }
  .container.busyCursor,
  .container.busyCursor * {
    cursor: progress !important;
  }
  .page {
    position: relative;
    isolation: isolate;
    border-left: var(--border-bar-size) solid var(--border-bar-bg);
    border-right: var(--border-bar-size) solid var(--border-bar-bg);
    border-bottom: var(--border-bar-size) solid var(--border-bar-bg);
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }
  .bg-layer {
    position: absolute;
    inset: -24px;
    z-index: -1;
    background-size: cover;
    background-position: center;
    background-repeat: no-repeat;
    pointer-events: none;
    will-change: opacity;
  }
  .page-content-wrapper {
    position: relative;
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .page-content {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
</style>
