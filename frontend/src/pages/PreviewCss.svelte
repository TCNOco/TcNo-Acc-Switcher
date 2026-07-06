<script lang="ts">
  import { onMount, tick } from "svelte";
  import { get } from "svelte/store";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import { t } from "../stores/i18n";
  import "../styles/Settings.scss";
  import "../styles/HomePlatforms.scss";
  import "../styles/actionbar.scss";
  import "../styles/gameshortcutbar.scss";
  import "../styles/toast.scss";
  import {
    openAlert,
    openAlertNoButton,
    openConfirm,
    openPrompt,
    openFolderPicker,
  } from "../stores/modal";
  import { pushToast } from "../stores/toast";
  import { contextMenu } from "../lib/actions/contextMenu";
  import type { MenuItemDef } from "../stores/contextMenu";
  import { platformIconFgHref } from "../lib/platformIcon";
  import ToastTypeIcon from "../components/ToastTypeIcon.svelte";
  import ReorderPointerGrid from "../components/ReorderPointerGrid.svelte";
  import ThemePickerControls from "../components/ThemePickerControls.svelte";
  import {
    focusShortcutArrowNavigationTarget,
    shortcutArrowNavigation,
  } from "../lib/actions/shortcutArrowNavigation";
  import { controllerSpatialNavigation } from "../lib/actions/controllerSpatialNavigation";

  type PvAccRow = {
    name: string;
    status?: "vac" | "limited";
    current?: boolean;
    steamId: string;
    when: string;
  };

  type PvShortcut = { label: string };
  type PvPlatformEdgeState = {
    id: string;
    label: string;
    className: string;
    selected?: boolean;
    current?: boolean;
    disabled?: boolean;
  };

  function textClass(name: string): string {
    const n = name.length;
    if (n < 7) return "shortText";
    if (n > 12) return "longText";
    return "";
  }

  function slotKey(x: string | null | undefined): string {
    return x ?? "";
  }

  function previewAccountGridLabel(id: string): string {
    const acc = pvAccounts[id];
    if (!acc) return id;
    const parts = [acc.name, acc.steamId, acc.when];
    if (acc.current) parts.push($t("Tooltip_CurrentAccount"));
    if (acc.status === "vac") parts.push("VAC");
    if (acc.status === "limited") parts.push("Limited");
    return parts.filter(Boolean).join(". ");
  }

  let platformIds = [
    "Steam",
    "GeForce Now",
    "BattleNet",
    "Epic Games",
    "Ubisoft",
    "Discord",
  ];

  const platformEdgeStates: PvPlatformEdgeState[] = [
    { id: "Steam", label: "Selected", className: "preview-platform-edge--selected", selected: true },
    { id: "Epic Games", label: "Current", className: "preview-platform-edge--current", current: true },
    { id: "Ubisoft", label: "Disabled", className: "preview-platform-edge--disabled", disabled: true },
    { id: "Discord", label: "Hover", className: "preview-platform-edge--hover" },
    { id: "BattleNet", label: "Drop target", className: "preview-platform-edge--drop-target platform_list_placeholder" },
  ];

  let accIds = ["pv-1", "pv-2", "pv-3", "pv-4", "pv-5"];
  let selectedPvAccId = "pv-1";

  const pvAccounts: Record<string, PvAccRow> = {
    "pv-1": {
      name: "Current Account",
      current: true,
      steamId: "76561198000000001",
      when: "07/03/2022 17:33",
    },
    "pv-2": {
      name: "Normal account",
      steamId: "76561198000000002",
      when: "07/03/2022 12:06",
    },
    "pv-3": {
      name: "Banned account",
      status: "vac",
      steamId: "76561198000000003",
      when: "07/03/2022 9:20",
    },
    "pv-4": {
      name: "Limited account",
      status: "limited",
      steamId: "76561198000000004",
      when: "07/03/2022 6:33",
    },
    "pv-5": {
      name: "Another account",
      steamId: "76561198000000005",
      when: "06/03/2022 14:00",
    },
  };

  let pinnedShortcutIds = ["pv-pin-1", "pv-pin-2"];
  let dropdownShortcutIds = ["pv-dd-a", "pv-dd-b", "pv-dd-c"];
  const pvShortcutMeta: Record<string, PvShortcut> = {
    "pv-pin-1": { label: "TcNo.lnk" },
    "pv-pin-2": { label: "Pinned shortcut" },
    "pv-dd-a": { label: "Shortcut A" },
    "pv-dd-b": { label: "Shortcut B" },
    "pv-dd-c": { label: "Shortcut C" },
  };

  const toastPermanentDurationMs = 2_147_483_647;

  let toastPermanent = false;
  let shortcutDdOpen = false;
  let previewShortcutDropdownEl: HTMLDivElement | null = null;
  let previewShortcutDropdownButtonEl: HTMLButtonElement | null = null;
  let settingsDdOpen = false;
  let modalLog: string[] = [];

  onMount(() => {
    previousPage.set({ page: "settings" });
  });

  $: appBarTitle.set($t("Title_Settings_TestCss"));

  function toastDuration(normalMs: number): number {
    return toastPermanent ? toastPermanentDurationMs : normalMs;
  }

  function noopPlatMenu(): MenuItemDef[] {
    const tr = get(t);
    return [
      { label: tr("Context_CreateShortcut"), action: () => {} },
      { label: tr("Context_HidePlatform"), action: () => {} },
    ];
  }

  function noopAccMenu(): MenuItemDef[] {
    const tr = get(t);
    return [
      { label: tr("Context_SwapTo"), action: () => {} },
      { label: tr("Context_ChangeName"), action: () => {} },
    ];
  }

  async function focusPreviewDropdownShortcut(edge: "first" | "last" = "first"): Promise<void> {
    await tick();
    focusShortcutArrowNavigationTarget(previewShortcutDropdownEl, edge);
  }

  function onPreviewShortcutDropdownKeydown(e: KeyboardEvent): void {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") {
      return;
    }
    e.preventDefault();
    e.stopPropagation();
    shortcutDdOpen = true;
    void focusPreviewDropdownShortcut(e.key === "ArrowUp" ? "last" : "first");
  }

  function closePreviewShortcutDropdownFromKeyboard(): void {
    if (!shortcutDdOpen) {
      return;
    }
    shortcutDdOpen = false;
    previewShortcutDropdownButtonEl?.focus({ preventScroll: true });
  }

  function openPreviewShortcutDropdownFromHotbarUp(): void {
    shortcutDdOpen = true;
    void focusPreviewDropdownShortcut("last");
  }

  function canOpenPreviewShortcutDropdownFromUp(active: HTMLElement): boolean {
    return active.closest(".shortcutDropdownWrap") instanceof HTMLElement;
  }

  function logModal(kind: string, detail: string) {
    const line = `${new Date().toLocaleTimeString()} — ${kind}: ${detail}`;
    modalLog = [line, ...modalLog].slice(0, 40);
  }

  async function testAlert() {
    await openAlert({
      title: $t("Preview_Modals"),
      body: $t("Modal_ConfirmAction"),
    });
    logModal("openAlert", "resolved");
  }

  async function testAlertNoButton() {
    await openAlertNoButton({
      title: $t("Preview_Modals"),
      body: $t("Modal_ConfirmAction"),
    });
    logModal("openAlertNoButton", "resolved");
  }

  async function testConfirmYesNo() {
    const r = await openConfirm({
      title: $t("Preview_Modals"),
      body: $t("Modal_ConfirmAction"),
      style: "yesno",
    });
    logModal("openConfirm (yes/no)", JSON.stringify(r));
  }

  async function testConfirmOkCancel() {
    const r = await openConfirm({
      title: $t("Preview_Modals"),
      body: $t("Modal_ConfirmAction"),
      style: "okcancel",
    });
    logModal("openConfirm (OK/cancel)", JSON.stringify(r));
  }

  async function testPromptText() {
    const r = await openPrompt({
      title: $t("Preview_Modals"),
      body: `${$t("Modal_ChangeUsername")}`,
      inputType: "text",
      initialValue: "demo",
    });
    logModal("openPrompt (text)", r === null ? "null (cancel)" : JSON.stringify(r));
  }

  async function testPromptPassword() {
    const r = await openPrompt({
      title: $t("Preview_Modals"),
      body: $t("Modal_SetPassword"),
      inputType: "password",
    });
    logModal("openPrompt (password)", r === null ? "null (cancel)" : `(length ${r.length})`);
  }

  async function testFolderPicker() {
    const r = await openFolderPicker({
      title: $t("Preview_Modals"),
      body: $t("Modal_SetUserdata"),
      initialPath: "",
    });
    logModal("openFolderPicker", r === null ? "null (cancel)" : JSON.stringify(r));
  }

  async function testFolderPickerWithFiles() {
    const r = await openFolderPicker({
      title: $t("Preview_Modals"),
      body: $t("Modal_SetBackground"),
      initialPath: "",
      dirsOnly: false,
      soughtFilename: "package.json",
      positiveLabel: $t("Modal_SetBackground_ChooseImage"),
    });
    logModal("openFolderPicker (dirsOnly: false)", r === null ? "null (cancel)" : JSON.stringify(r));
  }

  function toastSuccess() {
    pushToast({
      type: "success",
      title: "Saved",
      message: "Settings were applied (test toast).",
      duration: toastDuration(6000),
    });
  }

  function toastWarning() {
    pushToast({
      type: "warning",
      title: "Heads up",
      message: "Something may need your attention.",
      duration: toastDuration(8000),
    });
  }

  function toastError() {
    pushToast({
      type: "error",
      title: "",
      message: "A critical component could not be loaded. Please restart the application! (test)",
      duration: toastDuration(10000),
    });
  }

  function toastInfo() {
    pushToast({
      type: "info",
      title: "FYI",
      message: "This is an informational toast.",
      duration: toastDuration(5000),
    });
  }

  function toastViaWindowNotification() {
    window.notification?.new({
      type: "success",
      title: "window.notification",
      message: "Dispatched via window.notification.new (JS bridge style).",
      duration: toastDuration(5000),
    });
  }
