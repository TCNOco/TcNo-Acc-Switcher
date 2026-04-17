<script lang="ts">
    import { route, previousPage, appBarTitle } from '../stores/nav'
  
    $: appBarTitle.set('TcNo Account Switcher - Settings')
    previousPage.set({ page: 'home' })
    route.set({ page: 'settings' })
</script>

<div class="main-content">

</div>