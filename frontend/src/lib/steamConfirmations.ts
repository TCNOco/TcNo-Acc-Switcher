import { get, writable, type Readable } from "svelte/store";

export type ConfirmationDecision = "accept" | "deny";
export type ConfirmationStatus =
  | "loading"
  | "fresh"
  | "empty"
  | "stale-error"
  | "reauth"
  | "rate-limit"
  | "offline"
  | "canceled";

export type ConfirmationTextField = {
  label: string;
  value: string;
};

/** One item in a trade. The triple identifies it for a full description lookup. */
export type ConfirmationTradeItem = {
  appId: string;
  classId: string;
  instanceId: string;
  icon: string;
};

export type ConfirmationTradeSide = {
  header: string;
  items: ConfirmationTradeItem[];
};

/** The other side of a trade: who is receiving is the point of confirming one. */
export type ConfirmationTradeParty = {
  name: string;
  avatar: string;
  profileUrl: string;
  level: number;
  yearsBadge: string;
};

export type ConfirmationTrade = {
  partner: ConfirmationTradeParty | null;
  give: ConfirmationTradeSide;
  receive: ConfirmationTradeSide;
};

/** Steam's full description of one item, as its community site shows on hover. */
export type ConfirmationItem = {
  name: string;
  marketHashName: string;
  type: string;
  nameColor: string;
  icon: string;
  tradable: boolean;
  marketable: boolean;
  descriptions: { value: string; color: string }[];
  tags: { category: string; name: string; color: string }[];
};

/**
 * How the market for a listed item looks, as numbers the page words itself. A
 * zero count means Steam did not state one. `text` holds any line the backend
 * could not read as numbers, in Steam's own words — which is every line on an
 * account whose Steam language is not English.
 */
export type ConfirmationListingMarket = {
  forSale: number;
  forSalePrice: string;
  soldRecently: number;
  text: string[];
};

/**
 * A market listing with the boilerplate dropped. Steam's page is mostly advice
 * about how to post a listing and what to do if you did not create it; these are
 * the numbers the answer actually turns on.
 *
 * The amounts arrive verbatim but unlabelled, so the page names them from its
 * own translations. `prices` carries Steam's own labels only when the page could
 * not be read that way.
 */
export type ConfirmationListing = {
  receive: string;
  buyerPays: string;
  prices: ConfirmationTextField[];
  market: ConfirmationListingMarket;
  item: ConfirmationItem | null;
};

export type ConfirmationDetail = {
  fields: ConfirmationTextField[];
  trade: ConfirmationTrade | null;
  listing: ConfirmationListing | null;
};

/**
 * How the window lays a confirmation out. A trade moves items between two people
 * and takes the whole window; anything else is a few lines of text and shows in a
 * bar at the foot of the list. Unknown values from an older backend read as
 * "other", which is the modest layout.
 */
export type ConfirmationKind = "trade" | "market" | "other";

export type ConfirmationRow = {
  handle: string;
  kind?: ConfirmationKind;
  typeLabel: string;
  title: string;
  summary: string;
  details: ConfirmationTextField[];
  acceptLabel: string;
  denyLabel: string;
  /** App-relative path to the locally cached icon; "" when none is served. */
  icon?: string;
  /** Set once the confirmation is opened and its detail page is read. */
  trade?: ConfirmationTrade | null;
  listing?: ConfirmationListing | null;
};

export type ConfirmationFetchResult = {
  accountLabel: string;
  rows: ConfirmationRow[];
  fetchedAt: number;
};

export type ConfirmationFailureKind = "error" | "reauth" | "rate-limit" | "offline" | "canceled";

export class ConfirmationRequestError extends Error {
  constructor(
    public readonly kind: ConfirmationFailureKind,
    message: string,
    public readonly retryAfterMs = 0,
    /** Carried so a failure before any success can still name its account. */
    public readonly accountLabel = "",
  ) {
    super(message);
    this.name = "ConfirmationRequestError";
  }
}

/** What a silent session renewal achieved; see SteamConfirmationsAdapter.refreshSession. */
export type ConfirmationSessionRefresh = {
  refreshed: boolean;
  needsCredentials: boolean;
};

