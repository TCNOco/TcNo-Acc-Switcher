import { describe, expect, it } from "vitest";
import {
	  initialSteamGuardModalState,
	  acknowledgeSteamRevocationThenRefresh,
	  closeSteamGuardEnrollment,
  reduceSteamGuardModal,
  isPendingAccountId,
  SteamGuardCapabilityError,
  SteamGuardContentProtectionLease,
	  steamLoginAgainNextStep,
	  steamGuardAccountForState,
	  steamGuardListingAnchor,
		  steamGuardCodeCanAutoRefresh,
		  steamGuardCodeProgress,
	  steamGuardQRFailureMessage,
	  steamGuardRowState,
	  steamCredentialStep,
	  steamEnrollmentStep,
  type SteamGuardAccountRef,
  type SteamGuardAccountSummary,
  type SteamGuardCodeView,
	  type SteamGuardModalState,
	  type SteamCredentialResult,
	  type SteamLoginResult,
} from "./steamGuardModal";

const account: SteamGuardAccountRef = { id: "76561198000000001", username: "test_user" };
const view: SteamGuardCodeView = {
  account,
  code: "A2BCD",
	  expiresAt: 40_000,
	  timeStatus: "fresh",
	  unlockPersistence: "cached",
};

describe("Steam Guard modal state", () => {
  it.each([
    ["account", "loading"],
    ["all-accounts", "loading"],
    ["import", "import"],
    ["enrollment", "enrollment"],
    ["qr", "qr"],
    ["login-again", "login-again"],
    ["login-only-setup", "login-only-setup"],
    // An account the vault does not hold. It must not start on "loading": that
    // path asks for a code the account has no record to produce.
    ["setup", "setup"],
  ] as const)("opens the %s entry on the %s screen", (entry, screen) => {
    expect(initialSteamGuardModalState(account, entry).screen).toBe(screen);
  });

  it("carries only the account into the in-modal export authorization screen", () => {
    const state = reduceSteamGuardModal(
      { screen: "account-code", view },
      { type: "show-export-authorize", account },
    );
    expect(state).toEqual({ screen: "export-authorize", account });
  });

  it("returns from export authorization to the code screen without losing the account", () => {
    let state: SteamGuardModalState = reduceSteamGuardModal(
      { screen: "account-code", view },
      { type: "show-export-authorize", account },
    );
    expect(steamGuardAccountForState(state)).toEqual(account);
    state = reduceSteamGuardModal(state, { type: "show-code", view });
    expect(state.screen).toBe("account-code");
  });

  it("moves from loading to locked without retaining code data", () => {
    const loading = initialSteamGuardModalState(account, "account");
    const locked = reduceSteamGuardModal(loading, { type: "lock-account", account });

    expect(locked).toEqual({ screen: "locked", account });
    expect("view" in locked).toBe(false);
  });

  it("moves through every user-facing branch", () => {
    let state: SteamGuardModalState = initialSteamGuardModalState(account, "account");
    state = reduceSteamGuardModal(state, { type: "show-code", view });
    expect(state.screen).toBe("account-code");

    state = reduceSteamGuardModal(state, {
      type: "show-all",
      accounts: [{ ...account, locked: false, kind: "authenticator", sessionStatus: "valid" }],
    });
    expect(state.screen).toBe("all-accounts");

    state = reduceSteamGuardModal(state, { type: "show-import", account });
    expect(state.screen).toBe("import");
    state = reduceSteamGuardModal(state, { type: "show-enrollment", account });
    expect(state.screen).toBe("enrollment");
    state = reduceSteamGuardModal(state, { type: "show-qr", account });
    expect(state.screen).toBe("qr");
    state = reduceSteamGuardModal(state, { type: "show-login-again", account });
    expect(state.screen).toBe("login-again");
    state = reduceSteamGuardModal(state, { type: "show-export-authorize", account });
    expect(state.screen).toBe("export-authorize");
    state = reduceSteamGuardModal(state, {
      type: "show-recovery",
      account,
      message: "Restore backup.",
    });
    expect(state.screen).toBe("recovery");
    state = reduceSteamGuardModal(state, { type: "fail", account, message: "Failed." });
    expect(state.screen).toBe("error");
  });

	  it("derives the countdown only from expiry and current time", () => {
    expect(steamGuardCodeProgress(40_000, 10_000)).toBe(1);
    expect(steamGuardCodeProgress(40_000, 25_000)).toBe(0.5);
    expect(steamGuardCodeProgress(40_000, 40_000)).toBe(0);
    expect(steamGuardCodeProgress(40_000, 50_000)).toBe(0);
  });

	  it("does not expose an account from the all-accounts state", () => {
    expect(steamGuardAccountForState({ screen: "account-code", view })).toEqual(account);
    expect(steamGuardAccountForState({ screen: "all-accounts", accounts: [] })).toBeUndefined();
	  });

	  it("refreshes cached codes but not one-operation codes", () => {
		  expect(steamGuardCodeCanAutoRefresh(view)).toBe(true);
		  expect(steamGuardCodeCanAutoRefresh({ ...view, unlockPersistence: "one_operation" })).toBe(false);
	  });

	  it.each([
		["canceled", "SteamGuard_QRFailure_Canceled"],
		["busy", "SteamGuard_QRFailure_Busy"],
		["unsupported", "SteamGuard_QRFailure_Unsupported"],
		["capture-failed", "SteamGuard_QRFailure_CaptureFailed"],
	  ] as const)("maps the %s region outcome to a safe message", (state, message) => {
		expect(steamGuardQRFailureMessage({ state })).toContain(message);
	  });
	});

