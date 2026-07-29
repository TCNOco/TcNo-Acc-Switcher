<script lang="ts">
  import { tooltip as tooltipAction } from "../../lib/actions/tooltip";

  export let label: string = "";
  export let forId: string = "";
  export let note: string = "";
  export let tooltip: string = "";
  /** Text inputs that should take the pane's remaining width instead of hugging right. */
  export let wide: boolean = false;
  export let disabled: boolean = false;
</script>

<div
  class="settings-field"
  class:settings-field--wide={wide}
  class:settings-field--disabled={disabled}
  use:tooltipAction={tooltip || undefined}
>
  {#if forId}
    <label class="settings-field__label" for={forId}>{label}</label>
  {:else}
    <span class="settings-field__label">{label}</span>
  {/if}
  <div class="settings-field__control"><slot /></div>
  {#if note}
    <p class="settings-field__note">{note}</p>
  {/if}
</div>
