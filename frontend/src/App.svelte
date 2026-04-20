<script lang="ts">
  import { onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
  import TitleBar from './components/TitleBar.svelte'
  import AppModal from './components/AppModal.svelte'
  import Toast from './components/Toast.svelte'

  import Home from './pages/Home.svelte'
  import Settings from './pages/Settings.svelte'
  import Test from './pages/Test.svelte'
  import Platform from './pages/Platform.svelte'
  import PlatformSteam from './pages/PlatformSteam.svelte'
  import PlatformSettings from './pages/PlatformSettings.svelte'
  import ManagePlatforms from './pages/ManagePlatforms.svelte'
  import { route } from './stores/nav'
  import { actionBarStatus } from './stores/actionBarStatus'

  onMount(() => {
    const off = Events.On("action-bar-status", (ev) => {
      actionBarStatus.set(typeof ev.data === "string" ? ev.data : "");
    });
    return () => off?.();
  });
</script>

<div class="container">
  <TitleBar />
  <div class="page">
    {#if $route.page === 'home'}
      <Home />
    {:else if $route.page === 'settings'}
      <Settings />
    {:else if $route.page === 'test'}
      <Test />
    {:else if $route.page === 'platform'}
      {#if $route.platformName === 'Steam'}
        <PlatformSteam name={$route.platformName} />
      {:else}
        <Platform name={$route.platformName} />
      {/if}
    {:else if $route.page === 'platform-settings'}
      <PlatformSettings name={$route.platformName} />
    {:else if $route.page === 'manage-platforms'}
      <ManagePlatforms />
    {/if}
    <AppModal />
    <Toast />
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
    position: relative;
    border-left: var(--border-bar-size) solid var(--border-bar-bg);
    border-right: var(--border-bar-size) solid var(--border-bar-bg);
    border-bottom: var(--border-bar-size) solid var(--border-bar-bg);
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }
</style>
