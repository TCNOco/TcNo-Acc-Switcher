<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { tooltip as tooltipAction } from "../../lib/actions/tooltip";
  import { viewportDropdown } from "../../lib/actions/viewportDropdown";

  export let values: readonly string[];
  export let current: string;
  export let label: string = "";
  export let labelFn: (v: string) => string = (v) => v;
  export let tooltip: string = "";
  export let disabled: boolean = false;

  const dispatch = createEventDispatcher();
  let open = false;

  function toggle(): void {
    if (!disabled) open = !open;
  }

  function select(value: string): void {
    if (disabled) return;
    dispatch("select", { value });
    open = false;
  }
</script>

<div class="rowSetting rowDropdown" use:tooltipAction={tooltip || undefined}>
  <span>{label}</span>
  <div class="dropdown" class:show={open}>
    <button type="button" class="dropdown-toggle" on:click={toggle}>
      {labelFn(current)}
      <span class="caret" aria-hidden="true"></span>
    </button>
    {#if open}
      <ul class="custom-dropdown-menu dropdown-menu" use:viewportDropdown>
        {#each values as v}
          <li>
            <button type="button" class="dropdown-item" on:click={() => select(v)}>
              {labelFn(v)}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</div>
