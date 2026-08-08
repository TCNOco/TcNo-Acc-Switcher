import { get } from "svelte/store";
import { t } from "../stores/i18n";

export const STEAM_GUARD_CODE_LIFETIME_MS = 30_000;

export type SteamGuardAccountRef = {
  id: string;
  username: string;
  displayName?: string;
  /**
   * Switcher avatar handed in by the opener. Screens that run before the vault is
   * unlocked cannot look this up, so carrying it avoids a placeholder there.
   */
  imageUrl?: string;
  staticImageUrl?: string;
};

/**
 * Which shape the vault record has. A discriminant rather than a boolean because
 * the vault has grown a third shape once already.
 */
export type SteamGuardAccountKind = "authenticator" | "login-only";

/**
 * What the account's stored Steam session looks like, decided by Go from the
 * record alone. "unknown" is a real answer, not a placeholder: a token this build
 * cannot read says nothing either way, so the row claims nothing either.
 */
export type SteamGuardSessionStatus = "unknown" | "valid" | "needs-login";

export type SteamGuardAccountSummary = SteamGuardAccountRef & {
  locked: boolean;
  /**
   * Required, not optional, on purpose: every construction site has to be
   * visited, or one of them silently defaults to "authenticator" and the picker
   * sends a secret-less record to the code screen.
   */
  kind: SteamGuardAccountKind;
  /** Required for the same reason as kind: a default would assert a session state. */
  sessionStatus: SteamGuardSessionStatus;
  /** Switcher avatar for the picker row; absent when the account is not in the switcher. */
  imageUrl?: string;
  staticImageUrl?: string;
  vac?: boolean;
  limited?: boolean;
};

export function isLoginOnlySummary(summary: { kind?: SteamGuardAccountKind }): boolean {
  return summary.kind === "login-only";
}

export type SteamGuardRowState = "locked" | "login-again" | "login-only" | "ready" | "unverified";

/**
 * What a picker row's state badge says. Locked outranks everything: it is what the
 * user has to act on first. A lapsed session outranks the record's kind for the
 * same reason - a login-only account that cannot sign in is nothing but a sign-in
 * prompt. An undecidable session falls back to "unverified", which still reads
 * "Ready" (the vault is open, the account is usable) but claims nothing about Steam.
 */
export function steamGuardRowState(summary: SteamGuardAccountSummary): SteamGuardRowState {
  if (summary.locked) return "locked";
  if (summary.sessionStatus === "needs-login") return "login-again";
  if (isLoginOnlySummary(summary)) return "login-only";
  return summary.sessionStatus === "valid" ? "ready" : "unverified";
}

export type SteamGuardCodeView = {
  account: SteamGuardAccountRef;
  code: string;
  expiresAt: number;
  timeStatus: "fresh" | "stale" | "untrusted" | "unavailable";
  unlockPersistence: "cached" | "one_operation";
};

export type SteamGuardSensitiveGrant = {
  capability: string;
  lease: string;
  accountId: string;
};

export type SteamGuardQRScanResult = {
	state: "ready" | "no-code" | "multiple-codes" | "steam-not-found" | "no-window" | "unavailable" | "invalid-image" | "work-limit" | "canceled" | "busy" | "unsupported" | "capture-failed";
	attempt?: string;
	candidateCount?: number;
};

export type SteamGuardQRApproval = {
	accountName: string;
	deviceName?: string;
	ipAddress?: string;
	location?: string;
	platform: string;
	application: string;
	persistence: string;
	locationMismatch: boolean;
	highUsageLogin: boolean;
	previouslyUsedLocation: boolean;
	requestorDeviceTrustCode?: number;
};

export type SteamAuthPurpose = "login_again" | "add_authenticator" | "login_only";

/**
 * Marks an account id that is a single-use add-account attempt rather than a
 * SteamID64. Go issues these and is the only thing that accepts them; this side
 * only needs to know which of the two capability endpoints to ask, and which
 * controller methods to drive. Must match pendingAddPrefix in Go.
 */
export const PENDING_ACCOUNT_ID_PREFIX = "pending:";

