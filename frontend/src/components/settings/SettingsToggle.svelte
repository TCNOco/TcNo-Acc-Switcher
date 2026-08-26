<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { tooltip as tooltipAction } from "../../lib/actions/tooltip";

  export let id: string;
  export let checked: boolean = false;
  export let disabled: boolean = false;
  export let label: string = "";
  export let tooltip: string = "";
  /** Labels too long for one grid column read better across the whole pane. */
  export let span: boolean = false;
  /** The box only flips once the caller says so — for toggles gated behind a prompt. */
  export let manual: boolean = false;

  const dispatch = createEventDispatcher<{ change: boolean }>();

  function onClick(e: MouseEvent): void {
    if (!manual) return;
    e.preventDefault();
    if (disabled) return;
    dispatch("change", !checked);
  }

  function onChange(e: Event & { currentTarget: HTMLInputElement }): void {
    if (manual) return;
    dispatch("change", e.currentTarget.checked);
  }
</script>

<!-- Tooltip on the row, not the label: it should cover the checkbox too, and it
     anchors on whatever it is attached to. -->
<div
  class="rowSetting settings-toggle"
  class:is-span={span}
  class:is-disabled={disabled}
  use:tooltipAction={tooltip || undefined}
>
  <div class="form-check">
    <input type="checkbox" {id} {checked} {disabled} on:click={onClick} on:change={onChange} />
    <label class="form-check-label" for={id}></label>
  </div>
  <label class="settings-toggle__label" for={id}>
    {label}
  </label>
</div>
