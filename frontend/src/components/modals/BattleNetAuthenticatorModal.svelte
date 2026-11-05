<script lang="ts">
  import { onMount } from "svelte";
  import { Call, Events } from "@wailsio/runtime";
  import { t } from "../../stores/i18n";
  type Summary = {serial:string;label:string;pending:boolean;hasRestoreCode:boolean};
  const call = (method:string,...args:unknown[]) => Call.ByName(`TcNo-Acc-Switcher/internal/steamguard.Service.${method}`,...args);
  let token="", lease="", password="", keyfile="", backupKey="", source="", sourcePassword="", exportPassword="";
  let accounts:Summary[]=[], preview:Summary[]=[], selected:string[]=[], serial="", code="", expires=0, remaining=0, error="", busy=false, ready=false, configured=true;
  let closed=false, polling=false;
  let recovery: {serial:string;restoreCode:string;uri:string;qrImage:string}|null=null;

  async function grant() {
    if (token) { await call("EndSensitiveView",token,lease).catch(()=>{}); token=""; }
    const requestId=crypto.randomUUID();
    await new Promise<void>((resolve,reject)=>{
      const off=Events.On("steamguard:sensitive-view-grant",event=>{
        const data=event.data as {requestId:string;accountId:string;capability:string;lease:string};
        if(data?.requestId!==requestId||data.accountId!=="battlenet-authenticators")return;
        clearTimeout(timer);off();
        if(closed){void call("EndSensitiveView",data.capability,data.lease);reject(new Error("closed"));return;}
        token=data.capability;lease=data.lease;resolve();
      });
      const timer=setTimeout(()=>{off();reject(new Error("Authenticator view unavailable"));},5000);
      void call("RequestBattleNetView",requestId).catch(e=>{clearTimeout(timer);off();reject(e);});
    });
  }
  async function run(action:()=>Promise<void>) {
    if(busy)return;busy=true;error="";
    try{await action();}catch(e){code="";error=String(e);}finally{busy=false;}
  }
  async function refresh(){accounts=await call("ListBattleNetAccounts",token);if(!accounts.some(a=>a.serial===serial))serial=accounts[0]?.serial??"";ready=true;}
  async function unlock(){await run(async()=>{if(!configured){await call("Initialize",password,"");configured=true;await grant();}await call("UnlockBattleNet",password,keyfile,backupKey,false,token);password="";backupKey="";await refresh();});}
  async function pick(){await run(async()=>{source=await call("PickBattleNetImport");preview=[];});}
  async function inspect(){await run(async()=>{preview=await call("PreviewBattleNetImport",source,sourcePassword,token);selected=preview.filter(a=>!accounts.some(existing=>existing.serial===a.serial)).map(a=>a.serial);});}
  async function importSelected(){await run(async()=>{await call("ImportBattleNet",source,sourcePassword,selected,false,token);sourcePassword="";preview=[];await grant();await refresh();});}
  async function exportSelected(){await run(async()=>{const result=await call("ExportBattleNet",[serial],password,keyfile,backupKey,exportPassword,token);if(result){exportPassword="";password="";backupKey="";}});}

  onMount(()=>{
    void run(async()=>{const status=await call("GetSettingsStatus");configured=status.vaultConfigured;await grant();if(status.unlocked)await refresh();});
    const timer=setInterval(async()=>{
      remaining=Math.max(0,Math.ceil((expires-Date.now())/1000));
      if(!ready||!serial||busy||polling||closed)return;
      polling=true;
      try{const result=await call("GetBattleNetCode",serial,token);if(!closed){code=result.code;expires=result.expiresAt;remaining=Math.max(0,Math.ceil((expires-Date.now())/1000));}}catch{code="";recovery=null;ready=false;}finally{polling=false;}
    },1000);
    return()=>{closed=true;clearInterval(timer);code="";recovery=null;password="";sourcePassword="";exportPassword="";backupKey="";if(token)void call("EndSensitiveView",token,lease);};
  });