</script>

<div class="main-content main-spacing preview-css-page" use:controllerSpatialNavigation>
  <h1 class="SettingsHeader">{$t("Settings_PreviewCssHeader")}</h1>
  <p class="preview-css-intro">{$t("Settings_PreviewCss")}</p>

  <h2 class="SettingsHeader">{$t("Settings_Header_Theme")}</h2>
  <ThemePickerControls />

  <h2 class="SettingsHeader">{$t("Preview_Platforms")}</h2>
  <div class="preview_element preview_program_main">
    <div class="platformTable">
      <ReorderPointerGrid
        items={platformIds}
        listClass="platform_list"
        itemClass="platform_list_item platform_list_item--draggable"
        placeholderClass="platform_list_item platform_list_placeholder"
        ghostClass="platform_list_item platform_list_item--ghost"
        ariaLabel={$t("Preview_Platforms")}
        listRole="group"
        itemRole="button"
        itemAriaLabel={(id) => $t("Aria_OpenPlatform", { platform: id })}
        itemActivatesOnSpace={true}
        on:reorder={(e) => {
          platformIds = e.detail.items;
        }}
        on:itemclick={() => {}}
      >
        <svelte:fragment slot="item" let:rowId>
          {@const rid = slotKey(rowId)}
          <!-- svelte-ignore a11y-no-static-element-interactions -->
          <div class="platform_tile_ctx" use:contextMenu={noopPlatMenu} role="presentation">
            <div class="fgText {textClass(rid)}">
              <p>{rid.toUpperCase()}</p>
            </div>
            <div class="fgImg" aria-hidden="true">
              <svg viewBox="0 0 500 500" aria-hidden="true">
                <use href={platformIconFgHref(rid)} class="icoFG" />
              </svg>
            </div>
            <svg viewBox="0 0 2084 2084" class="icoBG" aria-hidden="true">
              <use href="img/platform/glass.svg#GLASS" class="icoGlass" />
            </svg>
          </div>
        </svelte:fragment>
      </ReorderPointerGrid>
    </div>
    <footer class="actionbar preview_fake_actionbar">
      <span class="actionbar__status">{$t("Preview_Platforms")}</span>
      <div class="actionbar__actions">
        <button type="button" class="btnicontext" aria-hidden="true">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512" aria-hidden="true"
            ><path
              d="M416 208H272V64c0-17.67-14.33-32-32-32h-32c-17.67 0-32 14.33-32 32v144H32c-17.67 0-32 14.33-32 32v32c0 17.67 14.33 32 32 32h144v144c0 17.67 14.33 32 32 32h32c17.67 0 32-14.33 32-32V304h144c17.67 0 32-14.33 32-32v-32c0-17.67-14.33-32-32-32z"
            /></svg
          >{$t("Button_ManagePlatforms")}</button
        >
        <button type="button" class="square" aria-hidden="true"
          ><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true"
            ><path
              d="M487.4 315.7l-42.6-24.6c4.3-23.2 4.3-47 0-70.2l42.6-24.6c4.9-2.8 7.1-8.6 5.5-14-11.1-35.6-30-67.8-54.7-94.6-3.8-4.1-10-5.1-14.8-2.3L380.8 110c-17.9-15.4-38.5-27.3-60.8-35.1V25.8c0-5.6-3.9-10.5-9.4-11.7-36.7-8.2-74.3-7.8-109.2 0-5.5 1.2-9.4 6.1-9.4 11.7V75c-22.2 7.9-42.8 19.8-60.8 35.1L88.7 85.5c-4.9-2.8-11-1.9-14.8 2.3-24.7 26.7-43.6 58.9-54.7 94.6-1.7 5.4.6 11.2 5.5 14L67.3 221c-4.3 23.2-4.3 47 0 70.2l-42.6 24.6c-4.9 2.8-7.1 8.6-5.5 14 11.1 35.6 30 67.8 54.7 94.6 3.8 4.1 10 5.1 14.8 2.3l42.6-24.6c17.9 15.4 38.5 27.3 60.8 35.1v49.2c0 5.6 3.9 10.5 9.4 11.7 36.7 8.2 74.3 7.8 109.2 0 5.5-1.2 9.4-6.1 9.4-11.7v-49.2c22.2-7.9 42.8-19.8 60.8-35.1l42.6 24.6c4.9 2.8 11 1.9 14.8-2.3 24.7-26.7 43.6-58.9 54.7-94.6 1.5-5.5-.7-11.3-5.6-14.1zM256 336c-44.1 0-80-35.9-80-80s35.9-80 80-80 80 35.9 80 80-35.9 80-80 80z"
            /></svg
          ></button
        >
        <button type="button" class="square" aria-hidden="true"
          ><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 384 512" aria-hidden="true"
            ><path
              d="M202.021 0C122.202 0 70.503 32.703 29.914 91.026c-7.363 10.58-5.093 25.086 5.178 32.874l43.138 32.709c10.373 7.865 25.132 6.026 33.253-4.148 25.049-31.381 43.63-49.449 82.757-49.449 30.764 0 68.816 19.799 68.816 49.631 0 22.552-18.617 34.134-48.993 51.164-35.423 19.86-82.299 44.576-82.299 106.405V320c0 13.255 10.745 24 24 24h72.471c13.255 0 24-10.745 24-24v-5.773c0-42.86 125.268-44.645 125.268-160.627C377.504 66.256 286.902 0 202.021 0zM192 373.459c-38.196 0-69.271 31.075-69.271 69.271 0 38.195 31.075 69.27 69.271 69.27s69.271-31.075 69.271-69.271-31.075-69.27-69.271-69.27z"
            /></svg
          ></button
        >
      </div>
    </footer>
  </div>

  <div class="preview_element preview-platform-edge-wrap">
    <h3 class="SettingsHeader preview-edge-subhead">Platform tile edge states</h3>
    <div class="platform_list preview-platform-edge-list" role="listbox" aria-label="Platform tile edge states">
      {#each platformEdgeStates as state}
        <div
          class={`platform_list_item platform_list_item--draggable preview-platform-edge ${state.className}`}
          role="option"
          aria-selected={state.selected ? "true" : "false"}
          aria-current={state.current ? "page" : undefined}
          aria-disabled={state.disabled ? "true" : undefined}
          tabindex={state.disabled ? -1 : 0}
        >
          <div class="platform_tile_ctx" role="presentation">
            <span class="preview-platform-state-badge">{state.label}</span>
            <div class="fgText {textClass(state.id)}">
              <p>{state.id.toUpperCase()}</p>
            </div>
            <div class="fgImg" aria-hidden="true">
              <svg viewBox="0 0 500 500" aria-hidden="true">
                <use href={platformIconFgHref(state.id)} class="icoFG" />
              </svg>
            </div>
            <svg viewBox="0 0 2084 2084" class="icoBG" aria-hidden="true">
              <use href="img/platform/glass.svg#GLASS" class="icoGlass" />
            </svg>
          </div>
        </div>
      {/each}
    </div>
  </div>

  <h2 class="SettingsHeader">{$t("Preview_Accounts")}</h2>
  <div class="preview_element preview_accounts_wrap">
    <div class="preview-acc-list-wrap" id="acc_list" aria-label={$t("Preview_Accounts")}>
      <ReorderPointerGrid
        items={accIds}
        listClass="acc_list"
        itemClass="acc_list_item acc_list_item--drag"
        placeholderClass="acc_list_item placeHolderAcc"
        ghostClass="acc_list_item acc_list_item--ghost"
        ariaLabel={$t("Preview_Accounts")}
        itemAriaLabel={previewAccountGridLabel}
        activeItem={selectedPvAccId}
        selectOnArrow={true}
        on:reorder={(e) => {
          accIds = e.detail.items;
        }}
        on:itemclick={(e) => {
          selectedPvAccId = e.detail.id;
        }}
      >
        <svelte:fragment slot="item" let:rowId>
          {@const rid = slotKey(rowId)}
          {@const acc = pvAccounts[rid]}
          {#if acc}
            {@const radioId = `pv-acc-${rid}`}
            <div class="acc_list_item_inner preview_list_item">
              <input
                type="radio"
                class="acc"
                id={radioId}
                name="pv-accounts"
                value={rid}
                tabindex="-1"
                bind:group={selectedPvAccId}
              />
              <label
                for={radioId}
                class="acc"
                class:currentAcc={acc.current}
                title={acc.current ? $t("Tooltip_CurrentAccount") : undefined}
                use:contextMenu={noopAccMenu}
              >
                <img
                  src="/img/BasicDefault.webp"
                  alt=""
                  draggable="false"
                  class:status_vac={acc.status === "vac"}
                  class:status_limited={acc.status === "limited"}
                />
                <h6>{acc.name}</h6>
                <p class="streamerCensor steamId">{acc.steamId}</p>
                <p>{acc.when}</p>
              </label>
            </div>
          {/if}
        </svelte:fragment>
      </ReorderPointerGrid>
    </div>

    <!-- overflow:visible so #shortcutDropdown (above the bar) is not clipped; .acc_list_actionbar defaults to overflow:hidden -->
    <div class="acc_list_actionbar preview_accounts_actionbar">
      <div class="statusBar">
        <div></div>
        <input id="pvCurrentStatus" type="text" value="" readonly spellcheck="false" tabindex="-1" />
      </div>
      <div class="gameShortcuts">
        <div class="preview_shortcut_bar">
          <div
            class="gameShortcutBar"
            role="group"
            aria-label={$t("Stats_GameShortcutsHotbar")}
            use:shortcutArrowNavigation={{
              capture: true,
              onEscape: closePreviewShortcutDropdownFromKeyboard,
              canOpenDropdownFromUp: canOpenPreviewShortcutDropdownFromUp,
              onHotbarUp: openPreviewShortcutDropdownFromHotbarUp,
            }}
          >
            <ReorderPointerGrid
              items={pinnedShortcutIds}
              listClass="shortcuts shortcutDndGrid"
              itemClass="shortcutDndCell"
              placeholderClass="shortcutDndGap shortcutPlaceholder"
              ghostClass="shortcutDndGhost"
              ariaLabel={$t("Stats_GameShortcuts")}
              on:reorder={(e) => {
                pinnedShortcutIds = e.detail.items;
              }}
              on:itemclick={() => {}}
            >
              <svelte:fragment slot="item" let:rowId>
                {@const sid = slotKey(rowId)}
                {@const meta = pvShortcutMeta[sid]}
                <button type="button" class="HasContextMenu" aria-label={meta?.label ?? sid}>
                  <img src="/img/BasicDefault.webp" alt="" draggable="false" />
                </button>
              </svelte:fragment>
            </ReorderPointerGrid>
            <div class="shortcutDropdownWrap">
              <button
                bind:this={previewShortcutDropdownButtonEl}
                type="button"
                id="shortcutDropdownBtn"
                class="square"
                class:flip={shortcutDdOpen}
                aria-expanded={shortcutDdOpen}
                aria-label={$t("Tooltip_ExpandShortcuts")}
                on:click={() => (shortcutDdOpen = !shortcutDdOpen)}
                on:keydown={onPreviewShortcutDropdownKeydown}
              >
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512" aria-hidden="true"
                  ><path
                    d="M201.4 137.4c12.5-12.5 32.8-12.5 45.3 0l160 160c12.5 12.5 12.5 32.8 0 45.3s-32.8 12.5-45.3 0L224 218.7 86.6 342.6c-12.5 12.5-32.8 12.5-45.3 0s-12.5-32.8 0-45.3l160-160z"
                  /></svg
                >
              </button>
              {#if shortcutDdOpen}
                <div bind:this={previewShortcutDropdownEl} class="shortcutDropdown gameShortcuts open" id="shortcutDropdown">
                  <ReorderPointerGrid
                    items={dropdownShortcutIds}
                    listClass="shortcutDropdownItems shortcutDndGrid"
                    itemClass="shortcutDndCell"
                    placeholderClass="shortcutDndGap shortcutPlaceholder"
                    ghostClass="shortcutDndGhost"
                    ariaLabel={$t("Stats_GameShortcuts")}
                    on:reorder={(e) => {
                      dropdownShortcutIds = e.detail.items;
                    }}
                    on:itemclick={() => {}}
                  >
                    <svelte:fragment slot="item" let:rowId>
                      {@const sid = slotKey(rowId)}
                      {@const meta = pvShortcutMeta[sid]}
                      <button type="button" class="HasContextMenu" aria-label={meta?.label ?? sid}>
                        <img src="/img/BasicDefault.webp" alt="" draggable="false" />
                      </button>
                    </svelte:fragment>
                  </ReorderPointerGrid>
                  <button type="button" id="btnOpenShortcutFolder" aria-label={$t("Tooltip_ShortcutFolder")}>
                    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512" aria-hidden="true"
                      ><path
                        d="M416 208H272V64c0-17.67-14.33-32-32-32h-32c-17.67 0-32 14.33-32 32v144H32c-17.67 0-32 14.33-32 32v32c0 17.67 14.33 32 32 32h144v144c0 17.67 14.33 32 32 32h32c17.67 0 32-14.33 32-32V304h144c17.67 0 32-14.33 32-32v-32c0-17.67-14.33-32-32-32z"
                      /></svg
                    >
                  </button>
                </div>
              {/if}
            </div>
            <button type="button" id="btnStartPlat" aria-label={$t("Button_Launch")}>
              <img src="/img/platform/Steam.svg" alt="" draggable="false" />
            </button>
          </div>
          <button type="button" id="btnAddNew" class="btnicontext" aria-label={$t("Button_AddNew")}>
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512" aria-hidden="true"
              ><path
                d="M416 208H272V64c0-17.67-14.33-32-32-32h-32c-17.67 0-32 14.33-32 32v144H32c-17.67 0-32 14.33-32 32v32c0 17.67 14.33 32 32 32h144v144c0 17.67 14.33 32 32 32h32c17.67 0 32-14.33 32-32V304h144c17.67 0 32-14.33 32-32v-32c0-17.67-14.33-32-32-32z"
              /></svg
            >{$t("Button_AddNew")}</button
          >
          <button type="button" class="btnicontext actionbar__login" aria-label={$t("Button_Login")}>
            {$t("Button_Login")}<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true">
              <path
                d="M217.9 105.9L340.7 228.7c7.2 7.2 11.3 17 11.3 27.3s-4.1 20.1-11.3 27.3L217.9 406.1c-6.4 6.4-15 9.9-24 9.9c-18.7 0-33.9-15.2-33.9-33.9V160c0-18.7 15.2-33.9 33.9-33.9c9 0 17.6 3.5 24 9.9z"
              /></svg
            >
          </button>
          <button type="button" id="pvSettingsButton" class="square" aria-label={$t("Tooltip_Settings")}>
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true"
              ><path
                d="M487.4 315.7l-42.6-24.6c4.3-23.2 4.3-47 0-70.2l42.6-24.6c4.9-2.8 7.1-8.6 5.5-14-11.1-35.6-30-67.8-54.7-94.6-3.8-4.1-10-5.1-14.8-2.3L380.8 110c-17.9-15.4-38.5-27.3-60.8-35.1V25.8c0-5.6-3.9-10.5-9.4-11.7-36.7-8.2-74.3-7.8-109.2 0-5.5 1.2-9.4 6.1-9.4 11.7V75c-22.2 7.9-42.8 19.8-60.8 35.1L88.7 85.5c-4.9-2.8-11-1.9-14.8 2.3-24.7 26.7-43.6 58.9-54.7 94.6-1.7 5.4.6 11.2 5.5 14L67.3 221c-4.3 23.2-4.3 47 0 70.2l-42.6 24.6c-4.9 2.8-7.1 8.6-5.5 14 11.1 35.6 30 67.8 54.7 94.6 3.8 4.1 10 5.1 14.8 2.3l42.6-24.6c17.9 15.4 38.5 27.3 60.8 35.1v49.2c0 5.6 3.9 10.5 9.4 11.7 36.7 8.2 74.3 7.8 109.2 0 5.5-1.2 9.4-6.1 9.4-11.7v-49.2c22.2-7.9 42.8-19.8 60.8-35.1l42.6 24.6c4.9 2.8 11 1.9 14.8-2.3 24.7-26.7 43.6-58.9 54.7-94.6 1.5-5.5-.7-11.3-5.6-14.1zM256 336c-44.1 0-80-35.9-80-80s35.9-80 80-80 80 35.9 80 80-35.9 80-80 80z"
              /></svg
            >
          </button>
          <button type="button" id="pvInfoButton" class="square" aria-label={$t("Tooltip_Info")}>
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 384 512" aria-hidden="true"
              ><path
                d="M202.021 0C122.202 0 70.503 32.703 29.914 91.026c-7.363 10.58-5.093 25.086 5.178 32.874l43.138 32.709c10.373 7.865 25.132 6.026 33.253-4.148 25.049-31.381 43.63-49.449 82.757-49.449 30.764 0 68.816 19.799 68.816 49.631 0 22.552-18.617 34.134-48.993 51.164-35.423 19.86-82.299 44.576-82.299 106.405V320c0 13.255 10.745 24 24 24h72.471c13.255 0 24-10.745 24-24v-5.773c0-42.86 125.268-44.645 125.268-160.627C377.504 66.256 286.902 0 202.021 0zM192 373.459c-38.196 0-69.271 31.075-69.271 69.271 0 38.195 31.075 69.27 69.271 69.27s69.271-31.075 69.271-69.271-31.075-69.27-69.271-69.27z"
              /></svg
            >
          </button>
        </div>
      </div>
    </div>
  </div>

  <div class="preview_element preview-account-edge-wrap">
    <h3 class="SettingsHeader preview-edge-subhead">Account row edge states</h3>
    <div class="preview-account-edge-grid" aria-label="Account row edge states">
      <div class="acc_list_item_inner preview_list_item preview-account-edge">
        <input
          type="radio"
          class="acc"
          id="pv-acc-edge-disabled"
          name="pv-acc-edge"
          value="disabled"
          disabled
          tabindex="-1"
        />
        <label for="pv-acc-edge-disabled" class="acc preview-account-edge--disabled" aria-disabled="true">
          <img src="/img/BasicDefault.webp" alt="" draggable="false" />
          <h6>Disabled account</h6>
          <p class="streamerCensor steamId">76561198000000006</p>
          <p>Unavailable</p>
        </label>
      </div>

      <div class="acc_list_item_inner preview_list_item preview-account-edge">
        <input type="radio" class="acc" id="pv-acc-edge-broken" name="pv-acc-edge" value="broken" tabindex="-1" />
        <label for="pv-acc-edge-broken" class="acc acc--broken preview-account-edge--broken">
          <img src="/img/BasicDefault.webp" alt="" draggable="false" />
          <h6>Broken data</h6>
          <span class="acc_broken_badge">Data needs repair</span>
          <p class="streamerCensor steamId">76561198000000007</p>
          <p>Missing fields</p>
        </label>
      </div>

      <div class="acc_list_item_inner preview_list_item preview-account-edge">
        <input type="radio" class="acc" id="pv-acc-edge-hover" name="pv-acc-edge" value="hover" tabindex="-1" />
        <label for="pv-acc-edge-hover" class="acc preview-account-edge--hover">
          <img src="/img/BasicDefault.webp" alt="" draggable="false" />
          <h6>Hover account</h6>
          <p class="streamerCensor steamId">76561198000000008</p>
          <p>Hover state</p>
        </label>
      </div>

      <div class="acc_list_item_inner preview_list_item preview-account-edge">
        <input type="radio" class="acc" id="pv-acc-edge-drop" name="pv-acc-edge" value="drop" tabindex="-1" />
        <label for="pv-acc-edge-drop" class="acc acc--profile-drop-target acc--drop-target preview-account-edge--drop-target">
          <div class="acc_profile_drop_overlay acc_profile_drop_overlay--hover" aria-hidden="true">
            <div class="acc_profile_drop_overlay__center">
              <div class="acc_profile_drop_overlay__icon" aria-hidden="true">
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"
                  ><path fill="currentColor" d="M5 20h14v-2H5v2zM19 9h-4V3H9v6H5l7 7 7-7z" /></svg
                >
              </div>
              <span class="acc_profile_drop_overlay__label">{$t("Drop_SetAccountIcon")}</span>
            </div>
          </div>
          <img src="/img/BasicDefault.webp" alt="" draggable="false" />
          <h6>Drop target</h6>
          <p class="streamerCensor steamId">76561198000000009</p>
          <p>Profile image</p>
        </label>
      </div>
    </div>
  </div>

  <h2 class="SettingsHeader">{$t("Preview_OverlayDropReceivers")}</h2>
  <div class="preview_element preview-overlay-drop-block">
    <p class="preview-overlay-drop-intro">{$t("Settings_PreviewOverlayDropReceivers")}</p>
    <div class="preview-overlay-drop-grid">
      <figure class="preview-overlay-drop-cell">
        <figcaption class="preview-overlay-drop-caption">{$t("Overlay_ProfileImageTitle")}</figcaption>
        <div class="acc-img-overlay acc-img-overlay--preview" role="presentation">
          <span class="acc-img-overlay__x" aria-hidden="true">&times;</span>
          <div class="acc-img-overlay__panel" role="dialog" aria-labelledby="pv-acc-img-overlay-title">
            <h2 id="pv-acc-img-overlay-title" class="acc-img-overlay__title">
              {$t("Overlay_ProfileImageTitle")}
            </h2>
            <p class="acc-img-overlay__hint">
              {$t("Overlay_ProfileImageHint", { name: pvAccounts["pv-1"].name })}
            </p>
            <div class="acc-img-overlay__dropzone">
              <span class="acc-img-overlay__dropicon" aria-hidden="true">
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="48" height="48"
                  ><path fill="currentColor" d="M5 20h14v-2H5v2zM19 9h-4V3H9v6H5l7 7 7-7z" /></svg
                >
              </span>
              <span class="acc-img-overlay__cta">{$t("Overlay_ProfileImageClickToBrowse")}</span>
            </div>
            <div class="acc-img-overlay__remove">{$t("Context_RemoveProfileImage")}</div>
          </div>
        </div>
      </figure>
      <figure class="preview-overlay-drop-cell">
        <figcaption class="preview-overlay-drop-caption">{$t("DropOverlay_CopyShortcut")}</figcaption>
        <div class="fileDropOverlay fileDropOverlay--preview" aria-hidden="true">
          <div class="fileDropOverlay__inner">
            <div class="fileDropOverlay__icon" aria-hidden="true">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"
                ><path
                  fill="currentColor"
                  d="M5 20h14v-2H5v2zM19 9h-4V3H9v6H5l7 7 7-7z"
                /></svg
              >
            </div>
            <p class="fileDropOverlay__text">{$t("DropOverlay_CopyShortcut")}</p>
          </div>
        </div>
      </figure>
    </div>

    <h3 class="SettingsHeader preview-overlay-drop-subhead">{$t("Preview_AccountDropOnRows")}</h3>
    <div class="preview-overlay-drop-grid">
      <figure class="preview-overlay-drop-cell preview-overlay-drop-cell--accrow">
        <figcaption class="preview-overlay-drop-caption">{$t("Preview_AccountDropRowNormal")}</figcaption>
        <div class="preview-overlay-accrow-host" role="presentation">
          <div class="acc_list_item_inner preview_list_item">
            <input
              type="radio"
              class="acc"
              id="pv-acc-drop-normal"
              name="pv-acc-drop-row-a"
              value="normal"
              tabindex="-1"
            />
            <label for="pv-acc-drop-normal" class="acc">
              <img src="/img/BasicDefault.webp" alt="" draggable="false" />
              <h6>{pvAccounts["pv-2"].name}</h6>
              <p class="streamerCensor steamId">{pvAccounts["pv-2"].steamId}</p>
              <p>{pvAccounts["pv-2"].when}</p>
            </label>
          </div>
        </div>
      </figure>
      <figure class="preview-overlay-drop-cell preview-overlay-drop-cell--accrow">
        <figcaption class="preview-overlay-drop-caption">{$t("Preview_AccountDropRowDragHover")}</figcaption>
        <div class="preview-overlay-accrow-host" role="presentation">
          <div class="acc_list_item_inner preview_list_item">
            <input
              type="radio"
              class="acc"
              id="pv-acc-drop-hover"
              name="pv-acc-drop-row-b"
              value="hover"
              tabindex="-1"
            />
            <label for="pv-acc-drop-hover" class="acc acc--profile-drop-target acc--drop-target">
              <div class="acc_profile_drop_overlay acc_profile_drop_overlay--hover" aria-hidden="true">
                <div class="acc_profile_drop_overlay__center">
                  <div class="acc_profile_drop_overlay__icon" aria-hidden="true">
                    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"
                      ><path
                        fill="currentColor"
                        d="M5 20h14v-2H5v2zM19 9h-4V3H9v6H5l7 7 7-7z"
                      /></svg
                    >
                  </div>
                  <span class="acc_profile_drop_overlay__label">{$t("Drop_SetAccountIcon")}</span>
                </div>
              </div>
              <img src="/img/BasicDefault.webp" alt="" draggable="false" />
              <h6>{pvAccounts["pv-2"].name}</h6>
              <p class="streamerCensor steamId">{pvAccounts["pv-2"].steamId}</p>
              <p>{pvAccounts["pv-2"].when}</p>
            </label>
          </div>
        </div>
      </figure>
    </div>
  </div>

  <h2 class="SettingsHeader">{$t("Preview_Notifications")}</h2>
  <div class="preview_element">
    <div class="preview-static-toast-host">
      <div class="toast-stack preview-static-toast-stack">
        <div class="toast toast--success preview-static-toast" role="status">
          <button
            type="button"
            class="toast__close"
              aria-label={$t("Aria_DismissNotification")}
            tabindex="-1"
            on:click|preventDefault|stopPropagation={() => {}}
          >
            <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
              <path
                fill="currentColor"
                d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
              />
            </svg>
          </button>
          <div class="toast__row">
            <div class="toast__icon" aria-hidden="true">
              <ToastTypeIcon type="success" />
            </div>
            <div class="toast__text">
              <div class="toast__title">Saved</div>
              <div class="toast__message">Settings were applied</div>
            </div>
          </div>
        </div>
        <div class="toast toast--warning preview-static-toast" role="status">
          <button
            type="button"
            class="toast__close"
              aria-label={$t("Aria_DismissNotification")}
            tabindex="-1"
            on:click|preventDefault|stopPropagation={() => {}}
          >
            <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
              <path
                fill="currentColor"
                d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
              />
            </svg>
          </button>
          <div class="toast__row">
            <div class="toast__icon" aria-hidden="true">
              <ToastTypeIcon type="warning" />
            </div>
            <div class="toast__text">
              <div class="toast__title">Heads up</div>
              <div class="toast__message">Something may need your attention.</div>
            </div>
          </div>
        </div>
        <div class="toast toast--error preview-static-toast" role="status">
          <button
            type="button"
            class="toast__close"
              aria-label={$t("Aria_DismissNotification")}
            tabindex="-1"
            on:click|preventDefault|stopPropagation={() => {}}
          >
            <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
              <path
                fill="currentColor"
                d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
              />
            </svg>
          </button>
          <div class="toast__row">
            <div class="toast__icon" aria-hidden="true">
              <ToastTypeIcon type="error" />
            </div>
            <div class="toast__text">
              <div class="toast__message">A critical component could not be loaded.</div>
            </div>
          </div>
        </div>
        <div class="toast toast--info preview-static-toast" role="status">
          <button
            type="button"
            class="toast__close"
              aria-label={$t("Aria_DismissNotification")}
            tabindex="-1"
            on:click|preventDefault|stopPropagation={() => {}}
          >
            <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
              <path
                fill="currentColor"
                d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
              />
            </svg>
          </button>
          <div class="toast__row">
            <div class="toast__icon" aria-hidden="true">
              <ToastTypeIcon type="info" />
            </div>
            <div class="toast__text">
              <div class="toast__title">FYI</div>
              <div class="toast__message">This is an informational toast.</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
  <h3 class="SettingsHeader preview-toast-live-heading">Live toasts</h3>
  <div class="modalTestPanel">
    <label class="toastPermanentRow">
      <input type="checkbox" class="toastPermanentCheckbox" bind:checked={toastPermanent} />
      <span>Permanent</span>
      <span class="toastPermanentNote">(× to close)</span>
    </label>
    <div class="modalTestButtons">
      <button type="button" class="btnicontext" on:click={toastSuccess}>Toast success</button>
      <button type="button" class="btnicontext" on:click={toastWarning}>Toast warning</button>
      <button type="button" class="btnicontext" on:click={toastError}>Toast error</button>
      <button type="button" class="btnicontext" on:click={toastInfo}>Toast info</button>
      <button type="button" class="btnicontext" on:click={toastViaWindowNotification}>window.notification</button>
      <button type="button" id="pvDisabledButton" class="btnicontext" disabled>Disabled button</button>
    </div>
  </div>

  <h2 class="SettingsHeader">{$t("Preview_Modals")}</h2>
  <div class="modalTestPanel">
    <pre class="modalTestOutput" aria-live="polite"
      >{#if modalLog.length === 0}<span class="modalTestPlaceholder">{$t("Preview_Modals")} — run a test below.</span
      >{:else}{modalLog.join("\n")}{/if}</pre
    >
    <div class="modalTestButtons">
      <button type="button" class="btnicontext" on:click={() => void testAlert()}>Alert</button>
      <button type="button" class="btnicontext" on:click={() => void testAlertNoButton()}>Alert (no button)</button>
      <button type="button" class="btnicontext" on:click={() => void testConfirmYesNo()}>Confirm Yes/No</button>
      <button type="button" class="btnicontext" on:click={() => void testConfirmOkCancel()}>Confirm OK/Cancel</button>
      <button type="button" class="btnicontext" on:click={() => void testPromptText()}>Prompt (text)</button>
      <button type="button" class="btnicontext" on:click={() => void testPromptPassword()}>Prompt (password)</button>
      <button type="button" class="btnicontext" on:click={() => void testFolderPicker()}>Folder picker</button>
      <button type="button" class="btnicontext" on:click={() => void testFolderPickerWithFiles()}>Folder + files</button>
    </div>
  </div>

  <h2 class="SettingsHeader">{$t("Preview_Settings")}</h2>
  <div class="preview_element">
    <div class="container mainblock">
      <div class="row">
        <div class="col-md-12 col-lg-9 col-xl-8 mx-auto settingsCol">
          <div class="form-text">
            <span>{$t("Settings_ImageExpiry")}</span>
            <input type="number" id="pvImageExpiry" min="0" max="365" value="30" readonly tabindex="-1" />
          </div>
          <div class="rowSetting">
            <div class="form-check">
              <input id="pvStreamer" class="form-check-input" type="checkbox" checked disabled />
              <label class="form-check-label" for="pvStreamer"></label>
            </div>
            <label for="pvStreamer">{$t("Settings_StreamerMode")}</label>
          </div>
          <div class="rowSetting">
            <span>Example text</span>
            <input type="text" id="pvText" value="Read-only sample" readonly tabindex="-1" />
          </div>
          <div class="rowSetting">
            <span>Placeholder text</span>
            <input type="text" id="pvPlaceholderText" placeholder="Placeholder sample" readonly tabindex="-1" />
          </div>
          <div class="rowSetting">
            <span>Invalid text</span>
            <input type="text" id="pvInvalidText" value="Invalid sample" aria-invalid="true" readonly tabindex="-1" />
          </div>
          <div class="rowSetting">
            <span>Disabled text</span>
            <input type="text" id="pvDisabledText" value="Disabled sample" disabled tabindex="-1" />
          </div>
          <h2 class="SettingsHeader">{$t("Settings_Header_Program")}</h2>
          <div class="rowSetting rowDropdown">
            <span>{$t("Settings_Header_ActiveBrowser")}</span>
            <div class="dropdown" class:show={settingsDdOpen}>
              <button type="button" class="dropdown-toggle" on:click={() => (settingsDdOpen = !settingsDdOpen)}>
                WebView
                <span class="caret" aria-hidden="true"></span>
              </button>
              {#if settingsDdOpen}
                <ul
                  class="custom-dropdown-menu dropdown-menu"
                >
                  <li role="none">
                    <button type="button" class="dropdown-item">WebView</button>
                  </li>
                  <li role="none">
                    <button type="button" class="dropdown-item">CEF</button>
                  </li>
                </ul>
              {/if}
            </div>
          </div>
          <div class="rowSetting">
            <div class="form-check">
              <input id="pvTray" class="form-check-input" type="checkbox" disabled />
              <label class="form-check-label" for="pvTray"></label>
            </div>
            <label for="pvTray">{$t("Settings_ExitToTray")}</label>
          </div>
        </div>
      </div>
    </div>
  </div>
</div>

<style lang="scss">
  .preview-css-page {
    padding-bottom: 2rem;
  }

  .preview-css-toolbar {
    margin-bottom: 0.5rem;
  }

  .preview-css-intro {
    margin: 0 0 1.25rem;
    color: var(--blackTernary, #a7abbe);
    line-height: 1.45;
  }

  .preview-overlay-drop-intro {
    margin: 0 0 1rem;
    color: var(--blackTernary, #a7abbe);
    line-height: 1.45;
    font-size: 0.95rem;
  }

  .preview-overlay-drop-grid {
    display: grid;
    gap: 1.25rem;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 300px), 1fr));
  }

  .preview-overlay-drop-cell {
    margin: 0;
    min-width: 0;
    pointer-events: none;
  }

  .preview-overlay-drop-caption {
    margin: 0 0 0.5rem;
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--blackSecondary, #c8cbd9);
  }

  .preview-overlay-drop-subhead {
    margin: 1.25rem 0 0.65rem;
    font-size: 1.05rem;
    border-bottom: none;
  }

  .preview-overlay-drop-cell--accrow {
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .preview-overlay-accrow-host {
    display: flex;
    justify-content: center;
    align-items: flex-start;
    padding: 0.35rem 0 0;
    min-height: 148px;
    width: 100%;
  }

  .preview-overlay-drop-block {
    padding: 1em;
  }

  .preview_program_main {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .preview_program_main .platformTable {
    min-height: 120px;
  }

  :global(.platform_tile_ctx) {
    position: relative;
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
  }

  .preview_fake_actionbar {
    position: relative;
    margin-top: 0.25rem;
  }

  .preview-edge-subhead {
    margin: 0 0 0.65rem;
    font-size: 1.05rem;
    border-bottom: none;
  }

  .preview-platform-edge-wrap,
  .preview-account-edge-wrap {
    padding: 1em;
  }

  .preview-platform-edge-list {
    align-items: flex-start;
    gap: 0.25rem;
  }

  .preview-platform-edge {
    overflow: hidden;
  }

  .preview-platform-state-badge {
    position: absolute;
    top: 0.4rem;
    left: 0.4rem;
    z-index: 2;
    padding: 0.12rem 0.32rem;
    border: 1px solid var(--preview-control-border, var(--accent));
    border-radius: 2px;
    background: var(--mainContentBackground, var(--code-background));
    color: var(--white);
    font-size: 0.68rem;
    line-height: 1.2;
    font-weight: 700;
  }

  .preview-platform-edge--selected {
    border-width: 3px;
    box-shadow: inset 0 0 0 2px var(--mainContentBackground, var(--code-background));
  }

  .preview-platform-edge--current {
    border-width: 3px;
    border-style: dashed;
  }

  .preview-platform-edge--disabled {
    cursor: not-allowed;
    opacity: 0.58;
    filter: grayscale(0.65);
  }

  .preview-platform-edge--hover {
    transform: scale(1.025);
    box-shadow: 0 4px 18px var(--shadow-color-35);
    filter: brightness(97%) saturate(1.1) contrast(102%);
  }

  .preview-platform-edge--drop-target {
    border-color: var(--preview-control-border, var(--input-number-border, var(--accent))) !important;
    border-width: 3px;
    border-style: dashed;
  }

  .preview_accounts_wrap {
    display: flex;
    flex-direction: column;
    min-height: 0;
    /* Let the shortcut dropdown extend above this card (same idea as .actionbar__actions { overflow: visible }) */
    overflow: visible;
  }

  .preview_accounts_actionbar {
    overflow: visible;
  }

  .preview-account-edge-grid {
    display: grid;
    gap: 0.9rem;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 155px), 1fr));
    justify-items: center;
  }

  .preview-account-edge {
    width: 145px;
    height: 135px;
  }

  .preview-account-edge--disabled {
    cursor: not-allowed;
    opacity: 0.58;
    filter: grayscale(0.65);
  }

  .preview-account-edge--hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 18px var(--shadow-color-35);
  }

  .preview-account-edge--broken .acc_broken_badge {
    display: inline-block;
    margin: 0.15rem 0.35rem 0;
    padding: 0.08rem 0.24rem;
    border: 1px solid var(--whiteSecondary);
    border-radius: 2px;
    background: var(--mainContentBackground, var(--code-background));
    color: var(--whiteSecondary);
    font-size: 0.72rem;
    font-weight: 700;
    line-height: 1.2;
  }

  /* Mirror .steam-acclist ghost/drag rules so preview account DnD matches Platform.svelte */
  .preview-acc-list-wrap :global(.acc_list) {
    grid-template-rows: repeat(auto-fill, 135px);
  }

  .preview-acc-list-wrap :global(.acc_list_item--drag) {
    cursor: grab;
    touch-action: none;
  }

  .preview-acc-list-wrap :global(.acc_list_item--ghost) {
    position: fixed;
    margin: 0;
    z-index: 10000;
    pointer-events: none;
    opacity: 0.96;
    cursor: grabbing;
    box-shadow: 0 12px 36px var(--shadow-color-50);
    left: 0;
    top: 0;
  }

  .preview_shortcut_bar {
    position: relative;
    display: flex;
    flex-direction: row;
    align-items: center;
    height: 100%;
    gap: 0;
    flex-wrap: wrap;

    button {
      height: 100%;
    }
  }

  .statusBar {
    width: 120px;
  }

  .shortcutDropdownWrap {
    position: relative;
    display: flex;
    align-items: center;
  }

  .shortcutDndGrid {
    display: flex;
    flex-flow: row wrap;
    align-content: flex-start;
    align-items: center;
    gap: 0;
  }

  .preview-static-toast-host {
    margin-bottom: 1rem;
    display: flex;
    flex-direction: row-reverse;
    padding:1em;
  }

  .settingsCol {
    padding: 1em;
    margin-top: 0;
  }

  .preview-static-toast-stack {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 0.5rem;
    max-width: 22rem;
    width: 100%;
    position: relative;
    z-index: 1;
  }

  .preview-static-toast {
    position: relative;
    width: 100%;
    padding: 0.65rem 2.25rem 0.65rem 0.65rem;
    border-radius: 2px;
    border: 1px solid transparent;
    box-shadow: var(--shadow, 0 4px 14px var(--shadow-color-35));
    background: var(--darker-code-background);
    color: var(--white);
    text-align: left;
  }

  .preview-static-toast .toast__row {
    display: flex;
    flex-direction: row;
    align-items: center;
    gap: 0.6rem;
    min-width: 0;
  }

  .preview-static-toast .toast__icon {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .preview-static-toast .toast__text {
    flex: 1;
    min-width: 0;
  }

  .preview-static-toast .toast__title {
    font-weight: 600;
    font-size: 1.1rem;
    margin-bottom: 0.2rem;
  }

  .preview-static-toast .toast__message {
    font-size: 1rem;
    line-height: 1.35;
    color: var(--whiteSecondary);
    opacity: 0.92;
    word-break: break-word;
    white-space: pre-line;
  }

  .preview-static-toast.toast--success {
    border-color: var(--notification-border, var(--modal-border, var(--border-bar-bg, transparent)));
    border-left-color: var(--notification-success-border, var(--green));
  }
  .preview-static-toast.toast--warning {
    border-color: var(--notification-border, var(--modal-border, var(--border-bar-bg, transparent)));
    border-left-color: var(--notification-warning-border, var(--orange));
  }
  .preview-static-toast.toast--error {
    border-color: var(--notification-border, var(--modal-border, var(--border-bar-bg, transparent)));
    border-left-color: var(--notification-error-border, var(--red));
  }
  .preview-static-toast.toast--info {
    border-color: var(--notification-border, var(--modal-border, var(--border-bar-bg, transparent)));
    border-left-color: var(--notification-info-border, var(--accent));
  }

  .preview-toast-live-heading {
    margin-top: 0.25rem;
    margin-bottom: 0.5rem;
    font-size: 1.1rem;
    border-bottom: none;
  }

  .toastPermanentRow {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem 0.6rem;
    margin: 0 0 0.75rem;
    cursor: pointer;
    user-select: none;
    font-size: 0.9rem;
    color: var(--whiteSecondary);
  }

  .toastPermanentCheckbox {
    opacity: 1;
    z-index: auto;
    width: 1.1rem;
    height: 1.1rem;
    margin: 0;
    cursor: pointer;
    accent-color: var(--accent);
  }

  .toastPermanentNote {
    flex: 1 1 100%;
    margin-left: 1.5rem;
    font-size: 0.78rem;
    color: var(--blackTernary, #a7abbe);
    font-weight: normal;
    cursor: pointer;
  }

  @media (min-width: 520px) {
    .toastPermanentNote {
      flex: 0 1 auto;
      margin-left: 0;
    }
  }

  .modalTestPanel {
    margin-bottom: 0;
  }

  .modalTestOutput {
    margin: 0 0 0.75rem;
    padding: 0.65rem 0.75rem;
    max-height: 11rem;
    overflow: auto;
    background: var(--even-darker-code-background);
    border: 1px solid var(--preview-control-border, var(--input-number-border, var(--button-bg)));
    color: var(--whiteSecondary);
    font-size: 11px;
    line-height: 1.45;
    white-space: pre-wrap;
    word-break: break-word;
  }

  .modalTestPlaceholder {
    color: var(--whiteSecondary);
  }

  .modalTestButtons {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
    align-items: center;
  }
</style>
