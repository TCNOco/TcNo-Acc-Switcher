<script lang="ts">
  import { tick, onMount } from "svelte";
  import { createEventDispatcher } from "svelte";
  import type { CrashReportChoice } from "../../stores/modal";
  import { t } from "../../stores/i18n";

  const dispatch = createEventDispatcher<{ resolve: CrashReportChoice }>();

  let yesEl: HTMLButtonElement | undefined;

  function choose(choice: CrashReportChoice): void {
    dispatch("resolve", choice);
  }

  onMount(() => {
    void tick().then(() =>
      requestAnimationFrame(() => {
        yesEl?.focus();
      }),
    );
  });
</script>

<div class="modal-block">
  <p class="modal-crash-report-body">{$t("Modal_CrashReport_Body")}</p>
  <div class="modal-inline-actions settingsCol inputAndButton">
    <span class="modal-actions-spacer"></span>
    <button type="button" class="btnicontext" on:click={() => choose("no")}>
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M4.6 2.9 12 10.3l7.4-7.4 1.7 1.7L13.7 12l7.4 7.4-1.7 1.7L12 13.7l-7.4 7.4-1.7-1.7L10.3 12 2.9 4.6z"/></svg>
      {$t("No")}
    </button>
    <button
      type="button"
      class="btnicontext modal-primary"
      bind:this={yesEl}
      on:click={() => choose("yes")}
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M3.2 12.0 5.6 9.6l3.8 3.8L18.4 4.4l2.4 2.4L9.4 18.2z"/></svg>
      {$t("Yes")}
    </button>
    <button type="button" class="btnicontext" on:click={() => choose("always")}>
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M0.07 13.98 5.79 20.89 11.29 8.34 7.91 6.86 4.81 13.91 2.93 11.62ZM10.67 13.98 16.39 20.89 21.89 8.34 18.51 6.86 15.41 13.91 13.53 11.62Z"/></svg>
      {$t("Button_Always")}
    </button>
  </div>
</div>

<style lang="scss">
  .modal-crash-report-body {
    margin: 0;
    white-space: pre-line;
    line-height: 1.45;
    color: var(--modal-body-fg, var(--whiteSecondary));
  }
</style>
