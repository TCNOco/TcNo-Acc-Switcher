import * as SteamGuardService from "../../bindings/TcNo-Acc-Switcher/internal/steamguard/service.js";
import * as SteamGuardModels from "../../bindings/TcNo-Acc-Switcher/internal/steamguard/models.js";
import * as SteamService from "../../bindings/TcNo-Acc-Switcher/internal/steam/steamservice.js";
import { Events } from "@wailsio/runtime";
import { bindSteamGuardMenuToModal } from "./steam/steamGuardModalHost";
import type {
  SteamGuardAccountRef,
  SteamGuardAccountSummary,
  SteamGuardCodeView,
  SteamGuardModalController,
	SteamGuardQRScanResult,
	SteamAuthPurpose,
	SteamCredentialResult,
	SteamEnrollmentStatus,
	SteamLoginResult,
} from "./steamGuardModal";
import { openAlert, openAlertNoButton, openPrompt, openPromptWithCheckbox, openSteamGuardModal } from "../stores/modal";
import SteamGuardRestoreModalBody from "../components/modals/SteamGuardRestoreModalBody.svelte";
import { get } from "svelte/store";
import { t } from "../stores/i18n";
import { pushToast } from "../stores/toast";
import { configureSteamGuardDropAdapter } from "../stores/steamGuardDrop";
import { configureSteamGuardSettingsAdapter } from "../stores/steamGuardSettings";
import { passwordPolicyMessage, validateNewPassword } from "./passwordPolicy";
import { publishSteamGuardActionAccounts } from "../stores/steamGuardAction";
import {
	ConfirmationRequestError,
	configureSteamConfirmationsWindow,
	type ConfirmationDecision,
	type ConfirmationFailureKind,
	type ConfirmationItem,
	type ConfirmationKind,
} from "./steamConfirmations";

/** Reads the locale at call time so a language switch applies to later prompts. */
function tr(key: string, vars?: Record<string, string | number>): string {
	return get(t)(key, vars);
}

function loginResult(result: SteamGuardModels.SteamLoginResult): SteamLoginResult {
	if (result.state !== "refreshed" && result.state !== "reauthentication_required") {
		throw new Error("Invalid Steam login response");
	}
	return {
		state: result.state,
		refreshTokenRenewed: result.refreshTokenRenewed,
		capabilityRefreshRequired: result.capabilityRefreshRequired,
		registryUpdated: result.registryUpdated,
	};
}

function enrollmentStatus(result: SteamGuardModels.SteamEnrollmentStatus): SteamEnrollmentStatus {
	const confirmation = result.confirmation === "sms" || result.confirmation === "email"
		? result.confirmation
		: "unknown";
	return {
		state: result.state,
		confirmation,
		phoneHint: result.phoneHint ?? "",
		retryAfterSeconds: result.retryAfterSeconds ?? 0,
		hasRetryAfter: result.hasRetryAfter ?? false,
		pending: result.pending,
		resumed: result.resumed ?? false,
		revocationViewAvailable: result.revocationViewAvailable ?? false,
		capabilityRefreshRequired: result.capabilityRefreshRequired,
		registryUpdated: result.registryUpdated,
	};
}

function credentialResult(result: SteamGuardModels.SteamCredentialResult): SteamCredentialResult {
	const allowedOutcomes = new Set<NonNullable<SteamCredentialResult["outcome"]>>([
		"session_updated", "enrollment_pending", "enrollment_not_started",
	]);
	const outcome = allowedOutcomes.has(result.outcome as NonNullable<SteamCredentialResult["outcome"]>)
		? result.outcome as NonNullable<SteamCredentialResult["outcome"]>
		: undefined;
	return {
		handle: result.handle ?? "",
		state: result.state,
		challenges: result.challenges ?? [],
		canSubmitEmailCode: result.canSubmitEmailCode,
		canSubmitDeviceCode: result.canSubmitDeviceCode,
		canPoll: result.canPoll,
		pollAfterMillis: result.pollAfterMillis ?? 0,
		expiresAtUnix: result.expiresAtUnix ?? 0,
		outcome,
		enrollment: result.enrollment ? enrollmentStatus(result.enrollment) : undefined,
		capabilityRefreshRequired: result.capabilityRefreshRequired,
		registryUpdated: result.registryUpdated,
	};
}

function authPurpose(purpose: SteamAuthPurpose): SteamGuardModels.SteamAuthPurpose {
	return purpose === "login_again"
		? SteamGuardModels.SteamAuthPurpose.SteamAuthPurposeLoginAgain
		: SteamGuardModels.SteamAuthPurpose.SteamAuthPurposeAddAuthenticator;
}

function usesSavedDataEncryption(status: unknown): boolean {
	return (status as { savedAccountDataEncrypted?: boolean }).savedAccountDataEncrypted === true;
}

function accountRef(id: string, username = ""): SteamGuardAccountRef {
  return { id, username, displayName: username || id };
}
function codeView(view: SteamGuardModels.CodeView): SteamGuardCodeView {
  return {
    account: accountRef(view.steamId64, view.accountName),
    code: view.code,
    expiresAt: view.expiresAt,
    timeStatus: normalizeTimeStatus(view.timeStatus),
    unlockPersistence: normalizeUnlockPersistence(view.unlockPersistence),
  };
}

function normalizeTimeStatus(value: string): SteamGuardCodeView["timeStatus"] {
  if (value === "fresh" || value === "stale" || value === "untrusted") return value;
  return "unavailable";
}

function normalizeUnlockPersistence(
  value: SteamGuardModels.UnlockPersistence,
): SteamGuardCodeView["unlockPersistence"] {
  return value === SteamGuardModels.UnlockPersistence.UnlockPersistenceCached
    ? "cached"
    : "one_operation";
}

