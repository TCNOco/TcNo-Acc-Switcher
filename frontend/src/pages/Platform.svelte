<script lang="ts">
  import { onMount } from "svelte";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import ActionBar from "../components/ActionBar.svelte";

  export let name: string;

  $: appBarTitle.set(name || "TcNo Account Switcher");
  $: if (name) {
    route.set({ page: "platform", platformName: name });
  }

  onMount(() => {
    previousPage.set({ page: "home" });
  });
</script>

<div class="main-content"></div>
<ActionBar />
