<script lang="ts">
  import { onMount } from "svelte";
  import { get } from "svelte/store";
  import { Events } from "@wailsio/runtime";
  import TitleBar from './components/TitleBar.svelte'
  import UpdateBar from './components/UpdateBar.svelte'
  import AppModal from './components/AppModal.svelte'
  import Toast from './components/Toast.svelte'
  import FileDropOverlay from './components/FileDropOverlay.svelte'
  import ContextMenu from './components/ContextMenu.svelte'
  import BackgroundDropZones from './components/BackgroundDropZones.svelte'

  import Home from './pages/Home.svelte'
  import Settings from './pages/Settings.svelte'
  import PreviewCss from './pages/PreviewCss.svelte'
  import Platform from './pages/Platform.svelte'
  import PlatformSteam from './pages/PlatformSteam.svelte'
  import PlatformSettings from './pages/PlatformSettings.svelte'
  import SteamAdvancedClearing from './pages/SteamAdvancedClearing.svelte'
  import ManagePlatforms from './pages/ManagePlatforms.svelte'
  import { route, applyNavigateJSON, navigateBackLikeButton, navigateForward } from './stores/nav'
  import { installPageStatsTracking } from "./lib/pageStatsTrack";
  import { actionBarStatus } from './stores/actionBarStatus'
  import { t } from "./stores/i18n";
  import { NotifyLaunchUpdateCheck } from "./lib/platformBindings";
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
  import { appBgInfo, platformBgInfo } from "./stores/backgroundImage";
  import type { AppBackgroundInfo } from "./stores/backgroundImage";
  import * as PlatformService from "../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

  let pageEl: HTMLDivElement;

  function resolveActiveBg(r: typeof $route, app: AppBackgroundInfo, plat: AppBackgroundInfo): AppBackgroundInfo | null {
    const isPlatformPage =
      r.page === "platform" ||
      r.page === "platform-settings" ||
      r.page === "steam-advanced-clearing";
    if (isPlatformPage && plat.hasImage) return plat;
    if (app.hasImage) return app;
    return null;
  }

  $: activeBg = resolveActiveBg($route, $appBgInfo, $platformBgInfo);

  $: if (pageEl) {
    if (activeBg) {
      pageEl.style.setProperty("--main-bg-image", `url(${JSON.stringify(activeBg.imageUrl)})`);
      pageEl.style.setProperty("--main-bg-opacity", String(activeBg.opacity));
      pageEl.style.setProperty("--main-bg-blur", `${activeBg.blur}px`);
    } else {
      pageEl.style.removeProperty("--main-bg-image");
      pageEl.style.removeProperty("--main-bg-opacity");
      pageEl.style.removeProperty("--main-bg-blur");
    }
  }

  /** Load/reload the platform background for the given platform name. */
  async function loadPlatformBg(platformName: string): Promise<void> {
    try {
      const info = await PlatformService.GetPlatformBackground(platformName);
      platformBgInfo.set(info);
    } catch {
      platformBgInfo.set({ hasImage: false, imageUrl: "", opacity: 0.6, blur: 4.0 });
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
      platformBgInfo.set({ hasImage: false, imageUrl: "", opacity: 0.6, blur: 4.0 });
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

    const offUpdateFail = Events.On("update-check-failed", () => {
      pushToast({
        type: "error",
        title: "",
        message: get(t)("Toast_UpdateCheckFail"),
        duration: 15000,
      });
    });

    const off = Events.On("action-bar-status", (ev) => {
      const raw = typeof ev.data === "string" ? ev.data : "";
      if (raw.startsWith("i18n:")) {
        const payload = raw.slice(5);
        const sep = "\u001f";
        const parts = payload.split(sep);
        const key = parts.shift() ?? "";
        if (parts.length > 1) {
          const vars: Record<string, string | number> = {};
          for (let i = 0; i < parts.length; i += 2) {
            const name = parts[i];
            if (!name) continue;
            vars[name] = parts[i + 1] ?? "";
          }
          actionBarStatus.set($t(key, vars));
        } else if (parts.length === 1) {
          actionBarStatus.set($t(key, { platform: parts[0] }));
        } else {
          actionBarStatus.set($t(key));
        }
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
      offSvgBridge?.();
    };
  });
</script>

<div class="container" class:busyCursor={$platformActionBusy.busy}>
  <FileDropOverlay />
  <ContextMenu />
  <TitleBar />
  <UpdateBar />
  <div class="page" bind:this={pageEl}>
    {#if $route.page === 'home'}
      <Home />
    {:else if $route.page === 'settings'}
      <Settings />
    {:else if $route.page === 'preview-css'}
      <PreviewCss />
    {:else if $route.page === 'platform'}
      {#if $route.platformName === 'Steam'}
        <PlatformSteam name={$route.platformName} />
      {:else}
        <Platform name={$route.platformName} />
      {/if}
    {:else if $route.page === 'platform-settings'}
      <PlatformSettings name={$route.platformName} />
    {:else if $route.page === 'steam-advanced-clearing'}
      <SteamAdvancedClearing />
    {:else if $route.page === 'manage-platforms'}
      <ManagePlatforms />
    {/if}
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

    /* Background image — rendered behind all page content.
       Negative inset hides blur edges; overflow:hidden on .page clips them. */
    &::before {
      content: '';
      position: absolute;
      inset: -24px;
      z-index: -1;
      background-image: var(--main-bg-image, none);
      background-size: cover;
      background-position: center;
      background-repeat: no-repeat;
      opacity: var(--main-bg-opacity, 0);
      filter: blur(var(--main-bg-blur, 0px));
      pointer-events: none;
      will-change: opacity, filter;
    }
  }
</style>