describe("Steam Guard content-protection lease", () => {
  it("acquires and releases exactly once", async () => {
    const calls: string[] = [];
    const lease = new SteamGuardContentProtectionLease({
      getCode: async () => null,
      unlock: async () => view,
		requestSensitiveView: async (accountId) => {
			calls.push("begin");
			return { capability: "cap-1", lease: "lease-1", accountId };
		},
		endSensitiveView: async (capability, nativeLease) => {
			calls.push(`end:${capability}:${nativeLease}`);
		},
	});

	await lease.acquire(account.id);
	expect(lease.capabilityFor(account.id)).toBe("cap-1");
	await lease.close();
	await lease.close();
	expect(calls).toEqual(["begin", "end:cap-1:lease-1"]);
	});

	it("releases a lease that resolves after the modal closes", async () => {
		let resolveLease: (grant: { capability: string; lease: string; accountId: string }) => void = () => {};
		const pendingLease = new Promise<{ capability: string; lease: string; accountId: string }>((resolve) => {
			resolveLease = resolve;
		});
    const released: string[] = [];
    const lease = new SteamGuardContentProtectionLease({
      getCode: async () => null,
      unlock: async () => view,
		requestSensitiveView: () => pendingLease,
		endSensitiveView: async (capability, nativeLease) => {
			released.push(`${capability}:${nativeLease}`);
		},
	});

	const acquiring = lease.acquire(account.id);
	await lease.close();
	resolveLease({ capability: "late-cap", lease: "late-lease", accountId: account.id });
	await acquiring;
	expect(released).toEqual(["late-cap:late-lease"]);
  });

  it("does not release a lease when native acquisition fails", async () => {
    const released: string[] = [];
    const lease = new SteamGuardContentProtectionLease({
      getCode: async () => null,
      unlock: async () => view,
		requestSensitiveView: async () => {
			throw new Error("content protection unavailable");
		},
		endSensitiveView: async (capability) => {
			released.push(capability);
		},
	});

	await expect(lease.acquire(account.id)).rejects.toThrow("content protection unavailable");
    await lease.close();
    expect(released).toEqual([]);
  });

  it("throws instead of silently holding no capability when the grant is for another account", async () => {
    const lease = new SteamGuardContentProtectionLease({
      getCode: async () => null,
      unlock: async () => view,
      requestSensitiveView: async () => ({ capability: "cap", lease: "lease", accountId: "other" }),
    });

    await expect(lease.acquire(account.id)).rejects.toThrow(SteamGuardCapabilityError);
    expect(lease.capabilityFor(account.id)).toBe("");
  });

  it("throws when the grant is missing its capability or lease", async () => {
    const lease = new SteamGuardContentProtectionLease({
      getCode: async () => null,
      unlock: async () => view,
      requestSensitiveView: async (accountId) => ({ capability: "", lease: "lease", accountId }),
    });

    await expect(lease.acquire(account.id)).rejects.toThrow(SteamGuardCapabilityError);
  });

  it("throws when the native sensitive-view request is unavailable", async () => {
    const lease = new SteamGuardContentProtectionLease({
      getCode: async () => null,
      unlock: async () => view,
    });

    await expect(lease.acquire(account.id)).rejects.toThrow(SteamGuardCapabilityError);
  });
});

