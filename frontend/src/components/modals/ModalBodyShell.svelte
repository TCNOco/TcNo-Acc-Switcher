<script lang="ts">
  import type { ComponentType, SvelteComponent } from "svelte";

  export let html: string | undefined = undefined;
  export let component: ComponentType<SvelteComponent> | undefined = undefined;
  export let componentProps: Record<string, unknown> | undefined = undefined;
</script>

{#if component}
  <div class="modal-html modal-text">
    <svelte:component this={component} {...componentProps ?? {}} />
  </div>
{:else if html}
  <div class="modal-html modal-text">{@html html}</div>
{/if}

<style lang="scss">
  .modal-text {
    margin: 0;
    color: var(--white, #fff);
    line-height: 1.45;
  }

  .modal-html :global(p) {
    margin: 0 0 0.5em;
  }

  .modal-html :global(p:last-child) {
    margin-bottom: 0;
  }
</style>
