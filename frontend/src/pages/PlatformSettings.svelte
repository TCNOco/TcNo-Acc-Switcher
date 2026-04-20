<script lang="ts">
    import { route, previousPage, appBarTitle } from '../stores/nav'
  
    export let name: string
  
    $: appBarTitle.set(name + ' - Settings' || 'TcNo Account Switcher')
    previousPage.set({ page: 'platform', platformName: name })
    route.set({ page: 'platform-settings', platformName: name })
</script>

<div class="main-content main-spacing">

</div>