function qrScanResult(result: { state: string; attempt?: string; candidateCount?: number }): SteamGuardQRScanResult {
		const states = new Set<SteamGuardQRScanResult["state"]>([
			"ready", "no-code", "multiple-codes", "steam-not-found", "no-window", "unavailable", "invalid-image", "work-limit",
			"canceled", "busy", "unsupported", "capture-failed",
	]);
	const state = states.has(result.state as SteamGuardQRScanResult["state"])
		? result.state as SteamGuardQRScanResult["state"]
		: "unavailable";
	return { state, attempt: result.attempt, candidateCount: result.candidateCount };
}

async function promptNewVaultPassword(): Promise<string | null> {
  let password = await openPrompt({
    title: tr("SteamGuard_Vault_NewPasswordTitle"),
    body: tr("SteamGuard_Vault_NewPasswordBody"),
    inputType: "password",
    positiveLabel: tr("SteamGuard_Continue"),
    negativeLabel: tr("SteamGuard_Cancel"),
  });
  if (!password) return null;
  const policyError = validateNewPassword(password);
  if (policyError) {
    password = "";
    await openAlert({ title: tr("SteamGuard_Vault_PasswordAlertTitle"), body: passwordPolicyMessage(policyError) });
    return null;
  }
  let confirmation = await openPrompt({
    title: tr("SteamGuard_Vault_ConfirmPasswordTitle"),
    body: tr("SteamGuard_Vault_ConfirmPasswordBody"),
    inputType: "password",
    positiveLabel: tr("SteamGuard_Vault_CreateVault"),
    negativeLabel: tr("SteamGuard_Cancel"),
  });
  if (confirmation !== password) {
    password = "";
    confirmation = "";
    await openAlert({ title: tr("SteamGuard_Vault_PasswordMismatchTitle"), body: tr("SteamGuard_Vault_PasswordMismatchBody") });
    return null;
  }
  confirmation = "";
  return password;
}

async function promptExistingVaultPassword(): Promise<{ password: string; rememberForSession: boolean } | null> {
  const result = await openPromptWithCheckbox({
    title: tr("SteamGuard_Unlock_PromptTitle"),
    body: tr("SteamGuard_Unlock_PromptBody"),
    inputType: "password",
    positiveLabel: tr("SteamGuard_Unlock"),
    negativeLabel: tr("SteamGuard_Cancel"),
    checkboxLabel: tr("SteamGuard_RememberMe"),
    checkboxInitial: false,
  });
  if (!result?.value) return null;
  return { password: result.value, rememberForSession: result.checked };
}

async function importedAccountSummary(
  imported: SteamGuardModels.ImportResult[],
  loadedSwitcherAccounts?: Awaited<ReturnType<typeof SteamService.GetSteamAccountsList>>,
): Promise<string> {
  let switcherAccounts = loadedSwitcherAccounts ?? [];
  if (!loadedSwitcherAccounts) {
    try {
      switcherAccounts = await SteamService.GetSteamAccountsList();
    } catch {
      // Import remains valid when the local Steam account list is unavailable.
    }
  }

  const switcherBySteamId = new Map(
    switcherAccounts.map((account) => [(account.steamId64 ?? "").trim(), account]),
  );
  const labels = imported.map((result) => {
    const steamId64 = (result.steamId64 ?? "").trim();
    const matched = switcherBySteamId.get(steamId64);
    return matched?.displayName?.trim()
      || matched?.personaName?.trim()
      || matched?.accountName?.trim()
      || (result.accountName ?? "").trim()
      || steamId64
      || tr("SteamGuard_Import_UnknownAccount");
  });
  const uniqueLabels = [...new Set(labels)];
  if (uniqueLabels.length === 0) return "";

  const heading = uniqueLabels.length === 1
    ? tr("SteamGuard_Import_AddedToAccount")
    : tr("SteamGuard_Import_AddedToAccounts");
  return `${heading}<br>${uniqueLabels.map(escapeHtml).join("<br>")}<br><br>`;
}

