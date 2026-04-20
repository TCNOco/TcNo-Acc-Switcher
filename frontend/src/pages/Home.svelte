<script lang="ts">
  import { onMount } from "svelte";
  import ActionBar from "../components/ActionBar.svelte";
  import ReorderPointerGrid from "../components/ReorderPointerGrid.svelte";
  import { route, appBarTitle } from "../stores/nav";
  import { t } from "../stores/i18n";
  import { openConfirm, openFolderPicker } from "../stores/modal";
  import { pushToast } from "../stores/toast";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import type { PlatformStartup } from "../../bindings/TcNo-Acc-Switcher/internal/platform/models.js";
  import { platformIconFgHref } from "../lib/platformIcon";
  import "../styles/HomePlatforms.scss";

  let startup: PlatformStartup | null = null;
  let homeOrder: string[] = [];
  let loadError: string | null = null;

  let navigating = false;

  $: appBarTitle.set("TcNo Account Switcher");

  function textClass(name: string): string {
    const n = name.length;
    if (n < 7) return "shortText";
    if (n > 12) return "longText";
    return "";
  }

  /** Slot props are typed loosely; keys are always strings here. */
  function slotKey(x: string | null | undefined): string {
    return x ?? "";
  }

  async function promptMissingPlatformsFile(): Promise<void> {
    const locate = await openConfirm({
      title: $t("Modal_Title_MissingPlatformsJson"),
      body: `<p>${$t("Modal_Body_MissingPlatformsJson")}</p>`,
      style: "yesno",
      positiveLabel: $t("Button_LocatePlatformsJson"),
      negativeLabel: $t("Button_RestoreBundledPlatforms"),
    });
    if (locate) {
      const path = await PlatformService.PickPlatformsJSON();
      if (path) await PlatformService.ApplyPlatformsJSONFile(path);
    } else {
      await PlatformService.RestoreDefaultPlatformsJSON();
    }
  }

  async function refreshStartup(skipMissingPrompt = false): Promise<void> {
    loadError = null;
    try {
      const s = await PlatformService.GetStartup();
      if (s.platformsFileMissing && !skipMissingPrompt) {
        await promptMissingPlatformsFile();
        await refreshStartup(true);
        return;
      }
      startup = s;
      homeOrder = s.homePlatformOrder ?? [];
      if (!s.platformsFileMissing && s.allPlatformNames?.length && homeOrder.length === 0) {
        route.set({ page: "manage-platforms" });
      }
    } catch (e) {
      loadError = e instanceof Error ? e.message : String(e);
    }
  }

  async function openPlatform(name: string): Promise<void> {
    if (navigating) return;
    navigating = true;
    try {
      const r = await PlatformService.ResolvePlatformLaunch(name);
      if (r.ok) {
        if (r.foundViaShortcut) {
          pushToast({
            type: "success",
            title: "",
            message: $t("Toast_FoundExeViaShortcut"),
            duration: 30000,
          });
        }
        route.set({ page: "platform", platformName: name });
        return;
      }
      if (r.needsManualLocate) {
        const picked = await openFolderPicker({
          title: $t("Modal_Title_LocatePlatform", { platform: name }),
          body: `<p>${$t("Modal_LocatePlatform", { platformExe: r.soughtExeName })}</p>`,
          initialPath: r.initialPath ?? "",
          dirsOnly: false,
          soughtFilename: r.soughtExeName,
          positiveLabel: $t("Modal_Button_Select"),
        });
        if (picked) {
          await PlatformService.ConfirmPlatformExePath(name, picked);
          route.set({ page: "platform", platformName: name });
        }
      }
    } catch (e) {
      loadError = e instanceof Error ? e.message : String(e);
    } finally {
      navigating = false;
    }
  }

  function onReorder(
    e: CustomEvent<{ items: string[] }>,
  ): void {
    homeOrder = e.detail.items;
    void PlatformService.SaveHomeOrder(e.detail.items).catch(() => {});
  }

  onMount(() => {
    void refreshStartup();
  });
</script>

<div class="main-content home-root">
  {#if loadError}
    <p class="home-msg">{loadError}</p>
  {:else if !startup}
    <p class="home-msg">…</p>
  {:else if startup.platformsFileMissing}
    <p class="home-msg">{$t("Modal_Body_MissingPlatformsJson")}</p>
  {:else}
    <div class="platformTable">
      <ReorderPointerGrid
        items={homeOrder}
        listClass="platform_list"
        itemClass="platform_list_item platform_list_item--draggable"
        placeholderClass="platform_list_item platform_list_placeholder"
        ghostClass="platform_list_item platform_list_item--ghost"
        ariaLabel={$t("Preview_Platforms")}
        on:reorder={onReorder}
        on:itemclick={(e) => void openPlatform(e.detail.id)}
      >
        <svelte:fragment slot="item" let:rowId>
          {@const rid = slotKey(rowId)}
          <div class="fgText {textClass(rid)}">
            <p>{rid.toUpperCase()}</p>
          </div>
          <div class="fgImg" aria-hidden="true">
            <svg viewBox="0 0 500 500" aria-hidden="true">
              <use href={platformIconFgHref(rid)} class="icoFG" />
            </svg>
          </div>
          <svg viewBox="0 0 2084 2084" class="icoBG" aria-hidden="true">
            <use href="img/platform/glass.svg#GLASS" class="icoGlass"></use>
          </svg>
        </svelte:fragment>
      </ReorderPointerGrid>
    </div>
  {/if}
</div>
<ActionBar />

<style lang="scss">
  .home-root {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .home-msg {
    padding: 1rem;
    color: var(--white);
  }

  :global(.platform_list_item--draggable) {
    cursor: grab;
    touch-action: none;
  }

  :global(.platform_list_item--ghost) {
    position: fixed;
    margin: 0;
    z-index: 10000;
    pointer-events: none;
    opacity: 0.96;
    cursor: grabbing;
    box-shadow: 0 12px 36px rgba(0, 0, 0, 0.5);
    transition: none;
    left: 0;
    top: 0;

    &:hover {
      transform: none !important;
      animation: none !important;
      filter: none !important;
      box-shadow: 0 12px 36px rgba(0, 0, 0, 0.5);
    }
  }
</style>
