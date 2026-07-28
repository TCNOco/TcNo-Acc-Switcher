<script lang="ts">
  import { tick, onMount } from "svelte";
  import { createEventDispatcher } from "svelte";
  import type { ComponentType, SvelteComponent } from "svelte";
  import ModalBodyShell from "./ModalBodyShell.svelte";
  import { t } from "../../stores/i18n";
  import type { PromptWithCheckboxResult } from "../../stores/modal";

  export let html: string | undefined = undefined;
  export let component: ComponentType<SvelteComponent> | undefined = undefined;
  export let componentProps: Record<string, unknown> | undefined = undefined;
  export let initialValue = "";
  export let positiveLabel = "";
  export let multiline = false;
  export let inputType: "text" | "password" = "text";
  export let checkboxLabel: string | undefined = undefined;
  export let checkboxInitial = false;
  export let returnCheckboxResult = false;
  export let modalId = 0;

  const dispatch = createEventDispatcher<{ resolve: string | PromptWithCheckboxResult | null }>();

  let value = initialValue;
  let checked = checkboxInitial;
  let lastInitialValue = initialValue;
  let inputEl: HTMLInputElement | HTMLTextAreaElement | undefined;
  let inputLabel = "";
  $: checkboxId = `modal-prompt-check-${modalId}`;

  function getPlainText(value: string | undefined): string {
    if (!value) return "";
    if (typeof DOMParser === "undefined") return "";
    return new DOMParser().parseFromString(value, "text/html").body.textContent?.replace(/\s+/g, " ").trim() ?? "";
  }

  function getDialogTitle(): string {
    const dialog = inputEl?.closest<HTMLElement>("[role='dialog']");
    const labelId = dialog?.getAttribute("aria-labelledby");
    if (!labelId) return "";
    return document.getElementById(labelId)?.textContent?.trim() ?? "";
  }

  $: if (initialValue !== lastInitialValue) {
    value = initialValue;
    lastInitialValue = initialValue;
  }

  $: inputLabel = inputType === "password"
    ? $t("Security_Password")
    : getPlainText(html) || getDialogTitle() || (multiline ? "Text input" : "Input");

  function ok(): void {
    dispatch("resolve", returnCheckboxResult ? { value, checked } : value);
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
        aria-label={inputLabel}
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
        aria-label={inputLabel}
      ></textarea>
    {:else}
      <input
        bind:this={inputEl}
        bind:value
        type="text"
        class="modal-input"
        autocomplete="off"
        aria-label={inputLabel}
        on:keydown={(e) => e.key === "Enter" && ok()}
      />
    {/if}
  </div>
  {#if checkboxLabel}
    <div class="modal-prompt-check">
      <span class="form-check">
        <input id={checkboxId} class="form-check-input" bind:checked type="checkbox" />
        <label class="form-check-label" for={checkboxId} aria-hidden="true"></label>
      </span>
      <label class="modal-prompt-check__text" for={checkboxId}>{checkboxLabel}</label>
    </div>
  {/if}
  <div class="modal-inline-actions settingsCol inputAndButton">
    <span class="modal-actions-spacer"></span>
    <button type="button" class="btnicontext modal-primary" on:click={ok}>
      {positiveLabel}
    </button>
  </div>
</div>

<style lang="scss">
  .modal-prompt-check {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-top: 0.75rem;
    cursor: pointer;
  }

  .modal-prompt-check__text {
    padding: 0;
    cursor: pointer;
  }
</style>
