<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
  import ActionBar from "../components/ActionBar.svelte";
  import ReorderPointerGrid from "../components/ReorderPointerGrid.svelte";
  import TagFilterBar from "../components/TagFilterBar.svelte";
  import AccountTagBubbles from "../components/AccountTagBubbles.svelte";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import SearchOverlay, { type SearchResultRow } from "../components/SearchOverlay.svelte";
  import { actionBarStatus } from "../stores/actionBarStatus";
  import {
    platformExeIconUrl,
    platformAction,
    selectedAccount as selectedAccountStore,
    platformLiveSessionId,
    platformAccountsRefresh,
  } from "../stores/platformPage";
  import { pushToast } from "../stores/toast";
  import { openPrompt } from "../stores/modal";
  import { locale, t } from "../stores/i18n";
  import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import { AccountDTO, AccountImagePatch } from "../../bindings/TcNo-Acc-Switcher/internal/basic/models.js";
  import { GetPlatformExeIcon, LaunchPlatform } from "../lib/platformBindings";
  import { formatToastWithError, formatWailsError } from "../lib/formatWailsError";
  import {
    isNeedsAdminError,
    offerRestartIfNeedsAdmin,
    preflightAdminForPlatform,
    reportLaunchFailure,
  } from "../lib/adminFlow";
  import { tooltip } from "../lib/actions/tooltip";
  import { contextMenu as ctxMenuAction } from "../lib/actions/contextMenu";
  import type { MenuItemDef } from "../stores/contextMenu";
  import * as Shortcuts from "wails-shortcuts-service";
  import { get } from "svelte/store";
  import { openConfirm } from "../stores/modal";
  import { offlineMode, offlineSafeImageSrc } from "../stores/offlineMode";
  import { fuzzyWordsMatch } from "../lib/searchFuzzy";
  import { formatLastLoginForLocale } from "../lib/formatLastLogin";
  import {
    buildTagsSectionMenuItem,
    openTagFilterMenu,
    type TagDefRow,
    type TagFilterMode,
  } from "../lib/accountTagsContext";
  import { closeSearchOverlay, searchOverlayCtrl } from "../stores/searchOverlay";
  import { platformListSort, type PlatformSortKind } from "../stores/platformListSort";

  const PROFILE_FALLBACK = "/img/BasicDefault.webp";

  /** Bindings class typing can lag `currentSession`; keep explicit for the list row. */
  type BasicRow = InstanceType<typeof AccountDTO> & {
    currentSession?: boolean;
    avatarPending?: boolean;
    tags?: TagDefRow[];
  };

  export let name: string;

  let accounts: BasicRow[] = [];
  let accountIds: string[] = [];
  let loadError = "";
  let selectedUniqueId = "";
  let offPlatformAction: (() => void) | undefined;
  let offAccountsRefresh: (() => void) | undefined;
  let offBasicImageEvent: (() => void) | undefined;
  let lastHandledActionId = 0;
  let basicListRefreshTimers: ReturnType<typeof setTimeout>[] = [];
  let basicAcclistEl: HTMLDivElement | undefined;
  let overlayQuery = "";
  let offSort: (() => void) | undefined;
  let lastHandledSortId = 0;
  let basicAvatarEpoch: Record<string, number> = {};

  let tagDefs: TagDefRow[] = [];
  let tagFilterMode: TagFilterMode = { kind: "all" };

  const SEARCH_MAX = 5;

  $: so = $searchOverlayCtrl;
  $: appBarTitle.set(name || "TcNo Account Switcher");
  $: basicSearchPrimary = buildBasicAccountRows(overlayQuery);
  $: if (name) {
    route.set({ page: "platform", platformName: name });
  }

  $: selectedAccountStore.set({
    platformKey: name,
    uniqueId: selectedUniqueId,
    displayName: accountById(selectedUniqueId)?.displayName ?? "",
    accountLogin: "",
  });

  function accountById(id: string): BasicRow | undefined {
    return accounts.find((a) => a.uniqueId === id);
  }

  function applyBasicImagePatch(p: AccountImagePatch): void {
    const plat = String(p.platformKey ?? "").trim();
    const uid = String(p.uniqueId ?? "").trim();
    if (!plat || !uid || plat !== name) {
      return;
    }
    let hit = false;
    accounts = accounts.map((r) => {
      if (r.uniqueId !== uid) {
        return r;
      }
      hit = true;
      const nextUrl =
        typeof p.imageUrl === "string" && p.imageUrl.trim() !== "" ? p.imageUrl.trim() : r.imageUrl;
      const nextPending =
        typeof p.avatarPending === "boolean" ? p.avatarPending : (r.avatarPending ?? false);
      return {
        ...r,
        imageUrl: nextUrl,
        avatarPending: nextPending,
      } as BasicRow;
    });
    if (hit) {
      basicAvatarEpoch = { ...basicAvatarEpoch, [uid]: (basicAvatarEpoch[uid] ?? 0) + 1 };
    }
  }

  $: tagFilterBarLabel =
    tagFilterMode.kind === "all"
      ? $t("Tags_All")
      : tagFilterMode.kind === "untagged"
        ? $t("Tags_Filter_Untagged")
        : tagFilterMode.name;

  $: displayAccountIds = (() => {
    const ids = accountIds;
    if (tagFilterMode.kind === "all") {
      return ids;
    }
    if (tagFilterMode.kind === "untagged") {
      return ids.filter((id) => (accountById(id)?.tags?.length ?? 0) === 0);
    }
    const tid = tagFilterMode.id;
    return ids.filter((id) => accountById(id)?.tags?.some((x) => x.id === tid));
  })();

  $: reorderDisabled = tagFilterMode.kind !== "all";

  $: {
    if (
      selectedUniqueId &&
      displayAccountIds.length > 0 &&
      !displayAccountIds.includes(selectedUniqueId)
    ) {
      selectedUniqueId = displayAccountIds[0] ?? "";
      touchStatus();
    }
  }

  function touchStatus(): void {
    const acc = accountById(selectedUniqueId);
    actionBarStatus.set(
      acc ? $t("Status_SelectedAccount", { name: acc.displayName || acc.uniqueId }) : "",
    );
  }

  function scheduleAccountsRefresh(): void {
    for (const t of basicListRefreshTimers) clearTimeout(t);
    basicListRefreshTimers = [];
    void loadAccounts();
    basicListRefreshTimers.push(
      setTimeout(() => void loadAccounts(), 700),
      setTimeout(() => void loadAccounts(), 2200),
    );
  }

  async function loadTagDefs(): Promise<void> {
    try {
      const rows = await BasicService.ListTagDefinitions(name);
      tagDefs = (rows as { id: string; name: string; color: string }[]).map((r) => ({
        id: r.id,
        name: r.name,
        color: r.color,
      }));
    } catch {
      tagDefs = [];
    }
  }

  async function loadAccounts(): Promise<void> {
    loadError = "";
    try {
      const rows = (await BasicService.GetAccounts(name)) as BasicRow[];
      accounts = rows;
      accountIds = rows.map((r) => r.uniqueId);
      const liveRow = rows.find((r) => r.currentSession);
      platformLiveSessionId.set({
        platformKey: name,
        uniqueId: (liveRow?.uniqueId ?? "").trim(),
      });
      const first = rows[0]?.uniqueId ?? "";
      const stillValid = selectedUniqueId && rows.some((r) => r.uniqueId === selectedUniqueId);
      selectedUniqueId = stillValid ? selectedUniqueId : first;
      touchStatus();
      await loadTagDefs();
    } catch (e) {
      loadError = formatWailsError(e) || String(e);
      accounts = [];
      accountIds = [];
      selectedUniqueId = "";
      actionBarStatus.set("");
      platformLiveSessionId.set({ platformKey: "", uniqueId: "" });
    }
  }

  function onItemClick(e: CustomEvent<{ id: string }>): void {
    selectedUniqueId = e.detail.id;
    touchStatus();
  }

  function onReorder(e: CustomEvent<{ items: string[] }>): void {
    accountIds = e.detail.items;
    BasicService.SaveAccountOrder(name, e.detail.items).catch(() => {});
  }

  function displayKeyForSort(uid: string): string {
    const a = accountById(uid);
    return (a?.displayName ?? uid).trim().toLowerCase();
  }

  function lastUsedMsForSort(uid: string): number {
    const a = accountById(uid);
    const t = Date.parse((a?.lastUsed ?? "").trim());
    return Number.isNaN(t) ? 0 : t;
  }

  function applyPlatformSort(kind: PlatformSortKind): void {
    if (kind === "steam_user_asc" || kind === "steam_user_desc") {
      return;
    }
    const ids = [...accountIds];
    const cmpAlpha = (x: string, y: string) =>
      displayKeyForSort(x).localeCompare(displayKeyForSort(y));
    switch (kind) {
      case "alpha_asc":
        ids.sort(cmpAlpha);
        break;
      case "alpha_desc":
        ids.sort((x, y) => -cmpAlpha(x, y));
        break;
      case "lastused_new_old":
      case "date_new_old":
        ids.sort(
          (x, y) =>
            lastUsedMsForSort(y) - lastUsedMsForSort(x) || cmpAlpha(x, y),
        );
        break;
      case "lastused_old_new":
      case "date_old_new":
        ids.sort(
          (x, y) =>
            lastUsedMsForSort(x) - lastUsedMsForSort(y) || cmpAlpha(x, y),
        );
        break;
      default:
        return;
    }
    accountIds = ids;
    BasicService.SaveAccountOrder(name, ids).catch(() => {});
  }

  function buildBasicAccountRows(q: string): SearchResultRow[] {
    const tr = get(t);
    const trimmed = q.trim();
    const hay = (acc: BasicRow) => {
      const blob = `${acc.displayName}\n${acc.uniqueId}\n${acc.note ?? ""}`;
      return fuzzyWordsMatch(trimmed, blob);
    };
    let list = trimmed ? accounts.filter((a) => hay(a)) : accounts.slice(0, SEARCH_MAX);
    if (trimmed) {
      list = list.slice(0, SEARCH_MAX);
    }
    return list.map((a) => ({
      key: `a:${a.uniqueId}`,
      title: a.displayName || a.uniqueId,
      badge: tr("Search_Section_Account"),
      accountIconUrl: offlineSafeImageSrc(
        get(offlineMode),
        a.imageUrl && !(a as { avatarPending?: boolean }).avatarPending ? a.imageUrl : undefined,
        PROFILE_FALLBACK,
      ),
    }));
  }

  async function onSearchPick(ev: CustomEvent<SearchResultRow>): Promise<void> {
    const row = ev.detail;
    closeSearchOverlay();
    if (row.key.startsWith("a:")) {
      const uid = row.key.slice(2);
      selectedUniqueId = uid;
      touchStatus();
      await swapToLogin();
    }
  }

  async function reportBasicSwitchFailure(e: unknown): Promise<void> {
    await offerRestartIfNeedsAdmin(e, name);
    if (isNeedsAdminError(e)) {
      return;
    }
    pushToast({
      type: "error",
      message: formatToastWithError($t("Toast_SwitchFailed"), e),
      duration: 8000,
    });
  }

  async function reportBasicSaveFailure(e: unknown): Promise<void> {
    await offerRestartIfNeedsAdmin(e, name);
    if (isNeedsAdminError(e)) {
      return;
    }
    pushToast({
      type: "error",
      message: formatToastWithError($t("Toast_SaveFailed"), e),
      duration: 8000,
    });
  }

  async function swapToLogin(): Promise<void> {
    if (!selectedUniqueId) {
      return;
    }
    try {
      await BasicService.SwapToAccount(name, selectedUniqueId, []);
      scheduleAccountsRefresh();
      pushToast({
        type: "success",
        message: $t("Toast_AccountSwitched"),
        duration: 4000,
      });
    } catch (e) {
      await reportBasicSwitchFailure(e);
    }
  }

  async function launchPlatformForSelection(): Promise<void> {
    const selected = accountById(selectedUniqueId);
    if (selectedUniqueId && selected && !selected.currentSession) {
      try {
        await BasicService.SwapToAccount(name, selectedUniqueId, []);
      } catch (e) {
        await reportBasicSwitchFailure(e);
        return;
      }
    }
    try {
      await LaunchPlatform(name);
      scheduleAccountsRefresh();
    } catch (e) {
      await reportLaunchFailure(e, name);
    }
  }

  async function saveCurrentPrompt(): Promise<void> {
    const displayName = await openPrompt({
      title: $t("Modal_SaveCurrent_Title"),
      body: $t("Modal_SaveCurrent_Body"),
      positiveLabel: $t("Button_SaveCurrent"),
      negativeLabel: $t("Button_Cancel"),
      initialValue: "",
    });
    if (displayName === null || !String(displayName).trim()) {
      return;
    }
    try {
      await BasicService.SaveCurrent(name, String(displayName).trim());
      await loadAccounts();
      pushToast({
        type: "success",
        message: $t("Toast_AccountSaved"),
        duration: 4000,
      });
    } catch (e) {
      await reportBasicSaveFailure(e);
    }
  }

  async function handlePlatformActionKind(
    kind: "login" | "addNew" | "launch" | "saveCurrent",
  ): Promise<void> {
    if (kind === "launch") {
      await launchPlatformForSelection();
      return;
    }
    if (kind === "addNew") {
      try {
        await BasicService.AddNew(name);
        scheduleAccountsRefresh();
        pushToast({
          type: "success",
          message: $t("Toast_AccountSwitched"),
          duration: 4000,
        });
      } catch (e) {
        await reportBasicSwitchFailure(e);
      }
      return;
    }
    if (kind === "saveCurrent") {
      await saveCurrentPrompt();
      return;
    }
    if (kind === "login") {
      await swapToLogin();
    }
  }

  function slotKey(x: string | null | undefined): string {
    return x ?? "";
  }

  function onTagFilterBarClick(ev: Event): void {
    openTagFilterMenu({
      ev: ev as MouseEvent,
      tagDefs,
      tr: get(t),
      onPick: (mode) => {
        tagFilterMode = mode;
      },
    });
  }

  function basicCtxMenu(rowId: string): () => MenuItemDef[] {
    return () => {
      const tr = get(t);
      const acc = accounts.find((a) => a.uniqueId === rowId);
      if (!acc) {
        return [];
      }
      return [
        {
          label: tr("Context_SwapTo"),
          action: () => {
            selectedUniqueId = rowId;
            touchStatus();
            void swapToLogin();
          },
        },
        {
          label: tr("Context_ChangeName"),
          action: async () => {
            const next = await openPrompt({
              title: tr("Context_ChangeName"),
              body: "",
              positiveLabel: tr("Ok"),
              negativeLabel: tr("Button_Cancel"),
              initialValue: acc.displayName ?? "",
            });
            if (next === null || !String(next).trim()) {
              return;
            }
            try {
              await BasicService.RenameAccount(name, rowId, String(next).trim());
              await loadAccounts();
              pushToast({
                type: "success",
                message: tr("Toast_AccountSaved"),
                duration: 3000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError(tr("Toast_SaveFailed"), e),
                duration: 8000,
              });
            }
          },
        },
        {
          label: tr("Context_CreateShortcut"),
          action: async () => {
            try {
              const p = await Shortcuts.CreateAccountShortcut(
                name,
                rowId,
                acc.displayName ?? rowId,
                "",
                "",
                "",
              );
              pushToast({
                type: "success",
                message: `${tr("Toast_ShortcutCreated")}\n${p}`,
                duration: 6000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError(tr("Toast_SwitchFailed"), e),
                duration: 8000,
              });
            }
          },
        },
        {
          label: tr("Context_ChangeImage"),
          action: async () => {
            const path = await openPrompt({
              title: tr("Context_ChangeImage"),
              body: "",
              positiveLabel: tr("Ok"),
              negativeLabel: tr("Button_Cancel"),
              initialValue: "",
            });
            if (path === null || !String(path).trim()) {
              return;
            }
            try {
              await BasicService.ChangeAccountImage(name, rowId, String(path).trim());
              await loadAccounts();
              pushToast({
                type: "success",
                message: tr("Toast_AccountSaved"),
                duration: 3000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError(tr("Toast_SaveFailed"), e),
                duration: 8000,
              });
            }
          },
        },
        {
          label: tr("Forget"),
          action: async () => {
            const ok = await openConfirm({
              title: tr("Forget"),
              body: tr("Prompt_ForgetAccount", { platform: name }),
              style: "yesno",
            });
            if (!ok) {
              return;
            }
            try {
              await BasicService.ForgetAccount(name, rowId);
              await loadAccounts();
              pushToast({
                type: "success",
                message: tr("Toast_AccountSaved"),
                duration: 3000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError(tr("Toast_SaveFailed"), e),
                duration: 8000,
              });
            }
          },
        },
        buildTagsSectionMenuItem({
          platformKey: name,
          uniqueId: rowId,
          assignedTags: (acc.tags ?? []) as TagDefRow[],
          tagDefs,
          tr,
          afterChange: async () => {
            await loadAccounts();
            await loadTagDefs();
          },
          onSuccess: () =>
            pushToast({
              type: "success",
              message: tr("Toast_AccountSaved"),
              duration: 2500,
            }),
          onError: (e) =>
            pushToast({
              type: "error",
              message: formatToastWithError(tr("Toast_SaveFailed"), e),
              duration: 8000,
            }),
        }),
        {
          label: tr("Notes"),
          action: async () => {
            const cur = await BasicService.GetAccountNote(name, rowId);
            const note = await openPrompt({
              title: tr("Notes"),
              body: tr("Modal_Title_AccountNotes", {
                accountName: acc.displayName ?? rowId,
              }),
              positiveLabel: tr("Ok"),
              negativeLabel: tr("Button_Cancel"),
              initialValue: cur ?? "",
              multiline: true,
            });
            if (note === null) {
              return;
            }
            try {
              await BasicService.SetAccountNote(name, rowId, String(note));
              await loadAccounts();
              pushToast({
                type: "success",
                message: tr("Toast_AccountSaved"),
                duration: 3000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError(tr("Toast_SaveFailed"), e),
                duration: 8000,
              });
            }
          },
        },
      ];
    };
  }

  onMount(() => {
    previousPage.set({ page: "home" });
    void preflightAdminForPlatform(name);
    void loadAccounts();
    void GetPlatformExeIcon(name).then((u: string) => platformExeIconUrl.set(u ?? ""));
    offSort = platformListSort.subscribe((sig) => {
      if (!sig || sig.id <= lastHandledSortId) {
        return;
      }
      lastHandledSortId = sig.id;
      applyPlatformSort(sig.kind);
    });
    offPlatformAction = platformAction.subscribe((v) => {
      if (!v || v.id === lastHandledActionId) {
        return;
      }
      lastHandledActionId = v.id;
      void handlePlatformActionKind(v.kind);
    });
    offAccountsRefresh = platformAccountsRefresh.subscribe((p) => {
      if (p.seq === 0 || p.platformKey !== name) {
        return;
      }
      scheduleAccountsRefresh();
    });
    offBasicImageEvent = Events.On("basic-account-image-updated", (ev) => {
      const raw = ev.data;
      const p =
        raw instanceof AccountImagePatch
          ? raw
          : AccountImagePatch.createFrom(raw as Record<string, unknown>);
      applyBasicImagePatch(p);
    });
    void (async () => {
      try {
        const remote = await BasicService.PlatformUsesRemoteProfileImages(name);
        if (remote) {
          void BasicService.StartBasicProfileImageRefresh(name);
        }
      } catch {
        /* ignore */
      }
    })();
  });

  onDestroy(() => {
    for (const t of basicListRefreshTimers) clearTimeout(t);
    basicListRefreshTimers = [];
    selectedAccountStore.set({ platformKey: "", uniqueId: "", displayName: "", accountLogin: "" });
    platformLiveSessionId.set({ platformKey: "", uniqueId: "" });
    platformAction.set(null);
    offPlatformAction?.();
    offSort?.();
    offAccountsRefresh?.();
    offBasicImageEvent?.();
    platformAccountsRefresh.set({ seq: 0, platformKey: "" });
    platformExeIconUrl.set("");
    actionBarStatus.set("");
  });
