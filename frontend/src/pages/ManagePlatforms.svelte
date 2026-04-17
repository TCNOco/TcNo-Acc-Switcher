<script lang="ts">
    import { route, previousPage, appBarTitle } from '../stores/nav'
  
    $: appBarTitle.set('TcNo Account Switcher - Platform Settings')
    previousPage.set({ page: 'home' })
    route.set({ page: 'manage-platforms' })
</script>

<div class="main-content">

</div>