export async function runImport(initialPaths?: string[]): Promise<void> {
  const paths = initialPaths?.filter(Boolean) ?? await SteamGuardService.PickMaFiles();
  if (paths.length === 0) return;

  const status = await SteamGuardService.GetSettingsStatus();
  let password = "";
  let rememberForSession = status.rememberPasswordForSession;
  if (status.vaultConfigured) {
    if (!status.unlocked) {
      const unlock = await promptExistingVaultPassword();
      if (!unlock) return;
      password = unlock.password;
      rememberForSession ||= unlock.rememberForSession;
    }
  } else {
    password = await promptNewVaultPassword() ?? "";
    if (!password) return;
  }

  let appPassword = "";
  if (!status.vaultConfigured && usesSavedDataEncryption(status)) {
    appPassword = await openPrompt({
      title: tr("SteamGuard_AppPassword_VerifyTitle"),
      body: tr("SteamGuard_AppPassword_ImportBody"),
      inputType: "password",
      positiveLabel: tr("SteamGuard_Verify"),
      negativeLabel: tr("SteamGuard_Cancel"),
    }) ?? "";
    if (!appPassword) {
      password = "";
      return;
    }
  }

  try {
    if (!status.vaultConfigured) {
      await SteamGuardService.Initialize(password, appPassword);
    } else {
      await SteamGuardService.SetFeatureEnabled(true);
    }
    let results = await SteamGuardService.ImportMaFiles(paths, password, "", rememberForSession);
    const encryptedPaths = results
      .filter((result) => result.errorCode === "legacy_wrong_password_or_corrupt")
      .map((result) => result.path);
    if (encryptedPaths.length > 0) {
      let legacyPassword = await openPrompt({
        title: tr("SteamGuard_Import_LegacyTitle"),
        body: encryptedPaths.length === 1
          ? tr("SteamGuard_Import_LegacyBodyOne")
          : tr("SteamGuard_Import_LegacyBodyMany"),
        inputType: "password",
        positiveLabel: tr("SteamGuard_Import_LegacyRetry"),
        negativeLabel: tr("SteamGuard_Cancel"),
      });
      if (legacyPassword) {
        try {
          const retried = await SteamGuardService.ImportMaFiles(
            encryptedPaths,
            password,
            legacyPassword,
            rememberForSession,
          );
          const retriedByPath = new Map(retried.map((result) => [result.path, result]));
          results = results.map((result) => retriedByPath.get(result.path) ?? result);
        } finally {
          legacyPassword = "";
        }
      }
    }
    const imported = results.filter((result) => result.imported);
    const failed = results.length - imported.length;
    let switcherAccounts:
      | Awaited<ReturnType<typeof SteamService.GetSteamAccountsList>>
      | undefined;
    if (imported.length > 0) {
      try {
        switcherAccounts = await SteamService.GetSteamAccountsList();
        publishSteamGuardActionAccounts(switcherAccounts);
      } catch {
        // The Steam page will republish availability on its next account load.
      }
    }
    const accountSummary = await importedAccountSummary(imported, switcherAccounts);
    const folder = (await SteamGuardService.GetSettingsStatus()).folderPath;
    await openAlert({
      title: tr("SteamGuard_Import_CompleteTitle"),
      body: `${imported.length === 1
        ? tr("SteamGuard_Import_CountOne")
        : tr("SteamGuard_Import_CountMany", { count: imported.length })}${failed ? tr("SteamGuard_Import_CountFailed", { failed }) : ""}.<br><br>` +
        `${accountSummary}` +
        `${tr("SteamGuard_BackupReminder")}<br><code>${escapeHtml(folder)}</code>`,
    });
  } finally {
    password = "";
    appPassword = "";
  }
}

type PickerSummary = { steamId64?: string; accountName?: string };
type PickerSwitcherAccount = {
  steamId64?: string;
  accountName?: string;
  displayName?: string;
  personaName?: string;
};
type PickerEnrichment = {
  steamId64?: string;
  imageUrl?: string;
  staticImageUrl?: string;
  vac?: boolean;
  ltd?: boolean;
  showVac?: boolean;
  showLimited?: boolean;
};

/**
 * Joins Steam Guard vault accounts to the switcher's account list and avatar enrichment
 * by steamId64. `locked` is derived from the real vault state: with a session unlock every
 * authenticator can produce a code, otherwise only the account whose capability is held.
 */
export function mergeSteamGuardAccountRows(source: {
  summaries: PickerSummary[];
  switcherAccounts?: PickerSwitcherAccount[];
  enrichment?: PickerEnrichment[];
  vaultUnlocked: boolean;
  activeAccountId: string;
}): SteamGuardAccountSummary[] {
  const byId = <T extends { steamId64?: string }>(rows: T[] | undefined): Map<string, T> =>
    new Map((rows ?? []).map((row) => [(row.steamId64 ?? "").trim(), row]));
  const switcher = byId(source.switcherAccounts);
  const enrichment = byId(source.enrichment);

  return source.summaries.map((summary) => {
    const steamId64 = (summary.steamId64 ?? "").trim();
    const username = (summary.accountName ?? "").trim()
      || switcher.get(steamId64)?.accountName?.trim()
      || steamId64;
    const matched = switcher.get(steamId64);
    const enriched = enrichment.get(steamId64);
    return {
      id: steamId64,
      username,
      displayName: matched?.displayName?.trim() || matched?.personaName?.trim() || username,
      locked: !source.vaultUnlocked && steamId64 !== source.activeAccountId,
      imageUrl: enriched?.imageUrl?.trim() || undefined,
      staticImageUrl: enriched?.staticImageUrl?.trim() || undefined,
      vac: enriched?.showVac === true && enriched.vac === true,
      limited: enriched?.showLimited === true && enriched.ltd === true,
    };
  });
}

/**
 * Switcher-side profile for one Steam Guard account: avatar plus display name, joined by
 * SteamID64. Unlike `listAccounts` this touches no vault capability, so screens that run while
 * the vault is locked (the unlock screen) can still show the account's avatar.
 */
export async function loadSteamGuardSwitcherProfile(
  steamId64: string,
  username: string,
): Promise<SteamGuardAccountSummary | null> {
  const [switcherAccounts, enrichment] = await Promise.all([
    SteamService.GetSteamAccountsList().catch((error: unknown) => {
      console.error("Steam Guard: switcher account list unavailable", error);
      return [];
    }),
    SteamService.GetSteamAccountsEnrichment().catch((error: unknown) => {
      console.error("Steam Guard: switcher avatar enrichment unavailable", error);
      return [];
    }),
  ]);
  const [row] = mergeSteamGuardAccountRows({
    summaries: [{ steamId64, accountName: username }],
    switcherAccounts,
    enrichment,
    vaultUnlocked: false,
    activeAccountId: steamId64,
  });
  return row ?? null;
}

async function showEnrollmentBackupWarning(): Promise<void> {
	const status = await SteamGuardService.GetSettingsStatus();
	await openAlert({
		title: tr("SteamGuard_Enrollment_AddedTitle"),
		body: `${tr("SteamGuard_BackupReminder")}<br><code>` +
			`${escapeHtml(status.folderPath)}</code>`,
	});
}

