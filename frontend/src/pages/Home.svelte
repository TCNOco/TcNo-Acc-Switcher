<script lang="ts">
    import ActionBar from '../components/ActionBar.svelte'
    import {Events} from "@wailsio/runtime";
    import {GreetService} from "../../bindings/changeme";
    import { route, appBarTitle } from '../stores/nav'
    $: appBarTitle.set('TcNo Account Switcher')
  
    let name: string = '';
    let result: string = 'Please enter your name below 👇';
    let time: string = 'Listening for Time event...';
  
    const doGreet = (): void => {
      let localName = name;
      if (!localName) {
        localName = 'anonymous';
      }
      GreetService.Greet(localName).then((resultValue: string) => {
        result = resultValue;
      }).catch((err: any) => {
        console.log(err);
      });
    }
  
    Events.On('time', (timeValue: any) => {
      time = timeValue.data;
    });


    function openPlatform(name: string) {
        route.set({ page: 'platform', platformName: name })
    }
</script>

<div class="main-content">
    <div>
        <span data-wml-openURL="https://wails.io">
        <img src="/wails.png" class="logo" alt="Wails logo"/>
        </span>
        <span data-wml-openURL="https://svelte.dev">
        <img src="/svelte.svg" class="logo svelte" alt="Svelte logo"/>
        </span>
    </div>
    <h1>Wails + Svelte</h1>
    <div aria-label="result" class="result">{result}</div>
    <div class="card">
        <div class="input-box">
        <input aria-label="input" class="input" bind:value={name} type="text" autocomplete="off"/>
        <button aria-label="greet-btn" class="btn" on:click={doGreet}>Greet</button>
        </div>
    </div>
    <div class="footer">
        <div><p>Click on the Wails logo to learn more</p></div>
        <div><p>{time}</p></div>
    </div>
</div>
<ActionBar />