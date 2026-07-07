<script lang="ts">
  import { createEventDispatcher, onMount, tick } from "svelte";
  import ModalBodyShell from "./ModalBodyShell.svelte";
  import { t } from "../../stores/i18n";
  import type { TagExpiryResult } from "../../stores/modal";

  export let tagName = "";
  export let initialScope: "account" | "all" = "account";
  export let initialDate = "";
  export let initialTime = "";
  export let positiveLabel = "";
  export let negativeLabel = "";

  const dispatch = createEventDispatcher<{ resolve: TagExpiryResult | null }>();

  let scope: "account" | "all" = initialScope;
  let dateValue = initialDate;
  let timeValue = initialTime;
  let dateInput: HTMLInputElement | undefined;

  function toUtcIso(dateText: string, timeText: string): string | null {
    if (!dateText || !timeText) return null;
    const local = new Date(`${dateText}T${timeText}`);
    if (Number.isNaN(local.getTime())) return null;
    return local.toISOString();
  }

  function cancel(): void {
    dispatch("resolve", null);
  }

  function save(): void {
    const expiresAt = toUtcIso(dateValue, timeValue);
    if (!expiresAt) return;
    dispatch("resolve", { scope, expiresAt });
  }

  $: canSave = !!toUtcIso(dateValue, timeValue);

  onMount(() => {
    void tick().then(() =>
      requestAnimationFrame(() => {
        dateInput?.focus();
      }),
    );
  });
</script>

<div class="modal-block">
  <ModalBodyShell html={$t("Tags_AddExpiryBody", { tag: tagName })} />

  <fieldset class="tag-expiry__scope">
    <legend class="tag-expiry__legend">{$t("Tags_ExpiryScope")}</legend>
    <label class="tag-expiry__option">
      <input type="radio" bind:group={scope} value="all" />
      <span>{$t("Tags_ExpiryScope_All")}</span>
    </label>
    <label class="tag-expiry__option">
      <input type="radio" bind:group={scope} value="account" />
      <span>{$t("Tags_ExpiryScope_AccountOnly")}</span>
    </label>
  </fieldset>

  <div class="tag-expiry__row">
    <label class="tag-expiry__field">
      <span class="tag-expiry__label">{$t("Tags_ExpiryDate")}</span>
      <input bind:this={dateInput} bind:value={dateValue} class="modal-input" type="date" />
    </label>
    <label class="tag-expiry__field">
      <span class="tag-expiry__label">{$t("Tags_ExpiryTime")}</span>
      <input bind:value={timeValue} class="modal-input" type="time" />
    </label>
  </div>

  <div class="modal-inline-actions settingsCol inputAndButton">
    <span class="modal-actions-spacer"></span>
    <button type="button" class="btnicontext modal-primary" disabled={!canSave} on:click={save}>
      {positiveLabel}
    </button>
    <button type="button" class="btnicontext" on:click={cancel}>
      {negativeLabel}
    </button>
  </div>
</div>

<style lang="scss">
  .tag-expiry__scope {
    margin: 0;
    padding: 0;
    border: 0;
    display: grid;
    gap: 0.45rem;
  }

  .tag-expiry__legend {
    margin: 0 0 0.15rem;
    font-size: 0.9rem;
    font-weight: 600;
  }

  .tag-expiry__option {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .tag-expiry__row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(8rem, 11rem);
    gap: 0.75rem;
  }

  .tag-expiry__field {
    display: grid;
    gap: 0.3rem;
    min-width: 0;
  }

  .tag-expiry__label {
    font-size: 0.85rem;
    font-weight: 600;
  }

  @media (max-width: 640px) {
    .tag-expiry__row {
      grid-template-columns: 1fr;
    }
  }
</style>