async function requestSensitiveView(accountId: string): Promise<{
  capability: string;
  lease: string;
  accountId: string;
}> {
  const requestId = crypto.randomUUID();
  return new Promise((resolve, reject) => {
    let settled = false;
    const finish = (error?: unknown, grant?: { capability: string; lease: string; accountId: string }): void => {
      if (settled) return;
      settled = true;
      clearTimeout(timeout);
      off();
      if (error) reject(error);
      else if (grant) resolve(grant);
      else reject(new Error("Steam Guard capability unavailable"));
    };
    const off = Events.On("steamguard:sensitive-view-grant", (event) => {
      const data = event.data as Record<string, unknown> | null;
      if (!data || data.requestId !== requestId || data.accountId !== accountId) return;
      if (typeof data.capability !== "string" || typeof data.lease !== "string") {
        finish(new Error("Invalid Steam Guard capability grant"));
        return;
      }
      finish(undefined, { capability: data.capability, lease: data.lease, accountId });
    });
    const timeout = setTimeout(() => finish(new Error("Steam Guard capability request timed out")), 5_000);
    void SteamGuardService.RequestSensitiveView(accountId, requestId).catch((error) => finish(error));
  });
}

async function requestConfirmationsGrant(): Promise<{ capability: string; accountId: string }> {
	const requestId = crypto.randomUUID();
	return new Promise((resolve, reject) => {
		let settled = false;
		const finish = (error?: unknown, grant?: { capability: string; accountId: string }): void => {
			if (settled) return;
			settled = true;
			clearTimeout(timeout);
			off();
			if (error) reject(error);
			else if (grant) resolve(grant);
			else reject(new Error("Confirmation capability unavailable"));
		};
		const off = Events.On("steamguard:confirmations-grant", (event) => {
			const data = event.data as Record<string, unknown> | null;
			if (!data || data.requestId !== requestId) return;
			if (typeof data.capability !== "string" || typeof data.accountId !== "string") {
				finish(new Error("Invalid confirmation capability grant"));
				return;
			}
			finish(undefined, { capability: data.capability, accountId: data.accountId });
		});
		const timeout = setTimeout(() => finish(new Error("Confirmation capability request timed out")), 5_000);
		void SteamGuardService.RequestConfirmationsCapability(requestId).catch((error) => finish(error));
	});
}

/**
 * Emitted by the confirmations window and handled by the main window: application-wide
 * Wails custom event carrying only a steamId64.
 */
const STEAM_GUARD_LOGIN_AGAIN_EVENT = "steamguard:login-again-request";

function installLoginAgainHandoff(): () => void {
	if (window.location.hash === "#/steam/confirmations") return () => {};
	return Events.On(STEAM_GUARD_LOGIN_AGAIN_EVENT, (event) => {
		const data = event.data as { accountId?: unknown } | null;
		const accountId = typeof data?.accountId === "string" ? data.accountId.trim() : "";
		if (!accountId) return;
		void (async () => {
			let username = accountId;
			try {
				const accounts = await SteamService.GetSteamAccountsList();
				const matched = accounts.find((row) => (row.steamId64 ?? "").trim() === accountId);
				username = matched?.accountName?.trim() || accountId;
			} catch (error) {
				console.error("Steam Guard login-again handoff: account name unavailable", error);
			}
			await openSteamGuardModal({
				account: { id: accountId, username },
				controller,
				entry: "login-again",
			});
		})();
	});
}

/**
 * Steam's description of one item. The same shape backs a hovered trade item and
 * the item a market listing is selling, so both come through here.
 */
function confirmationItem(item: SteamGuardModels.ConfirmationItemView): ConfirmationItem {
	return {
		name: item.name ?? "",
		marketHashName: item.marketHashName ?? "",
		type: item.type ?? "",
		nameColor: item.nameColor ?? "",
		icon: item.icon ?? "",
		tradable: item.tradable === true,
		marketable: item.marketable === true,
		descriptions: (item.descriptions ?? []).map((line) => ({
			value: line.value ?? "",
			color: line.color ?? "",
		})),
		tags: (item.tags ?? []).map((tag) => ({
			category: tag.category ?? "",
			name: tag.name ?? "",
			color: tag.color ?? "",
		})),
	};
}

/** Anything the backend does not name gets the modest layout, not the overlay. */
function confirmationKind(value: string | undefined): ConfirmationKind {
	return value === "trade" || value === "market" ? value : "other";
}

function confirmationFailure(state: string, retryAfterMs = 0, accountLabel = ""): ConfirmationRequestError {
	const kinds: Record<string, ConfirmationFailureKind> = {
		reauth: "reauth", "rate-limit": "rate-limit", offline: "offline", canceled: "canceled",
	};
	const kind = kinds[state] ?? "error";
	const messages: Record<ConfirmationFailureKind, string> = {
		error: tr("SteamGuard_Confirmations_Failure_Error"),
		reauth: tr("SteamGuard_Confirmations_Failure_Reauth"),
		"rate-limit": tr("SteamGuard_Confirmations_Failure_RateLimit"),
		offline: tr("SteamGuard_Confirmations_Failure_Offline"),
		canceled: tr("SteamGuard_Confirmations_Failure_Canceled"),
	};
	return new ConfirmationRequestError(kind, messages[kind], retryAfterMs, accountLabel);
}

