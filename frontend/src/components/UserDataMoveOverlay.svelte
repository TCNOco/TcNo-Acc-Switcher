<script lang="ts">
  import { t } from "../stores/i18n";
  import { userDataMoveOverlay } from "../stores/userDataMove";

  $: pct =
    $userDataMoveOverlay.total > 0
      ? Math.min(100, Math.round(($userDataMoveOverlay.done / $userDataMoveOverlay.total) * 100))
      : 0;
  $: indeterminate = $userDataMoveOverlay.total <= 0;
  $: label =
    $userDataMoveOverlay.phase === "restarting"
      ? $t("Overlay_UserDataMoveRestarting")
      : $t("Overlay_UserDataMove");
</script>

{#if $userDataMoveOverlay.active}
  <div class="fileDropOverlay userDataMoveOverlay" aria-busy="true" aria-live="polite">
    <div class="fileDropOverlay__inner">
      <div class="userDataMoveOverlay__spinner" aria-hidden="true"></div>
      <p class="fileDropOverlay__text">{label}</p>
      <div class="userDataMoveOverlay__bar" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={indeterminate ? undefined : pct}>
        <div
          class="userDataMoveOverlay__barFill"
          class:userDataMoveOverlay__barFill--indeterminate={indeterminate}
          style={indeterminate ? undefined : `width: ${pct}%`}
        ></div>
      </div>
      {#if !indeterminate && $userDataMoveOverlay.phase === "copying"}
        <p class="userDataMoveOverlay__pct">{pct}%</p>
      {/if}
    </div>
  </div>
{/if}

<style lang="scss">
  .userDataMoveOverlay {
    pointer-events: all;
    cursor: progress;
  }

  .userDataMoveOverlay__spinner {
    width: 3rem;
    height: 3rem;
    border: 3px solid var(--overlay-white-15);
    border-top-color: var(--accent-text-heading);
    border-radius: 50%;
    animation: userDataMoveSpin 0.85s linear infinite;
  }

  @keyframes userDataMoveSpin {
    to {
      transform: rotate(360deg);
    }
  }

  .userDataMoveOverlay__bar {
    width: min(280px, 70vw);
    height: 8px;
    border-radius: 999px;
    overflow: hidden;
    background: var(--overlay-white-12);
  }

  .userDataMoveOverlay__barFill {
    height: 100%;
    border-radius: inherit;
    background: var(--accent-text-heading);
    transition: width 0.15s ease-out;
  }

  .userDataMoveOverlay__barFill--indeterminate {
    width: 35% !important;
    animation: userDataMoveIndeterminate 1.2s ease-in-out infinite;
  }

  @keyframes userDataMoveIndeterminate {
    0% {
      transform: translateX(-120%);
    }
    100% {
      transform: translateX(320%);
    }
  }

  .userDataMoveOverlay__pct {
    margin: 0;
    font-size: 0.95rem;
    font-weight: 600;
    color: var(--whiteSecondary);
  }
</style>
