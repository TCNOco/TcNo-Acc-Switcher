<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { contextMenu, closeContextMenu } from "../stores/contextMenu";
  import ContextMenuNest from "./ContextMenuNest.svelte";

  function onDocPointerDown(ev: MouseEvent): void {
    const t = ev.target as Node | null;
    const menu = document.querySelector(".ctx-menu-root");
    if (menu && t && menu.contains(t)) {
      return;
    }
    closeContextMenu();
  }

  function onKey(ev: KeyboardEvent): void {
    if (ev.key === "Escape") {
      closeContextMenu();
    }
  }

  onMount(() => {
    window.addEventListener("pointerdown", onDocPointerDown, true);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("pointerdown", onDocPointerDown, true);
      window.removeEventListener("keydown", onKey);
    };
  });

  onDestroy(() => {
    closeContextMenu();
  });

  function clamp(v: number, min: number, max: number): number {
    return Math.max(min, Math.min(max, v));
  }

  $: pos = (() => {
    const m = $contextMenu;
    if (!m) {
      return { left: 0, top: 0 };
    }
    const pad = 8;
    const w = 260;
    const h = 320;
    const left = clamp(m.x, pad, window.innerWidth - w - pad);
    const top = clamp(m.y, pad, window.innerHeight - h - pad);
    return { left, top };
  })();
</script>

{#if $contextMenu}
  <ul
    class="ctx-menu-root contextmenu"
    style:left="{pos.left}px"
    style:top="{pos.top}px"
    role="menu"
  >
    <ContextMenuNest items={$contextMenu.items} depth={1} />
  </ul>
{/if}

<style lang="scss">
  .ctx-menu__btn {
    display: block;
    width: 100%;
    text-align: left;
    padding: 0.4rem 0.85rem;
    margin: 0;
    border: none;
    background: transparent;
    color: #fff;
    font: inherit;
    cursor: pointer;
    border-radius: 4px;
  }

  .ctx-menu__btn:hover {
    background: rgba(255, 255, 255, 0.08);
  }
</style>