function installConfirmationsBridge(): () => void {
	if (window.location.hash !== "#/steam/confirmations") return () => {};
	let capability = "";
	let disposed = false;

	const connect = async (): Promise<void> => {
		configureSteamConfirmationsWindow(null);
		if (capability) SteamGuardService.ReleaseConfirmationsCapability(capability);
		capability = "";
		try {
			const grant = await requestConfirmationsGrant();
			if (disposed) {
				SteamGuardService.ReleaseConfirmationsCapability(grant.capability);
				return;
			}
			capability = grant.capability;
			configureSteamConfirmationsWindow({
				accountId: grant.accountId,
				async fetch(_generation, signal) {
					const pending = SteamGuardService.ListConfirmations(capability);
					const cancel = (): void => {
						void pending.cancel();
						void SteamGuardService.CancelConfirmations(capability).catch((error: unknown) => {
							console.error("Steam Guard: confirmation refresh could not be canceled", error);
						});
					};
					if (signal.aborted) cancel();
					else signal.addEventListener("abort", cancel, { once: true });
					try {
						const result = await pending;
						if (result.state !== "fresh") {
						throw confirmationFailure(result.state, result.retryAfterMs ?? 0, result.accountLabel ?? "");
					}
						return {
							accountLabel: result.accountLabel ?? grant.accountId,
							fetchedAt: result.fetchedAt ?? Date.now(),
							rows: (result.rows ?? []).map((row) => ({
								handle: row.handle,
								kind: confirmationKind(row.kind),
								typeLabel: row.typeLabel,
								title: row.title,
								summary: row.summary,
								// Already a local path: the backend fetched and sanitized it,
								// because the webview's CSP refuses remote images.
								icon: row.icon ?? "",
								details: row.details ?? [],
								acceptLabel: row.acceptLabel,
								denyLabel: row.denyLabel,
							})),
						};
					} finally {
						signal.removeEventListener("abort", cancel);
					}
				},
				async decide(handle: string, decision: ConfirmationDecision) {
					const result = await SteamGuardService.DecideConfirmation(handle, decision, capability);
					if (result.state !== "ok") throw confirmationFailure(result.state, result.retryAfterMs ?? 0);
				},
				async decideMany(handles: string[], decision: ConfirmationDecision) {
					const result = await SteamGuardService.DecideConfirmations(handles, decision, capability);
					if (result.state !== "ok") throw confirmationFailure(result.state, result.retryAfterMs ?? 0);
				},
				async inspect(handle: string) {
					const result = await SteamGuardService.InspectConfirmation(handle, capability);
					if (result.state !== "ok") return { fields: [], trade: null, listing: null };
					const side = (raw: { header?: string; items?: unknown[] } | undefined) => ({
						header: raw?.header ?? "",
						items: ((raw?.items ?? []) as {
							appId?: string; classId?: string; instanceId?: string; icon?: string;
						}[]).map((item) => ({
							appId: item.appId ?? "",
							classId: item.classId ?? "",
							instanceId: item.instanceId ?? "",
							icon: item.icon ?? "",
						})),
					});
					const rawTrade = result.trade;
					const rawListing = result.listing;
					return {
						fields: (result.fields ?? []).map((field) => ({
							label: field.label,
							value: field.value,
						})),
						trade: rawTrade
							? {
								partner: rawTrade.partner
									? {
										name: rawTrade.partner.name ?? "",
										avatar: rawTrade.partner.avatar ?? "",
										profileUrl: rawTrade.partner.profileUrl ?? "",
										level: rawTrade.partner.level ?? 0,
										yearsBadge: rawTrade.partner.yearsBadge ?? "",
									}
									: null,
								give: side(rawTrade.give),
								receive: side(rawTrade.receive),
							}
							: null,
						listing: rawListing
							? {
								receive: rawListing.receive ?? "",
								buyerPays: rawListing.buyerPays ?? "",
								prices: (rawListing.prices ?? []).map((price) => ({
									label: price.label,
									value: price.value,
								})),
								market: {
									forSale: rawListing.market?.forSale ?? 0,
									forSalePrice: rawListing.market?.forSalePrice ?? "",
									soldRecently: rawListing.market?.soldRecently ?? 0,
									text: rawListing.market?.text ?? [],
								},
								item: rawListing.item ? confirmationItem(rawListing.item) : null,
							}
							: null,
					};
				},
				async inspectItem(appId: string, classId: string, instanceId: string) {
					const item = await SteamGuardService.InspectTradeItem(appId, classId, instanceId, capability);
					if (item.state !== "ok") return null;
					return confirmationItem(item);
				},
				async refreshSession() {
					const outcome = await SteamGuardService.RefreshConfirmationsSession(capability);
					if (!outcome.refreshed) {
						return { refreshed: false, needsCredentials: outcome.needsCredentials === true };
					}
					// The renewal wrote to the vault, so the generation moved and this
					// window's capability with it. Take a fresh one: every adapter method
					// closes over the variable, so replacing it here is enough.
					if (capability) SteamGuardService.ReleaseConfirmationsCapability(capability);
					capability = (await requestConfirmationsGrant()).capability;
					return { refreshed: true, needsCredentials: false };
				},
				async loginAgain() {
					// The confirmations window cannot host the Steam Guard modal, so ask the main
					// window to open it on the login-again path for this account.
					await Events.Emit(STEAM_GUARD_LOGIN_AGAIN_EVENT, { accountId: grant.accountId });
					throw new ConfirmationRequestError(
						"reauth",
						tr("SteamGuard_Confirmations_FinishSignIn"),
						0,
					);
				},
			});
		} catch (error) {
			configureSteamConfirmationsWindow({
				fetch: async () => { throw error; },
				decide: async () => { throw error; },
				loginAgain: async () => { throw error; },
			});
		}
	};

	const offContext = Events.On("steamguard:confirmations-context-changed", () => { void connect(); });
	void connect();
	return () => {
		disposed = true;
		offContext();
		configureSteamConfirmationsWindow(null);
		if (capability) SteamGuardService.ReleaseConfirmationsCapability(capability);
		capability = "";
	};
}

