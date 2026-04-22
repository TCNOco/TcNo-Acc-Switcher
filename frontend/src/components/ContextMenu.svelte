<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { contextMenu, closeContextMenu } from "../stores/contextMenu";

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
    const w = 220;
    const h = 200;
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
    {#each $contextMenu.items as item}
      {#if item.children?.length}
        <li class="hasSubmenu" role="none">
          <span class="ctx-menu__label">{item.label}</span>
          <ul class="submenu submenu1" role="menu">
            {#each item.children as ch}
              <li role="menuitem">
                <button
                  type="button"
                  class="ctx-menu__btn"
                  on:click={() => {
                    ch.action?.();
                    closeContextMenu();
                  }}>{ch.label}</button
                >
              </li>
            {/each}
          </ul>
        </li>
      {:else}
        <li role="menuitem">
          <button
            type="button"
            class="ctx-menu__btn"
            on:click={() => {
              item.action?.();
              closeContextMenu();
            }}>{item.label}</button
          >
        </li>
      {/if}
    {/each}
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
