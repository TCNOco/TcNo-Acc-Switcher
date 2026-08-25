<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { viewportDropdown } from "../../lib/actions/viewportDropdown";
  import { DUR, scaleFade } from "../../lib/animation";
  import { dropdownDismiss } from "../../lib/actions/dropdownDismiss";
  import SettingsField from "./SettingsField.svelte";

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

<SettingsField {label} {tooltip} {disabled}>
  <div class="dropdown" class:show={open} use:dropdownDismiss={open} on:dismiss={() => (open = false)}>
    <button type="button" class="dropdown-toggle" on:click={toggle}>
      {labelFn(current)}
      <span class="caret" aria-hidden="true"></span>
    </button>
    {#if open}
      <ul
        class="custom-dropdown-menu dropdown-menu"
        use:viewportDropdown
        in:scaleFade={{ start: 0.96, duration: DUR.instant }}
        out:scaleFade={{ start: 0.98, duration: DUR.instant }}
      >
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
</SettingsField>

<style lang="scss">
  .dropdown-toggle {
    position: relative;
    height: 38px;
  }
</style>
