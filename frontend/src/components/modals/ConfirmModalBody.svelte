<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import type { ComponentType, SvelteComponent } from "svelte";
  import ModalBodyShell from "./ModalBodyShell.svelte";

  export let html: string | undefined = undefined;
  export let component: ComponentType<SvelteComponent> | undefined = undefined;
  export let componentProps: Record<string, unknown> | undefined = undefined;
  export let positiveLabel = "";
  export let negativeLabel: string | undefined = undefined;
  export let style: string = "";

  const dispatch = createEventDispatcher<{ resolve: boolean }>();

  function positive(): void {
    dispatch("resolve", true);
  }

  function negative(): void {
    dispatch("resolve", false);
  }
</script>

<div class="modal-block">
  <ModalBodyShell
    {html}
    {component}
    {componentProps}
  />
  <div class="modal-inline-actions settingsCol inputAndButton">
    <span class="modal-actions-spacer"></span>
    <button type="button" class="btnicontext modal-primary" on:click={positive}>
      {positiveLabel}
    </button>
    {#if style === "yesno"}
      <button type="button" class="btnicontext" on:click={negative}>
        {negativeLabel}
      </button>
    {/if}
  </div>
</div>
