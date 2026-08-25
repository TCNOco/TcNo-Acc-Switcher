<script lang="ts">
  import { tooltip } from "../lib/actions/tooltip";
  import { DUR, scaleFade } from "../lib/animation";

  export let active = false;
  export let tooltipText = "";
  export let boundary: HTMLElement | undefined = undefined;
</script>

{#if active}
  <span
    class="live-session-badge"
    use:tooltip={tooltipText ? { text: tooltipText, placement: "right", boundary } : undefined}
    aria-label={tooltipText || ""}
    in:scaleFade={{ start: 0.2, duration: DUR.normal }}
    out:scaleFade={{ start: 0.4, duration: DUR.fast }}
  >
    <slot />
  </span>
{/if}

<style lang="scss">
  /* Marks the account whose session is live right now. The tile's dashed border
     says the same thing, but only to someone who already knows what a dashed
     border means here - and it is the same green whether or not the tile is also
     the selected one, so at a glance the two states looked identical.

     Anchored to `.acc_list_item`, the nearest positioned ancestor: `label.acc`
     is deliberately left unpositioned, since the profile-image drop overlay
     inside it already resolves against the same box. */
  .live-session-badge {
    position: absolute;
    top: 6px;
    left: 6px;
    z-index: 3;
    width: 9px;
    height: 9px;
    border-radius: 50%;
    background: var(--role-success, var(--green, #8aff80));
    /* The ring punches the dot out of whatever it lands on, so it stays legible
       over a bright avatar or a busy wallpaper. */
    box-shadow:
      0 0 0 2px var(--mainContentBackground, var(--code-background, #131a20)),
      0 0 7px var(--role-success, var(--green, #8aff80));
  }

  @media (forced-colors: active) {
    .live-session-badge {
      background: Highlight;
      box-shadow: 0 0 0 2px Canvas;
    }
  }
</style>
