<script lang="ts">
  /**
   * What the lock icon opens: the certificate serving the current page.
   *
   * The details come from a separate TLS handshake made by the application, not
   * from the connection the page was loaded over. That is stated here rather
   * than implied, because a popover that looks like a browser's is otherwise
   * making a claim it cannot back.
   */
  import { createEventDispatcher } from "svelte";
  import { t } from "../../stores/i18n";
  import type { Certificate } from "../../../bindings/TcNo-Acc-Switcher/internal/steambrowser/models.js";

  export let certificate: Certificate | null = null;
  export let error = "";

  const dispatch = createEventDispatcher<{ close: void }>();

  function formatDate(value: string): string {
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
  }

  /** Group the fingerprint so it can be compared by eye. */
  function groupFingerprint(value: string): string {
    return (value.match(/.{1,4}/g) ?? [value]).join(" ");
  }
</script>

<svelte:window on:keydown={(e) => e.key === "Escape" && dispatch("close")} />

<div class="cert" role="dialog" aria-label={$t("SteamBrowser_Cert_Title")}>
  <header class="cert__head">
    <strong>{$t("SteamBrowser_Cert_Title")}</strong>
    <button type="button" class="cert__close" on:click={() => dispatch("close")}
      aria-label={$t("Button_Close")}>✕</button>
  </header>

  {#if error}
    <p class="cert__error" role="alert">{error}</p>
  {:else if !certificate}
    <p class="cert__pending">{$t("SteamBrowser_Cert_Loading")}</p>
  {:else}
    <dl class="cert__grid">
      <dt>{$t("SteamBrowser_Cert_Host")}</dt><dd>{certificate.host}</dd>
      <dt>{$t("SteamBrowser_Cert_Subject")}</dt><dd>{certificate.subject || "—"}</dd>
      <dt>{$t("SteamBrowser_Cert_Issuer")}</dt><dd>{certificate.issuer || "—"}</dd>
      <dt>{$t("SteamBrowser_Cert_Valid")}</dt>
      <dd>{formatDate(certificate.notBefore)} — {formatDate(certificate.notAfter)}</dd>
      <dt>{$t("SteamBrowser_Cert_Protocol")}</dt>
      <dd>{certificate.tlsVersion}, {certificate.cipherSuite}</dd>
      <dt>{$t("SteamBrowser_Cert_Fingerprint")}</dt>
      <dd class="cert__mono">{groupFingerprint(certificate.sha256)}</dd>
      {#if certificate.dnsNames?.length}
        <dt>{$t("SteamBrowser_Cert_Names")}</dt>
        <dd>{certificate.dnsNames.join(", ")}</dd>
      {/if}
    </dl>
    {#if certificate.fromSeparateConnection}
      <p class="cert__note">{$t("SteamBrowser_Cert_SeparateConnection")}</p>
    {/if}
  {/if}
</div>

<style lang="scss">
  .cert {
    pointer-events: auto;
    position: absolute;
    top: 100%;
    left: 96px;
    z-index: 10;
    width: min(460px, calc(100vw - 32px));
    padding: 12px 14px;
    border: 1px solid var(--border, #2c3a4f);
    border-radius: 10px;
    background: var(--modal-bg, #1b2636);
    color: var(--text, #e6edf5);
    box-shadow: 0 12px 32px rgb(0 0 0 / 45%);
    font-size: 12px;
  }

  .cert__head {
    display: flex; align-items: center; justify-content: space-between;
    margin-bottom: 8px;
  }

  .cert__close {
    border: 0; background: transparent; cursor: pointer;
    color: var(--text-muted, #91a3ba); font-size: 12px;
  }

  .cert__grid {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 4px 12px;
    margin: 0;

    dt { color: var(--text-muted, #91a3ba); }
    dd { margin: 0; overflow-wrap: anywhere; }
  }

  .cert__mono { font-family: ui-monospace, Consolas, monospace; }

  .cert__note {
    margin: 10px 0 0;
    color: var(--text-muted, #91a3ba);
    font-size: 11px;
  }

  .cert__error { margin: 0; color: var(--danger, #d9534f); }
  .cert__pending { margin: 0; color: var(--text-muted, #91a3ba); }
</style>
