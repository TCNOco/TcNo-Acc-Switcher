<script lang="ts">
  import TitleBar from './components/TitleBar.svelte'

  import Home from './pages/Home.svelte'
  import Settings from './pages/Settings.svelte'
  import Platform from './pages/Platform.svelte'
  import PlatformSettings from './pages/PlatformSettings.svelte'
  import ManagePlatforms from './pages/ManagePlatforms.svelte'
  import { route } from './stores/nav'
</script>

<div class="container">
  <TitleBar />

  <div class="page">
    {#if $route.page === 'home'}
      <Home />
    {:else if $route.page === 'settings'}
      <Settings />
    {:else if $route.page === 'platform'}
      <Platform name={$route.platformName} />
    {:else if $route.page === 'platform-settings'}
      <PlatformSettings name={$route.platformName} />
    {:else if $route.page === 'manage-platforms'}
      <ManagePlatforms />
    {/if}
  </div>
</div>

<style>
  .container {
    background: var(--program-bg);
    height: 100vh;
    width: 100vw;
    display: flex;
    flex-direction: column;
  }
  .page {
    border-left: var(--border-bar-size) solid var(--border-bar-bg);
    border-right: var(--border-bar-size) solid var(--border-bar-bg);
    border-bottom: var(--border-bar-size) solid var(--border-bar-bg);
    flex: 1;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }
</style>
