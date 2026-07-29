<script lang="ts">
  export let title: string = "";
</script>

<section class="settings-group">
  {#if title}
    <h2 class="SettingsHeader">{title}</h2>
  {/if}
  <slot />
</section>