export interface SteamConfirmationsAdapter {
  fetch(generation: number, signal: AbortSignal): Promise<ConfirmationFetchResult>;
  decide(handle: string, decision: ConfirmationDecision): Promise<void | boolean>;
  /**
   * Decides several confirmations as one Steam request. Preferred for a batch:
   * single decisions signed within the same second carry identical signatures,
   * and Steam rejects the repeats as replays.
   */
  decideMany?(handles: string[], decision: ConfirmationDecision): Promise<void | boolean>;
  loginAgain(): Promise<void>;
  /**
   * Renews the stored session without user input, and repairs this window's
   * capability afterwards. Absent when the window has no working capability to
   * renew with, in which case only loginAgain is available.
   */
  refreshSession?(): Promise<ConfirmationSessionRefresh>;
  /** SteamID64 of the account this window serves, known before any fetch. */
  readonly accountId?: string;
  /**
   * Loads Steam's detail page for one confirmation. The list carries only a
   * headline, a few summary lines and one icon, so this is the only richer source.
   */
  inspect?(handle: string): Promise<ConfirmationDetail>;
  /** Describes one item of an open trade, so what is moving can be checked. */
  inspectItem?(appId: string, classId: string, instanceId: string): Promise<ConfirmationItem | null>;
}

export type SteamConfirmationsState = {
  status: ConfirmationStatus;
  /** The confirmation whose detail is being fetched, so the wait is visible. */
  detailLoadingHandle: string | null;
  accountId: string;
  accountLabel: string;
  rows: ConfirmationRow[];
  selectedHandle: string | null;
  /** Rows marked for a batch decision, by handle. Cleared as each one is decided. */
  checked: Record<string, boolean>;
  fetchedAt: number | null;
  refreshing: boolean;
  activeGeneration: number;
  pendingHandle: string | null;
  rowErrors: Record<string, string>;
  retryAt: number | null;
  message: string;
};

type TimerHandle = ReturnType<typeof setTimeout>;
type ConfirmationScheduler = {
  now(): number;
  set(delayMs: number, callback: () => void): TimerHandle;
  clear(handle: TimerHandle): void;
};

export interface SteamConfirmationsStore extends Readable<SteamConfirmationsState> {
  setAdapter(adapter: SteamConfirmationsAdapter | null): void;
  setVisible(visible: boolean): void;
  refresh(): Promise<void>;
  select(handle: string): void;
  /** Closes the open confirmation without deciding it. */
  deselect(): void;
  decide(handle: string, decision: ConfirmationDecision): Promise<void>;
  /** Marks or unmarks one row for a batch decision. */
  toggleChecked(handle: string): void;
  checkAll(): void;
  clearChecked(): void;
  /** Decides every checked row: one batched Steam call when the adapter offers it, otherwise one each in list order. */
  decideChecked(decision: ConfirmationDecision): Promise<void>;
  loginAgain(): Promise<void>;
  /** Full description of one traded item, or null when Steam will not describe it. */
  describeItem(item: ConfirmationTradeItem): Promise<ConfirmationItem | null>;
  cancel(): void;
  destroy(): void;
}

const POLL_INTERVAL_MS = 10_000;
const MAX_BACKOFF_MS = 120_000;

const defaultScheduler: ConfirmationScheduler = {
  now: () => Date.now(),
  set: (delayMs, callback) => setTimeout(callback, delayMs),
  clear: (handle) => clearTimeout(handle),
};

function initialState(): SteamConfirmationsState {
  return {
    status: "loading",
    detailLoadingHandle: null,
    accountId: "",
    accountLabel: "",
    rows: [],
    selectedHandle: null,
    checked: {},
    fetchedAt: null,
    refreshing: false,
    activeGeneration: 0,
    pendingHandle: null,
    rowErrors: {},
    retryAt: null,
    message: "",
  };
}

