<script lang="ts">
    import { route, previousPage, appBarTitle } from '../stores/nav'
    import ActionBar from '../components/ActionBar.svelte'
  
    export let name: string
  
    $: appBarTitle.set(name || 'TcNo Account Switcher')
    previousPage.set({ page: 'home' })
    route.set({ page: 'platform', platformName: name })
</script>

<div class="main-content">
</div>
<ActionBar />