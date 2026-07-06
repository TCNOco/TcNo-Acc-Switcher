<script lang="ts">
  import { onMount } from "svelte";
  import { fly } from "svelte/transition";
  import { cubicOut } from "svelte/easing";
  import { Events } from "@wailsio/runtime";
  import { t } from "../stores/i18n";
  import { openFeedbackModal } from "../stores/modal";
  import { controllerSpatialNavigation } from "../lib/actions/controllerSpatialNavigation";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import "../styles/toast.scss";
  import "../styles/stability-prompt.scss";

  type Phase = "asking" | "thanks" | "thanks_report";

  const DISMISS_MS_THANKS = 3000;
  const DISMISS_MS_REPORT = 30000;

  let visible = false;
  let platform = "";
  let phase: Phase = "asking";
  let contentKey = 0;
  let dismissTimer: ReturnType<typeof setTimeout> | null = null;
  let dismissDeadline: number | null = null;
  let hovered = false;
  let thumbPulse: "up" | "down" | null = null;

  function clearDismissTimer(): void {
    if (dismissTimer !== null) {
      clearTimeout(dismissTimer);
      dismissTimer = null;
    }
  }

  function dismissRemainingMs(): number {
    if (dismissDeadline === null) return 0;
    return Math.max(0, dismissDeadline - Date.now());
  }

  function armDismissTimer(ms: number): void {
    clearDismissTimer();
    dismissDeadline = Date.now() + ms;
    if (!hovered) {
      dismissTimer = setTimeout(() => dismiss(), ms);
    }
  }

  function dismiss(): void {
    clearDismissTimer();
    dismissDeadline = null;
    hovered = false;
    visible = false;
    phase = "asking";
    platform = "";
    thumbPulse = null;
  }

  function scheduleDismiss(ms = DISMISS_MS_THANKS): void {
    armDismissTimer(ms);
  }

  function onPromptEnter(): void {
    hovered = true;
    clearDismissTimer();
  }

  function onPromptLeave(): void {
    hovered = false;
    if (!visible || dismissDeadline === null) return;
    const remaining = dismissRemainingMs();
    if (remaining <= 0) {
      dismiss();
      return;
    }
    dismissTimer = setTimeout(() => dismiss(), remaining);
  }

  function showPrompt(p: string): void {
    clearDismissTimer();
    dismissDeadline = null;
    hovered = false;
    platform = p;
    phase = "asking";
    contentKey++;
    thumbPulse = null;
    visible = true;
  }

  function transitionPhase(next: Phase): void {
    phase = next;
    contentKey++;
  }

  async function onThumbUp(): Promise<void> {
    if (phase !== "asking" || !platform) return;
    thumbPulse = "up";
    void PlatformService.SubmitStabilityRating(platform, true);
    setTimeout(() => {
      transitionPhase("thanks");
      scheduleDismiss(DISMISS_MS_THANKS);
    }, 180);
  }

  async function onThumbDown(): Promise<void> {
    if (phase !== "asking" || !platform) return;
    thumbPulse = "down";
    void PlatformService.SubmitStabilityRating(platform, false);
    setTimeout(() => {
      transitionPhase("thanks_report");
      scheduleDismiss(DISMISS_MS_REPORT);
    }, 180);
  }

  async function onReportIssue(e: MouseEvent): Promise<void> {
    e.preventDefault();
    const p = platform;
    dismiss();
    await openFeedbackModal({ mode: "issue", platform: p });
  }

  onMount(() => {
    const off = Events.On("stability-prompt", (ev) => {
      const data = ev.data as { platform?: string } | undefined;
      const p = typeof data?.platform === "string" ? data.platform.trim() : "";
      if (p) showPrompt(p);
    });
    return () => {
      off();
      clearDismissTimer();
    };
  });
</script>

{#if visible}
  <div class="stability-prompt-root" aria-live="polite" use:controllerSpatialNavigation>
    <div
      class="toast toast--info stability-prompt"
      role="status"
      on:mouseenter={onPromptEnter}
      on:mouseleave={onPromptLeave}
    >
      <button
        type="button"
        class="toast__close"
        aria-label={$t("Aria_DismissNotification")}
        on:click|stopPropagation={dismiss}
      >
        <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
          <path
            fill="currentColor"
            d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
          />
        </svg>
      </button>

      <div class="stability-prompt__stage">
        {#key contentKey}
          <div
            class="stability-prompt__content"
            in:fly={{ y: 12, duration: 220, opacity: 0, easing: cubicOut }}
            out:fly={{ y: -14, duration: 200, opacity: 0, easing: cubicOut }}
          >
            {#if phase === "asking"}
              <div class="stability-prompt__row">
                <span class="stability-prompt__text">{$t("Stability_DidSwitchWork")}</span>
                <div class="stability-prompt__actions">
                  <button
                    type="button"
                    class="stability-prompt__thumb stability-prompt__thumb--up"
                    class:stability-prompt__thumb--pulse={thumbPulse === "up"}
                    aria-label={$t("Stability_ThumbUp")}
                    on:click={onThumbUp}
                  >
                    <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
                      <path
                        fill="currentColor"
                        d="M1 21h4V9H1v12zm22-11c0-1.1-.9-2-2-2h-6.31l.95-4.57.03-.32c0-.41-.17-.79-.44-1.06L14.17 1 7.59 7.59C7.22 7.95 7 8.45 7 9v10c0 1.1.9 2 2 2h9c.83 0 1.54-.5 1.84-1.22l3.02-7.05c.09-.23.14-.47.14-.73v-2z"
                      />
                    </svg>
                  </button>
                  <button
                    type="button"
                    class="stability-prompt__thumb stability-prompt__thumb--down"
                    class:stability-prompt__thumb--pulse={thumbPulse === "down"}
                    aria-label={$t("Stability_ThumbDown")}
                    on:click={onThumbDown}
                  >
                    <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
                      <path
                        fill="currentColor"
                        d="M15 3H6c-.83 0-1.54.5-1.84 1.22l-3.02 7.05c-.09.23-.14.47-.14.73v2c0 1.1.9 2 2 2h6.31l-.95 4.57-.03.32c0 .41.17.79.44 1.06L9.83 23 16.41 16.41c.37-.36.59-.86.59-1.41V5c0-1.1-.9-2-2-2zm4 0v12h4V3h-4z"
                      />
                    </svg>
                  </button>
                </div>
              </div>
            {:else if phase === "thanks"}
              <div class="stability-prompt__message">{$t("Stability_ThanksRating")}</div>
            {:else}
              <div class="stability-prompt__message stability-prompt__message--report">
                <div>{$t("Stability_ThanksReportLine1")}</div>
                <div>
                  {$t("Stability_ThanksReportAskPrefix")}
                  <button type="button" class="stability-prompt__link" on:click={onReportIssue}>
                    {$t("Stability_ReportIssue")}
                  </button>{$t("Stability_ThanksReportAskSuffix")}
                </div>
              </div>
            {/if}
          </div>
        {/key}
      </div>
    </div>
  </div>
{/if}