export function isPendingAccountId(id: string): boolean {
	return id.startsWith(PENDING_ACCOUNT_ID_PREFIX);
}

export type SteamLoginResult = {
	state: "refreshed" | "reauthentication_required" | "removed";
	refreshTokenRenewed: boolean;
	capabilityRefreshRequired: boolean;
	registryUpdated: boolean;
};

/**
 * A refreshed session is done; anything else means Steam rejected the saved refresh token and
 * the credential form must be shown directly — there is no intermediate confirmation step.
 */
export function steamLoginAgainNextStep(result: SteamLoginResult): "done" | "credentials" {
	return result.state === "refreshed" ? "done" : "credentials";
}

export type SteamEnrollmentStatus = {
	state: string;
	confirmation: "sms" | "email" | "unknown";
	phoneHint: string;
	retryAfterSeconds: number;
	hasRetryAfter: boolean;
	pending: boolean;
	resumed: boolean;
	revocationViewAvailable: boolean;
	capabilityRefreshRequired: boolean;
	registryUpdated: boolean;
};

/**
 * Whether an account's stored Steam session still works. This drives an affordance
 * only, so an inconclusive answer stays false rather than pointing the user at a
 * sign-in they may not need.
 */
export type SteamSessionState = {
  needsLogin: boolean;
  reason?: string;
};

/**
 * A session verdict taken after a renewal was attempted, so needsLogin means the
 * stored refresh token could not produce a working session — not merely that the
 * short-lived access token lapsed. The renewal writes to the vault, so
 * capabilityRefreshRequired means the caller's capability has to be re-acquired.
 */
export type SteamSessionRefreshState = SteamSessionState & {
  capabilityRefreshRequired: boolean;
};

/**
 * Where an export went. An empty path is the save-dialog cancel. manifestSkipped
 * means the maFile was written but its companion manifest was not, so SDA cannot
 * import it yet — a warning rather than a failure.
 */
export type SteamMaFileExportResult = {
  path: string;
  manifestSkipped: boolean;
};

export type SteamCredentialResult = {
	handle: string;
	state: string;
	challenges: string[];
	canSubmitEmailCode: boolean;
	canSubmitDeviceCode: boolean;
	canPoll: boolean;
	pollAfterMillis: number;
	expiresAtUnix: number;
	outcome?: "session_updated" | "enrollment_pending" | "enrollment_not_started";
	enrollment?: SteamEnrollmentStatus;
	capabilityRefreshRequired: boolean;
	registryUpdated: boolean;
	/** Set once Steam names the account. An add-account attempt has no other way
	 *  to learn its SteamID64, and any enrollment that follows is keyed on the
	 *  real id, never the pending one it logged in under. */
	steamId64?: string;
};

export type SteamRevocationView = {
	code: string;
	capabilityRefreshRequired: boolean;
};

export type SteamGuardVaultStatus = {
	configured: boolean;
	unlocked: boolean;
	rememberForSession: boolean;
	savedAccountDataEncrypted: boolean;
	/** A security key is enrolled, so unlocking can ask the device for it. */
	hasSecurityKey: boolean;
	/** Some way in needs nothing but the password. */
	passwordOpens: boolean;
};

export type SteamCredentialStep = "code" | "poll" | "complete" | "failed" | "pending";

export function steamCredentialStep(result: SteamCredentialResult): SteamCredentialStep {
	if (result.outcome) return "complete";
	if (["agreement_required", "canceled", "expired", "failed", "error"].includes(result.state)) return "failed";
	if (result.state === "challenge_required" &&
		!result.canSubmitEmailCode && !result.canSubmitDeviceCode && !result.canPoll) return "failed";
	if (result.canSubmitEmailCode || result.canSubmitDeviceCode) return "code";
	if (result.canPoll) return "poll";
	return "pending";
}

export async function acknowledgeSteamRevocationThenRefresh<T>(
	acknowledge: () => Promise<T>,
	refreshCapability: (result: T) => Promise<void>,
): Promise<T> {
	const result = await acknowledge();
	await refreshCapability(result);
	return result;
}