describe("Steam Guard login again", () => {
  const loginResult = (values: Partial<SteamLoginResult>): SteamLoginResult => ({
    state: "refreshed",
    refreshTokenRenewed: true,
    capabilityRefreshRequired: false,
    registryUpdated: false,
    ...values,
  });

  it("finishes without a password prompt when the refresh succeeds", () => {
    expect(steamLoginAgainNextStep(loginResult({ capabilityRefreshRequired: true }))).toBe("done");
  });

  it("goes straight to the credential form when Steam wants reauthentication", () => {
    expect(steamLoginAgainNextStep(loginResult({
      state: "reauthentication_required",
      capabilityRefreshRequired: false,
    }))).toBe("credentials");
  });
});

describe("Steam credential flow", () => {
	const result = (values: Partial<SteamCredentialResult>): SteamCredentialResult => ({
		handle: "opaque-handle",
		state: "waiting",
		challenges: [],
		canSubmitEmailCode: false,
		canSubmitDeviceCode: false,
		canPoll: false,
		pollAfterMillis: 1_000,
		expiresAtUnix: 0,
		capabilityRefreshRequired: false,
		registryUpdated: false,
		...values,
	});

	it("prefers a completed outcome over stale challenge flags", () => {
		expect(steamCredentialStep(result({ outcome: "session_updated", canSubmitEmailCode: true }))).toBe("complete");
	});

	it.each([
		[{ canSubmitEmailCode: true }, "code"],
		[{ canSubmitDeviceCode: true }, "code"],
		[{ canPoll: true }, "poll"],
		[{ state: "expired", canPoll: true }, "failed"],
		[{ state: "challenge_required" }, "failed"],
		[{}, "pending"],
	] as const)("maps a native credential response to its next UI step", (values, step) => {
		expect(steamCredentialStep(result(values))).toBe(step);
	});
});

describe("Steam enrollment capability order", () => {
	it("closes a pre-pending enrollment without loading an authenticator account", async () => {
		const calls: string[] = [];
		await closeSteamGuardEnrollment({
			cancelCredentials: async () => { calls.push("cancel-credentials"); },
			clearSecrets: () => { calls.push("clear-secrets"); },
			dismiss: () => { calls.push("dismiss"); },
		});
		expect(calls).toEqual(["cancel-credentials", "clear-secrets", "dismiss"]);
	});

	it("acknowledges with the reveal capability before acquiring the next generation", async () => {
		const calls: string[] = [];
		await acknowledgeSteamRevocationThenRefresh(
			async () => { calls.push("acknowledge"); },
			async () => { calls.push("refresh"); },
		);
		expect(calls).toEqual(["acknowledge", "refresh"]);
	});

	it("does not acquire a new capability when acknowledgment fails", async () => {
		const calls: string[] = [];
		await expect(acknowledgeSteamRevocationThenRefresh(
			async () => { throw new Error("rejected"); },
			async () => { calls.push("refresh"); },
		)).rejects.toThrow("rejected");
		expect(calls).toEqual([]);
	});
});

describe("Steam enrollment resume projection", () => {
	const status = (values: Partial<import("./steamGuardModal").SteamEnrollmentStatus>) => ({
		state: "awaiting_confirmation",
		confirmation: "sms" as const,
		phoneHint: "",
		retryAfterSeconds: 0,
		hasRetryAfter: false,
		pending: true,
		resumed: true,
		revocationViewAvailable: false,
		capabilityRefreshRequired: false,
		registryUpdated: false,
		...values,
	});

	it("re-reveals recovery after a restart before acknowledgment", () => {
		expect(steamEnrollmentStep(status({ revocationViewAvailable: true }))).toBe("recovery");
	});

	it("continues to confirmation after a persisted acknowledgment", () => {
		expect(steamEnrollmentStep(status({ revocationViewAvailable: false }))).toBe("confirmation");
	});

	it.each([
		[{ state: "not_started", pending: false }, "not-started"],
		[{ state: "complete", pending: false }, "complete"],
		[{ state: "rate_limited", pending: false }, "blocked"],
	] as const)("maps non-pending enrollment state", (values, step) => {
		expect(steamEnrollmentStep(status(values))).toBe(step);
	});
});