async function runVerifiedBackup(): Promise<void> {
  const status = await SteamGuardService.GetSettingsStatus();
  let steamGuardPassword = await openPrompt({
    title: tr("SteamGuard_Backup_VerifyTitle"),
    body: tr("SteamGuard_Backup_VerifyBody"),
    inputType: "password",
    positiveLabel: tr("SteamGuard_Backup_ChooseLocation"),
    negativeLabel: tr("SteamGuard_Cancel"),
  });
  if (!steamGuardPassword) return;

  let appPassword = "";
  if (usesSavedDataEncryption(status)) {
    appPassword = await openPrompt({
      title: tr("SteamGuard_AppPassword_VerifyTitle"),
      body: tr("SteamGuard_Backup_AppPasswordBody"),
      inputType: "password",
      positiveLabel: tr("SteamGuard_Continue"),
      negativeLabel: tr("SteamGuard_Cancel"),
    }) ?? "";
    if (!appPassword) {
      steamGuardPassword = "";
      return;
    }
  }

  try {
    const path = await SteamGuardService.CreateVerifiedBackup(steamGuardPassword, appPassword);
    if (path) {
      await openAlert({
        title: tr("SteamGuard_Backup_VerifiedTitle"),
        body: `${tr("SteamGuard_Backup_VerifiedBody")}<br><br>` +
          `${tr("SteamGuard_Backup_VerifiedPath")}<br><code>${escapeHtml(path)}</code>`,
      });
    }
  } finally {
    steamGuardPassword = "";
    appPassword = "";
  }
}

async function runRestore(): Promise<void> {
	const status = await SteamGuardService.GetSettingsStatus();
	if (status.vaultConfigured) {
		await openAlert({
			title: tr("SteamGuard_Restore_BlockedTitle"),
			body: tr("SteamGuard_Restore_BlockedBody"),
		});
		return;
	}
	let steamGuardPassword = await openPrompt({
		title: tr("SteamGuard_Restore_PasswordTitle"),
		body: tr("SteamGuard_Restore_PasswordBody"),
		inputType: "password",
		positiveLabel: tr("SteamGuard_Continue"),
		negativeLabel: tr("SteamGuard_Cancel"),
	});
	if (!steamGuardPassword) return;
	let backupAppPassword = await openPrompt({
		title: tr("SteamGuard_Restore_OuterTitle"),
		body: tr("SteamGuard_Restore_OuterBody"),
		inputType: "password",
		positiveLabel: tr("SteamGuard_Restore_ChooseFolder"),
		negativeLabel: tr("SteamGuard_Cancel"),
	});
	if (backupAppPassword === null) {
		steamGuardPassword = "";
		return;
	}
	let currentAppPassword = "";
	if (usesSavedDataEncryption(status)) {
		currentAppPassword = await openPrompt({
			title: tr("SteamGuard_Restore_CurrentAppTitle"),
			body: tr("SteamGuard_Restore_CurrentAppBody"),
			inputType: "password",
			positiveLabel: tr("SteamGuard_Restore_Confirm"),
			negativeLabel: tr("SteamGuard_Cancel"),
		}) ?? "";
		if (!currentAppPassword) {
			steamGuardPassword = "";
			backupAppPassword = "";
			return;
		}
	}
	try {
		const path = await SteamGuardService.RestoreVerifiedBackup(steamGuardPassword, backupAppPassword, currentAppPassword);
		if (path) {
			await openAlert({
				title: tr("SteamGuard_Restore_DoneTitle"),
				body: `${tr("SteamGuard_Restore_DoneBody")}<br><code>${escapeHtml(path)}</code><br><br>${tr("SteamGuard_Restore_DoneFollowUp")}`,
			});
		}
	} finally {
		steamGuardPassword = "";
		backupAppPassword = "";
		currentAppPassword = "";
	}
}

/**
 * Restores accounts from a backup into the configured vault. The backup is
 * staged and compared first; the user picks which accounts to bring across,
 * and a fresh verified backup of the current vault is written before anything
 * is replaced.
 */