function validateRows(rows: ConfirmationRow[]): ConfirmationRow[] {
  if (!Array.isArray(rows)) {
    throw new ConfirmationRequestError("error", "Invalid confirmation response");
  }
  const handles = new Set<string>();
  return rows.map((row) => {
    if (!row || typeof row !== "object"
      || typeof row.handle !== "string" || !row.handle || row.handle.length > 512
      || handles.has(row.handle) || !Array.isArray(row.details)) {
      throw new ConfirmationRequestError("error", "Invalid confirmation response");
    }
    handles.add(row.handle);
    return {
      ...row,
      details: row.details.map((field) => ({ label: String(field.label), value: String(field.value) })),
    };
  });
}

function decisionError(error: unknown): ConfirmationRequestError {
  if (error instanceof ConfirmationRequestError) return error;
  if (error instanceof DOMException && error.name === "AbortError") {
    return new ConfirmationRequestError("canceled", "Confirmation action was canceled. Try again.");
  }
  return new ConfirmationRequestError("error", "Confirmation action failed. Try again.");
}

function requestError(error: unknown): ConfirmationRequestError {
  if (error instanceof ConfirmationRequestError) return error;
  if (error instanceof DOMException && error.name === "AbortError") {
    return new ConfirmationRequestError("canceled", "Refresh canceled");
  }
  return new ConfirmationRequestError("error", "Refresh failed");
}

function statusForFailure(kind: ConfirmationFailureKind): ConfirmationStatus {
  return kind === "error" ? "stale-error" : kind;
}

/**
 * The i18n key explaining a non-fresh status, or "" when there is nothing to
 * say. Kept out of the page so the reauth banner's visibility is testable.
 */
export function confirmationStatusMessageKey(status: ConfirmationStatus): string {
  switch (status) {
    case "stale-error": return "SteamGuard_Confirmations_Stale";
    case "reauth": return "SteamGuard_Confirmations_Reauth";
    case "rate-limit": return "SteamGuard_Confirmations_RateLimit";
    case "offline": return "SteamGuard_Confirmations_Offline";
    case "canceled": return "SteamGuard_Confirmations_Canceled";
    default: return "";
  }
}

/**
 * Whether a confirmation is reviewed in the full-window overlay. Only a trade
 * fills one: a listing shown that way is a line of text in an empty screen.
 * The trade fallback covers a row whose kind a backend did not send.
 */
export function confirmationUsesOverlay(row: ConfirmationRow): boolean {
  return row.kind === "trade" || Boolean(row.trade);
}

export function confirmationActionsEnabled(state: SteamConfirmationsState, handle: string): boolean {
  return state.status === "fresh"
    && state.pendingHandle === null
    && state.rows.some((row) => row.handle === handle);
}

