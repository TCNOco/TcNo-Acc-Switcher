<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { get } from "svelte/store";
  import ReorderPointerGrid from "../components/ReorderPointerGrid.svelte";
  import SearchOverlay, { type SearchResultRow } from "../components/SearchOverlay.svelte";
  import { route, appBarTitle } from "../stores/nav";
  import { platformListSort, type PlatformSortKind } from "../stores/platformListSort";
  import { t } from "../stores/i18n";
  import { openConfirm, openFolderPicker } from "../stores/modal";
  import { pushToast } from "../stores/toast";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import type { PlatformStartup } from "../../bindings/TcNo-Acc-Switcher/internal/platform/models.js";
  import { platformIconFgHref } from "../lib/platformIcon";
  import { contextMenu } from "../lib/actions/contextMenu";
  import type { MenuItemDef } from "../stores/contextMenu";
  import * as Shortcuts from "wails-shortcuts-service";
  import { fuzzyWordsMatch } from "../lib/searchFuzzy";
  import { closeSearchOverlay, searchOverlayCtrl } from "../stores/searchOverlay";
  import { setPlatformAccountCounts } from "../stores/platformAccountsCache";
  import { prefetchPlatformPages } from "../lib/pageLoaders";
  import { homeScreenData } from "../stores/homeScreenData";
  import "../styles/HomePlatforms.scss";

  let startup: PlatformStartup | null = get(homeScreenData);
  let homeOrder: string[] = startup?.homePlatformOrder ?? [];
  let disabledPlatformNames: string[] = startup?.disabledPlatformNames ? [...startup.disabledPlatformNames] : [];
  let loadError: string | null = null;

  let navigating = false;
  let overlayQuery = "";
  let overlayQueryDebounceTimer: ReturnType<typeof setTimeout> | null = null;
  let debouncedOverlayQuery = "";
  let offHomeSort: (() => void) | undefined;
  let lastHandledHomeSortId = 0;
  let warmedPlatformPages = false;

  const SEARCH_MAX = 5;

  $: so = $searchOverlayCtrl;
  $: appBarTitle.set("TcNo Account Switcher");
  $: {
    const q = overlayQuery;
    if (overlayQueryDebounceTimer) clearTimeout(overlayQueryDebounceTimer);
    overlayQueryDebounceTimer = setTimeout(() => {
      debouncedOverlayQuery = q;
    }, 150);
  }
  $: homeSearchPrimary = buildHomePrimary(debouncedOverlayQuery);
  $: homeSearchDisabled = buildHomeDisabled(overlayQuery);

  $: if (startup && !startup.platformsFileMissing && !warmedPlatformPages) {
    warmedPlatformPages = true;
    requestAnimationFrame(() => prefetchPlatformPages());
  }

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
      homeScreenData.set(s);
      startup = s;
      homeOrder = s.homePlatformOrder ?? [];
      disabledPlatformNames = [...(s.disabledPlatformNames ?? [])];
      setPlatformAccountCounts(s.platformAccountCounts ?? {});
      if (!s.platformsFileMissing && s.allPlatformNames?.length && homeOrder.length === 0) {
        route.set({ page: "manage-platforms" });
      }
    } catch (e) {
      loadError = e instanceof Error ? e.message : String(e);
    }
  }

  async function tryResolvePlatform(name: string): Promise<boolean> {
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
      return true;
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
        return true;
      }
    }
    return false;
  }

  async function openPlatform(name: string): Promise<void> {
    if (navigating) return;
    navigating = true;
    try {
      await tryResolvePlatform(name);
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

  function applyHomePlatformOrderSort(kind: PlatformSortKind): void {
    if (kind !== "alpha_asc" && kind !== "alpha_desc") {
      return;
    }
    const ids = [...homeOrder];
    const cmp = (a: string, b: string) =>
      a.localeCompare(b, undefined, { sensitivity: "base" });
    ids.sort((x, y) => (kind === "alpha_asc" ? cmp(x, y) : -cmp(x, y)));
    homeOrder = ids;
    void PlatformService.SaveHomeOrder(ids).catch(() => {});
  }

  function buildHomePrimary(q: string): SearchResultRow[] {
    const tr = get(t);
    const dis = new Set(disabledPlatformNames.map((x) => x.trim().toLowerCase()));
    const enabled = homeOrder.filter((n) => !dis.has(n.trim().toLowerCase()));
    const trimmed = q.trim();
    let list = trimmed
      ? enabled.filter((n) => fuzzyWordsMatch(trimmed, n))
      : enabled.slice(0, SEARCH_MAX);
    if (trimmed) {
      list = list.slice(0, SEARCH_MAX);
    }
    return list.map((n) => ({
      key: `p:${n}`,
      title: n.toUpperCase(),
      badge: tr("Search_Section_Platform"),
      platformIconName: n,
    }));
  }

  function buildHomeDisabled(q: string): SearchResultRow[] {
    const tr = get(t);
    const trimmed = q.trim();
    if (!trimmed) {
      return [];
    }
    return disabledPlatformNames
      .filter((n) => fuzzyWordsMatch(trimmed, n))
      .slice(0, SEARCH_MAX)
      .map((n) => ({
        key: `d:${n}`,
        title: n.toUpperCase(),
        badge: tr("Search_Section_DisabledPlatform"),
        platformIconName: n,
        isCategory: true,
      }));
  }

  async function onSearchPick(ev: CustomEvent<SearchResultRow>): Promise<void> {
    const row = ev.detail;
    closeSearchOverlay();
    if (row.key.startsWith("p:")) {
      await openPlatform(row.key.slice(2));
      return;
    }
    if (row.key.startsWith("d:")) {
      const plat = row.key.slice(2);
      try {
        const next = disabledPlatformNames.filter((x) => x !== plat);
        await PlatformService.SetDisabledPlatforms(next);
        disabledPlatformNames = next;
        await refreshStartup(true);
        await openPlatform(plat);
      } catch (e) {
        pushToast({
          type: "error",
          title: "",
          message: e instanceof Error ? e.message : String(e),
          duration: 8000,
        });
      }
    }
  }

  function tileContextMenu(platformName: string): MenuItemDef[] {
    const tr = get(t);
    return [
      {
        label: tr("Context_CreateShortcut"),
        action: () => {
          void (async () => {
            try {
              await Shortcuts.CreatePlatformShortcut(platformName);
              pushToast({
                type: "success",
                title: "",
                message: tr("Toast_ShortcutCreated"),
                duration: 5000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                title: "",
                message: e instanceof Error ? e.message : String(e),
                duration: 8000,
              });
            }
          })();
        },
      },
      {
        label: tr("Context_HidePlatform"),
        action: () => {
          void (async () => {
            if (disabledPlatformNames.includes(platformName)) return;
            const next = [...disabledPlatformNames, platformName];
            try {
              await PlatformService.SetDisabledPlatforms(next);
              disabledPlatformNames = next;
              await refreshStartup(true);
            } catch (e) {
              pushToast({
                type: "error",
                title: "",
                message: e instanceof Error ? e.message : String(e),
                duration: 8000,
              });
            }
          })();
        },
      },
    ];
  }

  onMount(() => {
    void refreshStartup();
    offHomeSort = platformListSort.subscribe((sig) => {
      if (!sig || sig.id <= lastHandledHomeSortId) {
        return;
      }
      if (get(route).page !== "home") {
        return;
      }
      lastHandledHomeSortId = sig.id;
      applyHomePlatformOrderSort(sig.kind);
    });
  });

  onDestroy(() => {
    offHomeSort?.();
    if (overlayQueryDebounceTimer) clearTimeout(overlayQueryDebounceTimer);
  });
</script>

<div class="main-content home-root">
  <div class="platformTableHost">
    <SearchOverlay
      open={so.open}
      syncNonce={so.nonce}
      initialQuery={so.initialQuery}
      bind:query={overlayQuery}
      primaryRows={homeSearchPrimary}
      categoryRows={homeSearchDisabled}
      categoryHint={$t("Search_Hint_EnablePlatform")}
      gameRows={[]}
      gameHint=""
      on:close={() => closeSearchOverlay()}
      on:pick={(e) => void onSearchPick(e)}
    />
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
            <!-- svelte-ignore a11y-no-static-element-interactions -->
            <div class="platform_tile_ctx" use:contextMenu={() => tileContextMenu(rid)}>
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
            </div>
          </svelte:fragment>
        </ReorderPointerGrid>
      </div>
    {/if}
  </div>
</div>
