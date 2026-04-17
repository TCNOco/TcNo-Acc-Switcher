<script lang="ts">
    import { route, previousPage, appBarTitle } from '../stores/nav'
  
    export let name: string
  
    $: appBarTitle.set(name || 'TcNo Account Switcher')
    previousPage.set({ page: 'home' })
    route.set({ page: 'platform', platformName: name })
</script>