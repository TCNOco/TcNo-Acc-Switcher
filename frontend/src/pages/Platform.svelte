<script lang="ts">
  import { get } from "svelte/store";
  import { onDestroy, onMount } from "svelte";
  import ActionBar from "../components/ActionBar.svelte";
  import ReorderPointerGrid from "../components/ReorderPointerGrid.svelte";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import { actionBarStatus } from "../stores/actionBarStatus";
  import { platformExeIconUrl, platformAction } from "../stores/platformPage";
  import { pushToast } from "../stores/toast";
  import { openPrompt } from "../stores/modal";
  import { t } from "../stores/i18n";
  import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import { AccountDTO } from "../../bindings/TcNo-Acc-Switcher/internal/basic/models.js";
  import { GetPlatformExeIcon, LaunchPlatform } from "../lib/platformBindings";

  const PROFILE_FALLBACK = "/img/BasicDefault.webp";

  type BasicRow = InstanceType<typeof AccountDTO>;

  export let name: string;

  let accounts: BasicRow[] = [];
  let accountIds: string[] = [];
  let loadError = "";
  let selectedUniqueId = "";
  let offPlatformAction: (() => void) | undefined;
  let lastHandledActionId = 0;

  $: appBarTitle.set(name || "TcNo Account Switcher");
  $: if (name) {
    route.set({ page: "platform", platformName: name });
  }

  function accountById(id: string): BasicRow | undefined {
    return accounts.find((a) => a.uniqueId === id);
  }

  function touchStatus(): void {
    const acc = accountById(selectedUniqueId);
    actionBarStatus.set(acc ? acc.displayName || acc.uniqueId : "");
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
      loadError = String(e);
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
      await BasicService.SwapToAccount(name, selectedUniqueId);
      await loadAccounts();
      pushToast({
        type: "success",
        message: get(t)("Toast_AccountSwitched"),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: `${get(t)("Toast_SwitchFailed")} ${String(e)}`,
        duration: 8000,
      });
    }
  }

  async function saveCurrentPrompt(): Promise<void> {
    const displayName = await openPrompt({
      title: get(t)("Modal_SaveCurrent_Title"),
      body: get(t)("Modal_SaveCurrent_Body"),
      positiveLabel: get(t)("Button_SaveCurrent"),
      negativeLabel: get(t)("Button_Cancel"),
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
        message: get(t)("Toast_AccountSaved"),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: `${get(t)("Toast_SaveFailed")} ${String(e)}`,
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
      } catch (e) {
        pushToast({
          type: "error",
          message: `${get(t)("Toast_LaunchFailed")} ${String(e)}`,
          duration: 8000,
        });
      }
      return;
    }
    if (kind === "addNew") {
      try {
        await BasicService.AddNew(name);
        await loadAccounts();
        pushToast({
          type: "success",
          message: get(t)("Toast_AccountSwitched"),
          duration: 4000,
        });
      } catch (e) {
        pushToast({
          type: "error",
          message: `${get(t)("Toast_SwitchFailed")} ${String(e)}`,
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
      <div class="steam-acclist">
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
