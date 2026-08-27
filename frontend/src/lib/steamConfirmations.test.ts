import { get } from "svelte/store";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  ConfirmationRequestError,
  confirmationActionsEnabled,
  confirmationStatusMessageKey,
  confirmationUsesOverlay,
  createSteamConfirmationsStore,
  type ConfirmationFetchResult,
  type ConfirmationListing,
  type ConfirmationRow,
  type SteamConfirmationsAdapter,
} from "./steamConfirmations";

const row = {
  handle: "opaque:one",
  typeLabel: "Trade",
  title: "Trade with Example",
  summary: "Two items",
  details: [{ label: "Partner", value: "Example" }],
  acceptLabel: "Accept",
  denyLabel: "Decline",
};

const secondRow = {
  ...row,
  handle: "opaque:two",
};

const fresh = (rows: ConfirmationRow[] = [row]): ConfirmationFetchResult =>
  ({ accountLabel: "Account", rows, fetchedAt: 1000 });

function adapter(fetch: SteamConfirmationsAdapter["fetch"]): SteamConfirmationsAdapter {
  return { fetch, decide: vi.fn().mockResolvedValue(undefined), loginAgain: vi.fn().mockResolvedValue(undefined) };
}

async function settle(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
}

describe("Steam confirmations store", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("loads immediately when visible and polls every ten seconds without overlap", async () => {
    let release: ((value: ConfirmationFetchResult) => void) | undefined;
    const fetch = vi.fn(() => new Promise<ConfirmationFetchResult>((resolve) => { release = resolve; }));
    const store = createSteamConfirmationsStore(adapter(fetch));

    store.setVisible(true);
    void store.refresh();
    expect(fetch).toHaveBeenCalledTimes(1);
    release?.(fresh());
    await settle();
    expect(get(store).status).toBe("fresh");

    await vi.advanceTimersByTimeAsync(9_999);
    expect(fetch).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(fetch).toHaveBeenCalledTimes(2);
    store.destroy();
  });

  it("represents an empty successful response", async () => {
    const store = createSteamConfirmationsStore(adapter(vi.fn().mockResolvedValue(fresh([]))));
    await store.refresh();
    expect(get(store)).toMatchObject({ status: "empty", rows: [], selectedHandle: null });
    store.destroy();
  });

  it.each([
    ["reauth", "reauth"],
    ["rate-limit", "rate-limit"],
    ["offline", "offline"],
    ["canceled", "canceled"],
  ] as const)("maps %s failures to a typed state", async (kind, status) => {
    const store = createSteamConfirmationsStore(adapter(vi.fn().mockRejectedValue(new ConfirmationRequestError(kind, kind, 30_000))));
    await store.refresh();
    expect(get(store).status).toBe(status);
    store.destroy();
  });

  // Steam usually accepts the stored refresh token, so the user should not be asked
  // to sign in for something that needs no password.
  it("renews the session itself on reauth and carries on when that works", async () => {
    const fetch = vi.fn()
      .mockRejectedValueOnce(new ConfirmationRequestError("reauth", "session expired"))
      .mockResolvedValue(fresh());
    const refreshSession = vi.fn().mockResolvedValue({ refreshed: true, needsCredentials: false });
    const store = createSteamConfirmationsStore({ ...adapter(fetch), refreshSession });

    store.setVisible(true);
    await settle();
    await settle();

    expect(refreshSession).toHaveBeenCalledTimes(1);
    expect(get(store).status).toBe("fresh");
    store.destroy();
  });

  // A renewal that does not fix the fetch must not be retried in a loop; the user
  // has to be told instead.
  it("asks the user to sign in once a renewal does not help, and only renews once", async () => {
    const fetch = vi.fn().mockRejectedValue(new ConfirmationRequestError("reauth", "session expired"));
    const refreshSession = vi.fn().mockResolvedValue({ refreshed: true, needsCredentials: false });
    const store = createSteamConfirmationsStore({ ...adapter(fetch), refreshSession });

    store.setVisible(true);
    await settle();
    await settle();
    await vi.advanceTimersByTimeAsync(60_000);

    expect(refreshSession).toHaveBeenCalledTimes(1);
    expect(get(store).status).toBe("reauth");
    store.destroy();
  });

  it("goes straight to sign-in when the session cannot be renewed", async () => {
    const fetch = vi.fn().mockRejectedValue(new ConfirmationRequestError("reauth", "session expired"));
    const refreshSession = vi.fn().mockResolvedValue({ refreshed: false, needsCredentials: true });
    const store = createSteamConfirmationsStore({ ...adapter(fetch), refreshSession });

    store.setVisible(true);
    await settle();
    await settle();

    expect(refreshSession).toHaveBeenCalledTimes(1);
    expect(get(store).status).toBe("reauth");
    store.destroy();
  });

  it("stops polling on reauth and offers an explained login-again banner", async () => {
    const fetch = vi.fn().mockRejectedValue(new ConfirmationRequestError("reauth", "session expired"));
    const store = createSteamConfirmationsStore(adapter(fetch));
    store.setVisible(true);
    await settle();

    const state = get(store);
    expect(state.status).toBe("reauth");
    expect(state.refreshing).toBe(false);
    expect(state.message).toBe("session expired");
    expect(confirmationStatusMessageKey(state.status)).toBe("SteamGuard_Confirmations_Reauth");

    await vi.advanceTimersByTimeAsync(60_000);
    expect(fetch).toHaveBeenCalledTimes(1);
    store.destroy();
  });

  it("keeps the reauth banner when the login-again handoff asks the user to finish elsewhere", async () => {
    const fetch = vi.fn().mockRejectedValue(new ConfirmationRequestError("reauth", "session expired"));
    const handoff = vi.fn().mockRejectedValue(new ConfirmationRequestError("reauth", "finish in the main window"));
    const store = createSteamConfirmationsStore({ ...adapter(fetch), loginAgain: handoff });
    await store.refresh();

    await store.loginAgain();
    expect(handoff).toHaveBeenCalledTimes(1);
    expect(get(store)).toMatchObject({ status: "reauth", refreshing: false, message: "finish in the main window" });
    store.destroy();
  });

  it("maps every status to the banner copy that carries its explanation", () => {
    expect(confirmationStatusMessageKey("loading")).toBe("");
    expect(confirmationStatusMessageKey("fresh")).toBe("");
    expect(confirmationStatusMessageKey("empty")).toBe("");
    expect(confirmationStatusMessageKey("stale-error")).toBe("SteamGuard_Confirmations_Stale");
    expect(confirmationStatusMessageKey("rate-limit")).toBe("SteamGuard_Confirmations_RateLimit");
    expect(confirmationStatusMessageKey("offline")).toBe("SteamGuard_Confirmations_Offline");
    expect(confirmationStatusMessageKey("canceled")).toBe("SteamGuard_Confirmations_Canceled");
  });

  it("reviews only a trade full-window, so a listing does not open an empty screen", () => {
    expect(confirmationUsesOverlay({ ...row, kind: "trade" })).toBe(true);
    expect(confirmationUsesOverlay({ ...row, kind: "market" })).toBe(false);
    expect(confirmationUsesOverlay({ ...row, kind: "other" })).toBe(false);
    // A backend that names no kind still gets the room once items turn up.
    expect(confirmationUsesOverlay(row)).toBe(false);
    expect(confirmationUsesOverlay({
      ...row,
      trade: { partner: null, give: { header: "You give", items: [] }, receive: { header: "You get", items: [] } },
    })).toBe(true);
  });

  it("retains rows after a failed refresh but disables stale actions", async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(fresh())
      .mockRejectedValueOnce(new Error("temporary"));
    const store = createSteamConfirmationsStore(adapter(fetch));
    await store.refresh();
    await store.refresh();
    const state = get(store);
    expect(state.status).toBe("stale-error");
    expect(state.rows).toHaveLength(1);
    expect(confirmationActionsEnabled(state, row.handle)).toBe(false);
    store.destroy();
  });

  it("uses bounded exponential backoff after polling failures", async () => {
    const fetch = vi.fn().mockRejectedValue(new Error("temporary"));
    const store = createSteamConfirmationsStore(adapter(fetch));
    store.setVisible(true);
    await settle();
    expect(fetch).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(fetch).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(19_999);
    expect(fetch).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(1);
    expect(fetch).toHaveBeenCalledTimes(3);
    store.destroy();
  });

  it("pauses scheduled polling while the window is hidden", async () => {
    const fetch = vi.fn().mockResolvedValue(fresh());
    const store = createSteamConfirmationsStore(adapter(fetch));
    store.setVisible(true);
    await settle();
    store.setVisible(false);
    await vi.advanceTimersByTimeAsync(30_000);
    expect(fetch).toHaveBeenCalledTimes(1);
    store.destroy();
  });

  it("honors a rate-limit retry delay while visible", async () => {
    const fetch = vi.fn()
      .mockRejectedValueOnce(new ConfirmationRequestError("rate-limit", "wait", 30_000))
      .mockResolvedValue(fresh());
    const store = createSteamConfirmationsStore(adapter(fetch));
    store.setVisible(true);
    await settle();
    expect(get(store).retryAt).not.toBeNull();
    await vi.advanceTimersByTimeAsync(29_999);
    expect(fetch).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(fetch).toHaveBeenCalledTimes(2);
    store.destroy();
  });

  it("passes only the stable opaque handle and rejects an accept-versus-deny race", async () => {
    let release: (() => void) | undefined;
    const service = adapter(vi.fn().mockResolvedValue(fresh()));
    const decide = vi.fn(() => new Promise<void>((resolve) => { release = resolve; }));
    service.decide = decide;
    const store = createSteamConfirmationsStore(service);
    await store.refresh();

    const first = store.decide(row.handle, "accept");
    const second = store.decide(row.handle, "deny");
    expect(decide).toHaveBeenCalledTimes(1);
    expect(decide).toHaveBeenCalledWith("opaque:one", "accept");
    release?.();
    await first;
    await second;
    store.destroy();
  });

  it("removes only the selected opaque handle before reconciling identical visible rows", async () => {
    let releaseRefresh: ((value: ConfirmationFetchResult) => void) | undefined;
    const fetch = vi.fn()
      .mockResolvedValueOnce(fresh([row, secondRow]))
      .mockImplementationOnce(() => new Promise<ConfirmationFetchResult>((resolve) => { releaseRefresh = resolve; }));
    const service = adapter(fetch);
    const store = createSteamConfirmationsStore(service);
    await store.refresh();

    const action = store.decide(row.handle, "accept");
    await settle();
    expect(get(store).rows.map(({ handle }) => handle)).toEqual([secondRow.handle]);
    expect(fetch).toHaveBeenCalledTimes(2);

    releaseRefresh?.(fresh([secondRow]));
    await action;
    expect(get(store).rows.map(({ handle }) => handle)).toEqual([secondRow.handle]);
    store.destroy();
  });

  it("restores a rejected row at its original position with an actionable row error", async () => {
    const service = adapter(vi.fn().mockResolvedValue(fresh([row, secondRow])));
    service.decide = vi.fn().mockResolvedValue(false);
    const store = createSteamConfirmationsStore(service);
    await store.refresh();

    await store.decide(row.handle, "deny");
    const state = get(store);
    expect(state.rows.map(({ handle }) => handle)).toEqual([row.handle, secondRow.handle]);
    expect(state.rowErrors[row.handle]).toContain("Try again");
    expect(confirmationActionsEnabled(state, row.handle)).toBe(true);
    store.destroy();
  });

  it.each([
    ["timeout", new Error("network timeout")],
    ["cancel", new DOMException("aborted", "AbortError")],
    ["typed cancel", new ConfirmationRequestError("canceled", "Canceled. Try again.")],
  ])("restores a row after %s", async (_label, failure) => {
    const service = adapter(vi.fn().mockResolvedValue(fresh()));
    service.decide = vi.fn().mockRejectedValue(failure);
    const store = createSteamConfirmationsStore(service);
    await store.refresh();

    await store.decide(row.handle, "accept");
    const state = get(store);
    expect(state.rows).toEqual([row]);
    expect(state.rowErrors[row.handle]).toContain("Try again");
    store.destroy();
  });

  it("treats a changed opaque handle as a new confirmation even when visible fields match", async () => {
    const changedHandle = { ...row, handle: "opaque:replacement" };
    const fetch = vi.fn()
      .mockResolvedValueOnce(fresh([row]))
      .mockResolvedValueOnce(fresh([changedHandle]));
    const store = createSteamConfirmationsStore(adapter(fetch));
    await store.refresh();

    await store.decide(row.handle, "accept");
    expect(get(store).rows).toEqual([changedHandle]);
    // The replacement does not inherit the selection, and nothing is chosen on the
    // user's behalf: the action buttons decide a real trade or listing.
    expect(get(store).selectedHandle).toBeNull();
    store.destroy();
  });

  // The poll returns the list, not the per-confirmation detail.
  it("keeps a loaded detail when the list refreshes", async () => {
    const trade = {
      partner: { name: "Partner", avatar: "", profileUrl: "", level: 21, yearsBadge: "" },
      give: { header: "You will give up 1 item", items: [] },
      receive: { header: "You will receive 2 items", items: [] },
    };
    const fetch = vi.fn().mockResolvedValue(fresh([row]));
    const inspect = vi.fn().mockResolvedValue({ fields: [{ label: "Partner", value: "Someone" }], trade });
    const store = createSteamConfirmationsStore({ ...adapter(fetch), inspect });

    await store.refresh();
    store.select(row.handle);
    await settle();
    expect(get(store).rows[0].trade).toEqual(trade);

    await store.refresh();
    expect(get(store).rows[0].trade).toEqual(trade);
    expect(get(store).rows[0].details).toHaveLength(1);
    store.destroy();
  });

  // A listing arrives instead of the text fields, not alongside them: the fields
  // Steam serves for one are the boilerplate the listing exists to replace.
  it("keeps a loaded listing when the list refreshes", async () => {
    // Amounts and counts, not Steam's sentences: the page words these itself, so
    // a listing reads in the user's language rather than the Steam account's.
    const listing: ConfirmationListing = {
      receive: "R 0.89",
      buyerPays: "R 1.23",
      prices: [],
      market: { forSale: 17, forSalePrice: "R 1.23", soldRecently: 3, text: [] },
      item: null,
    };
    const fetch = vi.fn().mockResolvedValue(fresh([{ ...row, kind: "market" as const }]));
    const inspect = vi.fn().mockResolvedValue({ fields: [], trade: null, listing });
    const store = createSteamConfirmationsStore({ ...adapter(fetch), inspect });

    await store.refresh();
    store.select(row.handle);
    await settle();
    expect(get(store).rows[0].listing).toEqual(listing);

    await store.refresh();
    expect(get(store).rows[0].listing).toEqual(listing);
    store.destroy();
  });

  // Opening the neighbour instead puts an undecided trade in front of the user.
  it("closes the open confirmation after deciding it", async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(fresh([row, secondRow]))
      .mockResolvedValue(fresh([secondRow]));
    const store = createSteamConfirmationsStore(adapter(fetch));
    await store.refresh();
    store.select(row.handle);
    expect(get(store).selectedHandle).toBe(row.handle);

    await store.decide(row.handle, "accept");
    expect(get(store).selectedHandle).toBeNull();
    store.destroy();
  });

  // Discarding it because a poll landed first leaves the open trade with no items.
  it("keeps a detail that arrives during a refresh", async () => {
    let release: ((value: { fields: never[]; trade: null }) => void) | undefined;
    const fetch = vi.fn().mockResolvedValue(fresh([row]));
    const inspect = vi.fn(() => new Promise((resolve) => { release = resolve as never; }));
    const store = createSteamConfirmationsStore({ ...adapter(fetch), inspect: inspect as never });

    await store.refresh();
    store.select(row.handle);
    await store.refresh();
    release?.({ fields: [{ label: "Partner", value: "Someone" }] as never, trade: null });
    await settle();

    // The loaded value, not the one the list row already carried.
    expect(get(store).rows[0].details[0].value).toBe("Someone");
    store.destroy();
  });

  // The detail is fetched on open, so the gap between the header and the buttons
  // has to read as a wait rather than as a confirmation with nothing in it.
  it("reports which confirmation is loading its detail", async () => {
    let release: ((value: { fields: never[]; trade: null }) => void) | undefined;
    const inspect = vi.fn(() => new Promise((resolve) => { release = resolve as never; }));
    const store = createSteamConfirmationsStore({
      ...adapter(vi.fn().mockResolvedValue(fresh([row]))),
      inspect: inspect as never,
    });

    await store.refresh();
    expect(get(store).detailLoadingHandle).toBeNull();

    store.select(row.handle);
    expect(get(store).detailLoadingHandle).toBe(row.handle);

    release?.({ fields: [] as never, trade: null });
    await settle();
    expect(get(store).detailLoadingHandle).toBeNull();
    store.destroy();
  });

  it("stops reporting a load that failed", async () => {
    const inspect = vi.fn().mockRejectedValue(new Error("no detail"));
    const store = createSteamConfirmationsStore({
      ...adapter(vi.fn().mockResolvedValue(fresh([row]))),
      inspect,
    });

    await store.refresh();
    store.select(row.handle);
    await settle();

    expect(get(store).detailLoadingHandle).toBeNull();
    store.destroy();
  });

  it("selects nothing until the user picks a row", async () => {
    const store = createSteamConfirmationsStore(adapter(vi.fn().mockResolvedValue(fresh([row, secondRow]))));
    await store.refresh();

    expect(get(store).selectedHandle).toBeNull();
    store.select(secondRow.handle);
    expect(get(store).selectedHandle).toBe(secondRow.handle);
    store.destroy();
  });

  it("keeps selection by opaque handle when a refresh reorders rows", async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(fresh([row, secondRow]))
      .mockResolvedValueOnce(fresh([secondRow, row]));
    const store = createSteamConfirmationsStore(adapter(fetch));
    await store.refresh();
    store.select(secondRow.handle);
    await store.refresh();

    expect(get(store).rows.map(({ handle }) => handle)).toEqual([secondRow.handle, row.handle]);
    expect(get(store).selectedHandle).toBe(secondRow.handle);
    store.destroy();
  });

  it.each([
    ["null row", [null]],
    ["numeric handle", [{ ...row, handle: 42 }]],
    ["missing details", [{ ...row, details: null }]],
  ])("rejects malformed rows atomically: %s", async (_label, malformed) => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(fresh())
      .mockResolvedValueOnce(fresh(malformed as unknown as ConfirmationRow[]));
    const store = createSteamConfirmationsStore(adapter(fetch));
    await store.refresh();
    await store.refresh();

    expect(get(store).status).toBe("stale-error");
    expect(get(store).rows).toEqual([row]);
    store.destroy();
  });

  it("ignores a late refresh while deciding and reconciles with a new generation", async () => {
    let releaseLate: ((value: ConfirmationFetchResult) => void) | undefined;
    const fetch = vi.fn()
      .mockResolvedValueOnce(fresh([row]))
      .mockImplementationOnce(() => new Promise<ConfirmationFetchResult>((resolve) => { releaseLate = resolve; }))
      .mockResolvedValueOnce(fresh([]));
    const store = createSteamConfirmationsStore(adapter(fetch));
    await store.refresh();
    const lateRefresh = store.refresh();

    const action = store.decide(row.handle, "accept");
    expect(get(store).rows).toEqual([]);
    releaseLate?.(fresh([row]));
    await lateRefresh;
    await action;

    expect(fetch).toHaveBeenCalledTimes(3);
    expect(get(store)).toMatchObject({ status: "empty", rows: [] });
    store.destroy();
  });

  it("does not restore a failed decision after the store closes", async () => {
    let rejectDecision: ((reason: unknown) => void) | undefined;
    const service = adapter(vi.fn().mockResolvedValue(fresh()));
    service.decide = vi.fn(() => new Promise<void>((_resolve, reject) => { rejectDecision = reject; }));
    const store = createSteamConfirmationsStore(service);
    await store.refresh();

    const action = store.decide(row.handle, "deny");
    store.destroy();
    rejectDecision?.(new Error("late failure"));
    await action;

    expect(get(store)).toMatchObject({ status: "canceled", rows: [] });
  });

  it("waits for an aborted generation before loading a replacement adapter", async () => {
    let release: ((value: ConfirmationFetchResult) => void) | undefined;
    const first = adapter(vi.fn(() => new Promise<ConfirmationFetchResult>((resolve) => { release = resolve; })));
    const secondFetch = vi.fn().mockResolvedValue(fresh());
    const store = createSteamConfirmationsStore(first);
    store.setVisible(true);
    store.setAdapter(adapter(secondFetch));
    expect(secondFetch).not.toHaveBeenCalled();
    release?.(fresh());
    await settle();
    expect(secondFetch).toHaveBeenCalledTimes(1);
    store.destroy();
  });

  it("rejects duplicate opaque handles as a stale response", async () => {
    const store = createSteamConfirmationsStore(adapter(vi.fn().mockResolvedValue(fresh([row, row]))));
    await store.refresh();
    expect(get(store).status).toBe("stale-error");
    expect(get(store).rows).toEqual([]);
    store.destroy();
  });

  it("decides exactly the checked rows, in list order", async () => {
    const thirdRow = { ...row, handle: "opaque:three" };
    const fetch = vi.fn().mockResolvedValue(fresh([row, secondRow, thirdRow]));
    const decide = vi.fn().mockResolvedValue(undefined);
    const store = createSteamConfirmationsStore({ ...adapter(fetch), decide });
    await store.refresh();

    store.toggleChecked(thirdRow.handle);
    store.toggleChecked(row.handle);
    store.toggleChecked(secondRow.handle);
    store.toggleChecked(secondRow.handle); // unmarked again: must not be decided
    await store.decideChecked("accept");

    expect(decide.mock.calls).toEqual([[row.handle, "accept"], [thirdRow.handle, "accept"]]);
    expect(get(store).checked).toEqual({});
    store.destroy();
  });

  it("carries a batch past one failed row and keeps that row marked", async () => {
    const fetch = vi.fn().mockResolvedValue(fresh([row, secondRow]));
    const decide = vi.fn()
      .mockRejectedValueOnce(new ConfirmationRequestError("error", "Steam said no"))
      .mockResolvedValue(undefined);
    const store = createSteamConfirmationsStore({ ...adapter(fetch), decide });
    await store.refresh();

    store.checkAll();
    await store.decideChecked("deny");

    expect(decide).toHaveBeenCalledTimes(2);
    const state = get(store);
    expect(state.rowErrors[row.handle]).toBe("Steam said no");
    expect(state.checked[row.handle]).toBe(true);
    expect(state.checked[secondRow.handle]).toBeUndefined();
    store.destroy();
  });

  // A session failure will hit every remaining row the same way, so the batch
  // stops there instead of burning through the rest.
  it("stops a batch on reauth and leaves the rest undecided", async () => {
    const fetch = vi.fn().mockResolvedValue(fresh([row, secondRow]));
    const decide = vi.fn().mockRejectedValue(new ConfirmationRequestError("reauth", "session expired"));
    const store = createSteamConfirmationsStore({ ...adapter(fetch), decide });
    await store.refresh();

    store.checkAll();
    await store.decideChecked("accept");

    expect(decide).toHaveBeenCalledTimes(1);
    const state = get(store);
    expect(state.status).toBe("reauth");
    expect(state.rows).toHaveLength(2);
    expect(state.checked[secondRow.handle]).toBe(true);
    store.destroy();
  });

  // Steam rejects back-to-back single decisions signed within the same second
  // as replays, so a batch must go out as one request when the adapter can.
  it("prefers one batched call for several checked rows", async () => {
    const fetch = vi.fn().mockResolvedValue(fresh([row, secondRow]));
    const decide = vi.fn().mockResolvedValue(undefined);
    const decideMany = vi.fn().mockResolvedValue(undefined);
    const store = createSteamConfirmationsStore({ ...adapter(fetch), decide, decideMany });
    await store.refresh();

    store.checkAll();
    await store.decideChecked("accept");

    expect(decideMany.mock.calls).toEqual([[[row.handle, secondRow.handle], "accept"]]);
    expect(decide).not.toHaveBeenCalled();
    expect(get(store).checked).toEqual({});
    store.destroy();
  });

  it("keeps every mark when the batched call fails, with the error on each row", async () => {
    const fetch = vi.fn().mockResolvedValue(fresh([row, secondRow]));
    const decideMany = vi.fn().mockRejectedValue(new ConfirmationRequestError("error", "Steam said no"));
    const store = createSteamConfirmationsStore({ ...adapter(fetch), decideMany });
    await store.refresh();

    store.checkAll();
    await store.decideChecked("deny");

    const state = get(store);
    expect(state.rows).toHaveLength(2);
    expect(state.checked).toEqual({ [row.handle]: true, [secondRow.handle]: true });
    expect(state.rowErrors[row.handle]).toBe("Steam said no");
    expect(state.rowErrors[secondRow.handle]).toBe("Steam said no");
    store.destroy();
  });

  // A single marked row keeps the plain path: the batch endpoint buys nothing
  // and the per-row error handling is richer.
  it("decides one checked row without the batch call", async () => {
    const fetch = vi.fn().mockResolvedValue(fresh([row, secondRow]));
    const decide = vi.fn().mockResolvedValue(undefined);
    const decideMany = vi.fn().mockResolvedValue(undefined);
    const store = createSteamConfirmationsStore({ ...adapter(fetch), decide, decideMany });
    await store.refresh();

    store.toggleChecked(row.handle);
    await store.decideChecked("accept");

    expect(decide.mock.calls).toEqual([[row.handle, "accept"]]);
    expect(decideMany).not.toHaveBeenCalled();
    store.destroy();
  });

  // A decision acts on the set it captured, so edits made while it is in
  // flight would be silently ignored — better to not accept them at all.
  it("ignores mark edits while a decision is in flight", async () => {
    let release: (() => void) | undefined;
    const service = adapter(vi.fn().mockResolvedValue(fresh([row, secondRow])));
    service.decide = vi.fn(() => new Promise<void>((resolve) => { release = () => resolve(); }));
    const store = createSteamConfirmationsStore(service);
    await store.refresh();

    store.toggleChecked(secondRow.handle);
    void store.decide(row.handle, "accept");
    expect(get(store).pendingHandle).toBe(row.handle);

    store.toggleChecked(secondRow.handle);
    store.clearChecked();
    store.checkAll();
    expect(get(store).checked).toEqual({ [secondRow.handle]: true });

    release?.();
    await settle();
    store.destroy();
  });

  // A mark only means anything against the row it was put on.
  it("drops marks for rows a refresh no longer lists", async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(fresh([row, secondRow]))
      .mockResolvedValueOnce(fresh([secondRow]));
    const store = createSteamConfirmationsStore(adapter(fetch));
    await store.refresh();

    store.checkAll();
    await store.refresh();

    expect(get(store).checked).toEqual({ [secondRow.handle]: true });
    store.destroy();
  });
});