export async function closeSteamGuardEnrollment(actions: {
	cancelCredentials: () => Promise<void>;
	clearSecrets: () => void;
	dismiss: () => void;
}): Promise<void> {
	await actions.cancelCredentials();
	actions.clearSecrets();
	actions.dismiss();
}

/**
 * The outcome of promoting a login-only account to a full authenticator.
 *
 * needsLogin is not a failure: the stored session is simply too old to authorize
 * the enrollment, so the caller collects a password and takes the ordinary
 * add-authenticator route instead. accountName is the record's stored login
 * name, so that form starts already filled in.
 */
export type SteamGuardPromotion = {
  needsLogin: boolean;
  reason: string;
  accountName: string;
  enrollment?: SteamEnrollmentStatus;
  capabilityRefreshRequired: boolean;
  registryUpdated: boolean;
};

export type SteamEnrollmentStep = "not-started" | "recovery" | "confirmation" | "complete" | "blocked";

export function steamEnrollmentStep(status: SteamEnrollmentStatus): SteamEnrollmentStep {
	if (status.state === "not_started") return "not-started";
	if (!status.pending) return status.state === "complete" ? "complete" : "blocked";
	return status.revocationViewAvailable ? "recovery" : "confirmation";
}

export type SteamGuardModalEntry =
  | "account"
  | "all-accounts"
  | "import"
  | "enrollment"
  | "qr"
  | "login-again"
  | "login-only-setup"
  | "setup"
  | "add-account";

export type SteamGuardModalState =
  | { screen: "loading"; account: SteamGuardAccountRef }
  | { screen: "locked"; account: SteamGuardAccountRef }
  | { screen: "account-code"; view: SteamGuardCodeView }
  | { screen: "all-accounts"; accounts: SteamGuardAccountSummary[] }
  | { screen: "import"; account?: SteamGuardAccountRef }
  | { screen: "enrollment"; account: SteamGuardAccountRef }
  | { screen: "qr"; account: SteamGuardAccountRef }
  | { screen: "login-again"; account: SteamGuardAccountRef }
  // A login-only record has no shared secret, so it can never reach
  // account-code. These two screens are its whole surface.
  | { screen: "login-only"; account: SteamGuardAccountRef }
  | { screen: "login-only-setup"; account: SteamGuardAccountRef }
  // An account the vault does not hold yet. Nothing is stored for it, so this
  // screen only offers the three ways to start storing one.
  | { screen: "setup"; account: SteamGuardAccountRef }
  // An account nobody has named yet. It runs under a pending id until Steam
  // says which account the credentials belong to, so the ref only exists once
  // an attempt has been started.
  | { screen: "add-account"; account?: SteamGuardAccountRef }
  | { screen: "export-authorize"; account: SteamGuardAccountRef }
  | { screen: "recovery"; account?: SteamGuardAccountRef; message: string }
  | { screen: "error"; account?: SteamGuardAccountRef; message: string };

export type SteamGuardModalAction =
  | { type: "load-account"; account: SteamGuardAccountRef }
  | { type: "lock-account"; account: SteamGuardAccountRef }
  | { type: "show-code"; view: SteamGuardCodeView }
  | { type: "show-all"; accounts: SteamGuardAccountSummary[] }
  | { type: "show-import"; account?: SteamGuardAccountRef }
  | { type: "show-enrollment"; account: SteamGuardAccountRef }
  | { type: "show-qr"; account: SteamGuardAccountRef }
  | { type: "show-login-again"; account: SteamGuardAccountRef }
  | { type: "show-login-only"; account: SteamGuardAccountRef }
  | { type: "show-login-only-setup"; account: SteamGuardAccountRef }
  | { type: "show-setup"; account: SteamGuardAccountRef }
  | { type: "show-add-account"; account?: SteamGuardAccountRef }
  | { type: "show-export-authorize"; account: SteamGuardAccountRef }
  | { type: "show-recovery"; account?: SteamGuardAccountRef; message: string }
  | { type: "fail"; account?: SteamGuardAccountRef; message: string };