</script>

<div class="authenticator">
  {#if error}<p role="alert">{error}</p>{/if}
  {#if !ready}
    <p>{$t(configured?"Authenticator_Unlock":"Authenticator_Create")}</p>
    <label>{$t("Authenticator_Password")}<input type="password" bind:value={password} autocomplete="off" /></label>
    <label>{$t("Authenticator_Keyfile")}<input bind:value={keyfile} /></label>
    <label>{$t("Authenticator_BackupKey")}<input type="password" bind:value={backupKey} autocomplete="off" /></label>
    <button disabled={busy||!token} on:click={unlock}>{$t(configured?"Authenticator_Unlock":"Authenticator_Create")}</button>
  {:else}
    {#if accounts.length}
      <label>{$t("Authenticator_Account")}<select bind:value={serial} on:change={()=>{code="";recovery=null;}}>{#each accounts as account}<option value={account.serial}>{account.label||account.serial}</option>{/each}</select></label>
      <output class="auth-code" aria-label={$t("Authenticator_Code")}>{code||"••••••••"}</output>
      <button disabled={busy||!code} on:click={()=>run(async()=>{await call("CopyBattleNetCode",serial,token);})}>{$t("Authenticator_Copy")}</button>
      <progress max="30" value={remaining}></progress>
      <p>{$t("Authenticator_Seconds",{seconds:remaining})}</p>
      <button disabled={busy} on:click={()=>run(async()=>{recovery=recovery?null:await call("RevealBattleNetRecovery",serial,token);})}>{$t("Authenticator_Recovery")}</button>
      {#if recovery}<p>{$t("Authenticator_RecoveryWarning")}</p><p>{recovery.serial}</p><p>{recovery.restoreCode}</p><img src={recovery.qrImage} alt={$t("Authenticator_QR")} width="240" height="240" /><textarea readonly value={recovery.uri}></textarea>{/if}
    {/if}
    <fieldset disabled={busy}>
      <legend>{$t("Authenticator_Import")}</legend>
      <label>{$t("Authenticator_Source")}<input bind:value={source} /></label>
      <button on:click={pick}>{$t("Authenticator_ChooseFile")}</button>
      <label>{$t("Authenticator_SourcePassword")}<input type="password" bind:value={sourcePassword} autocomplete="off" /></label>
      <button disabled={!source} on:click={inspect}>{$t("Authenticator_Preview")}</button>
      {#each preview as account}
        <label><input type="checkbox" bind:group={selected} value={account.serial} disabled={accounts.some(a=>a.serial===account.serial)} />{account.label||account.serial}</label>
      {/each}
      {#if preview.length}<button disabled={!selected.length} on:click={importSelected}>{$t("Authenticator_Import")}</button>{/if}
    </fieldset>
    {#if serial}
      <fieldset disabled={busy}>
        <legend>{$t("Authenticator_Export")}</legend>
        <label>{$t("Authenticator_Password")}<input type="password" bind:value={password} autocomplete="off" /></label>
        <label>{$t("Authenticator_ExportPassword")}<input type="password" bind:value={exportPassword} autocomplete="new-password" /></label>
        <label>{$t("Authenticator_Keyfile")}<input bind:value={keyfile} /></label>
        <label>{$t("Authenticator_BackupKey")}<input type="password" bind:value={backupKey} autocomplete="off" /></label>
        <button disabled={!exportPassword} on:click={exportSelected}>{$t("Authenticator_Export")}</button>
      </fieldset>
    {/if}
  {/if}
</div>

<style>
  .authenticator{display:grid;gap:12px;min-width:360px}label{display:grid;gap:4px}input,select{width:100%}input[type=checkbox]{width:auto}fieldset{display:grid;gap:10px} .auth-code{font-family:monospace;font-size:2.5rem;text-align:center;letter-spacing:.15em}progress{width:100%}
</style>
