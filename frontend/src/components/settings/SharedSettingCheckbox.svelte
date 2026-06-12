<script lang="ts">
  export let id: string;
  export let checked: boolean = false;
  export let disabled: boolean = false;
  export let label: string = "";
  export let tooltip: string = "";

  import { tooltip as tooltipAction } from "../../lib/actions/tooltip";
  import { createEventDispatcher } from "svelte";
  const dispatch = createEventDispatcher();
</script>

<div class="rowSetting">
  <div class="form-check">
    <input
      type="checkbox"
      {id}
      bind:checked
      {disabled}
      on:change
    />
    <label class="form-check-label" for={id}></label>
  </div>
  <label for={id} use:tooltipAction={tooltip || undefined}>
    {label}
  </label>
</div>