export type SteamGuardModalController = {
  getCode: (accountId: string, capability: string) => Promise<SteamGuardCodeView | null>;
  unlock: (
    accountId: string,
    password: string,
    rememberForSession: boolean,
    capability: string,
  ) => Promise<SteamGuardCodeView>;
  /** Unlock for a vault whose slots need more than a password. The keyfile is
   *  passed as a path: only Go reads its contents. */
  unlockWithFactors?: (
    accountId: string,
    password: string,
    keyfilePath: string,
    backupKey: string,
    rememberForSession: boolean,
    capability: string,
  ) => Promise<SteamGuardCodeView>;
  /** Returns the chosen keyfile path, or "" if the user cancelled. */
  pickKeyfile?: () => Promise<string>;
  listAccounts?: (accountId: string, capability: string) => Promise<SteamGuardAccountSummary[]>;
  /**
   * Login-only records only. An authenticator's secrets exist nowhere else, so
   * removing one would be an unrecoverable loss; Go re-checks the record kind
   * under its own lock, and this gate is only UX.
   */
  removeLoginOnlyAccount?: (accountId: string, capability: string) => Promise<SteamLoginResult>;
  /**
   * Login-only records only. Starts enrollment from the session the record
   * already holds, so the user is only asked for what Steam insists on.
   */
  promoteLoginOnlyAccount?: (accountId: string, capability: string) => Promise<SteamGuardPromotion>;
  copyCode?: (accountId: string, capability: string) => Promise<void> | void;
	openConfirmations?: (accountId: string, capability: string) => Promise<void> | void;
	  loginAgain?: (accountId: string, capability: string) => Promise<SteamLoginResult>;
	  beginCredentialLogin?: (
		accountId: string,
		capability: string,
		accountName: string,
		password: string,
		purpose: SteamAuthPurpose,
	  ) => Promise<SteamCredentialResult>;
	  submitCredentialCode?: (
		accountId: string,
		capability: string,
		handle: string,
		challenge: string,
		code: string,
	  ) => Promise<SteamCredentialResult>;
	  pollCredentialLogin?: (
		accountId: string,
		capability: string,
		handle: string,
	  ) => Promise<SteamCredentialResult>;
	  cancelCredentialLogin?: (accountId: string, capability: string, handle: string) => Promise<void>;
	  /**
	   * Adding an account nobody has named yet. Steam does not say which account
	   * the credentials belong to until it authorises them, so these run under a
	   * single-use pending id from newAddAccountAttempt instead of a SteamID64,
	   * and the real id arrives on the result as steamId64.
	   */
	  newAddAccountAttempt?: () => Promise<string>;
	  requestAddAccountView?: (pendingId: string, requestId: string) => Promise<void>;
	  beginAddAccountLogin?: (
		pendingId: string,
		capability: string,
		accountName: string,
		password: string,
		purpose: SteamAuthPurpose,
	  ) => Promise<SteamCredentialResult>;
	  submitAddAccountCode?: (
		pendingId: string,
		capability: string,
		handle: string,
		challenge: string,
		code: string,
	  ) => Promise<SteamCredentialResult>;
	  pollAddAccountLogin?: (
		pendingId: string,
		capability: string,
		handle: string,
	  ) => Promise<SteamCredentialResult>;
	  cancelAddAccountLogin?: (pendingId: string, capability: string, handle: string) => Promise<void>;
	  /** Offline: reads the stored session only, so it costs no Steam request. */
	  steamSessionLocalState?: (accountId: string, capability: string) => Promise<SteamSessionState>;
	  /**
	   * Renews a lapsed session from the stored refresh token before reporting on it,
	   * so a day-old access token does not ask for a password the refresh token can
	   * still avoid. May write to the vault; see capabilityRefreshRequired.
	   */
	  ensureFreshSession?: (accountId: string, capability: string) => Promise<SteamSessionRefreshState>;
	  /** Asks Steam whether it still accepts the stored session. Read-only. */
	  probeSteamSession?: (accountId: string, capability: string) => Promise<SteamSessionState>;
	  getSteamGuardVaultStatus?: () => Promise<SteamGuardVaultStatus>;
	  initializeSteamGuardVault?: (password: string, appPassword: string) => Promise<void>;
	  /**
	   * Unlocks the vault itself rather than one account, for the screens that
	   * ADD an account - the account is not in the vault yet, so the
	   * account-level unlock has nothing to return. keyfilePath and backupKey
	   * are optional but must be offered: a vault protected by either cannot be
	   * opened by a password alone.
	   */
	  unlockSteamGuardVault?: (
		accountId: string,
		password: string,
		rememberForSession: boolean,
		capability: string,
		keyfilePath?: string,
		backupKey?: string,
	  ) => Promise<void>;
	  /** Resolves to the written path, or "" when the user cancelled the save dialog. */
	  /** maFilePassword encrypts the file the way SDA does; empty exports plaintext. */
	  exportMaFile?: (
		accountId: string,
		capability: string,
		password: string,
		maFilePassword: string,
	  ) => Promise<SteamMaFileExportResult>;
  importMaFile?: (accountId?: string) => Promise<void>;
	  resumeSteamGuardEnrollment?: (accountId: string, capability: string) => Promise<SteamEnrollmentStatus>;
	  revealSteamGuardRevocationCode?: (accountId: string, capability: string) => Promise<SteamRevocationView>;
	  acknowledgeSteamGuardRevocationCode?: (
		accountId: string,
		capability: string,
		code: string,
	  ) => Promise<SteamEnrollmentStatus>;
	  finalizeSteamGuardEnrollment?: (
		accountId: string,
		capability: string,
		confirmationCode: string,
	  ) => Promise<SteamEnrollmentStatus>;
	  cancelSteamGuardEnrollment?: (accountId: string, capability: string) => Promise<void>;
	  showEnrollmentBackupWarning?: () => Promise<void>;
	  captureQrFromSteam?: (accountId: string, capability: string) => Promise<SteamGuardQRScanResult>;
	  chooseQrScreenshot?: (accountId: string, capability: string) => Promise<SteamGuardQRScanResult | null>;
	  decodeQrScreenshot?: (accountId: string, path: string, capability: string) => Promise<SteamGuardQRScanResult>;
	  getQrApproval?: (accountId: string, attempt: string, capability: string) => Promise<SteamGuardQRApproval>;
	  authorizeQrLogin?: (accountId: string, attempt: string, capability: string) => Promise<void>;
	  dismissQrLogin?: (accountId: string, attempt: string, capability: string) => Promise<void>;
	  selectQrRegion?: (accountId: string, capability: string) => Promise<SteamGuardQRScanResult>;
	  cancelQrRegion?: (accountId: string, capability: string) => Promise<void>;
  recover?: (accountId?: string) => Promise<void>;
  requestSensitiveView?: (accountId: string) => Promise<SteamGuardSensitiveGrant>;
  endSensitiveView?: (capability: string, lease: string) => Promise<void>;
};