// The picker row shows one badge, so the states have to be ranked. What the user
// must act on comes first; a claim about the Steam session is only made when the
// stored token actually said something.
describe("Steam Guard picker row state", () => {
	const summary = (values: Partial<SteamGuardAccountSummary>): SteamGuardAccountSummary => ({
		...account,
		locked: false,
		kind: "authenticator",
		sessionStatus: "valid",
		...values,
	});

	it("puts locked ahead of every other state", () => {
		expect(steamGuardRowState(summary({ locked: true }))).toBe("locked");
		expect(steamGuardRowState(summary({ locked: true, kind: "login-only" }))).toBe("locked");
		expect(steamGuardRowState(summary({ locked: true, sessionStatus: "needs-login" }))).toBe("locked");
	});

	it("puts a lapsed session ahead of the record's kind", () => {
		expect(steamGuardRowState(summary({ sessionStatus: "needs-login" }))).toBe("login-again");
		expect(steamGuardRowState(summary({ kind: "login-only", sessionStatus: "needs-login" })))
			.toBe("login-again");
	});

	it("names a working session ready, and an unreadable one unverified", () => {
		expect(steamGuardRowState(summary({}))).toBe("ready");
		expect(steamGuardRowState(summary({ sessionStatus: "unknown" }))).toBe("unverified");
		expect(steamGuardRowState(summary({ kind: "login-only" }))).toBe("login-only");
	});
});

describe("add-account screen", () => {
	it("opens straight onto the screen when that is the entry", () => {
		const state = initialSteamGuardModalState({ id: "", username: "" }, "add-account");
		expect(state.screen).toBe("add-account");
	});

	// The screen exists before any attempt does, and carries the pending id once
	// one is minted so the credential steps have an identity to run under.
	it("carries the pending attempt once one exists", () => {
		const empty = reduceSteamGuardModal(
			{ screen: "loading", account: { id: "1", username: "a" } },
			{ type: "show-add-account" },
		);
		expect(empty).toEqual({ screen: "add-account", account: undefined });

		const pending = { id: "pending:0123456789abcdef0123456789abcdef", username: "new_account" };
		const withAttempt = reduceSteamGuardModal(empty, { type: "show-add-account", account: pending });
		expect(steamGuardAccountForState(withAttempt)?.id).toBe(pending.id);
	});

	it("recognises a pending id without mistaking a SteamID64 for one", () => {
		expect(isPendingAccountId("pending:0123456789abcdef0123456789abcdef")).toBe(true);
		expect(isPendingAccountId("76561198000000100")).toBe(false);
		expect(isPendingAccountId("")).toBe(false);
	});
});

describe("Steam Guard listing anchor", () => {
	const listed: SteamGuardAccountSummary[] = [
		{ id: "76561198000000001", username: "first", locked: false, kind: "authenticator", sessionStatus: "valid" },
		{ id: "76561198000000002", username: "second", locked: false, kind: "login-only", sessionStatus: "valid" },
	];
	const pending: SteamGuardAccountRef = { id: "pending:0123456789abcdef0123456789abcdef", username: "" };

	// The reported bug: Back from Add Account listed the vault under the attempt
	// the screen was running on, which Go refuses - it is not a vault record - and
	// the refusal surfaced as "accounts could not be loaded".
	it("never anchors on a pending add attempt", () => {
		expect(steamGuardListingAnchor([pending], listed)?.id).toBe("76561198000000001");
		expect(steamGuardListingAnchor([pending, null, { id: "", username: "" }], listed)?.id)
			.toBe("76561198000000001");
	});

	// The setup page names an account precisely because the vault does not hold
	// it, so it is no more listable an anchor than a pending attempt.
	it("prefers a record the last listing confirmed over one merely in hand", () => {
		const notInVault: SteamGuardAccountRef = { id: "76561198000000999", username: "unstored" };
		expect(steamGuardListingAnchor([notInVault], listed)?.id).toBe("76561198000000001");
		expect(steamGuardListingAnchor([listed[1], notInVault], listed)?.id).toBe("76561198000000002");
	});

	// No listing has succeeded yet, so there is nothing to confirm against and
	// the account in hand is the only anchor there is.
	it("falls back to the candidate when nothing has been listed", () => {
		expect(steamGuardListingAnchor([account], [])?.id).toBe(account.id);
	});

	it("has no anchor when every candidate is pending or empty and nothing is listed", () => {
		expect(steamGuardListingAnchor([pending, { id: " ", username: "" }, null], [])).toBeNull();
	});
});
