<script lang="ts">
  import type { MenuItemDef } from "../stores/contextMenu";
  import { closeContextMenu } from "../stores/contextMenu";

  export let items: MenuItemDef[] = [];
  /** submenu depth for CSS class */
  export let depth = 1;

  let searchQuery = "";

  function norm(s: string): string {
    return s.toLowerCase();
  }

  $: hasSearch = items[0]?.type === "search";
  $: tail = hasSearch ? items.slice(1) : items;
  $: q = norm(searchQuery.trim());
  $: rendered = !hasSearch || !q
    ? tail
    : tail.filter((it) => {
        if (it.type === "separator") {
          return false;
        }
        if (!it.label) {
          return false;
        }
        return norm(it.label).includes(q);
      });

  function submenuClass(d: number): string {
    return d <= 1 ? "submenu submenu1" : "submenu submenu2";
  }

  function stop(ev: Event): void {
    ev.stopPropagation();
  }

  function run(item: MenuItemDef): void {
    item.action?.();
    closeContextMenu();
  }
</script>

{#if hasSearch}
  <li class="contextSearch" role="none" on:pointerdown={stop}>
    <input
      type="search"
      class="ctx-menu__search"
      placeholder={items[0]?.label || ""}
      bind:value={searchQuery}
      on:pointerdown={stop}
      on:keydown|stopPropagation
    />
  </li>
{/if}

{#each rendered as item}
  {#if item.type === "separator"}
    <li class="ctx-sep" role="separator"><hr /></li>
  {:else if item.children?.length}
    <li class="hasSubmenu" role="none">
      <span class="ctx-menu__label">{item.label}</span>
      <ul class={submenuClass(depth)} role="menu">
        <svelte:self items={item.children ?? []} depth={depth + 1} />
      </ul>
    </li>
  {:else}
    <li role="menuitem">
      <button type="button" class="ctx-menu__btn" on:click={() => run(item)}
        >{item.label}</button
      >
    </li>
  {/if}
{/each}

<style lang="scss">
  .ctx-menu__search {
    width: 100%;
    box-sizing: border-box;
    margin: 0.15rem 0 0.35rem;
    padding: 0.35rem 0.5rem;
    border-radius: 4px;
    border: 1px solid rgba(255, 255, 255, 0.15);
    background: rgba(0, 0, 0, 0.35);
    color: #fff;
    font: inherit;
  }

  hr {
    border: none;
    border-top: 1px solid rgba(255, 255, 255, 0.12);
    margin: 0.25rem 0;
  }
</style>
