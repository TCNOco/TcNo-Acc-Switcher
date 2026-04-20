<script lang="ts">
  import { onMount } from "svelte";
  import ActionBar from "../components/ActionBar.svelte";
  import ReorderPointerGrid from "../components/ReorderPointerGrid.svelte";
  import { loadStringOrder, saveStringOrder } from "../lib/persistedStringOrder";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import "../styles/HomePlatforms.scss";

  export let name: string;

  function defaultAccounts(platform: string): string[] {
    return [
      `${platform} — Account 1`,
      `${platform} — Account 2`,
      `${platform} — Account 3`,
    ];
  }

  let accountOrder: string[] = [];

  $: appBarTitle.set(name || "TcNo Account Switcher");
  $: if (name) {
    route.set({ page: "platform", platformName: name });
  }

  $: storageKey = name ? `tcno.order.accounts.${encodeURIComponent(name)}` : "";

  $: if (storageKey && name) {
    accountOrder = loadStringOrder(storageKey, defaultAccounts(name));
  }

  function onAccountReorder(e: CustomEvent<{ items: string[] }>): void {
    accountOrder = e.detail.items;
    if (storageKey) saveStringOrder(storageKey, e.detail.items);
  }

  function slotKey(x: string | null | undefined): string {
    return x ?? "";
  }

  onMount(() => {
    previousPage.set({ page: "home" });
  });
</script>

<div class="main-content platform-accounts-root">
  {#if name}
    <div class="platformTable">
      <ReorderPointerGrid
        items={accountOrder}
        listClass="platform_list"
        itemClass="platform_list_item platform_list_item--draggable"
        placeholderClass="platform_list_item platform_list_placeholder"
        ghostClass="platform_list_item platform_list_item--ghost"
        ariaLabel="Accounts"
        on:reorder={onAccountReorder}
      >
        <svelte:fragment slot="item" let:rowId>
          {@const rid = slotKey(rowId)}
          <div class="platform-account-label">{rid}</div>
        </svelte:fragment>
        <svelte:fragment slot="ghost" let:rowId>
          {@const rid = slotKey(rowId)}
          <div class="platform-account-label">{rid}</div>
        </svelte:fragment>
      </ReorderPointerGrid>
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

  .platform-account-label {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0.5rem;
    font-weight: 800;
    color: var(--accent);
    text-align: center;
    text-shadow: 2px 2px 10px #00000099;
    font-size: min(2vw, 0.95rem);
    line-height: 1.2;
    z-index: 2;
    pointer-events: none;
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

  :global(.platform_list_item--draggable) {
    cursor: grab;
    touch-action: none;
  }
</style>
