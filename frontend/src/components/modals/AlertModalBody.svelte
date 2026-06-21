<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import type { ComponentType, SvelteComponent } from "svelte";
  import ModalBodyShell from "./ModalBodyShell.svelte";
  import { t } from "../../stores/i18n";

  export let dismissLabel: string | undefined = undefined;
  export let html: string | undefined = undefined;
  export let component: ComponentType<SvelteComponent> | undefined = undefined;
  export let componentProps: Record<string, unknown> | undefined = undefined;

  const dispatch = createEventDispatcher<{ resolve: void }>();

  function dismiss(): void {
    dispatch("resolve");
  }
</script>

<div class="modal-block">
  <ModalBodyShell
    {html}
    {component}
    {componentProps}
  />
  {#if dismissLabel !== undefined}
    <div class="modal-inline-actions settingsCol inputAndButton">
      <span class="modal-actions-spacer"></span>
      <button type="button" class="btnicontext modal-primary" on:click={dismiss}>
        {dismissLabel ?? $t("Ok")}
      </button>
    </div>
  {/if}
</div>
