<script lang="ts">
  import type { Readable } from "svelte/store";

  export let progress: Readable<number>;

  $: fraction = Math.max(0, Math.min(1, $progress));
  $: level = fraction > 0.4 ? "ok" : fraction > 0.2 ? "warn" : "low";
</script>

<span
  class="ctx-menu__progress ctx-menu__progress--{level}"
  style={`--ctx-progress: ${fraction}`}
  aria-hidden="true"
></span>
