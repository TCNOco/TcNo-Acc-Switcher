<script lang="ts">
  import { tick, onMount } from "svelte";
  import { createEventDispatcher } from "svelte";
  import type { ComponentType, SvelteComponent } from "svelte";
  import ModalBodyShell from "./ModalBodyShell.svelte";
  import { t } from "../../stores/i18n";

  export let html: string | undefined = undefined;
  export let component: ComponentType<SvelteComponent> | undefined = undefined;
  export let componentProps: Record<string, unknown> | undefined = undefined;
  export let initialValue = "";
  export let positiveLabel = "";
  export let multiline = false;
  export let inputType: "text" | "password" = "text";

  const dispatch = createEventDispatcher<{ resolve: string | null }>();

  let value = initialValue;
  let inputEl: HTMLInputElement | HTMLTextAreaElement | undefined;

  $: if (initialValue !== value) {
    value = initialValue;
  }

  function ok(): void {
    dispatch("resolve", value);
  }

  onMount(() => {
    void tick().then(() =>
      requestAnimationFrame(() => {
        inputEl?.focus();
        inputEl?.select?.();
      }),
    );
  });
</script>

<div class="modal-block">
  <ModalBodyShell
    {html}
    {component}
    {componentProps}
  />
  <div class="modal-input-row">
    {#if inputType === "password"}
      <input
        bind:this={inputEl}
        bind:value
        type="password"
        class="modal-input"
        autocomplete="off"
        on:keydown={(e) => e.key === "Enter" && ok()}
      />
    {:else if multiline}
      <textarea
        bind:this={inputEl}
        bind:value
        class="modal-input modal-input--multiline"
        rows="6"
        spellcheck="true"
        autocomplete="off"
      ></textarea>
    {:else}
      <input
        bind:this={inputEl}
        bind:value
        type="text"
        class="modal-input"
        autocomplete="off"
        on:keydown={(e) => e.key === "Enter" && ok()}
      />
    {/if}
  </div>
  <div class="modal-inline-actions settingsCol inputAndButton">
    <span class="modal-actions-spacer"></span>
    <button type="button" class="btnicontext modal-primary" on:click={ok}>
      {positiveLabel}
    </button>
  </div>
</div>
