<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
  import ActionBar from "../components/ActionBar.svelte";
  import ReorderPointerGrid from "../components/ReorderPointerGrid.svelte";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import { actionBarStatus } from "../stores/actionBarStatus";
  import {
    platformExeIconUrl,
    platformAction,
    selectedAccount as selectedAccountStore,
  } from "../stores/platformPage";
  import { pushToast } from "../stores/toast";
  import * as SteamService from "../../bindings/TcNo-Acc-Switcher/internal/steam/steamservice.js";
  import { GetPlatformExeIcon } from "../lib/platformBindings";
  import { AccountDTO, AccountPatch } from "../../bindings/TcNo-Acc-Switcher/internal/steam/models.js";
  import { locale, t } from "../stores/i18n";
  import { formatLastLoginForLocale } from "../lib/formatLastLogin";
  import { formatToastWithError, formatWailsError } from "../lib/formatWailsError";
  import { tooltip } from "../lib/actions/tooltip";
  import { contextMenu as ctxMenuAction } from "../lib/actions/contextMenu";
  import type { MenuItemDef } from "../stores/contextMenu";
  import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import * as Shortcuts from "../../bindings/TcNo-Acc-Switcher/internal/shortcuts/service.js";
  import { get } from "svelte/store";
  import { openConfirm, openPrompt } from "../stores/modal";

  const STEAM_USERDATA_ERR_KEYS = new Set([
    "Toast_NoValidSteamId",
    "Toast_SameAccount",
    "Toast_NoFindSteamUserdata",
    "Toast_NoFindGameBackup",
  ]);

  function mapSteamUserdataI18nError(
    err: unknown,
    tr: (k: string, v?: Record<string, string | number>) => string,
  ): string {
    return formatWailsError(err, {
      translateMessage: (k) => tr(k),
      i18nFirstLineKeys: STEAM_USERDATA_ERR_KEYS,
    });
  }

  /** Bindings row + client-only syncError; some JSON fields are explicit — TS sometimes omits quoted keys on generated classes. */
  type SteamAccountRow = InstanceType<typeof AccountDTO> & {
    syncError?: string;
    currentSession: boolean;
    showShortNotes: boolean;
    note: string;
  };

  /** Vite `public/img/` → served at `/img/`; shown until Steam profile image is ready */
  const PROFILE_PLACEHOLDER = "/img/BasicDefault.webp";

  /**
   * App IDs omitted:
   * 228980 = Steamworks Common Redistributables.
   */
  const STEAM_CONTEXT_MENU_HIDDEN_APP_IDS = new Set<string>(["228980"]);

  export let name: string;

  let steamAccounts: SteamAccountRow[] = [];
  /** Bumped when a row receives a patch so `{#key}` remounts that tile (avoids stuck "Updating…" on last row). */
  let rowEpoch: Record<string, number> = {};
  /** Scroll/content box for Steam tooltips (keeps popovers off the window chrome). */
  let steamAcclistEl: HTMLDivElement | null = null;
  let steamIds: string[] = [];
  let steamLoadError = "";
  let offSteamEvent: (() => void) | undefined;
  let offPlatformAction: (() => void) | undefined;
  let lastHandledActionId = 0;
  /** Cleared on destroy; staggered reloads pick up Steam writing loginusers after swap/launch. */
  let steamListRefreshTimers: ReturnType<typeof setTimeout>[] = [];
  /** Selected row for switching (radio group); drives footer status. */
  let selectedSteamId = "";

  let installedGames: { appId: string; name: string }[] = [];
  /** Per account: app IDs with `userdata/.../id` and with `Backups/Steam/.../id` (drives Game data submenu). */
  let gameDataBySteamId: Record<string, { userdata: Set<string>; backup: Set<string> }> = {};

  $: appBarTitle.set(name || "TcNo Account Switcher");
  $: if (name) {
    route.set({ page: "platform", platformName: name });
  }

  $: selectedAccountStore.set({
    platformKey: name,
    uniqueId: selectedSteamId,
  });

  function accountBySteamId(id: string): SteamAccountRow | undefined {
    return steamAccounts.find((a) => a.steamId64 === id);
  }

  function personaForStatus(acc: SteamAccountRow): string {
    const p = (acc.personaName ?? "").trim();
    if (p) return p;
    const a = (acc.accountName ?? "").trim();
    if (a) return a;
    return acc.steamId64;
  }

  function touchSteamActionBar(): void {
    const acc = accountBySteamId(selectedSteamId);
    actionBarStatus.set(
      acc ? $t("Status_SelectedAccount", { name: personaForStatus(acc) }) : "",
    );
  }

  function onSteamItemClick(e: CustomEvent<{ id: string }>): void {
    selectedSteamId = e.detail.id;
    touchSteamActionBar();
  }

  function applySteamPatch(p: AccountPatch): void {
    const id = String(p.steamId64 ?? "");
    let hit = false;
    steamAccounts = steamAccounts.map((r) => {
      if (r.steamId64 !== id) return r;
      hit = true;
      const errMsg = typeof p.error === "string" ? p.error : r.syncError ?? "";
      return {
        ...r,
        imageUrl: p.imageUrl || r.imageUrl,
        vac: p.vac,
        ltd: p.ltd,
        avatarPending: p.avatarPending,
        metaPending: p.metaPending,
        syncError: errMsg,
      } as SteamAccountRow;
    });
    if (hit) {
      rowEpoch = { ...rowEpoch, [id]: (rowEpoch[id] ?? 0) + 1 };
    }
    if (id === selectedSteamId) {
      touchSteamActionBar();
    }
  }

  function scheduleSteamAccountsRefresh(): void {
    for (const t of steamListRefreshTimers) clearTimeout(t);
    steamListRefreshTimers = [];
    void loadSteamAccounts();
    steamListRefreshTimers.push(
      setTimeout(() => void loadSteamAccounts(), 700),
      setTimeout(() => void loadSteamAccounts(), 2200),
    );
  }

  async function refreshGameDataAppSets(): Promise<void> {
    const ids = steamIds;
    if (ids.length === 0) {
      gameDataBySteamId = {};
      return;
    }
    try {
      const parts: [string, { userdata: Set<string>; backup: Set<string> }][] = await Promise.all(
        ids.map(async (id) => {
          try {
            const s = await SteamService.GetSteamGameDataAppIDSets(id);
            return [
              id,
              {
                userdata: new Set(s.userdataAppIds.map((x: string) => String(x).trim())),
                backup: new Set(s.backupAppIds.map((x: string) => String(x).trim())),
              },
            ] as [string, { userdata: Set<string>; backup: Set<string> }];
          } catch {
            return [
              id,
              { userdata: new Set<string>(), backup: new Set<string>() },
            ] as [string, { userdata: Set<string>; backup: Set<string> }];
          }
        }),
      );
      gameDataBySteamId = Object.fromEntries(parts);
    } catch {
      gameDataBySteamId = {};
    }
  }

  async function loadSteamAccounts(): Promise<void> {
    steamLoadError = "";
    try {
      const rows = (await SteamService.GetSteamAccounts()) as SteamAccountRow[];
      rowEpoch = {};
      steamAccounts = rows;
      steamIds = rows.map((r) => r.steamId64);
      const active = rows.find((r) => r.currentSession);
      const firstId = rows[0]?.steamId64 ?? "";
      const stillValid =
        selectedSteamId && rows.some((r) => r.steamId64 === selectedSteamId);
      selectedSteamId = stillValid
        ? selectedSteamId
        : active
          ? active.steamId64
          : firstId;
      touchSteamActionBar();
      SteamService.StartSteamProfileRefresh();
      await refreshGameDataAppSets();
    } catch (e) {
      steamLoadError = formatWailsError(e) || String(e);
      steamAccounts = [];
      steamIds = [];
      gameDataBySteamId = {};
      selectedSteamId = "";
      actionBarStatus.set("");
    }
  }

  function onAccountReorder(e: CustomEvent<{ items: string[] }>): void {
    steamIds = e.detail.items;
    SteamService.SaveSteamAccountOrder(e.detail.items).catch(() => {});
  }

  async function steamLoginSelected(): Promise<void> {
    if (!selectedSteamId) {
      return;
    }
    try {
      await SteamService.SwapToSteamAccount(selectedSteamId, -1);
      scheduleSteamAccountsRefresh();
      pushToast({
        type: "success",
        message: $t("Toast_AccountSwitched"),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SwitchFailed"), e),
        duration: 8000,
      });
    }
  }

  async function handlePlatformActionKind(
    kind: "login" | "addNew" | "launch" | "saveCurrent",
  ): Promise<void> {
    if (kind === "saveCurrent") {
      return;
    }
    if (kind === "launch") {
      try {
        await SteamService.LaunchSteam();
        scheduleSteamAccountsRefresh();
      } catch (e) {
        pushToast({
          type: "error",
          message: formatToastWithError($t("Toast_LaunchFailed"), e),
          duration: 8000,
        });
      }
      return;
    }
    if (kind === "addNew") {
      try {
        await SteamService.SteamAddNew();
        scheduleSteamAccountsRefresh();
        pushToast({
          type: "success",
          message: $t("Toast_AccountSwitched"),
          duration: 4000,
        });
      } catch (e) {
        pushToast({
          type: "error",
          message: formatToastWithError($t("Toast_SwitchFailed"), e),
          duration: 8000,
        });
      }
      return;
    }
    if (kind === "login") {
      await steamLoginSelected();
    }
  }

  function slotKey(x: string | null | undefined): string {
    return x ?? "";
  }

  async function clipboardWrite(text: string): Promise<void> {
    try {
      await navigator.clipboard.writeText(text);
      pushToast({ type: "success", message: $t("Toast_Copied"), duration: 2500 });
    } catch {
      pushToast({
        type: "error",
        message: $t("Toast_CopyFailed"),
        duration: 4000,
      });
    }
  }

  function steamCtxMenu(rid: string): () => MenuItemDef[] {
    return () => {
      const tr = get(t);
      const acc = steamAccounts.find((a) => a.steamId64 === rid);
      if (!acc) {
        return [];
      }

      const loginStates: { st: number; lab: string }[] = [
        { st: 7, lab: tr("Invisible") },
        { st: 0, lab: tr("Offline") },
        { st: 1, lab: tr("Online") },
        { st: 2, lab: tr("Busy") },
        { st: 3, lab: tr("Away") },
        { st: 4, lab: tr("Snooze") },
        { st: 5, lab: tr("LookingToTrade") },
        { st: 6, lab: tr("LookingToPlay") },
      ];

      const loginAsChildren: MenuItemDef[] = [
        { type: "search", label: tr("Context_Search") },
        ...loginStates.map((x) => ({
          label: x.lab,
          action: async () => {
            try {
              await SteamService.SwapToSteamAccount(rid, x.st);
              scheduleSteamAccountsRefresh();
              pushToast({
                type: "success",
                message: $t("Toast_AccountSwitched"),
                duration: 4000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError($t("Toast_SwitchFailed"), e),
                duration: 8000,
              });
            }
          },
        })),
      ];

      const copyChildren: MenuItemDef[] = [
        {
          label: tr("Context_CommunityUrl"),
          action: () =>
            void clipboardWrite(`https://steamcommunity.com/profiles/${rid}`),
        },
        {
          label: tr("Context_CommunityUsername"),
          action: () =>
            void clipboardWrite((acc.accountName ?? "").trim() || rid),
        },
        {
          label: tr("Context_LoginUsername"),
          action: () =>
            void clipboardWrite((acc.accountName ?? "").trim() || rid),
        },
        {
          label: tr("Context_CopySteamIdSubmenu"),
          children: [
            {
              label: tr("Context_Steam_Id64"),
              action: async () => {
                try {
                  const f = await SteamService.GetSteamIDFormats(rid);
                  void clipboardWrite(f["ID64"] ?? rid);
                } catch {
                  void clipboardWrite(rid);
                }
              },
            },
            {
              label: tr("Context_Steam_Id3"),
              action: async () => {
                try {
                  const f = await SteamService.GetSteamIDFormats(rid);
                  void clipboardWrite(f["ID3"] ?? "");
                } catch {
                  /* ignore */
                }
              },
            },
            {
              label: tr("Context_Steam_Id32"),
              action: async () => {
                try {
                  const f = await SteamService.GetSteamIDFormats(rid);
                  void clipboardWrite(f["ID32"] ?? "");
                } catch {
                  /* ignore */
                }
              },
            },
          ],
        },
      ];

      const shortcutChildren: MenuItemDef[] = [
        { type: "search", label: tr("Context_Search") },
        {
          label: tr("Context_CreateShortcut"),
          action: async () => {
            try {
              const p = await Shortcuts.CreateAccountShortcut(
                "Steam",
                rid,
                acc.personaName ?? rid,
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
                message: formatToastWithError($t("Toast_SwitchFailed"), e),
                duration: 8000,
              });
            }
          },
        },
        ...loginStates.map((x) => ({
          label: `${tr("Context_CreateShortcut")} (${x.lab})`,
          action: async () => {
            try {
              const p = await Shortcuts.CreateAccountShortcut(
                "Steam",
                rid,
                acc.personaName ?? rid,
                String(x.st),
                x.lab,
              );
              pushToast({
                type: "success",
                message: `${tr("Toast_ShortcutCreated")}\n${p}`,
                duration: 6000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError($t("Toast_SwitchFailed"), e),
                duration: 8000,
              });
            }
          },
        })),
      ];

      const gsets = gameDataBySteamId[rid];
      const gameDataSubmenuItems: MenuItemDef[] = [];
      for (const g of installedGames) {
        const aid = String(g.appId).trim();
        const hasUser = gsets?.userdata.has(aid) ?? false;
        const hasBackup = gsets?.backup.has(aid) ?? false;
        if (!hasUser && !hasBackup) {
          continue;
        }
        const children: MenuItemDef[] = [];
        if (hasUser) {
          children.push({
            label: tr("Context_Game_CopySettingsFrom"),
            action: async () => {
              try {
                await SteamService.CopySteamGameSettingsFrom(rid, g.appId);
                pushToast({
                  type: "success",
                  message: tr("Toast_SettingsCopied"),
                  duration: 5000,
                });
                void refreshGameDataAppSets();
              } catch (e) {
                pushToast({
                  type: "error",
                  message: mapSteamUserdataI18nError(e, tr),
                  duration: 8000,
                });
              }
            },
          });
        }
        if (hasBackup) {
          children.push({
            label: tr("Context_Game_RestoreSettingsTo"),
            action: async () => {
              try {
                await SteamService.RestoreSteamGameSettingsTo(rid, g.appId);
                pushToast({
                  type: "success",
                  message: tr("Toast_GameDataRestored"),
                  duration: 5000,
                });
                void refreshGameDataAppSets();
              } catch (e) {
                pushToast({
                  type: "error",
                  message: mapSteamUserdataI18nError(e, tr),
                  duration: 8000,
                });
              }
            },
          });
        }
        if (hasUser) {
          children.push({
            label: tr("Context_Game_BackupData"),
            action: async () => {
              try {
                const folder = await SteamService.BackupSteamGameData(rid, g.appId);
                pushToast({
                  type: "success",
                  message: tr("Toast_GameBackupDone", { folderLocation: folder }),
                  duration: 8000,
                });
                void refreshGameDataAppSets();
              } catch (e) {
                pushToast({
                  type: "error",
                  message: mapSteamUserdataI18nError(e, tr),
                  duration: 8000,
                });
              }
            },
          });
        }
        gameDataSubmenuItems.push({ label: g.name, children });
      }

      const gameChildren: MenuItemDef[] =
        gameDataSubmenuItems.length === 0
          ? [
              {
                type: "item",
                label: tr("Context_GameData_NoFolders"),
              },
            ]
          : [{ type: "search", label: tr("Context_Search") }, ...gameDataSubmenuItems];

      const launchChildren: MenuItemDef[] = [
        { type: "search", label: tr("Context_Search") },
        ...installedGames.map((g) => ({
          label: g.name,
          action: async () => {
            try {
              await SteamService.LoginAndLaunchGame(rid, -1, g.appId);
              scheduleSteamAccountsRefresh();
              pushToast({
                type: "success",
                message: $t("Toast_GameLaunchRequested"),
                duration: 4000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError($t("Toast_LaunchFailed"), e),
                duration: 8000,
              });
            }
          },
        })),
      ];

      return [
        {
          label: tr("Context_SwapTo"),
          action: async () => {
            selectedSteamId = rid;
            touchSteamActionBar();
            await steamLoginSelected();
          },
        },
        {
          label: tr("Context_Game_LoginAndLaunch"),
          children: launchChildren,
        },
        {
          label: tr("Context_LoginAsSubmenu"),
          children: loginAsChildren,
        },
        {
          label: tr("Context_CopySubmenu"),
          children: copyChildren,
        },
        {
          label: tr("Context_CreateShortcut"),
          children: shortcutChildren,
        },
        {
          label: tr("Forget"),
          action: async () => {
            const ok = await openConfirm({
              title: tr("Forget"),
              body: tr("Prompt_ForgetSteam"),
              style: "yesno",
            });
            if (!ok) {
              return;
            }
            try {
              await SteamService.ForgetSteamAccount(rid);
              scheduleSteamAccountsRefresh();
              pushToast({
                type: "success",
                message: $t("Toast_AccountSaved"),
                duration: 4000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError($t("Toast_SaveFailed"), e),
                duration: 8000,
              });
            }
          },
        },
        {
          label: tr("Notes"),
          action: async () => {
            const cur = await BasicService.GetAccountNote("Steam", rid);
            const note = await openPrompt({
              title: tr("Notes"),
              body: tr("Modal_Title_AccountNotes", {
                accountName: acc.personaName ?? rid,
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
              await BasicService.SetAccountNote("Steam", rid, String(note));
              await loadSteamAccounts();
              pushToast({
                type: "success",
                message: $t("Toast_AccountSaved"),
                duration: 4000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError($t("Toast_SaveFailed"), e),
                duration: 8000,
              });
            }
          },
        },
        {
          label: tr("Context_ManageSubmenu"),
          children: [
            {
              label: tr("Context_GameDataSubmenu"),
              children: gameChildren,
            },
            {
              label: tr("Context_Steam_OpenUserdata"),
              action: async () => {
                try {
                  await SteamService.OpenUserdataFolder(rid);
                } catch (e) {
                  pushToast({
                    type: "error",
                    message: formatToastWithError($t("Toast_LaunchFailed"), e),
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
                  await SteamService.ChangeAccountImage(rid, String(path).trim());
                  await loadSteamAccounts();
                  pushToast({
                    type: "success",
                    message: $t("Toast_AccountSaved"),
                    duration: 4000,
                  });
                } catch (e) {
                  pushToast({
                    type: "error",
                    message: formatToastWithError($t("Toast_SaveFailed"), e),
                    duration: 8000,
                  });
                }
              },
            },
          ],
        },
      ];
    };
  }

  onMount(() => {
    previousPage.set({ page: "home" });
    void (async () => {
      await loadSteamAccounts();
      try {
        const rows = await SteamService.GetInstalledGames();
        installedGames = rows
          .filter((r) => !STEAM_CONTEXT_MENU_HIDDEN_APP_IDS.has(String(r.appId).trim()))
          .map((r) => ({
            appId: r.appId,
            name: r.name,
          }));
      } catch {
        installedGames = [];
      }
    })();
    void GetPlatformExeIcon(name).then((u: string) => platformExeIconUrl.set(u ?? ""));
    offSteamEvent = Events.On("steam-account-updated", (ev) => {
      const raw = ev.data;
      const p =
        raw instanceof AccountPatch
          ? raw
          : AccountPatch.createFrom(raw as Record<string, unknown>);
      applySteamPatch(p);
    });
    offPlatformAction = platformAction.subscribe((v) => {
      if (!v || v.id === lastHandledActionId) {
        return;
      }
      lastHandledActionId = v.id;
      void handlePlatformActionKind(v.kind);
    });
  });

  onDestroy(() => {
    for (const t of steamListRefreshTimers) clearTimeout(t);
    steamListRefreshTimers = [];
    selectedAccountStore.set({ platformKey: "", uniqueId: "" });
    platformAction.set(null);
    offSteamEvent?.();
    offPlatformAction?.();
    platformExeIconUrl.set("");
    actionBarStatus.set("");
  });
</script>

<div class="main-content platform-accounts-root">
  {#if name}
    <div class="platformTable">
      {#if steamLoadError}
        <p class="platform-accounts-hint">{steamLoadError}</p>
      {/if}
      <div class="steam-acclist" bind:this={steamAcclistEl}>
        <ReorderPointerGrid
          items={steamIds}
          listClass="acc_list"
          itemClass="acc_list_item acc_list_item--drag"
          placeholderClass="acc_list_item placeHolderAcc"
          ghostClass="acc_list_item acc_list_item--ghost"
          ariaLabel="Steam accounts"
          on:reorder={onAccountReorder}
          on:itemclick={onSteamItemClick}
        >
          <svelte:fragment slot="item" let:rowId>
            {@const rid = slotKey(rowId)}
            <!-- Inline .find so updates to steamAccounts re-run this block (helpers are not dependency-tracked). -->
            {@const acc = steamAccounts.find((a) => a.steamId64 === rid)}
            {@const radioId = `steam-acc-${rid}`}
            {#key `${rid}-${rowEpoch[rid] ?? 0}`}
            <div class="acc_list_item_inner">
              <input
                type="radio"
                class="acc"
                id={radioId}
                name="steam-accounts"
                value={rid}
                bind:group={selectedSteamId}
                on:change={touchSteamActionBar}
              />
              <label
                for={radioId}
                class="acc"
                class:currentAcc={acc?.currentSession}
                use:ctxMenuAction={{
                  items: steamCtxMenu(rid),
                  beforeOpen: () => {
                    selectedSteamId = rid;
                    touchSteamActionBar();
                  },
                }}
                use:tooltip={acc?.currentSession
                  ? {
                      text: $t("Tooltip_CurrentAccount"),
                      placement: "right",
                      boundary: steamAcclistEl,
                    }
                  : undefined}
                on:dblclick|preventDefault={() => {
                  selectedSteamId = rid;
                  touchSteamActionBar();
                  void steamLoginSelected();
                }}
              >
                <img
                  class:status_vac={acc?.showVac && acc?.vac}
                  class:status_limited={acc?.showLimited && acc?.ltd}
                  src={acc?.imageUrl && !acc?.avatarPending ? acc.imageUrl : PROFILE_PLACEHOLDER}
                  alt=""
                  draggable="false"
                />
                {#if acc?.showAccUsername && acc?.accountName}
                  <p class="streamerCensor">{acc.accountName}</p>
                {/if}
                <h6 class="displayName">{acc?.personaName ?? rid}</h6>
                {#if acc?.showShortNotes && acc?.note?.trim()}
                  <p class="acc_note">{acc.note}</p>
                {/if}
                {#if acc?.showSteamId}
                  <p class="streamerCensor steamId">{acc.steamId64}</p>
                {/if}
                {#if acc?.showLastLogin && acc?.lastLogin}
                  <p class="lastLogin">{formatLastLoginForLocale(acc.lastLogin, $locale)}</p>
                {/if}
                {#if acc?.syncError}
                  <div class="steam_meta_err" title={acc.syncError}>{acc.syncError}</div>
                {:else if acc?.avatarPending}
                  <div class="steam_meta_pending">Updating…</div>
                {/if}
              </label>
            </div>
            {/key}
          </svelte:fragment>
        </ReorderPointerGrid>
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

</style>