async function runRestoreMerge(): Promise<void> {
	const status = await SteamGuardService.GetSettingsStatus();
	if (!status.vaultConfigured) return;
	let password = await openPrompt({
		title: tr("SteamGuard_Restore_PasswordTitle"),
		body: tr("SteamGuard_RestoreMerge_PasswordBody"),
		inputType: "password",
		positiveLabel: tr("SteamGuard_Restore_ChooseFolder"),
		negativeLabel: tr("SteamGuard_Cancel"),
	});
	if (!password) return;
	let backupPassword = "";
	let backupAppPassword = "";
	let currentAppPassword = "";
	let staged = false;
	try {
		if (usesSavedDataEncryption(status)) {
			backupAppPassword = await openPrompt({
				title: tr("SteamGuard_Restore_OuterTitle"),
				body: tr("SteamGuard_Restore_OuterBody"),
				inputType: "password",
				positiveLabel: tr("SteamGuard_Continue"),
				negativeLabel: tr("SteamGuard_Cancel"),
			}) ?? "";
			if (!backupAppPassword) return;
			currentAppPassword = await openPrompt({
				title: tr("SteamGuard_Restore_CurrentAppTitle"),
				body: tr("SteamGuard_Restore_CurrentAppBody"),
				inputType: "password",
				positiveLabel: tr("SteamGuard_Continue"),
				negativeLabel: tr("SteamGuard_Cancel"),
			}) ?? "";
			if (!currentAppPassword) return;
		}
		let plan = await SteamGuardService.PlanRestoreMerge(password, backupPassword, backupAppPassword);
		if (plan.state === "canceled") return;
		staged = true;
		// A backup made before a password change opens with its own password;
		// the stage survives the retry so the folder is not picked again.
		while (plan.state === "backup_password") {
			const entered = await openPrompt({
				title: tr("SteamGuard_RestoreMerge_BackupPasswordTitle"),
				body: tr("SteamGuard_RestoreMerge_BackupPasswordBody"),
				inputType: "password",
				positiveLabel: tr("SteamGuard_Continue"),
				negativeLabel: tr("SteamGuard_Cancel"),
			});
			if (!entered) return;
			backupPassword = entered;
			plan = await SteamGuardService.PlanRestoreMerge(password, backupPassword, backupAppPassword);
		}
		if (plan.state !== "ok") return;
		const accounts = (plan.accounts ?? []).map((account) => ({
			steamId64: account.steamId64,
			accountName: account.accountName,
			exists: account.exists,
			backupTokenExpiry: account.backupTokenExpiry ?? 0,
			currentTokenExpiry: account.currentTokenExpiry ?? 0,
		}));
		if (accounts.length === 0) {
			await openAlert({
				title: tr("SteamGuard_RestoreMerge_ChooseTitle"),
				body: tr("SteamGuard_RestoreMerge_Empty"),
			});
			return;
		}
		const selected = await new Promise<string[] | null>((resolve) => {
			void openAlertNoButton({
				title: tr("SteamGuard_RestoreMerge_ChooseTitle"),
				bodyComponent: SteamGuardRestoreModalBody,
				bodyProps: { accounts, onDone: resolve },
			});
		});
		if (!selected || selected.length === 0) return;
		const result = await SteamGuardService.CommitRestoreMerge(
			password, backupPassword, backupAppPassword, currentAppPassword, selected,
		);
		staged = false;
		await openAlert({
			title: tr("SteamGuard_RestoreMerge_DoneTitle"),
			body: `${tr("SteamGuard_RestoreMerge_DoneBody", { added: result.added, replaced: result.replaced })}` +
				`<br><code>${escapeHtml(result.safetyBackupPath)}</code>`,
		});
	} finally {
		if (staged) {
			void SteamGuardService.CancelRestoreMerge().catch((error: unknown) => {
				console.error("Steam Guard: restore stage could not be discarded", error);
			});
		}
		password = "";
		backupPassword = "";
		backupAppPassword = "";
		currentAppPassword = "";
	}
}

async function runSteamGuardPasswordChange(currentPassword: string, newPassword: string): Promise<void> {
  const status = await SteamGuardService.GetSettingsStatus();
  let appPassword = "";
  if (usesSavedDataEncryption(status)) {
    appPassword = await openPrompt({
      title: tr("SteamGuard_AppPassword_VerifyTitle"),
      body: tr("SteamGuard_PasswordChange_AppPasswordBody"),
      inputType: "password",
      positiveLabel: tr("SteamGuard_PasswordChange_Confirm"),
      negativeLabel: tr("SteamGuard_Cancel"),
    }) ?? "";
    if (!appPassword) return;
  }
  try {
    await SteamGuardService.ChangePassword(currentPassword, newPassword, appPassword);
    const updated = await SteamGuardService.GetSettingsStatus();
    await openAlert({
      title: tr("SteamGuard_PasswordChange_DoneTitle"),
      body: `${tr("SteamGuard_PasswordChange_DoneBody")}<br><br>` +
        `${tr("SteamGuard_PasswordChange_FolderLabel")}<br><code>${escapeHtml(updated.folderPath)}</code>`,
    });
  } finally {
    appPassword = "";
  }
}