/** Thrown when a sensitive-view grant is missing, malformed, or bound to another account. */
export class SteamGuardCapabilityError extends Error {
	constructor(message = "Steam Guard capability unavailable") {
		super(message);
		this.name = "SteamGuardCapabilityError";
	}
}

export class SteamGuardContentProtectionLease {
	private grant: SteamGuardSensitiveGrant | null = null;
  private closed = false;

  constructor(private readonly controller: SteamGuardModalController) {}

	async acquire(accountId: string): Promise<void> {
		if (this.closed) return;
		if (!this.controller.requestSensitiveView) throw new SteamGuardCapabilityError();
		const grant = await this.controller.requestSensitiveView(accountId);
		if (!grant.capability || !grant.lease || grant.accountId !== accountId) {
			throw new SteamGuardCapabilityError("Steam Guard returned a capability for another account");
		}
		if (this.closed) {
			await this.controller.endSensitiveView?.(grant.capability, grant.lease);
			return;
		}
		const previous = this.grant;
		this.grant = grant;
		if (previous) await this.controller.endSensitiveView?.(previous.capability, previous.lease);
	}

	capabilityFor(accountId: string): string {
		return this.grant?.accountId === accountId ? this.grant.capability : "";
	}

	revoke(): void {
		this.grant = null;
	}

