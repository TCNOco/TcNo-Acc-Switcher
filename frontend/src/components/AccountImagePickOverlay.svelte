<script lang="ts">
  import { modalFocus } from "../lib/modalFocus";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { t } from "../stores/i18n";

  /** When true, overlay is shown */
  export let open = false;
  /** Account display name for copy */
  export let accountDisplayName = "";
  /** Show "Remove image" when current avatar is user-set */
  export let showRemoveButton = false;
  export let onClose: () => void;
  export let onApplyPath: (path: string) => Promise<void>;
  export let onRemoveManual: () => Promise<void>;

  let busy = false;
  let closeButtonEl: HTMLButtonElement | undefined;
  let panelEl: HTMLDivElement | undefined;

  function close(): void {
    if (busy) {
      return;
    }
    onClose();
  }

  async function onPickClick(): Promise<void> {
    if (busy) {
      return;
    }
    busy = true;
    try {
      const path = await PlatformService.PickProfileImageFile();
      const p = String(path ?? "").trim();
      if (!p) {
        return;
      }
      await onApplyPath(p);
      onClose();
    } catch {
      /* user cancelled or dialog error — stay open */
    } finally {
      busy = false;
    }
  }

  async function onRemoveClick(): Promise<void> {
    if (busy || !showRemoveButton) {
      return;
    }
    busy = true;
    try {
      await onRemoveManual();
      onClose();
    } finally {
      busy = false;
    }
  }

</script>

{#if open}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <!-- `self`: only backdrop clicks — panel/X/button children don't bubble as same target -->
  <div
    class="acc-img-overlay"
    use:modalFocus={{ initialFocus: () => panelEl, onEscape: close }}
    role="presentation"
    on:click|self={close}
  >
    <button
      bind:this={closeButtonEl}
      type="button"
      class="acc-img-overlay__x"
      aria-label={$t("Button_Close")}
      on:click={close}
      >&times;</button
    >
    <div
      bind:this={panelEl}
      class="acc-img-overlay__panel"
      role="dialog"
      aria-modal="true"
      aria-labelledby="acc-img-overlay-title"
      tabindex="-1"
    >
      <h2 id="acc-img-overlay-title" class="acc-img-overlay__title">
        {$t("Overlay_ProfileImageTitle")}
      </h2>
      <p class="acc-img-overlay__hint">
        {$t("Overlay_ProfileImageHint", { name: accountDisplayName || "—" })}
      </p>
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <button
        type="button"
        class="acc-img-overlay__dropzone"
        disabled={busy}
        on:click={() => void onPickClick()}
      >
        <span class="acc-img-overlay__dropicon" aria-hidden="true">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="48" height="48"
            ><path
              fill="currentColor"
              d="M5 20h14v-2H5v2zM19 9h-4V3H9v6H5l7 7 7-7z"
            /></svg
          >
        </span>
        <span class="acc-img-overlay__cta">{$t("Overlay_ProfileImageClickToBrowse")}</span>
      </button>
      {#if showRemoveButton}
        <button type="button" class="acc-img-overlay__remove" disabled={busy} on:click={() => void onRemoveClick()}>
          {$t("Context_RemoveProfileImage")}
        </button>
      {/if}
    </div>
  </div>
{/if}
