<script lang="ts">
  import { Browser } from "@wailsio/runtime";
  import { onMount } from "svelte";
  import { get } from "svelte/store";
  import * as BasicService from "../../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import { t } from "../../stores/i18n";
  import { pushToast } from "../../stores/toast";
  import { formatToastWithError } from "../../lib/formatWailsError";
  import { offlineMode } from "../../stores/offlineMode";

  /** Matches backend `gameStatVarNeedsComputation` for form fields. */
  function gameStatDefNeedsComputation(defVal: string): boolean {
    const v = defVal.trim();
    if (!v) {
      return false;
    }
    // Plain single token autofill should remain editable (prefill only).
    if (/^%[A-Za-z0-9_]+%$/.test(v)) {
      return false;
    }
    if (v.includes("%")) {
      return true;
    }
    return /\|\s*\w+\s*\(/.test(v);
  }

  export let platformKey: string;
  export let uniqueId: string;
  export let displayName: string;
  export let onApplied: (() => void) | undefined = undefined;

  type Screen = "list" | "vars";
  let screen: Screen = "list";
  let editGame = "";
  let loading = true;
  let enabledGames: string[] = [];
  let disabledGames: string[] = [];
  let busy = false;

  let requiredLabels: Record<string, string> = {};
  let requiredSpecs: Record<string, { autofill: string; display: string; placeholder: string }> = {};
  let formValues: Record<string, string> = {};
  let resolvedValues: Record<string, string> = {};
  let hiddenToggles: Record<string, { hidden: boolean; toggleText: string }> = {};
  let hiddenPicked = new Set<string>();
  let attribution: { header: string; image: string; text: string; link: string; dimensions: string } | null = null;

  const defaultAttributionHeader = "Data source:";

  function normalizeVarSpecs(
    src: Record<string, { autofill?: string; display?: string; placeholder?: string } | undefined> | null | undefined,
  ): Record<string, { autofill: string; display: string; placeholder: string }> {
    const out: Record<string, { autofill: string; display: string; placeholder: string }> = {};
    if (!src) {
      return out;
    }
    for (const [k, v] of Object.entries(src)) {
      out[k] = {
        autofill: String(v?.autofill ?? ""),
        display: String(v?.display ?? ""),
        placeholder: String(v?.placeholder ?? ""),
      };
    }
    return out;
  }

  function normalizeStringMap(src: Record<string, string | undefined> | null | undefined): Record<string, string> {
    const out: Record<string, string> = {};
    if (!src) {
      return out;
    }
    for (const [k, v] of Object.entries(src)) {
      out[k] = String(v ?? "");
    }
    return out;
  }

  function normalizeAttribution(
    src: { header?: string; image?: string; text?: string; link?: string; dimensions?: string } | null | undefined,
  ): { header: string; image: string; text: string; link: string; dimensions: string } | null {
    const link = String(src?.link ?? "").trim();
    if (!link) {
      return null;
    }
    const header = String(src?.header ?? "").trim();
    return {
      header: header || defaultAttributionHeader,
      image: String(src?.image ?? "").trim(),
      text: String(src?.text ?? "").trim(),
      link,
      dimensions: String(src?.dimensions ?? "").trim(),
    };
  }

  function parseAttributionDimensions(raw: string): { width: number; height: number } | null {
    const m = String(raw ?? "").trim().match(/^(\d+)x(\d+)$/i);
    if (!m) {
      return null;
    }
    const width = Number(m[1]);
    const height = Number(m[2]);
    if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
      return null;
    }
    return { width, height };
  }

  function attributionImageStyle(dimensions: string): string {
    const parsed = parseAttributionDimensions(dimensions);
    if (!parsed) {
      return "";
    }
    return `width:${parsed.width}px;max-width:100%;aspect-ratio:${parsed.width}/${parsed.height};height:auto;`;
  }

  function attributionImageSrc(imagePath: string): string {
    const p = imagePath.trim();
    if (!p) {
      return "";
    }
    if (p.startsWith("/") || /^https?:\/\//i.test(p)) {
      return p;
    }
    return p.startsWith("img/") ? p : `img/${p.replace(/^\/+/, "")}`;
  }

  function openAttributionLink(url: string): void {
    if (!url.trim()) {
      return;
    }
    if (get(offlineMode)) {
      pushToast({
        type: "info",
        message: get(t)("Toast_OfflineModeNoLinks"),
        duration: 5000,
      });
      return;
    }
    void Browser.OpenURL(url);
  }

  function normalizeHiddenToggles(
    src: Record<string, { hidden?: boolean; toggleText?: string } | undefined> | null | undefined,
  ): Record<string, { hidden: boolean; toggleText: string }> {
    const out: Record<string, { hidden: boolean; toggleText: string }> = {};
    if (!src) {
      return out;
    }
    for (const [k, v] of Object.entries(src)) {
      out[k] = {
        hidden: Boolean(v?.hidden),
        toggleText: String(v?.toggleText ?? ""),
      };
    }
    return out;
  }

  async function loadList(): Promise<void> {
    loading = true;
    try {
      const [en, dis] = await Promise.all([
        BasicService.GetEnabledGames(platformKey, uniqueId),
        BasicService.GetDisabledGames(platformKey, uniqueId),
      ]);
      enabledGames = en ?? [];
      disabledGames = dis ?? [];
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError(get(t)("Toast_FailedStatsLoad"), e),
        duration: 8000,
      });
    } finally {
      loading = false;
    }
  }

  async function openVarsForGame(game: string): Promise<void> {
    editGame = game;
    screen = "vars";
    busy = true;
    attribution = null;
    try {
      const [req, exist, res, hid, attr] = await Promise.all([
        BasicService.GetRequiredVarSpecs(game),
        BasicService.GetExistingVars(game, uniqueId),
        BasicService.GetResolvedGameStatVars(platformKey, game, uniqueId),
        BasicService.GetHiddenMetrics(game, uniqueId),
        BasicService.GetGameAttribution(game),
      ]);
      requiredSpecs = normalizeVarSpecs(req as Record<
        string,
        { autofill?: string; display?: string; placeholder?: string } | undefined
      >);
      requiredLabels = Object.fromEntries(
        Object.entries(requiredSpecs).map(([k, v]) => [k, v.display || k]),
      );
      resolvedValues = normalizeStringMap(res as Record<string, string | undefined>);
      const ex = normalizeStringMap(exist as Record<string, string | undefined>);
      formValues = {};
      for (const k of Object.keys(requiredSpecs)) {
        const def = requiredSpecs[k]?.autofill ?? "";
        const ev = (ex[k] ?? "").trim();
        if (def.trim() === "%ACCOUNTID%" && !ev) {
          formValues[k] = uniqueId;
        } else if (ev === "" && /^%[A-Za-z0-9_]+%$/.test(def.trim())) {
          // Plain token autofill should pre-populate editable fields.
          formValues[k] = (resolvedValues[k] ?? "").trim();
        } else {
          formValues[k] = ev;
        }
      }
      hiddenToggles = normalizeHiddenToggles(hid as Record<
        string,
        { hidden?: boolean; toggleText?: string } | undefined
      >);
      hiddenPicked = new Set(
        Object.entries(hiddenToggles)
          .filter(([, v]) => v?.hidden)
          .map(([k]) => k),
      );
      attribution = normalizeAttribution(
        attr as { header?: string; image?: string; text?: string; link?: string; dimensions?: string },
      );
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError(get(t)("Toast_SaveFailed"), e),
        duration: 8000,
      });
      screen = "list";
    } finally {
      busy = false;
    }
  }

  function toggleMetric(key: string, next: boolean): void {
    if (next) {
      hiddenPicked.delete(key);
    } else {
      hiddenPicked.add(key);
    }
    hiddenPicked = new Set(hiddenPicked);
  }

  async function saveVars(): Promise<void> {
    if (!editGame) {
      return;
    }
    const tr = get(t);
    busy = true;
    const setGameVarsRejected = "SetGameVarsRejected";
    try {
      const hiddenList = Object.keys(hiddenToggles).filter((k) => hiddenPicked.has(k));
      try {
        const ok = await BasicService.SetGameVars(platformKey, editGame, uniqueId, formValues, hiddenList);
        if (!ok) {
          throw new Error(setGameVarsRejected);
        }
      } catch (e) {
        const base =
          e instanceof Error && e.message === setGameVarsRejected
            ? tr("Toast_SaveFailed")
            : tr("Toast_GameStatsLoadFail", { Game: editGame ?? "" });
        pushToast({
          type: "error",
          message: formatToastWithError(base, e),
          duration: 8000,
        });
        return;
      }
      pushToast({
        type: "success",
        message: tr("Toast_AccountSaved"),
        duration: 3000,
      });
      onApplied?.();
      screen = "list";
      await loadList();
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError(tr("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      busy = false;
    }
  }

  async function doRefresh(): Promise<void> {
    if (!editGame || get(offlineMode)) {
      return;
    }
    busy = true;
    try {
      await BasicService.RefreshGameStats(platformKey, editGame, uniqueId);
      pushToast({
        type: "success",
        message: get(t)("Toast_AccountSaved"),
        duration: 3000,
      });
      onApplied?.();
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError(get(t)("Toast_GameStatsLoadFail", { Game: editGame }), e),
        duration: 8000,
      });
    } finally {
      busy = false;
    }
  }

  async function onDisabledToggle(game: string, checked: boolean): Promise<void> {
    if (!checked) {
      return;
    }
    await openVarsForGame(game);
  }

  async function disableGame(game: string): Promise<void> {
    busy = true;
    try {
      await BasicService.DisableGame(game, uniqueId);
      await loadList();
      onApplied?.();
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError(get(t)("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      busy = false;
    }
  }

  onMount(() => {
    void loadList();
  });

  function checkboxChecked(ev: Event): boolean {
    return (ev.currentTarget as HTMLInputElement | null)?.checked ?? false;
  }

  $: tr = $t;
</script>

<div class="gamestats-modal">
  {#if screen === "list"}
    <p class="modal-lead">
      {tr("Modal_GameStats_Header", { accountName: displayName || uniqueId })}
    </p>
    {#if loading}
      <p class="muted">…</p>
    {:else}
      <div class="gamestats-scroll">
        {#if enabledGames.length === 0 && disabledGames.length === 0}
          <p class="muted">{tr("Stats_Disabled")}</p>
        {/if}
        {#each enabledGames as g}
          <div class="gamestats-row">
            <label class="gamestats-check">
              <input
                type="checkbox"
                checked
                disabled={busy}
                on:change={(ev) => {
                  const on = checkboxChecked(ev);
                  if (!on) {
                    void disableGame(g);
                  }
                }}
              />
              <span>{g}</span>
            </label>
            <span class="gamestats-actions">
              <button type="button" class="linkbtn" disabled={busy} on:click={() => void openVarsForGame(g)}>
                {tr("Edit")}
              </button>
              <button
                type="button"
                class="linkbtn"
                disabled={busy || $offlineMode}
                on:click={() => {
                  editGame = g;
                  void doRefresh();
                }}
              >
                {tr("Refresh")}
              </button>
            </span>
          </div>
        {/each}
        {#each disabledGames as g}
          <div class="gamestats-row">
            <label class="gamestats-check">
              <input
                type="checkbox"
                checked={false}
                disabled={busy}
                on:change={(ev) => {
                  const on = checkboxChecked(ev);
                  void onDisabledToggle(g, on);
                }}
              />
              <span>{g}</span>
            </label>
          </div>
        {/each}
      </div>
    {/if}
  {:else}
    <p class="modal-lead">
      {tr("Modal_GameVars_Header", {
        game: editGame,
        username: displayName || uniqueId,
        platform: platformKey,
      })}
    </p>
    {#if busy}
      <p class="muted">…</p>
    {:else}
      <div class="gamestats-scroll">
        {#each Object.entries(requiredLabels) as [varKey, rawLabel]}
          {@const spec = requiredSpecs[varKey]}
          {@const defStr = String(spec?.autofill ?? rawLabel ?? "")}
          {@const parts = String(rawLabel).split("[")}
          {@const label = String(spec?.display ?? rawLabel ?? varKey).trim() || varKey}
          {@const placeholder = (spec?.placeholder ?? (parts.length > 1 ? parts[1].replace("]", "").trim() : "")).trim()}
          {@const computed = gameStatDefNeedsComputation(defStr)}
          <div class="field-row">
            <span class="field-label">{label}</span>
            {#if computed}
              <input
                type="text"
                class="modal-input"
                readonly
                value={resolvedValues[varKey] ?? formValues[varKey] ?? (defStr.trim() === "%ACCOUNTID%" ? uniqueId : "")}
              />
            {:else}
              <input
                type="text"
                class="modal-input"
                spellcheck="false"
                placeholder={placeholder}
                bind:value={formValues[varKey]}
              />
            {/if}
          </div>
        {/each}

        {#if Object.keys(hiddenToggles).length > 0}
          <h6 class="gamestats-sub">{tr("Stats_MetricsToShow")}</h6>
          {#each Object.entries(hiddenToggles) as [metricKey, meta]}
            <label class="metric-row">
              <input
                type="checkbox"
                checked={!hiddenPicked.has(metricKey)}
                on:change={(ev) => toggleMetric(metricKey, checkboxChecked(ev))}
              />
              <span>{meta.toggleText || metricKey}</span>
            </label>
          {/each}
        {/if}

        {#if attribution}
          <div class="gamestats-attribution">
            <h6 class="gamestats-sub">{attribution.header}</h6>
            <div class="attribution-inset">
              {#if attribution.image}
                <button
                  type="button"
                  class="attribution-image-btn"
                  title={attribution.link}
                  on:click={() => openAttributionLink(attribution?.link ?? "")}
                >
                  <img
                    src={attributionImageSrc(attribution.image)}
                    alt=""
                    draggable="false"
                    class:has-dimensions={Boolean(parseAttributionDimensions(attribution.dimensions))}
                    style={attributionImageStyle(attribution.dimensions)}
                  />
                </button>
              {:else if attribution.text}
                <p class="attribution-text">
                  <strong>{tr("Stats_MetricsProvidedBy")}</strong>
                  <button
                    type="button"
                    class="linkbtn attribution-link"
                    on:click={() => openAttributionLink(attribution?.link ?? "")}
                  >
                    {attribution.text}
                  </button>
                </p>
              {/if}
            </div>
            <p class="attribution-note">{tr("Stats_MetricsAffiliationNote")}</p>
          </div>
        {/if}

        <div class="modal-actions">
          <button
            type="button"
            class="btnicontext"
            disabled={busy}
            on:click={() => {
              screen = "list";
              editGame = "";
              attribution = null;
            }}
          >
            {tr("Button_Back")}
          </button>
          <button type="button" class="btnicontext" disabled={busy || $offlineMode} on:click={() => void doRefresh()}>
            {tr("Refresh")}
          </button>
          <button type="button" class="btnicontext modal-primary" disabled={busy} on:click={() => void saveVars()}>
            {tr("Submit")}
          </button>
        </div>
      </div>
    {/if}
  {/if}
</div>

<style lang="scss">
  .gamestats-modal {
    min-width: min(440px, 92vw);
    max-width: 640px;
  }
  .modal-lead {
    margin: 0 0 0.75rem;
    line-height: 1.4;
  }
  .gamestats-scroll {
    max-height: min(60vh, 480px);
    overflow: auto;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    --gamestats-row-indent: calc(1rem + 0.35rem);
  }
  .gamestats-sub {
    margin: 0.5rem 0 0.15rem;
    font-size: 0.8rem;
    opacity: 0.85;
  }
  .gamestats-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
  }
  .gamestats-check {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    cursor: pointer;
  }
  .gamestats-actions {
    display: flex;
    gap: 0.35rem;
    flex-shrink: 0;
  }
  .linkbtn {
    background: transparent;
    border: 0;
    color: var(--accent);
    cursor: pointer;
    padding: 0.15rem 0.35rem;
    font: inherit;
    &:disabled {
      opacity: 0.45;
      cursor: not-allowed;
    }
  }
  .field-row {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .field-label {
    font-size: 0.85rem;
    opacity: 0.9;
  }
  .modal-input {
    padding: 8px;
    background: var(--even-darker-code-background);
    border: 2px solid var(--button-bg);
    color: var(--whiteSecondary);
    font: inherit;
    width: 100%;
    box-sizing: border-box;
  }
  .metric-row {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.85rem;
  }
  .gamestats-attribution {
    margin-top: 0.15rem;
  }
  .attribution-inset {
    padding-left: var(--gamestats-row-indent);
  }
  .attribution-text {
    margin: 0;
    font-size: 0.85rem;
    line-height: 1.4;
  }
  .attribution-link {
    font-weight: normal;
    padding: 0;
  }
  .attribution-note {
    margin: 0.35rem 0 0;
    font-size: 0.85rem;
    line-height: 1.4;
    opacity: 0.85;
  }
  .attribution-image-btn {
    display: inline-block;
    padding: 0;
    margin: 0;
    border: 0;
    background: transparent;
    cursor: pointer;
    line-height: 0;
    img {
      max-height: 2.25rem;
      width: auto;
      display: block;
      &.has-dimensions {
        max-height: none;
      }
    }
    &:disabled {
      opacity: 0.45;
      cursor: not-allowed;
    }
  }
  .modal-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
    justify-content: flex-end;
    margin-top: 0.75rem;
  }
  .modal-primary {
    font-weight: 600;
  }
  .muted {
    opacity: 0.7;
    font-size: 0.85rem;
  }
</style>