  async close(): Promise<void> {
    if (this.closed) return;
    this.closed = true;
		const grant = this.grant;
		this.grant = null;
		if (grant) await this.controller.endSensitiveView?.(grant.capability, grant.lease);
	}
}

export function initialSteamGuardModalState(
  account: SteamGuardAccountRef,
  entry: SteamGuardModalEntry,
): SteamGuardModalState {
  if (entry === "import") return { screen: "import", account };
  if (entry === "enrollment") return { screen: "enrollment", account };
  if (entry === "qr") return { screen: "qr", account };
  if (entry === "login-again") return { screen: "login-again", account };
  if (entry === "login-only-setup") return { screen: "login-only-setup", account };
  if (entry === "setup") return { screen: "setup", account };
  if (entry === "add-account") return { screen: "add-account" };
  return { screen: "loading", account };
}

export function reduceSteamGuardModal(
  _state: SteamGuardModalState,
  action: SteamGuardModalAction,
): SteamGuardModalState {
  switch (action.type) {
    case "load-account":
      return { screen: "loading", account: action.account };
    case "lock-account":
      return { screen: "locked", account: action.account };
    case "show-code":
      return { screen: "account-code", view: action.view };
    case "show-all":
      return { screen: "all-accounts", accounts: action.accounts };
    case "show-import":
      return { screen: "import", account: action.account };
    case "show-enrollment":
      return { screen: "enrollment", account: action.account };
    case "show-qr":
      return { screen: "qr", account: action.account };
    case "show-login-again":
      return { screen: "login-again", account: action.account };
    case "show-login-only":
      return { screen: "login-only", account: action.account };
    case "show-login-only-setup":
      return { screen: "login-only-setup", account: action.account };
    case "show-setup":
      return { screen: "setup", account: action.account };
    case "show-add-account":
      return { screen: "add-account", account: action.account };
    case "show-export-authorize":
      return { screen: "export-authorize", account: action.account };
    case "show-recovery":
      return { screen: "recovery", account: action.account, message: action.message };
    case "fail":
      return { screen: "error", account: action.account, message: action.message };
  }
}

export function steamGuardCodeProgress(expiresAt: number, now: number): number {
  if (!Number.isFinite(expiresAt) || !Number.isFinite(now)) return 0;
  return Math.max(0, Math.min(1, (expiresAt - now) / STEAM_GUARD_CODE_LIFETIME_MS));
}

export function steamGuardCodeCanAutoRefresh(view: SteamGuardCodeView): boolean {
  return view.unlockPersistence === "cached";
}

export function steamGuardQRFailureMessage(result: SteamGuardQRScanResult): string {
	// Reads the locale at call time so a language switch applies to later scans.
	const tr = get(t);
	switch (result.state) {
		case "steam-not-found": return tr("SteamGuard_QRFailure_SteamNotFound");
		case "no-window": return tr("SteamGuard_QRFailure_NoWindow");
		case "no-code": return tr("SteamGuard_QRFailure_NoCode");
		case "multiple-codes": return tr("SteamGuard_QRFailure_MultipleCodes");
		case "invalid-image": return tr("SteamGuard_QRFailure_InvalidImage");
		case "work-limit": return tr("SteamGuard_QRFailure_WorkLimit");
		case "canceled": return tr("SteamGuard_QRFailure_Canceled");
		case "busy": return tr("SteamGuard_QRFailure_Busy");
		case "unsupported": return tr("SteamGuard_QRFailure_Unsupported");
		case "capture-failed": return tr("SteamGuard_QRFailure_CaptureFailed");
		default: return tr("SteamGuard_QRFailure_Unavailable");
	}
}

export function steamGuardAccountForState(state: SteamGuardModalState): SteamGuardAccountRef | undefined {
  if (state.screen === "account-code") return state.view.account;
  if (state.screen === "all-accounts") return undefined;
  return state.account;
}