export function createSteamConfirmationsStore(
  initialAdapter: SteamConfirmationsAdapter | null = null,
  scheduler: ConfirmationScheduler = defaultScheduler,
): SteamConfirmationsStore {
  const state = writable<SteamConfirmationsState>(initialState());
  let adapter = initialAdapter;
  let visible = false;
  let destroyed = false;
  let generation = 0;
  let failureCount = 0;
  /** One silent renewal per stretch of failures, so a renewal that does not help cannot spin. */
  let silentRefreshAttempted = false;
  let timer: TimerHandle | null = null;
  let controller: AbortController | null = null;
  let inFlight: Promise<void> | null = null;
  let refreshAfterFlight = false;

  function clearTimer(): void {
    if (timer !== null) scheduler.clear(timer);
    timer = null;
  }

  function schedule(delayMs: number): void {
    clearTimer();
    if (!visible || destroyed || !adapter) return;
    timer = scheduler.set(Math.max(0, delayMs), () => {
      timer = null;
      void refresh();
    });
  }

  function nextBackoff(): number {
    return Math.min(POLL_INTERVAL_MS * (2 ** Math.max(0, failureCount - 1)), MAX_BACKOFF_MS);
  }

  async function runRefresh(activeAdapter: SteamConfirmationsAdapter, activeGeneration: number, signal: AbortSignal): Promise<void> {
    try {
      const result = await activeAdapter.fetch(activeGeneration, signal);
      if (destroyed || activeGeneration !== generation) return;
      const previous = get(state);
      // A refresh returns the list, not the per-confirmation detail. Carrying the
      // detail across keeps an open trade on screen: without this the poll
      // replaced the open row with a bare one and emptied the overlay.
      const priorByHandle = new Map(previous.rows.map((row) => [row.handle, row]));
      const rows = validateRows(result.rows).map((row) => {
        const prior = priorByHandle.get(row.handle);
        if (!prior) return row;
        return {
          ...row,
          details: row.details.length > 0 ? row.details : prior.details,
          trade: row.trade ?? prior.trade,
          listing: row.listing ?? prior.listing,
        };
      });
      const rowErrors = Object.fromEntries(
        Object.entries(previous.rowErrors).filter(([handle]) => rows.some((row) => row.handle === handle)),
      );
      // Nothing is selected until the user picks a row: the action buttons decide
      // a real trade or listing, so they must not appear against a row that was
      // chosen for the user.
      const selectedHandle = previous.selectedHandle && rows.some((row) => row.handle === previous.selectedHandle)
        ? previous.selectedHandle
        : null;
      // A mark only means anything against the row it was put on; a row Steam no
      // longer lists takes its mark with it.
      const checked = Object.fromEntries(
        Object.entries(previous.checked).filter(([handle]) => rows.some((row) => row.handle === handle)),
      );
      failureCount = 0;
      silentRefreshAttempted = false;
      state.set({
        status: rows.length === 0 ? "empty" : "fresh",
        detailLoadingHandle: previous.detailLoadingHandle,
        accountId: previous.accountId,
        accountLabel: result.accountLabel,
        rows,
        selectedHandle,
        checked,
        fetchedAt: result.fetchedAt,
        refreshing: false,
        activeGeneration,
        pendingHandle: null,
        rowErrors,
        retryAt: null,
        message: "",
      });
      schedule(POLL_INTERVAL_MS);
    } catch (error) {
      if (destroyed || activeGeneration !== generation) return;
      const failure = requestError(error);
      // Steam usually accepts the stored refresh token, so try that before asking
      // the user to sign in for something that needs no password. Once per failed
      // run: if the renewal does not fix the fetch, the user has to be told.
      if (failure.kind === "reauth" && !silentRefreshAttempted && activeAdapter?.refreshSession) {
        silentRefreshAttempted = true;
        try {
          const outcome = await activeAdapter.refreshSession();
          if (destroyed || activeGeneration !== generation) return;
          if (outcome.refreshed) {
            // This runs inside the current flight, and refresh() returns the
            // in-flight promise rather than starting a new one — so ask for the
            // retry the way the store already defers one.
            refreshAfterFlight = true;
            return;
          }
        } catch (refreshError) {
          console.error("Steam confirmations: session could not be renewed", refreshError);
          if (destroyed || activeGeneration !== generation) return;
        }
      }
      const previous = get(state);
      failureCount += 1;
      const delay = failure.kind === "rate-limit"
        ? Math.max(failure.retryAfterMs, nextBackoff())
        : nextBackoff();
      state.set({
        ...previous,
        status: statusForFailure(failure.kind),
        // A label already learned wins; the failure only supplies one when no
        // refresh has ever succeeded.
        accountLabel: previous.accountLabel || failure.accountLabel,
        refreshing: false,
        activeGeneration,
        pendingHandle: null,
        retryAt: failure.kind === "rate-limit" ? scheduler.now() + delay : null,
        message: failure.message,
      });
      if (failure.kind !== "reauth" && failure.kind !== "canceled") schedule(delay);
    }
  }

  /**
   * Fills in one row's detail fields. Best effort and never surfaced as a failure:
   * the confirmation is still decidable on what the list already showed.
   */
  async function loadDetails(handle: string): Promise<void> {
    const activeAdapter = adapter;
    if (!activeAdapter?.inspect) return;
    state.update((current) => ({ ...current, detailLoadingHandle: handle }));
    try {
      const detail = await activeAdapter.inspect(handle);
      // Staleness is about the confirmation, not the list: a poll finishing while
      // this was in flight used to throw the detail away and leave the open trade
      // with no items.
      if (destroyed || adapter !== activeAdapter) return;
      if (!get(state).rows.some((row) => row.handle === handle)) return;
      state.update((current) => ({
        ...current,
        rows: current.rows.map((row) => row.handle === handle
          ? { ...row, details: detail.fields, trade: detail.trade, listing: detail.listing }
          : row),
      }));
    } catch (error) {
      console.error("Steam confirmations: detail could not be loaded", error);
    } finally {
      state.update((current) => current.detailLoadingHandle === handle
        ? { ...current, detailLoadingHandle: null }
        : current);
    }
  }

  function refresh(): Promise<void> {
    if (destroyed || !adapter) {
      state.update((current) => ({
        ...current,
        status: "canceled",
        refreshing: false,
        message: "Confirmation window is not connected",
      }));
      return Promise.resolve();
    }
    if (inFlight) return inFlight;
    if (get(state).pendingHandle !== null) return Promise.resolve();
    clearTimer();
    generation += 1;
    const requestController = new AbortController();
    controller = requestController;
    const activeAdapter = adapter;
    const activeGeneration = generation;
    state.update((current) => ({
      ...current,
      status: current.rows.length === 0 ? "loading" : current.status,
      refreshing: true,
      activeGeneration,
      message: "",
    }));
    const task = runRefresh(activeAdapter, activeGeneration, requestController.signal);
    inFlight = task;
    const cleanup = () => {
      if (inFlight === task) inFlight = null;
      if (controller === requestController) controller = null;
      if (refreshAfterFlight && !destroyed) {
        refreshAfterFlight = false;
        void refresh();
      }
    };
    void task.then(cleanup, cleanup);
    return task;
  }

  async function decide(handle: string, decision: ConfirmationDecision): Promise<void> {
    const current = get(state);
    if (!adapter || !confirmationActionsEnabled(current, handle)) return;
    const activeAdapter = adapter;
    const removedIndex = current.rows.findIndex((row) => row.handle === handle);
    const removedRow = current.rows[removedIndex];
    if (!removedRow) return;
    clearTimer();
    generation += 1;
    controller?.abort();
    const remainingRows = current.rows.filter((row) => row.handle !== handle);
    const remainingErrors = { ...current.rowErrors };
    delete remainingErrors[handle];
    const remainingChecked = { ...current.checked };
    delete remainingChecked[handle];
    // Deciding closes the confirmation rather than opening its neighbour: the
    // detail is a full-window overlay, so stepping to the next one puts an
    // undecided trade in front of the user as the result of deciding this one.
    const nextSelectedHandle = current.selectedHandle === handle ? null : current.selectedHandle;
    state.set({
      ...current,
      status: remainingRows.length === 0 ? "empty" : "fresh",
      rows: remainingRows,
      selectedHandle: nextSelectedHandle,
      checked: remainingChecked,
      pendingHandle: handle,
      rowErrors: remainingErrors,
      refreshing: false,
      activeGeneration: generation,
      message: "",
    });
    try {
      const accepted = await activeAdapter.decide(handle, decision);
      if (accepted === false) {
        throw new ConfirmationRequestError("error", "Steam did not confirm the action. Try again.");
      }
      if (destroyed || adapter !== activeAdapter) return;
      state.update((value) => ({ ...value, pendingHandle: null }));
      if (inFlight) await inFlight;
      if (destroyed || adapter !== activeAdapter) return;
      await refresh();
    } catch (error) {
      if (destroyed || adapter !== activeAdapter) return;
      const failure = decisionError(error);
      failureCount += 1;
      const delay = failure.kind === "rate-limit"
        ? Math.max(failure.retryAfterMs, nextBackoff())
        : nextBackoff();
      state.update((value) => ({
        ...value,
        rows: value.rows.some((row) => row.handle === handle)
          ? value.rows
          : [...value.rows.slice(0, Math.min(removedIndex, value.rows.length)), removedRow, ...value.rows.slice(Math.min(removedIndex, value.rows.length))],
        selectedHandle: value.selectedHandle ?? handle,
        status: failure.kind === "error" || failure.kind === "canceled"
          ? "fresh"
          : statusForFailure(failure.kind),
        pendingHandle: null,
        rowErrors: { ...value.rowErrors, [handle]: failure.message },
        retryAt: failure.kind === "rate-limit" ? scheduler.now() + delay : null,
        message: failure.message,
      }));
      if (failure.kind !== "reauth" && failure.kind !== "canceled") {
        schedule(delay);
      }
    }
  }

  /**
   * Decides every checked row — exactly the marked set, nothing else. One
   * Steam request for the whole batch when the adapter offers it: single
   * decisions signed within the same second carry identical signatures, and
   * Steam rejects the repeats as replays. Without that, one call per row in
   * list order; a row that plainly fails keeps its mark and shows its error
   * while the rest continue, and a failure that would hit every remaining row
   * the same way (reauth, rate limit, offline, cancel) stops the batch.
   */
  async function decideChecked(decision: ConfirmationDecision): Promise<void> {
    const current = get(state);
    if (!adapter || current.status !== "fresh" || current.pendingHandle !== null) return;
    const handles = current.rows.filter((row) => current.checked[row.handle]).map((row) => row.handle);
    if (handles.length === 0) return;
    const activeAdapter = adapter;
    clearTimer();
    generation += 1;
    controller?.abort();
    let stopFailure: ConfirmationRequestError | null = null;
    if (activeAdapter.decideMany && handles.length > 1) {
      state.update((value) => ({
        ...value,
        pendingHandle: handles[0],
        refreshing: false,
        activeGeneration: generation,
        message: "",
      }));
      try {
        const accepted = await activeAdapter.decideMany(handles, decision);
        if (accepted === false) {
          throw new ConfirmationRequestError("error", "Steam did not confirm the action. Try again.");
        }
        if (destroyed || adapter !== activeAdapter) return;
        state.update((value) => {
          const gone = new Set(handles);
          return {
            ...value,
            rows: value.rows.filter((row) => !gone.has(row.handle)),
            checked: Object.fromEntries(Object.entries(value.checked).filter(([handle]) => !gone.has(handle))),
            rowErrors: Object.fromEntries(Object.entries(value.rowErrors).filter(([handle]) => !gone.has(handle))),
            selectedHandle: value.selectedHandle !== null && gone.has(value.selectedHandle) ? null : value.selectedHandle,
            pendingHandle: null,
          };
        });
        await refresh();
      } catch (error) {
        if (destroyed || adapter !== activeAdapter) return;
        const failure = decisionError(error);
        failureCount += 1;
        // The batch is one request, so its failure belongs to every row in it:
        // none of them were decided.
        state.update((value) => ({
          ...value,
          status: failure.kind === "error" || failure.kind === "canceled" ? value.status : statusForFailure(failure.kind),
          pendingHandle: null,
          rowErrors: {
            ...value.rowErrors,
            ...Object.fromEntries(handles.map((handle) => [handle, failure.message])),
          },
          message: failure.message,
        }));
        const delay = failure.kind === "rate-limit"
          ? Math.max(failure.retryAfterMs, nextBackoff())
          : nextBackoff();
        if (failure.kind === "rate-limit") {
          state.update((value) => ({ ...value, retryAt: scheduler.now() + delay }));
        }
        if (failure.kind !== "reauth" && failure.kind !== "canceled") schedule(delay);
      }
      return;
    }
    for (const handle of handles) {
      state.update((value) => ({
        ...value,
        pendingHandle: handle,
        refreshing: false,
        activeGeneration: generation,
        message: "",
      }));
      try {
        const accepted = await activeAdapter.decide(handle, decision);
        if (accepted === false) {
          throw new ConfirmationRequestError("error", "Steam did not confirm the action. Try again.");
        }
        if (destroyed || adapter !== activeAdapter) return;
        state.update((value) => {
          const rows = value.rows.filter((row) => row.handle !== handle);
          const checked = { ...value.checked };
          delete checked[handle];
          const rowErrors = { ...value.rowErrors };
          delete rowErrors[handle];
          return {
            ...value,
            rows,
            checked,
            rowErrors,
            selectedHandle: value.selectedHandle === handle ? null : value.selectedHandle,
          };
        });
      } catch (error) {
        if (destroyed || adapter !== activeAdapter) return;
        const failure = decisionError(error);
        failureCount += 1;
        state.update((value) => ({
          ...value,
          status: failure.kind === "error" || failure.kind === "canceled" ? value.status : statusForFailure(failure.kind),
          rowErrors: { ...value.rowErrors, [handle]: failure.message },
          message: failure.message,
        }));
        if (failure.kind !== "error") {
          stopFailure = failure;
          break;
        }
      }
    }
    state.update((value) => ({ ...value, pendingHandle: null }));
    if (destroyed || adapter !== activeAdapter) return;
    if (stopFailure) {
      const delay = stopFailure.kind === "rate-limit"
        ? Math.max(stopFailure.retryAfterMs, nextBackoff())
        : nextBackoff();
      if (stopFailure.kind === "rate-limit") {
        state.update((value) => ({ ...value, retryAt: scheduler.now() + delay }));
      }
      if (stopFailure.kind !== "reauth" && stopFailure.kind !== "canceled") schedule(delay);
      return;
    }
    await refresh();
  }

  async function loginAgain(): Promise<void> {
    if (!adapter || destroyed) return;
    state.update((current) => ({ ...current, refreshing: true, message: "" }));
    try {
      await adapter.loginAgain();
      // Confirmations window auto-closes when the main window takes focus for login.
      // If login completes, the session should be refreshed on the next poll.
    } catch (error) {
      const failure = requestError(error);
      state.update((current) => ({
        ...current,
        status: statusForFailure(failure.kind),
        refreshing: false,
        message: failure.message,
      }));
    }
  }

  function cancel(): void {
    clearTimer();
    generation += 1;
    controller?.abort();
    state.update((current) => ({
      ...current,
      status: "canceled",
      refreshing: false,
      activeGeneration: generation,
      pendingHandle: null,
      retryAt: null,
      message: "Refresh canceled",
    }));
  }

  return {
    subscribe: state.subscribe,
    setAdapter(nextAdapter) {
      cancel();
      adapter = nextAdapter;
      failureCount = 0;
      silentRefreshAttempted = false;
      state.set({ ...initialState(), accountId: nextAdapter?.accountId ?? "" });
      if (visible) {
        if (inFlight) refreshAfterFlight = true;
        else void refresh();
      }
    },
    setVisible(nextVisible) {
      visible = nextVisible;
      clearTimer();
      if (visible) void refresh();
    },
    refresh,
    select(handle) {
      let selected = false;
      state.update((current) => {
        selected = current.rows.some((row) => row.handle === handle);
        return selected ? { ...current, selectedHandle: handle } : current;
      });
      // Steam serves the richer per-confirmation detail on its own page, so it is
      // loaded when a row is opened rather than for every row of every refresh.
      if (selected) void loadDetails(handle);
    },
    async describeItem(item) {
      const activeAdapter = adapter;
      if (!activeAdapter?.inspectItem) return null;
      try {
        return await activeAdapter.inspectItem(item.appId, item.classId, item.instanceId);
      } catch (error) {
        console.error("Steam confirmations: item could not be described", error);
        return null;
      }
    },
    deselect() {
      state.update((current) => current.selectedHandle === null
        ? current
        : { ...current, selectedHandle: null });
    },
    decide,
    // All three ignore edits while a decision is in flight: the batch acts on
    // the set it captured, so a mark changed meanwhile would be silently
    // ignored and leave the bar disagreeing with what actually happened.
    toggleChecked(handle) {
      state.update((current) => {
        if (current.pendingHandle !== null || !current.rows.some((row) => row.handle === handle)) return current;
        const checked = { ...current.checked };
        if (checked[handle]) delete checked[handle];
        else checked[handle] = true;
        return { ...current, checked };
      });
    },
    checkAll() {
      state.update((current) => current.pendingHandle !== null ? current : {
        ...current,
        checked: Object.fromEntries(current.rows.map((row) => [row.handle, true])),
      });
    },
    clearChecked() {
      state.update((current) => current.pendingHandle !== null ? current : { ...current, checked: {} });
    },
    decideChecked,
    loginAgain,
    cancel,
    destroy() {
      destroyed = true;
      cancel();
      adapter = null;
    },
  };
}

// One route-local instance is reused by the singleton confirmations window.
export const steamConfirmationsWindow = createSteamConfirmationsStore();

export function configureSteamConfirmationsWindow(adapter: SteamConfirmationsAdapter | null): void {
  steamConfirmationsWindow.setAdapter(adapter);
}