function escapeHtml(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

const controller: SteamGuardModalController = {
	requestSensitiveView,
	endSensitiveView: (capability, lease) => SteamGuardService.EndSensitiveView(capability, lease),
	async getCode(accountId, capability) {
		const view = await SteamGuardService.GetCode(accountId, capability);
		return view ? codeView(view) : null;
	},
	async unlock(accountId, password, rememberForSession, capability) {
		return codeView(await SteamGuardService.UnlockAccount(accountId, password, rememberForSession, capability));
	},
	copyCode: (accountId, capability) => SteamGuardService.CopyCode(accountId, capability),
	openConfirmations: (accountId, capability) => SteamGuardService.OpenConfirmations(accountId, capability),
	loginAgain: (accountId, capability) => SteamGuardService.LoginAgain(accountId, capability).then(loginResult),
	beginCredentialLogin: (accountId, capability, accountName, password, purpose) =>
		SteamGuardService.BeginCredentialLogin(accountId, capability, accountName, password, authPurpose(purpose)).then(credentialResult),
	submitCredentialCode: (accountId, capability, handle, challenge, code) =>
		SteamGuardService.SubmitCredentialCode(accountId, capability, handle, challenge, code).then(credentialResult),
	pollCredentialLogin: (accountId, capability, handle) =>
		SteamGuardService.PollCredentialLogin(accountId, capability, handle).then(credentialResult),
	cancelCredentialLogin: (accountId, capability, handle) =>
		SteamGuardService.CancelCredentialLogin(accountId, capability, handle),
	steamSessionLocalState: (accountId, capability) =>
		SteamGuardService.SteamSessionLocalState(accountId, capability)
			.then((state) => ({ needsLogin: state.needsLogin === true, reason: state.reason })),
	probeSteamSession: (accountId, capability) =>
		SteamGuardService.ProbeSteamSession(accountId, capability)
			.then((state) => ({ needsLogin: state.needsLogin === true, reason: state.reason })),
	async getSteamGuardVaultStatus() {
		const status = await SteamGuardService.GetSettingsStatus();
		return {
			configured: status.vaultConfigured,
			unlocked: status.unlocked,
			rememberForSession: status.rememberPasswordForSession,
			savedAccountDataEncrypted: usesSavedDataEncryption(status),
		};
	},
	async initializeSteamGuardVault(password, appPassword) {
		await SteamGuardService.Initialize(password, appPassword);
		await SteamGuardService.SetFeatureEnabled(true);
	},
	unlockSteamGuardVault: (accountId, password, rememberForSession, capability) =>
		SteamGuardService.UnlockSteamGuardVault(accountId, password, rememberForSession, capability),
	resumeSteamGuardEnrollment: (accountId, capability) =>
		SteamGuardService.ResumeSteamGuardEnrollment(accountId, capability).then(enrollmentStatus),
	revealSteamGuardRevocationCode: (accountId, capability) =>
		SteamGuardService.RevealSteamGuardRevocationCode(accountId, capability),
	acknowledgeSteamGuardRevocationCode: (accountId, capability, code) =>
		SteamGuardService.AcknowledgeSteamGuardRevocationCode(accountId, capability, code)
			.then(enrollmentStatus),
		async finalizeSteamGuardEnrollment(accountId, capability, confirmationCode) {
			const status = enrollmentStatus(
				await SteamGuardService.FinalizeSteamGuardEnrollment(accountId, capability, confirmationCode),
			);
			try {
				publishSteamGuardActionAccounts(await SteamService.GetSteamAccountsList());
			} catch {
				// The Steam page will republish availability on its next account load.
			}
			return status;
		},
	showEnrollmentBackupWarning,
	cancelSteamGuardEnrollment: (accountId, capability) =>
		SteamGuardService.CancelSteamGuardEnrollment(accountId, capability),
	async listAccounts(accountId, capability) {
		const summaries = await SteamGuardService.ListAccounts(accountId, capability);
		const [switcherAccounts, enrichment, status] = await Promise.all([
			SteamService.GetSteamAccountsList().catch((error: unknown) => {
				console.error("Steam Guard picker: switcher account list unavailable", error);
				return [];
			}),
			SteamService.GetSteamAccountsEnrichment().catch((error: unknown) => {
				console.error("Steam Guard picker: switcher avatar enrichment unavailable", error);
				return [];
			}),
			SteamGuardService.GetSettingsStatus(),
		]);
		return mergeSteamGuardAccountRows({
			summaries,
			switcherAccounts,
			enrichment,
			vaultUnlocked: status.unlocked,
			activeAccountId: accountId,
		});
  },
  async importMaFile() {
    await runImport();
  },
	exportMaFile: (accountId, capability, password, maFilePassword) =>
		SteamGuardService.ExportMaFile(accountId, password, maFilePassword, false, capability)
			.then((result) => ({
				path: result.path ?? "",
				manifestSkipped: result.manifestSkipped === true,
			})),
	async captureQrFromSteam(accountId, capability) {
		return qrScanResult(await SteamGuardService.ScanSteamQR(accountId, capability));
	},
	async chooseQrScreenshot(accountId, capability) {
		const path = await SteamGuardService.PickQRScreenshot();
		if (!path) return null;
		return qrScanResult(await SteamGuardService.DecodeQRScreenshot(accountId, path, capability));
	},
	async decodeQrScreenshot(accountId, path, capability) {
		return qrScanResult(await SteamGuardService.DecodeQRScreenshot(accountId, path, capability));
	},
	async getQrApproval(accountId, attempt, capability) {
		return SteamGuardService.GetQRApproval(accountId, attempt, capability);
	},
	authorizeQrLogin: (accountId, attempt, capability) =>
		SteamGuardService.AuthorizeQRLogin(accountId, attempt, capability),
		dismissQrLogin: (accountId, attempt, capability) =>
			SteamGuardService.DismissQRLogin(accountId, attempt, capability),
		async selectQrRegion(accountId, capability) {
			return qrScanResult(await SteamGuardService.SelectQRRegion(accountId, capability));
		},
		cancelQrRegion: (accountId, capability) =>
			SteamGuardService.CancelQRRegion(accountId, capability),
		recover: async () => runRestore(),
};

export function installSteamGuardBridge(): () => void {
	const uninstallConfirmations = installConfirmationsBridge();
	const uninstallLoginAgainHandoff = installLoginAgainHandoff();
  configureSteamGuardSettingsAdapter({
    getSettingsStatus: () => SteamGuardService.GetSettingsStatus(),
    setRememberPasswordForSession: (enabled) => SteamGuardService.SetRememberPasswordForSession(enabled),
    changePassword: runSteamGuardPasswordChange,
    lockNow: () => SteamGuardService.LockNow(),
    openFolder: () => SteamGuardService.OpenFolder(),
    createVerifiedBackup: runVerifiedBackup,
    restoreFromBackup: runRestoreMerge,
  });

  configureSteamGuardDropAdapter({
    importMaFiles: (paths) => runImport(paths),
    decodeQrScreenshot: async () => {
      throw new Error("Steam QR decoding is not available in this checkpoint");
    },
    reportError() {
      pushToast({ type: "error", message: tr("SteamGuard_Error_DroppedFile"), duration: 8000 });
    },
  });

  const unbindMenu = bindSteamGuardMenuToModal(controller);
  return () => {
		uninstallConfirmations();
		uninstallLoginAgainHandoff();
    unbindMenu();
    configureSteamGuardDropAdapter(null);
    configureSteamGuardSettingsAdapter(null);
  };
}
