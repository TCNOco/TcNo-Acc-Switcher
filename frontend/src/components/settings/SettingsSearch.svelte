<script lang="ts">
  import { get } from "svelte/store";
  import { createEventDispatcher } from "svelte";
  import { t } from "../../stores/i18n";
  import { activeModal } from "../../stores/modal";

  export let value: string = "";

  let input: HTMLInputElement | undefined;

  const dispatch = createEventDispatcher<{ escape: void }>();

  function isTypingTarget(target: EventTarget | null): boolean {
    const el = target as HTMLElement | null;
    if (!el) return false;
    return (
      el.tagName === "INPUT" ||
      el.tagName === "TEXTAREA" ||
      el.tagName === "SELECT" ||
      el.isContentEditable
    );
  }

  function clear(): void {
    value = "";
    input?.focus();
  }

  function onWindowKeyDown(e: KeyboardEvent): void {
    // A modal owns the keyboard while it is open.
    if (get(activeModal)) return;

    if (e.key === "Escape") {
      if (!value) {
        e.preventDefault();
        dispatch("escape");
        return;
      }
      e.preventDefault();
      value = "";
      input?.blur();
      return;
    }

    if (e.ctrlKey || e.altKey || e.metaKey) return;
    if (isTypingTarget(e.target)) return;
    // Single-character keys only, so shortcuts and navigation keys pass through.
    // Space is excluded: as a first character it filters nothing, and it would
    // steal activation from whatever button has focus.
    if (e.key.length !== 1 || e.key === " ") return;

    // The keystroke landed on the page, not the field — replay it into the box
    // rather than letting the first character of what someone typed vanish.
    e.preventDefault();
    value += e.key;
    input?.focus();
  }
</script>

<div class="settings-search">
  <svg class="settings-search__icon" viewBox="0 0 512 512" aria-hidden="true">
    <path
      d="M505 442.7L405.3 343c-4.5-4.5-10.6-7-17-7H372c27.6-35.3 44-79.7 44-128C416 93.1 322.9 0 208 0S0 93.1 0 208s93.1 208 208 208c48.3 0 92.7-16.4 128-44v16.3c0 6.4 2.5 12.5 7 17l99.7 99.7c9.4 9.4 24.6 9.4 33.9 0l28.3-28.3c9.4-9.4 9.4-24.6.1-34zM208 336c-70.7 0-128-57.2-128-128 0-70.7 57.2-128 128-128 70.7 0 128 57.2 128 128 0 70.7-57.2 128-128 128z"
    />
  </svg>
  <input
    bind:this={input}
    bind:value
    type="text"
    class="settings-search__input"
    placeholder={$t("Settings_SearchPlaceholder")}
    aria-label={$t("Settings_SearchPlaceholder")}
    spellcheck="false"
    autocomplete="off"
  />
  {#if value}
    <button
      type="button"
      class="settings-search__clear"
      aria-label={$t("Settings_SearchClear")}
      on:click={clear}
    >
      ×
    </button>
  {/if}
</div>
<svelte:window on:keydown={onWindowKeyDown} />
