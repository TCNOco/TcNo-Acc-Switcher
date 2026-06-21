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
      {$t("No")}
    </button>
    <button
      type="button"
      class="btnicontext modal-primary"
      bind:this={yesEl}
      on:click={() => choose("yes")}
    >
      {$t("Yes")}
    </button>
    <button type="button" class="btnicontext" on:click={() => choose("always")}>
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
