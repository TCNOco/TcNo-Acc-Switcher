<script lang="ts">
  import { onMount } from "svelte";
  import { fly } from "svelte/transition";
  import { cubicOut } from "svelte/easing";
  import { Events } from "@wailsio/runtime";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { t } from "../stores/i18n";
  import "../styles/UpdateBar.scss";

  let showBanner = false;
  let dismissed = false;

  $: visible = showBanner && !dismissed;

  onMount(() => {
    const off = Events.On("app-update-available", () => {
      showBanner = true;
      dismissed = false;
    });
    return () => {
      off?.();
    };
  });

  function closeBar(): void {
    dismissed = true;
  }

  async function openDownloadPage(): Promise<void> {
    try {
      await PlatformService.OpenUpdateDownloadPage();
    } catch {
      /* ignore */
    }
  }

  async function onBarClick(e: MouseEvent): Promise<void> {
    const el = e.target as HTMLElement | null;
    if (!el) {
      return;
    }
    if (el.closest(".updateBar__close")) {
      return;
    }
    await openDownloadPage();
  }

  function onBarKeydown(e: KeyboardEvent): void {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      void openDownloadPage();
    }
  }
</script>

{#if visible}
  <div
    class="updateBar"
    role="button"
    tabindex="0"
    transition:fly={{ y: -12, duration: 220, easing: cubicOut }}
    on:click={onBarClick}
    on:keydown={onBarKeydown}
  >
    <span class="updateBar__label">{$t("Update")}</span>
    <button
      type="button"
      class="updateBar__close"
      id="closeUpdateBar"
      aria-label={$t("Button_Close")}
      on:click|stopPropagation={closeBar}
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true">
        <!-- Font Awesome Free 5.15.4 -->
        <path
          d="M256 8C119 8 8 119 8 256s111 248 248 248 248-111 248-248S393 8 256 8zm121.6 313.1c4.7 4.7 4.7 12.3 0 17L338 377.6c-4.7 4.7-12.3 4.7-17 0L256 312l-65.1 65.6c-4.7 4.7-12.3 4.7-17 0L134.4 338c-4.7-4.7-4.7-12.3 0-17l65.6-65-65.6-65.1c-4.7-4.7-4.7-12.3 0-17l39.6-39.6c4.7-4.7 12.3-4.7 17 0l65 65.7 65.1-65.6c4.7-4.7 12.3-4.7 17 0l39.6 39.6c4.7 4.7 4.7 12.3 0 17L312 256l65.6 65.1z"
        />
      </svg>
    </button>
  </div>
{/if}
