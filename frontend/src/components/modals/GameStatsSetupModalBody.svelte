<script lang="ts">
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
    try {
      const [req, exist, res, hid] = await Promise.all([
        BasicService.GetRequiredVarSpecs(game),
        BasicService.GetExistingVars(game, uniqueId),
        BasicService.GetResolvedGameStatVars(platformKey, game, uniqueId),
        BasicService.GetHiddenMetrics(game, uniqueId),
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

        <div class="modal-actions">
          <button
            type="button"
            class="btnicontext"
            disabled={busy}
            on:click={() => {
              screen = "list";
              editGame = "";
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
    border: 1px solid var(--button-bg);
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
