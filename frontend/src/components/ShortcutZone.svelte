<script lang="ts">
  import { t } from "../stores/i18n";
  import { tooltip } from "../lib/actions/tooltip";
  import { contextMenu } from "../lib/actions/contextMenu";
  import { scrollbarWidthVar } from "../lib/actions/scrollbarWidthVar";
  import type { ShortcutDTO } from "../../bindings/TcNo-Acc-Switcher/internal/shortcuts/models.js";
  import type { MenuItemDef } from "../stores/contextMenu";
  import { offlineMode, offlineSafeImageSrc } from "../stores/offlineMode";

  const FALLBACK = "/img/icons/file.svg";

  export let zone: string;
  export let zoneClass = "";
  export let expandClass = false;
  export let displaySlots: (string | null)[] = [];
  export let meta: Record<string, ShortcutDTO> = {};
  export let contextMenuFor: (fn: string) => () => MenuItemDef[] = () => () => [];
  export let onTileClick: (row: ShortcutDTO) => void = () => {};
  export let onCellPointerDown: (e: PointerEvent, z: string, id: string) => void =
    () => {};
  export let el: HTMLDivElement | null = null;
</script>

<div
  bind:this={el}
  class={zoneClass}
  class:expandShortcuts={expandClass}
  role="list"
  aria-label={$t("Stats_GameShortcuts")}
  use:scrollbarWidthVar={{
    enabled: zoneClass.includes("shortcutDropdownItems"),
    targetSelector: ".shortcutDropdown",
  }}
>
  {#each displaySlots as slot, i (slot === null ? `${zone}g-${i}` : `${zone}-${i}-${slot}`)}
    {#if slot === null}
      <div
        class="shortcutDndGap shortcutPlaceholder"
        role="presentation"
        data-dnd-cell
        data-dnd-gap="true"
        data-dnd-visual={i}
      ></div>
    {:else}
      {@const row = meta[slot]}
      {#if row}
        {@const src = offlineSafeImageSrc($offlineMode, row.iconUrl, FALLBACK)}
        <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
        <div
          class="shortcutDndCell"
          role="listitem"
          data-dnd-cell
          data-dnd-visual={i}
          data-dnd-name={slot}
          on:pointerdown={(e) => onCellPointerDown(e, zone, slot)}
        >
          <button
            type="button"
            class="HasContextMenu"
            aria-label={row.displayName}
            use:tooltip={row.displayName}
            use:contextMenu={contextMenuFor(slot)}
            on:click={() => onTileClick(row)}
          >
            <img src={src} alt="" draggable="false" />
          </button>
        </div>
      {/if}
    {/if}
  {/each}
</div>