</script>

<div class="main-content platform-accounts-root">
  {#if name}
    <div class="platformTableHost">
      <SearchOverlay
        open={so.open}
        syncNonce={so.nonce}
        initialQuery={so.initialQuery}
        bind:query={overlayQuery}
        primaryRows={basicSearchPrimary}
        categoryRows={[]}
        categoryHint=""
        gameRows={[]}
        gameHint=""
        on:close={() => closeSearchOverlay()}
        on:pick={(e) => void onSearchPick(e)}
      />
      <div class="platformTable">
      {#if loadError}
        <p class="platform-accounts-hint">{loadError}</p>
      {/if}
      <div class="steam-acclist" bind:this={basicAcclistEl}>
        {#if tagDefs.length > 0}
          <TagFilterBar label={tagFilterBarLabel} onClick={onTagFilterBarClick} />
        {/if}
        <ReorderPointerGrid
          items={displayAccountIds}
          reorderDisabled={reorderDisabled}
          listClass="acc_list"
          itemClass="acc_list_item acc_list_item--drag"
          placeholderClass="acc_list_item placeHolderAcc"
          ghostClass="acc_list_item acc_list_item--ghost"
          ariaLabel="Accounts"
          on:reorder={onReorder}
          on:itemclick={onItemClick}
        >
          <svelte:fragment slot="item" let:rowId>
            {@const rid = slotKey(rowId)}
            {@const acc = accounts.find((a) => a.uniqueId === rid)}
            {@const radioId = `basic-acc-${rid}`}
            <div class="acc_list_item_inner">
              <input
                type="radio"
                class="acc"
                id={radioId}
                name="basic-accounts"
                value={rid}
                bind:group={selectedUniqueId}
                on:change={touchStatus}
              />
              <label
                for={radioId}
                class="acc"
                class:currentAcc={acc?.currentSession}
                use:ctxMenuAction={{
                  items: basicCtxMenu(rid),
                  beforeOpen: () => {
                    selectedUniqueId = rid;
                    touchStatus();
                  },
                }}
                use:tooltip={acc?.currentSession
                  ? {
                      text: $t("Tooltip_CurrentAccount"),
                      placement: "right",
                      boundary: basicAcclistEl,
                    }
                  : undefined}
                on:dblclick|preventDefault={() => {
                  selectedUniqueId = rid;
                  touchStatus();
                  void swapToLogin();
                }}
              >
                {#key `${rid}:${basicAvatarEpoch[rid] ?? 0}`}
                  <img
                    src={offlineSafeImageSrc(
                      $offlineMode,
                      acc?.imageUrl && !acc?.avatarPending ? acc.imageUrl : undefined,
                      PROFILE_FALLBACK,
                    )}
                    alt=""
                    draggable="false"
                  />
                {/key}
                <h6 class="displayName">{acc?.displayName ?? rid}</h6>
                <AccountTagBubbles tags={acc?.tags ?? []} />
                {#if acc?.note}
                  <p class="acc_note">{acc.note}</p>
                {/if}
                {#if acc?.showLastUsed && (acc?.lastUsed ?? "").trim()}
                  <p class="acc_lastused">
                    {formatLastLoginForLocale(acc.lastUsed, $locale)}
                  </p>
                {/if}
              </label>
            </div>
          </svelte:fragment>
        </ReorderPointerGrid>
      </div>
    </div>
    </div>
  {/if}
</div>
<ActionBar />

<style lang="scss">
  .platform-accounts-root {
    display: flex;
    flex-direction: column;
    min-height: 0;
    flex: 1;
  }

  .platform-accounts-hint {
    margin: 0.75rem 1rem 0;
    font-size: 0.85rem;
    color: var(--white, #fff);
    opacity: 0.85;
  }

  .platformTableHost {
    position: relative;
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }

  .acc_lastused {
    margin: 0.15rem 0 0;
    font-size: 0.72rem;
    opacity: 0.75;
  }
</style>
