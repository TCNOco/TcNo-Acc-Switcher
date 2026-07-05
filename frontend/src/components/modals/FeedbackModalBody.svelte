<script lang="ts">
  import { tick, onMount } from "svelte";
  import { createEventDispatcher } from "svelte";
  import { get } from "svelte/store";
  import * as PlatformService from "../../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { openExternalUrl } from "../../lib/openExternalUrl";
  import { t } from "../../stores/i18n";
  import { cancelActiveModal } from "../../stores/modal";
  import { offlineMode } from "../../stores/offlineMode";
  import { pushToast } from "../../stores/toast";
  import { formatToastWithError } from "../../lib/formatWailsError";

  const DISCORD_URL = "https://s.tcno.co/AccSwitcherDiscord";
  const FEEDBACK_MAX_LENGTH = 2000;

  export let mode: "issue" | "suggestion" = "issue";
  export let platform = "";

  const dispatch = createEventDispatcher<{ resolve: string | null }>();

  let value = "";
  let sendLog = true;
  let feedbackEl: HTMLTextAreaElement | undefined;

  $: submitDisabled = value.trim().length === 0;

  async function submit(): Promise<void> {
    if (submitDisabled) return;
    const text = value.trim();
    const kind = mode === "issue" ? "switch_issue" : "feature_suggestion";
    try {
      await PlatformService.SubmitFeedback(kind, platform, text, mode === "issue" && sendLog);
      dispatch("resolve", text);
      pushToast({
        type: "success",
        message: get(t)("Toast_FeedbackThanks"),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError(get(t)("Toast_FeedbackSubmitFailed"), e),
        duration: 8000,
      });
    }
  }

  function cancel(): void {
    dispatch("resolve", null);
  }

  function openDiscordLink(e: MouseEvent): void {
    e.preventDefault();
    void openExternalUrl(DISCORD_URL);
  }

  onMount(() => {
    void tick().then(() =>
      requestAnimationFrame(() => {
        feedbackEl?.focus();
      }),
    );
  });
</script>

<div class="modal-block">
  <p id="feedback-body" class="modal-feedback-body">
    {mode === "issue" ? $t("Feedback_Issue_Body") : $t("Feedback_Suggestion_Body")}
  </p>
  <div class="modal-input-row modal-feedback-input-row">
    <textarea
      bind:this={feedbackEl}
      bind:value
      class="modal-input modal-input--multiline"
      rows="6"
      maxlength={FEEDBACK_MAX_LENGTH}
      spellcheck="true"
      autocomplete="off"
      aria-label={mode === "issue" ? $t("Feedback_Issue_Title") : $t("Feedback_Suggestion_Title")}
      aria-describedby="feedback-body feedback-char-count"
    ></textarea>
  </div>
  <div id="feedback-char-count" class="modal-feedback-char-count" aria-live="polite">
    {value.length} / {FEEDBACK_MAX_LENGTH}
  </div>
  {#if mode === "issue" && !$offlineMode}
    <label class="modal-feedback-attach-log">
      <input type="checkbox" bind:checked={sendLog} />
      <span>{$t("Feedback_AttachLog")}</span>
    </label>
    <p class="modal-feedback-attach-log-hint">{$t("Feedback_AttachLog_Hint")}</p>
  {/if}
  <div class="modal-inline-actions settingsCol inputAndButton">
    <span class="modal-actions-spacer"></span>
    <button type="button" class="btnicontext" on:click={cancel}>
      {$t("Button_Cancel")}
    </button>
    <button
      type="button"
      class="btnicontext modal-primary"
      disabled={submitDisabled}
      on:click={submit}
    >
      {$t("Feedback_Submit")}
    </button>
  </div>
  <div class="modal-feedback-footer">
    <button type="button" class="fancyLink modal-feedback-discord" on:click={openDiscordLink}>
      {$t("Feedback_DiscordLink")}
    </button>
  </div>
</div>

<style lang="scss">
  .modal-feedback-body {
    margin: 0;
    color: var(--modal-body-fg, var(--whiteSecondary, #fff));
    line-height: 1.4;
  }

  .modal-feedback-char-count {
    margin: 0.2rem 0 0;
    font-size: 0.85rem;
    color: var(--modal-muted-fg, var(--blackTernary, #a7abbe));
    text-align: right;
  }

  .modal-feedback-attach-log {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin: 0.75rem 0 0.25rem;
    cursor: pointer;
    color: var(--modal-body-fg, var(--whiteSecondary, #fff));
    font-size: 0.95rem;
  }

  .modal-feedback-attach-log-hint {
    margin: 0 0 0.5rem;
    font-size: 0.85rem;
    color: var(--modal-muted-fg, var(--blackTernary, #a7abbe));
    line-height: 1.35;
  }

  .modal-feedback-footer {
    margin-top: 0.35rem;
    text-align: center;
  }

  .modal-feedback-discord {
    font-size: 0.95rem;
  }
</style>
