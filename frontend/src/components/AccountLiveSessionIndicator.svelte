<script lang="ts">
  import { tooltip } from "../lib/actions/tooltip";

  export let active = false;
  export let tooltipText = "";
  export let boundary: HTMLElement | undefined = undefined;
</script>

{#if active}
  <span
    class="live-session-badge"
    use:tooltip={tooltipText ? { text: tooltipText, placement: "right", boundary } : undefined}
    aria-label={tooltipText || ""}
  >
    <slot />
  </span>
{/if}
