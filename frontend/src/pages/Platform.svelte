<script lang="ts">
  import { onDestroy, onMount } from "svelte";
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
  import { openPrompt } from "../stores/modal";
  import { t } from "../stores/i18n";
  import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import { AccountDTO } from "../../bindings/TcNo-Acc-Switcher/internal/basic/models.js";
  import { GetPlatformExeIcon, LaunchPlatform } from "../lib/platformBindings";
  import { formatToastWithError, formatWailsError } from "../lib/formatWailsError";
  import { tooltip } from "../lib/actions/tooltip";
  import { contextMenu as ctxMenuAction } from "../lib/actions/contextMenu";
  import type { MenuItemDef } from "../stores/contextMenu";
  import * as Shortcuts from "../../bindings/TcNo-Acc-Switcher/internal/shortcuts/service.js";
  import { get } from "svelte/store";
  import { openConfirm } from "../stores/modal";

  const PROFILE_FALLBACK = "/img/BasicDefault.webp";

  /** Bindings class typing can lag `currentSession`; keep explicit for the list row. */
  type BasicRow = InstanceType<typeof AccountDTO> & { currentSession?: boolean };

  export let name: string;

  let accounts: BasicRow[] = [];
  let accountIds: string[] = [];
  let loadError = "";
  let selectedUniqueId = "";
  let offPlatformAction: (() => void) | undefined;
  let lastHandledActionId = 0;
  let basicListRefreshTimers: ReturnType<typeof setTimeout>[] = [];
  let basicAcclistEl: HTMLDivElement | undefined;

  $: appBarTitle.set(name || "TcNo Account Switcher");
  $: if (name) {
    route.set({ page: "platform", platformName: name });
  }

  $: selectedAccountStore.set({
    platformKey: name,
    uniqueId: selectedUniqueId,
  });

  function accountById(id: string): BasicRow | undefined {
    return accounts.find((a) => a.uniqueId === id);
  }

  function touchStatus(): void {
    const acc = accountById(selectedUniqueId);
    actionBarStatus.set(acc ? acc.displayName || acc.uniqueId : "");
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

  async function loadAccounts(): Promise<void> {
    loadError = "";
    try {
      const rows = (await BasicService.GetAccounts(name)) as BasicRow[];
      accounts = rows;
      accountIds = rows.map((r) => r.uniqueId);
      const first = rows[0]?.uniqueId ?? "";
      const stillValid = selectedUniqueId && rows.some((r) => r.uniqueId === selectedUniqueId);
      selectedUniqueId = stillValid ? selectedUniqueId : first;
      touchStatus();
    } catch (e) {
      loadError = formatWailsError(e) || String(e);
      accounts = [];
      accountIds = [];
      selectedUniqueId = "";
      actionBarStatus.set("");
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
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SwitchFailed"), e),
        duration: 8000,
      });
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
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
    }
  }

  async function handlePlatformActionKind(
    kind: "login" | "addNew" | "launch" | "saveCurrent",
  ): Promise<void> {
    if (kind === "launch") {
      try {
        await LaunchPlatform(name);
        scheduleAccountsRefresh();
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
        await BasicService.AddNew(name);
        scheduleAccountsRefresh();
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
    void loadAccounts();
    void GetPlatformExeIcon(name).then((u: string) => platformExeIconUrl.set(u ?? ""));
    offPlatformAction = platformAction.subscribe((v) => {
      if (!v || v.id === lastHandledActionId) {
        return;
      }
      lastHandledActionId = v.id;
      void handlePlatformActionKind(v.kind);
    });
  });

  onDestroy(() => {
    for (const t of basicListRefreshTimers) clearTimeout(t);
    basicListRefreshTimers = [];
    selectedAccountStore.set({ platformKey: "", uniqueId: "" });
    platformAction.set(null);
    offPlatformAction?.();
    platformExeIconUrl.set("");
    actionBarStatus.set("");
  });
</script>

<div class="main-content platform-accounts-root">
  {#if name}
    <div class="platformTable">
      {#if loadError}
        <p class="platform-accounts-hint">{loadError}</p>
      {/if}
      <div class="steam-acclist" bind:this={basicAcclistEl}>
        <ReorderPointerGrid
          items={accountIds}
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
                <img
                  src={acc?.imageUrl ? acc.imageUrl : PROFILE_FALLBACK}
                  alt=""
                  draggable="false"
                />
                <h6 class="displayName">{acc?.displayName ?? rid}</h6>
                {#if acc?.note}
                  <p class="acc_note">{acc.note}</p>
                {/if}
              </label>
            </div>
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
