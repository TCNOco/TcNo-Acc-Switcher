<script lang="ts">
  import { onDestroy, onMount, tick } from "svelte";
  import { Events } from "@wailsio/runtime";
  import {
	    initialSteamGuardModalState,
	    acknowledgeSteamRevocationThenRefresh,
		closeSteamGuardEnrollment,
    reduceSteamGuardModal,
    SteamGuardCapabilityError,
    SteamGuardContentProtectionLease,
	    steamGuardAccountForState,
	    steamGuardCodeCanAutoRefresh,
		    steamGuardCodeProgress,
			steamGuardQRFailureMessage,
			steamCredentialStep,
			steamEnrollmentStep,
			steamLoginAgainNextStep,
	    type SteamGuardAccountRef,
	    type SteamGuardAccountSummary,
	    type SteamGuardCodeView,
	    type SteamGuardModalAction,
    type SteamGuardModalController,
    type SteamGuardModalEntry,
    type SteamGuardModalState,
		type SteamGuardQRApproval,
			type SteamGuardQRScanResult,
			type SteamAuthPurpose,
			type SteamCredentialResult,
			type SteamEnrollmentStatus,
			type SteamGuardVaultStatus,
			type SteamMaFileExportResult,
  } from "../../lib/steamGuardModal";
	  import { t } from "../../stores/i18n";
	  import { setSteamGuardDropTarget } from "../../stores/steamGuardDrop";
	  import { dismissModal } from "../../stores/modal";
	  import { passwordPolicyMessage, validateNewPassword } from "../../lib/passwordPolicy";
	  import { pushToast } from "../../stores/toast";
	  import { formatToastWithError, formatUnknownError } from "../../lib/formatWailsError";
	  import { requestModalAutoFit } from "../../lib/modalFrame";
	  import { controllerSpatialNavigation } from "../../lib/actions/controllerSpatialNavigation";
	  import SteamAccountAvatar from "../SteamAccountAvatar.svelte";
	  import { loadSteamGuardSwitcherProfile } from "../../lib/steamGuardBridge";
	  import type { SteamAccountRow } from "../../lib/steam/types";

	/** Font Awesome Free v5.15.4 solid glyphs, inlined like the rest of the app's icons. */
	const ICONS = {
		shield: { box: "0 0 512 512", path: "M466.5 83.7l-192-80a48.15 48.15 0 0 0-36.9 0l-192 80C27.7 91.1 16 108.6 16 128c0 198.5 114.5 335.7 221.5 380.3 11.8 4.9 25.1 4.9 36.9 0C360.1 472.6 496 349.3 496 128c0-19.4-11.7-36.9-29.5-44.3z" },
		key: { box: "0 0 512 512", path: "M512 176.001C512 273.203 433.202 352 336 352c-11.22 0-22.19-1.062-32.827-3.069l-24.012 27.014A23.999 23.999 0 0 1 261.223 384H224v40c0 13.255-10.745 24-24 24h-40v40c0 13.255-10.745 24-24 24H24c-13.255 0-24-10.745-24-24v-78.059c0-6.365 2.529-12.47 7.029-16.971l161.802-161.802C163.108 213.814 160 195.271 160 176 160 78.798 238.797.001 335.999 0 433.488-.001 512 78.511 512 176.001zM336 128c0 26.51 21.49 48 48 48s48-21.49 48-48-21.49-48-48-48-48 21.49-48 48z" },
		qrcode: { box: "0 0 448 512", path: "M0 224h192V32H0v192zM64 96h64v64H64V96zm192-64v192h192V32H256zm128 128h-64V96h64v64zM0 480h192V288H0v192zm64-128h64v64H64v-64zm352-64h32v128h-96v-32h-32v96h-64V288h96v32h64v-32zm0 160h32v32h-32v-32zm-64 0h32v32h-32v-32z" },
		fileExport: { box: "0 0 576 512", path: "M384 121.941V128H256V0h6.059c6.365 0 12.47 2.529 16.971 7.029l97.941 97.941A24.005 24.005 0 0 1 384 121.941zM248 160c-13.2 0-24-10.8-24-24V0H24C10.745 0 0 10.745 0 24v464c0 13.255 10.745 24 24 24h336c13.255 0 24-10.745 24-24V160H248zm189.75 42.938l-96 96c-15.121 15.12-40.971 4.393-40.971-16.971V240h-64v-64h64v-41.967c0-21.346 25.833-32.104 40.971-16.971l96 96c9.373 9.373 9.373 24.569 0 33.938z" },
		image: { box: "0 0 512 512", path: "M464 448H48c-26.51 0-48-21.49-48-48V112c0-26.51 21.49-48 48-48h416c26.51 0 48 21.49 48 48v288c0 26.51-21.49 48-48 48zM112 120c-30.928 0-56 25.072-56 56s25.072 56 56 56 56-25.072 56-56-25.072-56-56-56zM64 384h384V272l-87.515-87.515c-4.686-4.686-12.284-4.686-16.971 0L208 320l-55.515-55.515c-4.686-4.686-12.284-4.686-16.971 0L64 336v48z" },
		crop: { box: "0 0 512 512", path: "M488 352h-40V96c0-17.67-14.33-32-32-32H192v64h192v256h-32V128H160V0H96v64H24C10.745 64 0 74.745 0 88v48c0 13.255 10.745 24 24 24h72v264c0 13.255 10.745 24 24 24h232v64h64v-64h72c13.255 0 24-10.745 24-24v-48c0-13.255-10.745-24-24-24z" },
		list: { box: "0 0 512 512", path: "M48 48a48 48 0 1 0 0 96 48 48 0 1 0 0-96zm448 32H176c-8.8 0-16-7.2-16-16V48c0-8.8 7.2-16 16-16h320c8.8 0 16 7.2 16 16v16c0 8.8-7.2 16-16 16zM48 208a48 48 0 1 0 0 96 48 48 0 1 0 0-96zm448 32H176c-8.8 0-16-7.2-16-16v-16c0-8.8 7.2-16 16-16h320c8.8 0 16 7.2 16 16v16c0 8.8-7.2 16-16 16zM48 368a48 48 0 1 0 0 96 48 48 0 1 0 0-96zm448 32H176c-8.8 0-16-7.2-16-16v-16c0-8.8 7.2-16 16-16h320c8.8 0 16 7.2 16 16v16c0 8.8-7.2 16-16 16z" },
		back: { box: "0 0 448 512", path: "M257.5 445.1l-22.2 22.2c-9.4 9.4-24.6 9.4-33.9 0L7 273c-9.4-9.4-9.4-24.6 0-33.9L201.4 44.7c9.4-9.4 24.6-9.4 33.9 0l22.2 22.2c9.5 9.5 9.3 25-.4 34.3L136.6 216H424c13.3 0 24 10.7 24 24v32c0 13.3-10.7 24-24 24H136.6l120.5 114.8c9.8 9.3 10 24.8.4 34.3z" },
	} as const;

  export let account: SteamGuardAccountRef;
  export let entry: SteamGuardModalEntry = "account";
  export let knownAccounts: SteamGuardAccountSummary[] = [];
  export let controller: SteamGuardModalController;

  let state: SteamGuardModalState = initialSteamGuardModalState(account, entry);
  const contentProtection = new SteamGuardContentProtectionLease(controller);
  let bodyEl: HTMLDivElement | undefined;
  let password = "";
  let rememberForSession = false;
  let openAllAccountsOnReady = entry === "all-accounts";
  /**
   * The account to return to when leaving the picker, or null when the picker was
   * the destination itself — opened from the action bar, nothing sits behind it, so
   * leaving means closing rather than dropping into an account never chosen.
   */
  let pickerReturnAccount: SteamGuardAccountRef | null = null;
  let busy = false;
  let refreshing = false;
  let inlineError = "";
  let statusMessage = "";
  let statusTimer: ReturnType<typeof setTimeout> | undefined;
  let now = Date.now();
  let clockTimer: ReturnType<typeof setInterval> | undefined;
  let offRevoked: (() => void) | undefined;
	let qrStage: "idle" | "scanning" | "approval" | "authorizing" | "authorized" | "error" = "idle";
	let qrMessage = "";
	let qrAttempt = "";
	let qrApproval: SteamGuardQRApproval | null = null;
	let qrRegionSelecting = false;
	let qrNeedsLogin = false;
	let authPurpose: SteamAuthPurpose = "login_again";
	let authStage: "idle" | "refreshing" | "credentials" | "challenge" | "polling" | "success" | "error" = "idle";
	let authAccountName = account.username;
	let authPassword = "";
	let authCode = "";
	let authHandle = "";
	let authChallenge = "";
	let authMessage = "";
	let authResult: SteamCredentialResult | null = null;
	/** One stored-code attempt per sign-in: a code Steam rejected must not be resubmitted. */
	let storedDeviceCodeTried = false;
	let lastPageKey = "";
	/** Highlights Login Again when the stored Steam session will not work. */
	let sessionNeedsLogin = false;
	let sessionCheckedAccount = "";
	let authPollTimer: ReturnType<typeof setTimeout> | undefined;
	/** Seconds the success screen shows before returning to the account's code. */
	const LOGIN_SUCCESS_SECONDS = 3;
	let successCountdown = 0;
	let successTimer: ReturnType<typeof setInterval> | undefined;
	let enrollmentStatus: SteamEnrollmentStatus | null = null;
	let enrollmentRetryAt = 0;
	let enrollmentStage: "idle" | "checking" | "vault" | "recovery" | "confirmation" | "complete" | "error" = "idle";
	let vaultStatus: SteamGuardVaultStatus | null = null;
	let vaultPassword = "";
	let vaultPasswordConfirmation = "";
	let vaultAppPassword = "";
	let revocationCode = "";
	let revocationConfirmation = "";
	let confirmationCode = "";
	let exportPassword = "";
	/** Optional: encrypts the exported maFile the way SDA does. Empty exports plaintext. */
	let exportMaFilePassword = "";
	let exportError = "";
	/** Last loaded picker rows, reused for the avatar/display name on single-account screens. */
	let knownSummaries: SteamGuardAccountSummary[] = knownAccounts;
	/**
	 * Switcher-side profiles keyed by SteamID64. The unlock screen runs while the vault is still
	 * locked, so `listAccounts` is unavailable there and the avatar comes from the switcher's own
	 * account list plus avatar enrichment instead.
	 */
	let switcherProfiles: Record<string, SteamGuardAccountSummary> = {};

	const PROFILE_FALLBACK = "/img/BasicDefault.webp";

	/** Minimal row for the shared avatar component; the modal never shows mini-profiles. */
	function avatarRow(summary: SteamGuardAccountSummary): SteamAccountRow {
		return {
			steamId64: summary.id,
			imageUrl: summary.imageUrl ?? "",
			staticImageUrl: summary.staticImageUrl ?? "",
			avatarPending: false,
			vac: summary.vac === true,
			ltd: summary.limited === true,
			showVac: summary.vac === true,
			showLimited: summary.limited === true,
			showMiniProfile: false,
			showAvatarFrame: false,
		} as unknown as SteamAccountRow;
	}

	function hasAvatar(summary: SteamGuardAccountSummary | undefined): boolean {
		return !!(summary?.imageUrl?.trim() || summary?.staticImageUrl?.trim());
	}

	/**
	 * Avatar/display-name source for a single-account screen. The first candidate that
	 * actually carries an avatar wins, so a picker row or opener-supplied image is never
	 * shadowed by an avatar-less one; otherwise the usual precedence applies.
	 */
	function summaryOf(
		target: SteamGuardAccountRef,
		locked: boolean,
		summaries: SteamGuardAccountSummary[],
		profiles: Record<string, SteamGuardAccountSummary>,
	): SteamGuardAccountSummary {
		const candidates = [
			summaries.find((summary) => summary.id === target.id),
			profiles[target.id],
			{ ...target, locked },
		];
		return candidates.find(hasAvatar) ?? candidates.find(Boolean) ?? { ...target, locked };
	}

	/**
	 * Best-effort avatar/display-name lookup for openers that supplied no avatar. An
	 * avatar-less result is not cached, so a later attempt can still succeed once the
	 * switcher's enrichment is available.
	 */
	async function ensureSwitcherProfile(target: SteamGuardAccountRef): Promise<void> {
		if (!target.id || hasAvatar(switcherProfiles[target.id])) return;
		if (target.imageUrl?.trim() || target.staticImageUrl?.trim()) return;
		if (knownSummaries.some((summary) => hasAvatar(summary) && summary.id === target.id)) return;
		try {
			const profile = await loadSteamGuardSwitcherProfile(target.id, target.username);
			if (!profile) return;
			if (!hasAvatar(profile)) {
				console.warn("Steam Guard: no switcher avatar for this account", target.id);
				return;
			}
			switcherProfiles = { ...switcherProfiles, [target.id]: profile };
		} catch (error) {
			console.error("Steam Guard: account profile could not be loaded", error);
		}
	}

	/**
	 * Generic line plus the reason Steam gave. Without the reason a dead end (wrong
	 * password, throttled account) is indistinguishable from a transient failure, so
	 * the only affordance left is retrying something that cannot succeed.
	 */
	function withFailureReason(message: string, error: unknown): string {
		const reason = formatUnknownError(error).split("\n")[0]?.trim() ?? "";
		return reason ? `${message} ${reason}` : message;
	}

	function reportFailure(prefix: string, error: unknown): void {
		console.error(prefix, error);
		pushToast({ type: "error", message: formatToastWithError(prefix, error), duration: 8_000 });
	}

	function reportSuccess(message: string): void {
		pushToast({ type: "success", message });
		announce(message);
	}

	/**
	 * Acquires the sensitive-view capability on demand. Every vault write rotates the
	 * generation, so a capability held from an earlier step can already be invalid.
	 */
	async function ensureCapability(currentAccount: SteamGuardAccountRef): Promise<string> {
		if (!contentProtection.capabilityFor(currentAccount.id)) {
			await contentProtection.acquire(currentAccount.id);
		}
		const capability = contentProtection.capabilityFor(currentAccount.id);
		if (!capability) throw new SteamGuardCapabilityError();
		return capability;
	}

  $: codeProgress = state.screen === "account-code"
    ? steamGuardCodeProgress(state.view.expiresAt, now)
    : 0;
	$: remainingSeconds = state.screen === "account-code"
    ? Math.max(0, Math.ceil((state.view.expiresAt - now) / 1_000))
	    : 0;
	$: oneOperationCode = state.screen === "account-code" && state.view.unlockPersistence === "one_operation";
	$: enrollmentRetrySeconds = Math.max(0, Math.ceil((enrollmentRetryAt - now) / 1_000));
	$: exportAccount = steamGuardAccountForState(state) ?? account;
	$: exportAccountSummary = summaryOf(exportAccount, false, knownSummaries, switcherProfiles);
	// The unlock screen names one account only when that is the account being
	// unlocked. Narrowed inline on each use: state is a union keyed on screen.
	$: lockedAccountSummary = state.screen === "locked" && !openAllAccountsOnReady
		? summaryOf(state.account, true, knownSummaries, switcherProfiles)
		: null;
	$: if (state.screen === "locked" && !openAllAccountsOnReady) void ensureSwitcherProfile(state.account);
	// The code screen shows the same identity as every other screen.
	$: codeAccountSummary = state.screen === "account-code"
		? summaryOf(state.view.account, false, knownSummaries, switcherProfiles)
		: { ...account, locked: false };
	$: if (state.screen === "account-code") void ensureSwitcherProfile(state.view.account);
	// The export screen shows the same identity as everywhere else, so it needs the
	// same switcher lookup behind it.
	$: if (state.screen === "export-authorize") void ensureSwitcherProfile(exportAccount);
	// Every screen holds different content, so each one gets its own fit even if the
	// user resized an earlier screen. The sub-stages count: a credential form and a
	// polling spinner are as different in height as two screens are.
	$: currentPageKey = `${state.screen}:${authStage}:${enrollmentStage}`;
	$: if (currentPageKey !== lastPageKey) {
		lastPageKey = currentPageKey;
		requestModalAutoFit();
	}
	$: if (state.screen === "account-code") void checkSessionNeedsLogin(state.view.account);
	$: setSteamGuardDropTarget(
		state.screen === "qr" ? "qr" : "none",
		state.screen === "qr" ? decodeDroppedQRScreenshot : null,
	);

  function focusCurrentScreen(): void {
    void tick().then(() =>
      requestAnimationFrame(() => {
	        bodyEl?.querySelector<HTMLElement>("[data-steamguard-autofocus], [data-steamguard-focus]")?.focus();
      }),
    );
  }

  function transition(action: SteamGuardModalAction, moveFocus = true): void {
    state = reduceSteamGuardModal(state, action);
    inlineError = "";
    if (moveFocus) focusCurrentScreen();
  }

  function announce(message: string): void {
    statusMessage = message;
    if (statusTimer) clearTimeout(statusTimer);
    statusTimer = setTimeout(() => {
      statusMessage = "";
    }, 2_500);
  }

  function runOnEnter(event: KeyboardEvent, action: () => Promise<void>): void {
    if (event.key !== "Enter" || event.isComposing) return;
    event.preventDefault();
    void action();
  }

  async function accountSummaries(currentAccount: SteamGuardAccountRef): Promise<SteamGuardAccountSummary[]> {
    const capability = await ensureCapability(currentAccount);
    const summaries = controller.listAccounts
      ? await controller.listAccounts(currentAccount.id, capability)
      : knownAccounts;
    knownSummaries = summaries;
    return summaries;
  }

  async function showReadyAccount(view: SteamGuardCodeView): Promise<void> {
    if (!openAllAccountsOnReady || view.unlockPersistence === "one_operation") {
      openAllAccountsOnReady = false;
      transition({ type: "show-code", view });
      return;
    }

    openAllAccountsOnReady = false;
    // Reached by unlocking, not from an account: there is nothing behind it.
    pickerReturnAccount = null;
    try {
      transition({ type: "show-all", accounts: await accountSummaries(view.account) });
    } catch (error) {
      console.error("Steam Guard: account list could not be loaded", error);
      transition({
        type: "fail",
        account: view.account,
        message: $t("SteamGuard_Error_AccountsNotLoaded"),
      });
    }
  }

  async function loadAccount(nextAccount: SteamGuardAccountRef): Promise<void> {
    if (busy) return;
    transition({ type: "load-account", account: nextAccount });
    busy = true;
    try {
		await contentProtection.acquire(nextAccount.id);
		const capability = await ensureCapability(nextAccount);
		const view = await controller.getCode(nextAccount.id, capability);
      if (view) {
        await showReadyAccount(view);
      } else {
        transition({ type: "lock-account", account: nextAccount });
      }
    } catch (error) {
      console.error("Steam Guard: account could not be loaded", error);
      transition({
        type: "fail",
        account: nextAccount,
        message: $t("SteamGuard_Error_AccountNotLoaded"),
      });
    } finally {
      busy = false;
    }
  }

  async function unlockAccount(): Promise<void> {
    if (busy || state.screen !== "locked" || password.length === 0) return;
    const lockedAccount = state.account;
    busy = true;
    inlineError = "";

    let pending: Promise<import("../../lib/steamGuardModal").SteamGuardCodeView>;
    try {
		const capability = await ensureCapability(lockedAccount);
		pending = controller.unlock(lockedAccount.id, password, rememberForSession, capability);
    } catch (error) {
      console.error("Steam Guard: unlock could not start", error);
      password = "";
      busy = false;
      inlineError = $t("SteamGuard_Error_UnlockStartFailed");
      focusCurrentScreen();
      return;
    }
    password = "";

    try {
      await showReadyAccount(await pending);
    } catch (error) {
      console.error("Steam Guard: unlock was rejected", error);
      inlineError = $t("SteamGuard_Error_PasswordRejected");
      focusCurrentScreen();
    } finally {
      busy = false;
    }
  }

  async function refreshExpiredCode(): Promise<void> {
    if (refreshing || state.screen !== "account-code") return;
    const currentAccount = state.view.account;
    refreshing = true;
    try {
		const capability = await ensureCapability(currentAccount);
		const view = await controller.getCode(currentAccount.id, capability);
      if (view) {
        transition({ type: "show-code", view }, false);
        announce($t("SteamGuard_Announce_NewCodeReady"));
      } else {
        transition({ type: "lock-account", account: currentAccount });
      }
    } catch (error) {
      console.error("Steam Guard: next code could not be generated", error);
      transition({
        type: "fail",
        account: currentAccount,
        message: $t("SteamGuard_Error_NextCodeFailed"),
      });
    } finally {
      refreshing = false;
    }
  }

  async function copyCurrentCode(): Promise<void> {
    if (state.screen !== "account-code" || state.view.unlockPersistence === "one_operation") return;
    try {
      if (!controller.copyCode) throw new Error("Secure clipboard unavailable");
		const capability = await ensureCapability(state.view.account);
		await controller.copyCode(state.view.account.id, capability);
      reportSuccess($t("SteamGuard_Code_Copied"));
    } catch (error) {
      reportFailure($t("SteamGuard_Error_CodeCopyFailed"), error);
    }
  }

	function unlockForAnotherOperation(): void {
		if (state.screen !== "account-code" || state.view.unlockPersistence !== "one_operation") return;
		const currentAccount = state.view.account;
		transition({ type: "lock-account", account: currentAccount });
		announce($t("SteamGuard_Announce_UnlockToContinue"));
	}

  async function showAllAccounts(): Promise<void> {
    if (busy) return;
    busy = true;
    try {
      const currentAccount = steamGuardAccountForState(state) ?? account;
      // Opened from an account, so leaving the picker goes back to that account.
      pickerReturnAccount = currentAccount;
      transition({ type: "show-all", accounts: await accountSummaries(currentAccount) });
    } catch (error) {
      console.error("Steam Guard: account list could not be loaded", error);
      transition({ type: "fail", message: $t("SteamGuard_Error_AccountsNotLoaded") });
    } finally {
      busy = false;
    }
  }

  /** Leaves the account picker the way the user entered it. */
  function leaveAllAccounts(): void {
    if (pickerReturnAccount) {
      void loadAccount(pickerReturnAccount);
      return;
    }
    dismissModal();
  }

  async function showConfirmations(): Promise<void> {
		if (state.screen !== "account-code" || !controller.openConfirmations) return;
		const currentAccount = state.view.account;
		try {
			const capability = await ensureCapability(currentAccount);
			await controller.openConfirmations(currentAccount.id, capability);
    } catch (error) {
      reportFailure($t("SteamGuard_Error_ConfirmationsOpenFailed"), error);
    }
  }

  async function runPlaceholderAction(
    action: (() => Promise<void>) | undefined,
    successMessage: string,
  ): Promise<void> {
    if (!action || busy) return;
    busy = true;
    try {
      await action();
      announce(successMessage);
    } catch (error) {
      console.error("Steam Guard: action failed", error);
      const currentAccount = steamGuardAccountForState(state);
      transition({ type: "fail", account: currentAccount, message: $t("SteamGuard_Error_ActionFailed") });
    } finally {
      busy = false;
    }
  }

  function backToAccount(): void {
    const currentAccount = steamGuardAccountForState(state) ?? account;
    void loadAccount(currentAccount);
  }

	/** One click: refresh the saved session, and only ask for a password if Steam rejects it. */
	function showLoginAgainState(): void {
		if (state.screen !== "account-code") return;
		transition({ type: "show-login-again", account: state.view.account }, false);
		void startLoginAgain();
	}

	function showExportState(): void {
		if (state.screen !== "account-code") return;
		const currentAccount = state.view.account;
		exportPassword = "";
		exportError = "";
		transition({ type: "show-export-authorize", account: currentAccount });
		if (!knownSummaries.some((summary) => summary.id === currentAccount.id)) {
			void accountSummaries(currentAccount).catch((error: unknown) => {
				console.error("Steam Guard: export avatar could not be loaded", error);
			});
		}
	}

  function startImport(): void {
    if (state.screen !== "import") return;
    const action = controller.importMaFile;
    const accountId = state.account?.id;
    void runPlaceholderAction(action ? () => action(accountId) : undefined, $t("SteamGuard_Import_Started"));
  }

	function capabilityFor(currentAccount: SteamGuardAccountRef): string {
		return contentProtection.capabilityFor(currentAccount.id);
	}

	async function refreshCapabilityIfRequired(
		currentAccount: SteamGuardAccountRef,
		required: boolean,
	): Promise<void> {
		if (required) await contentProtection.acquire(currentAccount.id);
	}

	function clearAuthTimer(): void {
		if (authPollTimer) clearTimeout(authPollTimer);
		authPollTimer = undefined;
		if (successTimer) clearInterval(successTimer);
		successTimer = undefined;
	}

	function clearAuthSecrets(): void {
		authPassword = "";
		authCode = "";
		revocationCode = "";
		revocationConfirmation = "";
		confirmationCode = "";
		vaultPassword = "";
		vaultPasswordConfirmation = "";
		vaultAppPassword = "";
		exportPassword = "";
	}

	function showCredentialForm(purpose: SteamAuthPurpose, currentAccount: SteamGuardAccountRef): void {
		clearAuthTimer();
		authPurpose = purpose;
		authStage = "credentials";
		authAccountName = currentAccount.username;
		authHandle = "";
		authChallenge = "";
		authMessage = "";
		authResult = null;
		storedDeviceCodeTried = false;
		if (purpose === "add_authenticator") enrollmentStage = "idle";
		clearAuthSecrets();
		focusCurrentScreen();
	}

	async function startEnrollment(): Promise<void> {
		if (state.screen !== "enrollment" || busy || !controller.resumeSteamGuardEnrollment) return;
		const currentAccount = state.account;
		busy = true;
		inlineError = "";
		enrollmentStage = "checking";
		authMessage = $t("SteamGuard_Enrollment_CheckingVault");
		try {
			const status = await controller.getSteamGuardVaultStatus?.();
			if (status && (!status.configured || !status.unlocked)) {
				vaultStatus = status;
				rememberForSession = status.rememberForSession;
				enrollmentStage = "vault";
				authMessage = "";
				focusCurrentScreen();
				return;
			}
			await resumeEnrollment(currentAccount);
		} catch (error) {
			console.error("Steam Guard: setup could not be resumed", error);
			enrollmentStage = "error";
			authMessage = $t("SteamGuard_Error_SetupResumeFailed");
		} finally {
			busy = false;
		}
	}

	async function resumeEnrollment(currentAccount: SteamGuardAccountRef): Promise<void> {
		const capability = await ensureCapability(currentAccount);
		authMessage = $t("SteamGuard_Enrollment_CheckingSetup");
		const status = await controller.resumeSteamGuardEnrollment?.(currentAccount.id, capability);
		if (!status) throw new Error("Enrollment status unavailable");
		await refreshCapabilityIfRequired(currentAccount, status.capabilityRefreshRequired);
		if (steamEnrollmentStep(status) === "not-started") showCredentialForm("add_authenticator", currentAccount);
		else await prepareEnrollment(currentAccount, status);
	}

	async function submitVaultPreparation(): Promise<void> {
		if (busy || state.screen !== "enrollment" || enrollmentStage !== "vault" || !vaultStatus || !vaultPassword) return;
		const currentAccount = state.account;
		if (!vaultStatus.configured) {
			const policyError = validateNewPassword(vaultPassword);
			if (policyError) {
				inlineError = passwordPolicyMessage(policyError);
				clearAuthSecrets();
				focusCurrentScreen();
				return;
			}
			if (vaultPasswordConfirmation !== vaultPassword) {
				inlineError = $t("SteamGuard_Error_VaultPasswordMismatch");
				clearAuthSecrets();
				focusCurrentScreen();
				return;
			}
			if (vaultStatus.savedAccountDataEncrypted && !vaultAppPassword) {
				inlineError = $t("SteamGuard_Error_AppPasswordRequired");
				return;
			}
		}
		busy = true;
		inlineError = "";
		let pending: Promise<void>;
		try {
			if (vaultStatus.configured) {
				if (!controller.unlockSteamGuardVault) throw new Error("Steam Guard vault unlock unavailable");
				pending = controller.unlockSteamGuardVault(
					currentAccount.id,
					vaultPassword,
					rememberForSession,
					await ensureCapability(currentAccount),
				);
			} else {
				if (!controller.initializeSteamGuardVault) throw new Error("Steam Guard vault setup unavailable");
				pending = controller.initializeSteamGuardVault(vaultPassword, vaultAppPassword);
			}
		} catch (error) {
			console.error("Steam Guard: vault unlock could not start", error);
			clearAuthSecrets();
			busy = false;
			inlineError = $t("SteamGuard_Error_VaultUnlockStartFailed");
			return;
		}
		clearAuthSecrets();
		try {
			await pending;
		} catch (error) {
			console.error("Steam Guard: vault could not be opened", error);
			inlineError = vaultStatus?.configured
				? $t("SteamGuard_Error_PasswordRejected")
				: $t("SteamGuard_Error_VaultCreateFailed");
			busy = false;
			return;
		}
		vaultStatus = null;
		enrollmentStage = "checking";
		try {
			await resumeEnrollment(currentAccount);
		} catch (error) {
			console.error("Steam Guard: setup could not continue after vault unlock", error);
			enrollmentStage = "error";
			authMessage = $t("SteamGuard_Error_SetupContinueFailed");
		} finally {
			busy = false;
		}
	}

	async function prepareEnrollment(
		currentAccount: SteamGuardAccountRef,
		status: SteamEnrollmentStatus,
	): Promise<void> {
		enrollmentStatus = status;
		enrollmentRetryAt = status.hasRetryAfter
			? Date.now() + Math.max(0, status.retryAfterSeconds) * 1_000
			: 0;
		authMessage = "";
		const step = steamEnrollmentStep(status);
		if (step === "complete") {
			enrollmentStage = "complete";
			announce($t("SteamGuard_Enrollment_Complete"));
			return;
		}
		if (step === "blocked") {
			enrollmentStage = "error";
			if (status.state === "phone_required") authMessage = $t("SteamGuard_Error_PhoneRequired");
			else if (status.state === "already_has_authenticator") authMessage = $t("SteamGuard_Error_AlreadyHasAuthenticator");
			else if (status.state === "rate_limited") authMessage = $t("SteamGuard_Error_RateLimitedSetup");
			else if (status.state === "reauthentication_required") showCredentialForm("add_authenticator", currentAccount);
			else authMessage = $t("SteamGuard_Error_SetupBlocked");
			return;
		}
		if (step === "recovery" && controller.revealSteamGuardRevocationCode) {
			const capability = await ensureCapability(currentAccount);
			const view = await controller.revealSteamGuardRevocationCode(currentAccount.id, capability);
			revocationCode = view.code;
			enrollmentStage = "recovery";
			focusCurrentScreen();
			return;
		}
		if (status.state === "confirmation_code_rejected") {
			authMessage = $t("SteamGuard_Error_ConfirmationCodeRejected");
		} else if (status.state === "authenticator_code_retry") {
			authMessage = $t("SteamGuard_Error_AuthenticatorRetry");
		} else if (status.state === "rate_limited") {
			authMessage = $t("SteamGuard_Error_RateLimitedRetry");
		}
		enrollmentStage = "confirmation";
		focusCurrentScreen();
	}

	async function beginCredentialLogin(): Promise<void> {
		if (busy || !controller.beginCredentialLogin || authAccountName.trim().length === 0 || authPassword.length === 0) return;
		const currentAccount = steamGuardAccountForState(state) ?? account;
		busy = true;
		authMessage = $t("SteamGuard_Challenge_SigningIn");
		let pending: Promise<SteamCredentialResult>;
		try {
			pending = controller.beginCredentialLogin(
				currentAccount.id,
				await ensureCapability(currentAccount),
				authAccountName.trim(),
				authPassword,
				authPurpose,
			);
		} catch (error) {
			console.error("Steam Guard: Steam sign-in could not start", error);
			authPassword = "";
			busy = false;
			authStage = "error";
			authMessage = withFailureReason($t("SteamGuard_Error_SignInStartFailed"), error);
			return;
		}
		authPassword = "";
		try {
			await handleCredentialResult(currentAccount, await pending);
		} catch (error) {
			console.error("Steam Guard: Steam sign-in could not continue", error);
			authStage = "error";
			authMessage = withFailureReason($t("SteamGuard_Error_SignInContinueFailed"), error);
		} finally {
			busy = false;
		}
	}

	function defaultChallenge(result: SteamCredentialResult): string {
		if (result.canSubmitEmailCode) return "email_code";
		if (result.canSubmitDeviceCode) return "device_code";
		return "";
	}

	/**
	 * Answers Steam's device-code challenge with the code this vault generates for the
	 * account, so re-authenticating an account we already hold the authenticator for
	 * does not ask the user to copy a code out of this same app.
	 *
	 * Returns false when the code is unavailable (locked vault, account not in the
	 * vault) or Steam rejects it, leaving the caller to show the manual form. Tried at
	 * most once per sign-in attempt: a rejected code must not spin.
	 */
	async function autoSubmitStoredDeviceCode(
		currentAccount: SteamGuardAccountRef,
		result: SteamCredentialResult,
	): Promise<boolean> {
		if (!result.canSubmitDeviceCode || storedDeviceCodeTried) return false;
		if (!controller.submitCredentialCode || !controller.getCode) return false;
		const handle = result.handle || authHandle;
		if (!handle) return false;
		storedDeviceCodeTried = true;

		const previousStage = authStage;
		const previousMessage = authMessage;
		authStage = "polling";
		authMessage = $t("SteamGuard_Challenge_UsingStoredCode");
		try {
			const capability = await ensureCapability(currentAccount);
			const view = await controller.getCode(currentAccount.id, capability);
			const code = view?.code?.trim() ?? "";
			if (!code) {
				authStage = previousStage;
				authMessage = previousMessage;
				return false;
			}
			const next = await controller.submitCredentialCode(
				currentAccount.id,
				capability,
				handle,
				"device_code",
				code,
			);
			await handleCredentialResult(currentAccount, next);
			return true;
		} catch (error) {
			// Expected whenever the vault is locked or Steam refuses the code; the
			// manual form is the fallback, so this is not surfaced as a failure.
			console.warn("Steam Guard: stored code could not answer the challenge", error);
			authStage = previousStage;
			authMessage = previousMessage;
			return false;
		}
	}

	/**
	 * Marks Login Again when this account's stored Steam session will not work, so the
	 * user is pointed at it before hitting a failure elsewhere.
	 *
	 * The stored session answers first because it costs nothing; Steam is then asked
	 * the same question the confirmations page asks, which also catches a session
	 * revoked before its expiry. Neither step is allowed to raise a false alarm: only
	 * a definite refusal sets the flag, and every failure leaves it as it was.
	 */
	async function checkSessionNeedsLogin(currentAccount: SteamGuardAccountRef): Promise<void> {
		if (!currentAccount.id || sessionCheckedAccount === currentAccount.id) return;
		sessionCheckedAccount = currentAccount.id;
		sessionNeedsLogin = false;

		let capability = "";
		try {
			capability = await ensureCapability(currentAccount);
		} catch (error) {
			console.warn("Steam Guard: session state needs a capability", error);
			return;
		}
		try {
			const local = await controller.steamSessionLocalState?.(currentAccount.id, capability);
			if (local?.needsLogin) sessionNeedsLogin = true;
		} catch (error) {
			console.warn("Steam Guard: stored session could not be read", error);
		}
		if (sessionNeedsLogin) return;
		try {
			const probed = await controller.probeSteamSession?.(currentAccount.id, capability);
			if (probed?.needsLogin) sessionNeedsLogin = true;
		} catch (error) {
			console.warn("Steam Guard: session could not be checked with Steam", error);
		}
	}

	/**
	 * Confirms the sign-in worked before leaving the screen. Returning to the code
	 * immediately made a success indistinguishable from the form simply closing.
	 */
	function startLoginSuccessCountdown(currentAccount: SteamGuardAccountRef): void {
		clearAuthTimer();
		// The session just changed, so the old verdict no longer describes it.
		sessionCheckedAccount = "";
		sessionNeedsLogin = false;
		authStage = "success";
		successCountdown = LOGIN_SUCCESS_SECONDS;
		successTimer = setInterval(() => {
			successCountdown -= 1;
			if (successCountdown > 0) return;
			clearAuthTimer();
			authStage = "idle";
			void loadAccount(currentAccount);
		}, 1_000);
	}

	async function handleCredentialResult(
		currentAccount: SteamGuardAccountRef,
		result: SteamCredentialResult,
	): Promise<void> {
		clearAuthTimer();
		authResult = result;
		authHandle = result.handle || authHandle;
		const step = steamCredentialStep(result);
		if (step === "failed") {
			authHandle = "";
			authStage = "error";
			authMessage = result.state === "expired"
				? $t("SteamGuard_Error_SignInExpired")
				: result.state === "agreement_required"
					? $t("SteamGuard_Error_AgreementRequired")
					: result.state === "challenge_required"
						? $t("SteamGuard_Error_ChallengeUnsupported")
						: $t("SteamGuard_Error_SignInRejected");
			return;
		}
		if (step === "complete") {
			authHandle = "";
			await refreshCapabilityIfRequired(currentAccount, result.capabilityRefreshRequired);
			if (result.outcome === "enrollment_pending") {
				const status = result.enrollment ?? await controller.resumeSteamGuardEnrollment?.(
					currentAccount.id,
					capabilityFor(currentAccount),
				);
				if (!status) throw new Error("Enrollment status unavailable");
				await refreshCapabilityIfRequired(currentAccount, status.capabilityRefreshRequired);
				await prepareEnrollment(currentAccount, status);
				return;
			}
			if (result.outcome === "enrollment_not_started") {
				enrollmentStage = "error";
				authStage = "error";
				authMessage = $t("SteamGuard_Error_EnrollmentNotStarted");
				return;
			}
			announce($t("SteamGuard_LoginAgain_RefreshedAnnounce"));
			busy = false;
			startLoginSuccessCountdown(currentAccount);
			return;
		}
		if (step === "code") {
			const rejectedStoredCode = storedDeviceCodeTried;
			if (await autoSubmitStoredDeviceCode(currentAccount, result)) return;
			authChallenge = defaultChallenge(result);
			authStage = "challenge";
			authMessage = rejectedStoredCode
				? $t("SteamGuard_Challenge_StoredCodeRejected")
				: $t("SteamGuard_Challenge_EnterCode");
		focusCurrentScreen();
		return;
		}
		if (step === "poll") {
			authStage = "polling";
			authMessage = $t("SteamGuard_Challenge_ApproveInMobile");
		const delay = Math.max(750, Math.min(result.pollAfterMillis || 1_000, 10_000));
		authPollTimer = setTimeout(() => void pollCredentialLogin(), delay);
		return;
		}
		authStage = "polling";
		authMessage = $t("SteamGuard_Challenge_PreparingNextStep");
	}

	async function submitCredentialCode(): Promise<void> {
		if (busy || !controller.submitCredentialCode || !authHandle || !authChallenge || !authCode) return;
		const currentAccount = steamGuardAccountForState(state) ?? account;
		busy = true;
		let pending: Promise<SteamCredentialResult>;
		try {
			pending = controller.submitCredentialCode(
				currentAccount.id,
				await ensureCapability(currentAccount),
				authHandle,
				authChallenge,
				authCode,
			);
		} catch (error) {
			console.error("Steam Guard: Steam code could not be submitted", error);
			authCode = "";
			busy = false;
			authMessage = withFailureReason($t("SteamGuard_Error_CodeSubmitFailed"), error);
			return;
		}
		authCode = "";
		try {
			await handleCredentialResult(currentAccount, await pending);
		} catch (error) {
			console.error("Steam Guard: Steam code was rejected", error);
			authMessage = withFailureReason($t("SteamGuard_Error_CodeRejected"), error);
		} finally {
			busy = false;
		}
	}

	async function pollCredentialLogin(): Promise<void> {
		if (busy || !controller.pollCredentialLogin || !authHandle) return;
		const currentAccount = steamGuardAccountForState(state) ?? account;
		busy = true;
		try {
			await handleCredentialResult(
				currentAccount,
				await controller.pollCredentialLogin(
					currentAccount.id,
					await ensureCapability(currentAccount),
					authHandle,
				),
			);
		} catch (error) {
			console.error("Steam Guard: sign-in status could not be checked", error);
			authStage = "error";
			authMessage = withFailureReason($t("SteamGuard_Error_SignInStatusFailed"), error);
		} finally {
			busy = false;
		}
	}

	async function cancelCredentialLogin(): Promise<void> {
		clearAuthTimer();
		const currentAccount = steamGuardAccountForState(state) ?? account;
		const handle = authHandle;
		authHandle = "";
		clearAuthSecrets();
		if (handle && controller.cancelCredentialLogin) {
			const capability = capabilityFor(currentAccount);
			if (capability) {
				await controller.cancelCredentialLogin(currentAccount.id, capability, handle).catch((error: unknown) => {
					console.error("Steam Guard: pending Steam sign-in could not be canceled", error);
				});
			}
		}
		authStage = "idle";
		if (state.screen === "enrollment" && enrollmentStage === "checking") enrollmentStage = "idle";
	}

	async function acknowledgeRevocationCode(): Promise<void> {
		if (busy || state.screen !== "enrollment" || !controller.acknowledgeSteamGuardRevocationCode) return;
		if (!revocationCode || revocationConfirmation !== revocationCode) {
			authMessage = $t("SteamGuard_Error_RecoveryCodeMismatch");
			return;
		}
		const currentAccount = state.account;
		busy = true;
		let pending: Promise<SteamEnrollmentStatus>;
		try {
			pending = controller.acknowledgeSteamGuardRevocationCode(
				currentAccount.id,
				await ensureCapability(currentAccount),
				revocationConfirmation,
			);
		} catch (error) {
			console.error("Steam Guard: recovery code acknowledgment could not start", error);
			clearAuthSecrets();
			busy = false;
			authMessage = $t("SteamGuard_Error_RecoveryAckStartFailed");
			return;
		}
		revocationCode = "";
		revocationConfirmation = "";
		try {
			await acknowledgeSteamRevocationThenRefresh(
				() => pending,
				(status) => refreshCapabilityIfRequired(currentAccount, status.capabilityRefreshRequired),
			);
			enrollmentStage = "confirmation";
			authMessage = $t("SteamGuard_RecoveryCode_Acknowledged");
			focusCurrentScreen();
		} catch (error) {
			console.error("Steam Guard: recovery code could not be acknowledged", error);
			enrollmentStage = "error";
			authMessage = $t("SteamGuard_Error_RecoveryAckFailed");
		} finally {
			busy = false;
		}
	}

	async function finalizeEnrollment(): Promise<void> {
		if (busy || state.screen !== "enrollment" || !controller.finalizeSteamGuardEnrollment || !confirmationCode) return;
		const currentAccount = state.account;
		busy = true;
		let pending: Promise<SteamEnrollmentStatus>;
		try {
			pending = controller.finalizeSteamGuardEnrollment(
				currentAccount.id,
				await ensureCapability(currentAccount),
				confirmationCode,
			);
		} catch (error) {
			console.error("Steam Guard: confirmation code could not be submitted", error);
			confirmationCode = "";
			busy = false;
			authMessage = $t("SteamGuard_Error_ConfirmationSubmitFailed");
			return;
		}
		confirmationCode = "";
		try {
			const status = await pending;
			await refreshCapabilityIfRequired(currentAccount, status.capabilityRefreshRequired);
			await prepareEnrollment(currentAccount, status);
			if (!status.pending) {
				busy = false;
				await controller.showEnrollmentBackupWarning?.().catch((error: unknown) => {
					console.error("Steam Guard: backup reminder could not be shown", error);
				});
			}
		} catch (error) {
			console.error("Steam Guard: confirmation code was rejected", error);
			authMessage = $t("SteamGuard_Error_ConfirmationRejected");
		} finally {
			busy = false;
		}
	}

	async function cancelEnrollment(): Promise<void> {
		if (state.screen !== "enrollment" || busy) return;
		await closeSteamGuardEnrollment({
			cancelCredentials: cancelCredentialLogin,
			clearSecrets: () => {
				clearAuthSecrets();
				enrollmentStatus = null;
				vaultStatus = null;
				authMessage = "";
			},
			dismiss: dismissModal,
		});
	}

	async function restoreEnrollmentVaultPrompt(currentAccount: SteamGuardAccountRef): Promise<void> {
		clearAuthTimer();
		authHandle = "";
		clearAuthSecrets();
		try {
			await contentProtection.acquire(currentAccount.id);
			vaultStatus = await controller.getSteamGuardVaultStatus?.() ?? {
				configured: true,
				unlocked: false,
				rememberForSession: false,
				savedAccountDataEncrypted: false,
			};
			rememberForSession = vaultStatus.rememberForSession;
			enrollmentStage = "vault";
			authStage = "idle";
			inlineError = $t("SteamGuard_Error_VaultTimedOut");
			focusCurrentScreen();
		} catch (error) {
			console.error("Steam Guard: enrollment flow could not be reopened", error);
			enrollmentStage = "error";
			authMessage = $t("SteamGuard_Error_EnrollmentReopenFailed");
		}
	}

  function showQrState(): void {
		if (state.screen !== "enrollment" && state.screen !== "account-code") return;
		const currentAccount = state.screen === "account-code" ? state.view.account : state.account;
		transition({ type: "show-qr", account: currentAccount });
		void scanSteamWindow();
  }

  async function scanSteamWindow(): Promise<void> {
    if (state.screen !== "qr" || busy || !controller.captureQrFromSteam) return;
		const currentAccount = state.account;
		busy = true;
		qrStage = "scanning";
		qrMessage = $t("SteamGuard_QR_LookingForSteam");
		qrApproval = null;
		qrAttempt = "";
		try {
			const capability = await ensureCapability(currentAccount);
			await handleQRScanResult(currentAccount, capability, await controller.captureQrFromSteam(currentAccount.id, capability));
		} catch (error) {
			console.error("Steam Guard: Steam could not be scanned for a QR code", error);
			qrStage = "error";
			qrMessage = $t("SteamGuard_QR_ScanFailed");
		} finally {
			busy = false;
		}
  }

  async function chooseQrScreenshot(): Promise<void> {
    if (state.screen !== "qr" || busy || !controller.chooseQrScreenshot) return;
		const currentAccount = state.account;
		busy = true;
		qrStage = "scanning";
		qrMessage = $t("SteamGuard_QR_ReadingScreenshot");
		try {
			const capability = await ensureCapability(currentAccount);
			const result = await controller.chooseQrScreenshot(currentAccount.id, capability);
			if (result) await handleQRScanResult(currentAccount, capability, result);
			else resetQRState();
		} catch (error) {
			console.error("Steam Guard: screenshot could not be read", error);
			qrStage = "error";
			qrMessage = $t("SteamGuard_QR_ScreenshotUnreadable");
		} finally {
			busy = false;
		}
  }

	async function decodeDroppedQRScreenshot(path: string): Promise<void> {
		if (state.screen !== "qr" || busy || !controller.decodeQrScreenshot) return;
		const currentAccount = state.account;
		busy = true;
		qrStage = "scanning";
		qrMessage = $t("SteamGuard_QR_ReadingDropped");
		try {
			const capability = await ensureCapability(currentAccount);
			await handleQRScanResult(currentAccount, capability, await controller.decodeQrScreenshot(currentAccount.id, path, capability));
		} finally {
			busy = false;
		}
	}

	async function handleQRScanResult(
		currentAccount: SteamGuardAccountRef,
		capability: string,
		result: SteamGuardQRScanResult,
	): Promise<void> {
		qrAttempt = "";
		qrApproval = null;
		qrNeedsLogin = false;
		if (result.state !== "ready" || !result.attempt) {
			qrStage = "error";
			qrMessage = steamGuardQRFailureMessage(result);
			return;
		}
		if (!controller.getQrApproval) {
			qrStage = "error";
			qrMessage = $t("SteamGuard_QR_ApprovalUnavailable");
			return;
		}
		qrAttempt = result.attempt;
		qrMessage = $t("SteamGuard_QR_CheckingRequester");
		try {
			qrApproval = await controller.getQrApproval(currentAccount.id, qrAttempt, capability);
			qrStage = "approval";
			qrMessage = "";
		} catch (error) {
			console.error("Steam Guard: QR sign-in request could not be read", error);
			qrStage = "error";
			qrNeedsLogin = true;
			qrMessage = $t("SteamGuard_QR_NeedsLogin");
		}
	}

	async function authorizeQRLogin(): Promise<void> {
		if (state.screen !== "qr" || qrStage !== "approval" || !qrAttempt || !controller.authorizeQrLogin) return;
		const currentAccount = state.account;
		busy = true;
		qrStage = "authorizing";
		try {
			await controller.authorizeQrLogin(currentAccount.id, qrAttempt, await ensureCapability(currentAccount));
			qrAttempt = "";
			qrApproval = null;
			qrStage = "authorized";
			qrMessage = $t("SteamGuard_QR_Authorized");
			reportSuccess($t("SteamGuard_QR_AuthorizedToast"));
		} catch (error) {
			console.error("Steam Guard: QR approval was rejected", error);
			qrAttempt = "";
			qrApproval = null;
			qrStage = "error";
			qrNeedsLogin = true;
			qrMessage = $t("SteamGuard_QR_ApprovalRejected");
		} finally {
			busy = false;
		}
	}

	async function dismissQRLogin(): Promise<void> {
		if (state.screen !== "qr") return;
		const currentAccount = state.account;
		const capability = contentProtection.capabilityFor(currentAccount.id);
		const attempt = qrAttempt;
		resetQRState();
		if (attempt && capability && controller.dismissQrLogin) {
			await controller.dismissQrLogin(currentAccount.id, attempt, capability).catch((error: unknown) => {
				console.error("Steam Guard: QR sign-in request could not be dismissed", error);
			});
		}
	}

	function resetQRState(): void {
		qrStage = "idle";
		qrMessage = "";
		qrAttempt = "";
		qrApproval = null;
		qrNeedsLogin = false;
	}

	async function selectQrRegion(): Promise<void> {
		if (state.screen !== "qr" || busy || !controller.selectQrRegion) return;
		const currentAccount = state.account;
		busy = true;
		qrRegionSelecting = true;
		qrStage = "scanning";
		qrMessage = $t("SteamGuard_QR_RegionInstruction");
		qrApproval = null;
		qrAttempt = "";
		try {
			const capability = await ensureCapability(currentAccount);
			await handleQRScanResult(
				currentAccount,
				capability,
				await controller.selectQrRegion(currentAccount.id, capability),
			);
		} catch (error) {
			console.error("Steam Guard: screen region could not be scanned", error);
			qrStage = "error";
			qrMessage = $t("SteamGuard_QR_RegionFailed");
		} finally {
			qrRegionSelecting = false;
			busy = false;
		}
	}

	async function cancelQrRegion(): Promise<void> {
		if (state.screen !== "qr" || !qrRegionSelecting || !controller.cancelQrRegion) return;
		const currentAccount = state.account;
		const capability = contentProtection.capabilityFor(currentAccount.id);
		if (!capability) return;
		qrMessage = $t("SteamGuard_QR_CancelingRegion");
		await controller.cancelQrRegion(currentAccount.id, capability).catch((error: unknown) => {
			console.error("Steam Guard: screen region selection could not be canceled", error);
		});
	}

	/**
	 * "Login again" goes straight to the password form, with the warning shown on
	 * it. No silent-refresh interstitial: every automatic renewal has already been
	 * tried by the time a user reaches this, and a token refresh can "succeed"
	 * while leaving a session Steam still refuses — reporting success and
	 * stopping there was a lie.
	 */
	function startLoginAgain(): void {
		if (state.screen !== "login-again" || busy) return;
		const currentAccount = state.account;
		showCredentialForm("login_again", currentAccount);
		authMessage = $t("SteamGuard_LoginAgain_TokenRejected");
	}

	/** Authorizes the plaintext export from inside the modal so the sensitive-view lease survives. */
	async function submitExport(): Promise<void> {
		if (busy || state.screen !== "export-authorize" || !controller.exportMaFile || !exportPassword) return;
		const currentAccount = state.account;
		busy = true;
		exportError = "";
		let pending: Promise<SteamMaFileExportResult>;
		try {
			pending = controller.exportMaFile(
				currentAccount.id,
				await ensureCapability(currentAccount),
				exportPassword,
				exportMaFilePassword,
			);
		} catch (error) {
			console.error("Steam Guard: maFile export could not start", error);
			exportPassword = "";
			exportMaFilePassword = "";
			busy = false;
			exportError = $t("SteamGuard_Export_StartFailed");
			return;
		}
		exportPassword = "";
		exportMaFilePassword = "";
		try {
			const result = await pending;
			if (!result.path) {
				// Empty path is the save-dialog cancel contract, not a failure.
				announce($t("SteamGuard_Export_Canceled"));
				return;
			}
			if (result.manifestSkipped) {
				// The file was written; it just cannot be imported into SDA yet.
				pushToast({ type: "warning", message: $t("SteamGuard_Export_ManifestKept"), duration: 12_000 });
			} else {
				reportSuccess($t("SteamGuard_Export_Success"));
			}
			busy = false;
			await loadAccount(currentAccount);
		} catch (error) {
			console.error("Steam Guard: maFile export failed", error);
			exportError = $t("SteamGuard_Export_Failed");
			reportFailure($t("SteamGuard_Export_Failed"), error);
		} finally {
			busy = false;
			focusCurrentScreen();
		}
	}

  function startRecovery(): void {
    if (state.screen !== "recovery") return;
    const action = controller.recover;
    const accountId = state.account?.id;
    void runPlaceholderAction(action ? () => action(accountId) : undefined, $t("SteamGuard_Recovery_Started"));
  }

  function showRecoveryState(): void {
    if (state.screen !== "error") return;
    transition({
      type: "show-recovery",
      account: state.account,
      message: $t("SteamGuard_Recovery_Body"),
    });
  }

  onMount(() => {
			offRevoked = Events.On("steamguard:sensitive-view-revoked", () => {
				contentProtection.revoke();
				const currentAccount = steamGuardAccountForState(state) ?? account;
				if (state.screen === "enrollment") {
					void restoreEnrollmentVaultPrompt(currentAccount);
					announce($t("SteamGuard_Announce_Locked"));
					return;
				}
				transition({ type: "lock-account", account: currentAccount });
			announce($t("SteamGuard_Announce_Locked"));
		});
			if (entry === "account" || entry === "all-accounts") {
				void loadAccount(account);
			} else {
				void contentProtection.acquire(account.id).then(async () => {
					if (entry === "qr") void scanSteamWindow();
					if (entry === "login-again") void startLoginAgain();
				}).catch((error: unknown) => {
					console.error("Steam Guard: screen-capture protection could not be enabled", error);
					reportFailure($t("SteamGuard_Error_ContentProtectionFailed"), error);
				}).finally(() => {
					if (entry === "enrollment") void startEnrollment();
				});
			focusCurrentScreen();
    }
	    clockTimer = setInterval(() => {
	      now = Date.now();
	      if (state.screen === "account-code" && now >= state.view.expiresAt) {
				if (steamGuardCodeCanAutoRefresh(state.view)) {
					void refreshExpiredCode();
				} else {
					const currentAccount = state.view.account;
					transition({ type: "lock-account", account: currentAccount });
					announce($t("SteamGuard_Announce_UnlockForNextCode"));
				}
	      }
	    }, 250);
  });

	  onDestroy(() => {
			clearAuthTimer();
			void cancelCredentialLogin().finally(() => contentProtection.close()).catch((error: unknown) => {
			console.error("Steam Guard: modal teardown failed", error);
		});
		offRevoked?.();
    setSteamGuardDropTarget("none");
    if (clockTimer) clearInterval(clockTimer);
    if (statusTimer) clearTimeout(statusTimer);
	    password = "";
			clearAuthSecrets();
			authHandle = "";
		qrAttempt = "";
		qrApproval = null;
		qrRegionSelecting = false;
		exportPassword = "";
  });
</script>

<div class="steam-guard" bind:this={bodyEl} aria-busy={busy}>
  <p class="sr-only" aria-live="polite" aria-atomic="true">{statusMessage}</p>

  {#if state.screen === "loading"}
    <section class="steam-guard__center" aria-busy="true">
      <h2 class="steam-guard__heading" data-steamguard-focus tabindex="-1">
        {openAllAccountsOnReady ? $t("SteamGuard_Title") : state.account.username}
      </h2>
      <p>{$t("SteamGuard_Loading")}</p>
    </section>
  {:else if state.screen === "locked"}
    <form class="steam-guard__stack steam-guard__unlock-form" on:submit|preventDefault={unlockAccount}>
      <!--
        Only when this unlock is for one account. Opened from the action bar it
        leads to the account picker instead, and naming an account there would
        claim a choice the user has not made yet.
      -->
      {#if !openAllAccountsOnReady}
        <div class="steam-guard__identity">
          <span class="steam-guard__identity-avatar">
            <SteamAccountAvatar account={avatarRow(lockedAccountSummary ?? { ...state.account, locked: true })} fallback={PROFILE_FALLBACK} />
          </span>
          <span class="steam-guard__identity-name">
            <span class="steam-guard__identity-display">{state.account.username}</span>
            {#if lockedAccountSummary?.displayName && lockedAccountSummary.displayName !== state.account.username}
              <small>{lockedAccountSummary.displayName}</small>
            {/if}
          </span>
        </div>
      {/if}
      <p>{$t("SteamGuard_Unlock_Body")}</p>
      <label class="steam-guard__field" for="steam-guard-password">
        <span>{$t("SteamGuard_Field_Password")}</span>
        <input
          id="steam-guard-password"
          class="modal-input"
          bind:value={password}
          type="password"
          autocomplete="current-password"
          disabled={busy}
          data-steamguard-focus
          aria-invalid={inlineError ? "true" : undefined}
          aria-describedby={inlineError ? "steam-guard-unlock-error" : undefined}
          on:keydown={(event) => runOnEnter(event, unlockAccount)}
        />
      </label>
      {#if inlineError}
        <p id="steam-guard-unlock-error" class="steam-guard__error" role="alert">{inlineError}</p>
      {/if}
      <div class="steam-guard__actions steam-guard__actions--split">
        <div class="steam-guard__check">
          <span class="form-check">
            <input id="steam-guard-remember-session" class="form-check-input" bind:checked={rememberForSession} type="checkbox" disabled={busy} />
            <label class="form-check-label" for="steam-guard-remember-session" aria-hidden="true"></label>
          </span>
          <label for="steam-guard-remember-session">{$t("SteamGuard_RememberMe")}</label>
        </div>
        <button class="btnicontext modal-primary" type="button" disabled={busy || password.length === 0} on:click={unlockAccount}>
          <svg class="steam-guard__icon" viewBox={ICONS.key.box} aria-hidden="true"><path d={ICONS.key.path} /></svg>
          {busy ? $t("SteamGuard_Unlocking") : $t("SteamGuard_Unlock")}
        </button>
      </div>
    </form>
  {:else if state.screen === "account-code"}
    <section class="steam-guard__stack">
      <div class="steam-guard__identity" data-steamguard-focus tabindex="-1">
        <span class="steam-guard__identity-avatar">
          <SteamAccountAvatar account={avatarRow(codeAccountSummary)} fallback={PROFILE_FALLBACK} />
        </span>
        <span class="steam-guard__identity-name">
          <span class="steam-guard__identity-display">{codeAccountSummary.username}</span>
          {#if codeAccountSummary.displayName && codeAccountSummary.displayName !== codeAccountSummary.username}
            <small>{codeAccountSummary.displayName}</small>
          {/if}
        </span>
      </div>
      <button
	        type="button"
	        class="steam-guard__code"
	        disabled={!controller.copyCode || oneOperationCode}
	        style={`--steam-guard-code-progress:${codeProgress * 100}%`}
	        aria-label={oneOperationCode
					? $t("SteamGuard_Code_ViewLabel", { code: state.view.code, username: state.view.account.username })
					: $t("SteamGuard_Code_CopyLabel", { code: state.view.code, username: state.view.account.username })}
	        aria-describedby={oneOperationCode
					? "steam-guard-code-expiry steam-guard-one-operation-warning"
					: "steam-guard-code-expiry"}
        on:click={copyCurrentCode}
      >
        <span aria-hidden="true">{state.view.code}</span>
        <span class="steam-guard__countdown" aria-hidden="true"></span>
      </button>
	      <p id="steam-guard-code-expiry" class="steam-guard__expiry">
	        {$t("SteamGuard_Code_Expiry", { seconds: remainingSeconds })}
	      </p>
			{#if oneOperationCode}
				<p id="steam-guard-one-operation-warning" class="steam-guard__warning" role="status">
					{$t("SteamGuard_Code_OneOperationWarning")}
				</p>
			{/if}
      {#if state.view.timeStatus !== "fresh"}
        <p class="steam-guard__warning" role="status">
          {#if state.view.timeStatus === "stale"}
            {$t("SteamGuard_Time_Stale")}
          {:else if state.view.timeStatus === "untrusted"}
            {$t("SteamGuard_Time_Untrusted")}
          {:else}
            {$t("SteamGuard_Time_Unavailable")}
          {/if}
        </p>
      {/if}
			{#if oneOperationCode}
				<div class="steam-guard__actions steam-guard__actions--stretch">
					<button type="button" class="btnicontext modal-primary" disabled={busy} on:click={unlockForAnotherOperation}>
						<svg class="steam-guard__icon" viewBox={ICONS.key.box} aria-hidden="true"><path d={ICONS.key.path} /></svg>
						{$t("SteamGuard_Code_UnlockAgain")}
					</button>
				</div>
			{:else}
				<div class="steam-guard__actions steam-guard__actions--stretch">
					<button
						type="button"
						class="btnicontext modal-primary"
						disabled={!controller.openConfirmations}
						on:click={showConfirmations}
					>
						<svg class="steam-guard__icon" viewBox={ICONS.shield.box} aria-hidden="true"><path d={ICONS.shield.path} /></svg>
						{$t("SteamGuard_Code_ViewConfirmations")}
					</button>
				</div>
				<div class="steam-guard__grid" use:controllerSpatialNavigation>
					<button
						type="button"
						class="btnicontext"
						class:steam-guard__suggested={sessionNeedsLogin}
						disabled={busy}
						on:click={showLoginAgainState}
					>
						<svg class="steam-guard__icon" viewBox={ICONS.key.box} aria-hidden="true"><path d={ICONS.key.path} /></svg>
						{$t("SteamGuard_Code_LoginAgain")}
					</button>
					<button
						type="button"
						class="btnicontext"
						disabled={busy || !controller.captureQrFromSteam}
						on:click={showQrState}
					>
						<svg class="steam-guard__icon" viewBox={ICONS.qrcode.box} aria-hidden="true"><path d={ICONS.qrcode.path} /></svg>
						{$t("SteamGuard_LoginWithQR")}
					</button>
					<button
						type="button"
						class="btnicontext"
						disabled={busy || !controller.exportMaFile}
						on:click={showExportState}
					>
						<svg class="steam-guard__icon" viewBox={ICONS.fileExport.box} aria-hidden="true"><path d={ICONS.fileExport.path} /></svg>
						{$t("SteamGuard_Code_ExportMaFile")}
					</button>
				</div>
			{/if}
      <div class="steam-guard__footer">
        <button
          type="button"
          class="steam-guard__link"
	          disabled={busy || oneOperationCode}
          on:click={showAllAccounts}
        >
          <svg class="steam-guard__icon" viewBox={ICONS.list.box} aria-hidden="true"><path d={ICONS.list.path} /></svg>
          {$t("SteamGuard_Code_ShowAllAccounts")}
        </button>
      </div>
    </section>
  {:else if state.screen === "all-accounts"}
    <section class="steam-guard__stack steam-guard__stack--fill">
      <h2 class="steam-guard__heading" data-steamguard-focus tabindex="-1">{$t("SteamGuard_AllAccounts_Title")}</h2>
      {#if state.accounts.length > 0}
        <ul class="steam-guard__accounts" use:controllerSpatialNavigation>
          {#each state.accounts as listedAccount (listedAccount.id)}
            <li>
              <button type="button" disabled={busy} on:click={() => loadAccount(listedAccount)}>
                <span class="steam-guard__accounts-avatar">
                  <SteamAccountAvatar account={avatarRow(listedAccount)} fallback={PROFILE_FALLBACK} />
                </span>
                <span class="steam-guard__accounts-name">
                  <span class="steam-guard__accounts-username">{listedAccount.username}</span>
                  {#if listedAccount.displayName && listedAccount.displayName !== listedAccount.username}
                    <small class="steam-guard__accounts-display">{listedAccount.displayName}</small>
                  {/if}
                </span>
                <small class="steam-guard__accounts-state">{listedAccount.locked ? $t("SteamGuard_AllAccounts_Locked") : $t("SteamGuard_AllAccounts_Ready")}</small>
              </button>
            </li>
          {/each}
        </ul>
      {:else}
        <p>{$t("SteamGuard_AllAccounts_Empty")}</p>
      {/if}
      <div class="steam-guard__footer">
        <button type="button" class="steam-guard__link" disabled={busy} on:click={leaveAllAccounts}>
          <svg class="steam-guard__icon" viewBox={ICONS.back.box} aria-hidden="true"><path d={ICONS.back.path} /></svg>
          {pickerReturnAccount ? $t("SteamGuard_Back") : $t("Button_Close")}
        </button>
      </div>
    </section>
  {:else if state.screen === "import"}
    <section class="steam-guard__stack">
      <h2 class="steam-guard__heading" data-steamguard-focus tabindex="-1">{$t("SteamGuard_Import_Title")}</h2>
      {#if state.account}<p class="steam-guard__account">{state.account.username}</p>{/if}
      <p>{$t("SteamGuard_Import_Body")}</p>
      <div class="steam-guard__actions steam-guard__actions--end">
        <button
          type="button"
          class="btnicontext modal-primary"
          disabled={!controller.importMaFile || busy}
          on:click={startImport}
        >{$t("SteamGuard_Import_Choose")}</button>
        {#if state.account}<button type="button" class="btnicontext" on:click={backToAccount}>{$t("SteamGuard_Back")}</button>{/if}
      </div>
    </section>
  {:else if state.screen === "enrollment"}
    <section class="steam-guard__stack">
      <h2 class="steam-guard__account" data-steamguard-focus tabindex="-1">{state.account.username}</h2>
		{#if enrollmentStage === "checking" || authStage === "refreshing"}
			<p role="status">{authMessage}</p>
			<div class="steam-guard__qr-progress" aria-hidden="true"></div>
		{:else if enrollmentStage === "vault" && vaultStatus}
			<form class="steam-guard__stack steam-guard__unlock-form" on:submit|preventDefault={submitVaultPreparation}>
				<p>{vaultStatus.configured
					? $t("SteamGuard_Enrollment_VaultUnlockBody")
					: $t("SteamGuard_Enrollment_VaultCreateBody")}</p>
				<label class="steam-guard__field" for="steam-enrollment-vault-password">
					<span>{$t("SteamGuard_Field_Password")}</span>
					<input id="steam-enrollment-vault-password" class="modal-input" bind:value={vaultPassword} type="password" autocomplete={vaultStatus.configured ? "current-password" : "new-password"} disabled={busy} data-steamguard-autofocus on:keydown={(event) => runOnEnter(event, submitVaultPreparation)} />
				</label>
				{#if vaultStatus.configured}
					<div class="steam-guard__check">
						<span class="form-check">
							<input id="steam-enrollment-remember-session" class="form-check-input" bind:checked={rememberForSession} type="checkbox" disabled={busy} />
							<label class="form-check-label" for="steam-enrollment-remember-session" aria-hidden="true"></label>
						</span>
						<label for="steam-enrollment-remember-session">{$t("SteamGuard_RememberMe")}</label>
					</div>
				{:else}
					<p class="steam-guard__hint">{$t("SteamGuard_Enrollment_VaultHint")}</p>
					<label class="steam-guard__field" for="steam-enrollment-vault-confirmation">
						<span>{$t("SteamGuard_Field_ConfirmPassword")}</span>
						<input id="steam-enrollment-vault-confirmation" class="modal-input" bind:value={vaultPasswordConfirmation} type="password" autocomplete="new-password" disabled={busy} on:keydown={(event) => runOnEnter(event, submitVaultPreparation)} />
					</label>
					{#if vaultStatus.savedAccountDataEncrypted}
						<label class="steam-guard__field" for="steam-enrollment-app-password">
							<span>{$t("SteamGuard_Field_AppPassword")}</span>
							<input id="steam-enrollment-app-password" class="modal-input" bind:value={vaultAppPassword} type="password" autocomplete="current-password" disabled={busy} on:keydown={(event) => runOnEnter(event, submitVaultPreparation)} />
						</label>
						<p class="steam-guard__hint">{$t("SteamGuard_Enrollment_AppPasswordHint")}</p>
					{/if}
				{/if}
				{#if inlineError}<p class="steam-guard__error" role="alert">{inlineError}</p>{/if}
				<div class="steam-guard__actions steam-guard__actions--end">
					<button type="button" class="btnicontext modal-primary" disabled={busy || !vaultPassword || (!vaultStatus.configured && !vaultPasswordConfirmation) || (!vaultStatus.configured && vaultStatus.savedAccountDataEncrypted && !vaultAppPassword)} on:click={submitVaultPreparation}>{vaultStatus.configured ? (busy ? $t("SteamGuard_Unlocking") : $t("SteamGuard_Unlock")) : (busy ? $t("SteamGuard_Vault_CreatingVault") : $t("SteamGuard_Vault_CreateVault"))}</button>
					<button type="button" class="btnicontext" disabled={busy} on:click={cancelEnrollment}>{$t("SteamGuard_Back")}</button>
				</div>
			</form>
		{:else if authStage === "credentials"}
			<form class="steam-guard__stack" on:submit|preventDefault={beginCredentialLogin}>
				<p>{$t("SteamGuard_Enrollment_CredentialsBody")}</p>
				<label class="steam-guard__field" for="steam-enrollment-account">
					<span>{$t("SteamGuard_Field_SteamAccountName")}</span>
					<input id="steam-enrollment-account" class="modal-input" bind:value={authAccountName} autocomplete="username" disabled={busy} on:keydown={(event) => runOnEnter(event, beginCredentialLogin)} />
				</label>
				<label class="steam-guard__field" for="steam-enrollment-password">
					<span>{$t("SteamGuard_Field_SteamPassword")}</span>
					<input id="steam-enrollment-password" class="modal-input" bind:value={authPassword} type="password" autocomplete="current-password" disabled={busy} data-steamguard-autofocus on:keydown={(event) => runOnEnter(event, beginCredentialLogin)} />
				</label>
				<div class="steam-guard__actions steam-guard__actions--end">
					<button type="submit" class="btnicontext modal-primary" disabled={busy || !authAccountName.trim() || !authPassword}>{$t("SteamGuard_SignIn")}</button>
					<button type="button" class="btnicontext" disabled={busy} on:click={cancelCredentialLogin}>{$t("SteamGuard_Cancel")}</button>
				</div>
			</form>
		{:else if authStage === "challenge" && authResult}
			<form class="steam-guard__stack" on:submit|preventDefault={submitCredentialCode}>
				<p>{authMessage}</p>
				{#if authResult.canSubmitEmailCode && authResult.canSubmitDeviceCode}
					<fieldset class="steam-guard__challenge-options">
						<legend>{$t("SteamGuard_Challenge_CodeSource")}</legend>
						<label><input type="radio" bind:group={authChallenge} value="email_code" /> {$t("SteamGuard_Challenge_EmailCode")}</label>
						<label><input type="radio" bind:group={authChallenge} value="device_code" /> {$t("SteamGuard_Challenge_DeviceCode")}</label>
					</fieldset>
				{/if}
				<label class="steam-guard__field" for="steam-enrollment-code">
					<span>{authChallenge === "email_code" ? $t("SteamGuard_Challenge_EmailCode") : $t("SteamGuard_Challenge_DeviceCode")}</span>
					<input id="steam-enrollment-code" class="modal-input" bind:value={authCode} autocomplete="one-time-code" disabled={busy} data-steamguard-autofocus on:keydown={(event) => runOnEnter(event, submitCredentialCode)} />
				</label>
				<div class="steam-guard__actions steam-guard__actions--end">
					<button type="submit" class="btnicontext modal-primary" disabled={busy || !authCode}>{$t("SteamGuard_Challenge_Submit")}</button>
					<button type="button" class="btnicontext" disabled={busy} on:click={cancelCredentialLogin}>{$t("SteamGuard_Cancel")}</button>
				</div>
			</form>
		{:else if authStage === "polling"}
			<p role="status">{authMessage}</p>
			<div class="steam-guard__qr-progress" aria-hidden="true"></div>
			<div class="steam-guard__actions steam-guard__actions--end">
				<button type="button" class="btnicontext" disabled={busy} on:click={pollCredentialLogin}>{$t("SteamGuard_CheckNow")}</button>
				<button type="button" class="btnicontext" disabled={busy} on:click={cancelCredentialLogin}>{$t("SteamGuard_Cancel")}</button>
			</div>
		{:else if enrollmentStage === "recovery"}
			<p class="steam-guard__warning" role="alert">{$t("SteamGuard_RecoveryCode_Warning")}</p>
			<p>{$t("SteamGuard_RecoveryCode_ResumeHint")}</p>
			<code class="steam-guard__recovery-code" aria-label={$t("SteamGuard_RecoveryCode_Label")}>{revocationCode}</code>
			<form class="steam-guard__stack" on:submit|preventDefault={acknowledgeRevocationCode}>
				<label class="steam-guard__field" for="steam-recovery-confirmation">
					<span>{$t("SteamGuard_RecoveryCode_ConfirmLabel")}</span>
					<input id="steam-recovery-confirmation" class="modal-input" bind:value={revocationConfirmation} autocomplete="off" spellcheck="false" disabled={busy} data-steamguard-autofocus on:keydown={(event) => runOnEnter(event, acknowledgeRevocationCode)} />
				</label>
				{#if authMessage}<p class="steam-guard__error" role="alert">{authMessage}</p>{/if}
				<div class="steam-guard__actions steam-guard__actions--end">
					<button type="submit" class="btnicontext modal-primary" disabled={busy || !revocationConfirmation}>{$t("SteamGuard_RecoveryCode_Saved")}</button>
					<button type="button" class="btnicontext" disabled={busy} on:click={cancelEnrollment}>{$t("SteamGuard_CloseAndResume")}</button>
				</div>
			</form>
		{:else if enrollmentStage === "confirmation"}
			<form class="steam-guard__stack" on:submit|preventDefault={finalizeEnrollment}>
				<p>{authMessage || (enrollmentStatus?.confirmation === "email"
					? $t("SteamGuard_Enrollment_EmailCodeBody")
					: $t("SteamGuard_Enrollment_SmsCodeBody"))}</p>
				{#if enrollmentStatus?.phoneHint}<p class="steam-guard__hint">{$t("SteamGuard_Enrollment_Destination", { hint: enrollmentStatus.phoneHint })}</p>{/if}
				<p class="steam-guard__hint">{$t("SteamGuard_Enrollment_CloseHint")}</p>
				{#if enrollmentRetrySeconds > 0}<p class="steam-guard__warning" role="status">{$t("SteamGuard_Enrollment_RetryIn", { seconds: enrollmentRetrySeconds })}</p>{/if}
				<label class="steam-guard__field" for="steam-enrollment-confirmation">
					<span>{$t("SteamGuard_Field_ConfirmationCode")}</span>
					<input id="steam-enrollment-confirmation" class="modal-input" bind:value={confirmationCode} autocomplete="one-time-code" disabled={busy || enrollmentRetrySeconds > 0} data-steamguard-autofocus on:keydown={(event) => runOnEnter(event, finalizeEnrollment)} />
				</label>
				<div class="steam-guard__actions steam-guard__actions--end">
					<button type="submit" class="btnicontext modal-primary" disabled={busy || !confirmationCode || enrollmentRetrySeconds > 0}>{$t("SteamGuard_Enrollment_Finish")}</button>
					<button type="button" class="btnicontext" disabled={busy} on:click={cancelEnrollment}>{$t("SteamGuard_CloseAndResume")}</button>
				</div>
			</form>
		{:else if enrollmentStage === "complete"}
			<p>{$t("SteamGuard_Enrollment_Complete")}</p>
			<div class="steam-guard__actions steam-guard__actions--end">
				<button type="button" class="btnicontext modal-primary" disabled={!controller.showEnrollmentBackupWarning} on:click={() => controller.showEnrollmentBackupWarning?.()}>{$t("SteamGuard_Enrollment_ShowBackupFolder")}</button>
				<button type="button" class="btnicontext" on:click={backToAccount}>{$t("SteamGuard_Enrollment_ViewCode")}</button>
			</div>
		{:else if authStage === "success"}
			<div class="steam-guard__success">
				<svg class="steam-guard__success-mark" viewBox="0 0 24 24" aria-hidden="true"><path d="M20 6 9 17l-5-5" /></svg>
				<p role="status">{$t("SteamGuard_LoginSuccess_Countdown", { seconds: successCountdown })}</p>
			</div>
		{:else if enrollmentStage === "error" || authStage === "error"}
			<p class="steam-guard__error" role="alert">{authMessage}</p>
			<div class="steam-guard__actions steam-guard__actions--end">
				<button type="button" class="btnicontext modal-primary" disabled={busy} on:click={startEnrollment}>{$t("SteamGuard_TryAgain")}</button>
				<button type="button" class="btnicontext" disabled={busy} on:click={cancelEnrollment}>{$t("SteamGuard_Back")}</button>
			</div>
		{:else}
			<p>{$t("SteamGuard_Enrollment_IntroBody")}</p>
			<div class="steam-guard__actions steam-guard__actions--end">
				<button type="button" class="btnicontext modal-primary" disabled={!controller.resumeSteamGuardEnrollment || busy} on:click={startEnrollment}>{$t("SteamGuard_Enrollment_Add")}</button>
				<button type="button" class="btnicontext" disabled={busy} on:click={showQrState}>{$t("SteamGuard_LoginWithQR")}</button>
				<button type="button" class="btnicontext" disabled={busy} on:click={backToAccount}>{$t("SteamGuard_Back")}</button>
			</div>
		{/if}
    </section>
  {:else if state.screen === "qr"}
    <section class="steam-guard__stack">
      <h2 class="steam-guard__account" data-steamguard-focus tabindex="-1">{state.account.username}</h2>
		{#if qrStage === "approval" && qrApproval}
			<div class="steam-guard__qr-approval" role="group" aria-label={$t("SteamGuard_QR_RequestLabel")}>
				<h3>{$t("SteamGuard_QR_AuthorizeHeading", { name: qrApproval.accountName || state.account.username })}</h3>
				<dl>
					{#if qrApproval.deviceName}<div><dt>{$t("SteamGuard_QR_Device")}</dt><dd>{qrApproval.deviceName}</dd></div>{/if}
					<div><dt>{$t("SteamGuard_QR_Client")}</dt><dd>{qrApproval.platform} · {qrApproval.application}</dd></div>
					{#if qrApproval.location}<div><dt>{$t("SteamGuard_QR_Location")}</dt><dd>{qrApproval.location}</dd></div>{/if}
					{#if qrApproval.ipAddress}<div><dt>{$t("SteamGuard_QR_IPAddress")}</dt><dd>{qrApproval.ipAddress}</dd></div>{/if}
					<div><dt>{$t("SteamGuard_QR_Session")}</dt><dd>{qrApproval.persistence}</dd></div>
				</dl>
				{#if qrApproval.locationMismatch || qrApproval.highUsageLogin}
					<p class="steam-guard__warning" role="alert">
						{$t("SteamGuard_QR_UnusualWarning")}
					</p>
				{/if}
			</div>
			<div class="steam-guard__actions steam-guard__actions--end">
				<button type="button" class="btnicontext modal-primary" disabled={busy} on:click={authorizeQRLogin}>
					{$t("SteamGuard_QR_Authorize")}
				</button>
				<button type="button" class="btnicontext" disabled={busy} on:click={dismissQRLogin}>{$t("SteamGuard_Cancel")}</button>
			</div>
		{:else}
			<p>{$t("SteamGuard_QR_Body")}</p>
			{#if qrMessage}
				<p class:steam-guard__error={qrStage === "error"} class="steam-guard__qr-status" role="status">
					{qrMessage}
				</p>
			{/if}
			{#if qrStage === "scanning" || qrStage === "authorizing"}
				<div class="steam-guard__qr-progress" aria-hidden="true"></div>
			{/if}
			{#if qrNeedsLogin}
				<div class="steam-guard__actions steam-guard__actions--stretch">
					<button
						type="button"
						class="btnicontext"
						disabled={busy || !controller.beginCredentialLogin}
						on:click={() => { transition({ type: "show-login-again", account: state.screen === "qr" ? state.account : account }, false); showCredentialForm("login_again", steamGuardAccountForState(state) ?? account); }}
					>
						<svg class="steam-guard__icon" viewBox={ICONS.key.box} aria-hidden="true"><path d={ICONS.key.path} /></svg>
						{$t("SteamGuard_SignInWithPassword")}
					</button>
				</div>
			{/if}
			<div class="steam-guard__actions steam-guard__actions--column" use:controllerSpatialNavigation>
				<button
					type="button"
					class="btnicontext modal-primary"
					disabled={!controller.captureQrFromSteam || busy}
					on:click={scanSteamWindow}
				>
					<svg class="steam-guard__icon" viewBox={ICONS.qrcode.box} aria-hidden="true"><path d={ICONS.qrcode.path} /></svg>
					{$t("SteamGuard_QR_ScanAgain")}
				</button>
				<button
					type="button"
					class="btnicontext"
					disabled={!controller.chooseQrScreenshot || busy}
					on:click={chooseQrScreenshot}
				>
					<svg class="steam-guard__icon" viewBox={ICONS.image.box} aria-hidden="true"><path d={ICONS.image.path} /></svg>
					{$t("SteamGuard_QR_ChooseScreenshot")}
				</button>
					{#if qrRegionSelecting}
						<button
							type="button"
							class="btnicontext"
							disabled={!controller.cancelQrRegion}
							on:click={cancelQrRegion}
						>
							<svg class="steam-guard__icon" viewBox={ICONS.crop.box} aria-hidden="true"><path d={ICONS.crop.path} /></svg>
							{$t("SteamGuard_QR_CancelRegion")}
						</button>
					{:else}
						<button
							type="button"
							class="btnicontext"
							disabled={!controller.selectQrRegion || busy}
							on:click={selectQrRegion}
						>
							<svg class="steam-guard__icon" viewBox={ICONS.crop.box} aria-hidden="true"><path d={ICONS.crop.path} /></svg>
							{$t("SteamGuard_QR_SelectRegion")}
						</button>
					{/if}
				<button type="button" class="btnicontext" disabled={busy} on:click={() => { void dismissQRLogin().then(backToAccount); }}>
					<svg class="steam-guard__icon" viewBox={ICONS.back.box} aria-hidden="true"><path d={ICONS.back.path} /></svg>
					{$t("SteamGuard_Back")}
				</button>
			</div>
		{/if}
    </section>
  {:else if state.screen === "login-again"}
    <section class="steam-guard__stack">
      <h2 class="steam-guard__account" data-steamguard-focus tabindex="-1">{state.account.username}</h2>
		{#if authStage === "credentials"}
			<div class="steam-guard__stack">
				<p>{authMessage || $t("SteamGuard_LoginAgain_TokenRejected")}</p>
				<label class="steam-guard__field" for="steam-login-account"><span>{$t("SteamGuard_Field_SteamAccountName")}</span><input id="steam-login-account" class="modal-input" bind:value={authAccountName} autocomplete="username" disabled={busy} on:keydown={(event) => runOnEnter(event, beginCredentialLogin)} /></label>
				<label class="steam-guard__field" for="steam-login-password"><span>{$t("SteamGuard_Field_SteamPassword")}</span><input id="steam-login-password" class="modal-input" bind:value={authPassword} type="password" autocomplete="current-password" disabled={busy} data-steamguard-autofocus on:keydown={(event) => runOnEnter(event, beginCredentialLogin)} /></label>
				<!-- Cancel leaves the login-again flow entirely: with the refresh
				     interstitial gone, staying on this screen at an idle stage strands
				     the user on its "Refreshing…" fallback with nothing running. -->
				<div class="steam-guard__actions steam-guard__actions--end"><button type="button" class="btnicontext modal-primary" disabled={busy || !authAccountName.trim() || !authPassword} on:click={beginCredentialLogin}>{$t("SteamGuard_SignIn")}</button><button type="button" class="btnicontext" disabled={busy} on:click={() => { void cancelCredentialLogin().then(backToAccount); }}>{$t("SteamGuard_Cancel")}</button></div>
			</div>
		{:else if authStage === "challenge" && authResult}
			<form class="steam-guard__stack" on:submit|preventDefault={submitCredentialCode}>
				<p>{authMessage}</p>
				{#if authResult.canSubmitEmailCode && authResult.canSubmitDeviceCode}<fieldset class="steam-guard__challenge-options"><legend>{$t("SteamGuard_Challenge_CodeSource")}</legend><label><input type="radio" bind:group={authChallenge} value="email_code" /> {$t("SteamGuard_Challenge_EmailCode")}</label><label><input type="radio" bind:group={authChallenge} value="device_code" /> {$t("SteamGuard_Challenge_DeviceCode")}</label></fieldset>{/if}
				<label class="steam-guard__field" for="steam-login-code"><span>{authChallenge === "email_code" ? $t("SteamGuard_Challenge_EmailCode") : $t("SteamGuard_Challenge_DeviceCode")}</span><input id="steam-login-code" class="modal-input" bind:value={authCode} autocomplete="one-time-code" disabled={busy} data-steamguard-autofocus on:keydown={(event) => runOnEnter(event, submitCredentialCode)} /></label>
				<div class="steam-guard__actions steam-guard__actions--end"><button type="submit" class="btnicontext modal-primary" disabled={busy || !authCode}>{$t("SteamGuard_Challenge_Submit")}</button><button type="button" class="btnicontext" disabled={busy} on:click={() => { void cancelCredentialLogin().then(backToAccount); }}>{$t("SteamGuard_Cancel")}</button></div>
			</form>
		{:else if authStage === "success"}
			<div class="steam-guard__success">
				<svg class="steam-guard__success-mark" viewBox="0 0 24 24" aria-hidden="true"><path d="M20 6 9 17l-5-5" /></svg>
				<p role="status">{$t("SteamGuard_LoginSuccess_Countdown", { seconds: successCountdown })}</p>
			</div>
		{:else if authStage === "polling" || authStage === "refreshing"}
			<p role="status">{authMessage}</p>
			<div class="steam-guard__qr-progress" aria-hidden="true"></div>
			{#if authHandle}<div class="steam-guard__actions steam-guard__actions--end"><button type="button" class="btnicontext" disabled={busy} on:click={pollCredentialLogin}>{$t("SteamGuard_CheckNow")}</button><button type="button" class="btnicontext" disabled={busy} on:click={() => { void cancelCredentialLogin().then(backToAccount); }}>{$t("SteamGuard_Cancel")}</button></div>{/if}
		{:else if authStage === "error"}
			<p class="steam-guard__error" role="alert">{authMessage}</p>
			<div class="steam-guard__actions steam-guard__actions--end"><button type="button" class="btnicontext modal-primary" disabled={busy} on:click={() => showCredentialForm("login_again", steamGuardAccountForState(state) ?? account)}>{$t("SteamGuard_SignInWithPassword")}</button><button type="button" class="btnicontext" disabled={busy} on:click={() => { void cancelCredentialLogin().then(backToAccount); }}>{$t("SteamGuard_Back")}</button></div>
		{:else}
			<p role="status">{$t("SteamGuard_LoginAgain_Refreshing")}</p>
			<div class="steam-guard__qr-progress" aria-hidden="true"></div>
			<div class="steam-guard__footer">
				<button type="button" class="steam-guard__link" disabled={busy} on:click={backToAccount}>
					<svg class="steam-guard__icon" viewBox={ICONS.back.box} aria-hidden="true"><path d={ICONS.back.path} /></svg>
					{$t("SteamGuard_Back")}
				</button>
			</div>
		{/if}
    </section>
  {:else if state.screen === "export-authorize"}
    <section class="steam-guard__stack">
      <div class="steam-guard__identity">
        <span class="steam-guard__identity-avatar">
          <SteamAccountAvatar account={avatarRow(exportAccountSummary)} fallback={PROFILE_FALLBACK} />
        </span>
        <span class="steam-guard__identity-name">
          <span class="steam-guard__identity-display">{exportAccountSummary.username}</span>
          {#if exportAccountSummary.displayName && exportAccountSummary.displayName !== exportAccountSummary.username}
            <small>{exportAccountSummary.displayName}</small>
          {/if}
        </span>
      </div>
      <p>{$t("SteamGuard_Export_Body")}</p>
      <form class="steam-guard__stack" on:submit|preventDefault={submitExport}>
        <label class="steam-guard__field" for="steam-guard-export-password">
          <span>{$t("SteamGuard_Field_ConfirmVaultPassword")}</span>
          <input
            id="steam-guard-export-password"
            class="modal-input"
            bind:value={exportPassword}
            type="password"
            autocomplete="current-password"
            disabled={busy}
            data-steamguard-autofocus
            aria-invalid={exportError ? "true" : undefined}
            aria-describedby={exportError ? "steam-guard-export-error" : undefined}
            on:keydown={(event) => runOnEnter(event, submitExport)}
          />
        </label>
        <label class="steam-guard__field" for="steam-guard-export-mafile-password">
          <span>{$t("SteamGuard_Field_MaFilePassword")}</span>
          <input
            id="steam-guard-export-mafile-password"
            class="modal-input"
            bind:value={exportMaFilePassword}
            type="password"
            autocomplete="new-password"
            disabled={busy}
            aria-describedby="steam-guard-export-mafile-hint"
            on:keydown={(event) => runOnEnter(event, submitExport)}
          />
          <small id="steam-guard-export-mafile-hint" class="steam-guard__hint">{$t("SteamGuard_Export_MaFilePasswordHint")}</small>
        </label>
        {#if exportError}
          <p id="steam-guard-export-error" class="steam-guard__error" role="alert">{exportError}</p>
        {/if}
        <div class="steam-guard__actions steam-guard__actions--split">
          <button type="button" class="btnicontext" disabled={busy} on:click={backToAccount}>
            <svg class="steam-guard__icon" viewBox={ICONS.back.box} aria-hidden="true"><path d={ICONS.back.path} /></svg>
            {$t("SteamGuard_Cancel")}
          </button>
          <button
            type="submit"
            class="btnicontext modal-primary"
            disabled={busy || !exportPassword || !controller.exportMaFile}
          >
            <svg class="steam-guard__icon" viewBox={ICONS.fileExport.box} aria-hidden="true"><path d={ICONS.fileExport.path} /></svg>
            {busy ? $t("SteamGuard_Export_Exporting") : $t("SteamGuard_Export_ChooseLocation")}
          </button>
        </div>
      </form>
    </section>
  {:else if state.screen === "recovery"}
    <section class="steam-guard__stack">
      <h2 class="steam-guard__heading" data-steamguard-focus tabindex="-1">{$t("SteamGuard_Recovery_Title")}</h2>
      <p>{state.message}</p>
      <div class="steam-guard__actions steam-guard__actions--end">
        <button
          type="button"
          class="btnicontext modal-primary"
          disabled={!controller.recover || busy}
          on:click={startRecovery}
        >{$t("SteamGuard_Recovery_Restore")}</button>
        {#if state.account}<button type="button" class="btnicontext" on:click={backToAccount}>{$t("SteamGuard_Back")}</button>{/if}
      </div>
    </section>
  {:else if state.screen === "error"}
    <section class="steam-guard__stack">
      <h2 class="steam-guard__heading" data-steamguard-focus tabindex="-1">{$t("SteamGuard_Error_Title")}</h2>
      <p class="steam-guard__error" role="alert">{state.message}</p>
      <div class="steam-guard__actions steam-guard__actions--end">
        {#if state.account}
          <button type="button" class="btnicontext modal-primary" on:click={backToAccount}>{$t("SteamGuard_TryAgain")}</button>
        {/if}
        <button
          type="button"
          class="btnicontext"
          on:click={showRecoveryState}
        >{$t("SteamGuard_Recovery_Options")}</button>
      </div>
    </section>
  {/if}
</div>

<style lang="scss">
  /* 8px rhythm */
  $sg-1: 0.5rem;
  $sg-2: 1rem;
  $sg-half: 0.25rem;

  /*
   * Sizing contract: this element's natural height is what the modal frame auto-sizer measures,
   * so it must never be clamped (no max-height, no percentage height) or the frame freezes at
   * whatever height it already had. Width fills the frame up to a readable maximum and centres;
   * auto block margins centre short screens vertically and collapse to 0 once content overflows,
   * at which point `.modal-scroll` (the shell) provides the only scrollbar.
   */
  .steam-guard {
    display: flex;
    flex: 0 1 auto;
    flex-direction: column;
    width: 100%;
    min-width: 0;
    max-width: 34rem;
    margin: auto;
    color: var(--white, #fff);
  }

  /*
   * The pre-ready measuring pass runs inside a `width: max-content` frame, where a percentage
   * width is indeterminate. A definite preferred width there makes the first measurement (and
   * therefore the spawn size) deterministic.
   */
  :global(.modalFG:not(.modalFG--ready)) .steam-guard {
    width: 30rem;
  }

  /* The global `button { margin: 0 0.25em }` rule fights every flex row's gap. */
  .steam-guard :global(button) {
    margin: 0;
  }

  .steam-guard__stack,
  .steam-guard__center {
    display: grid;
    gap: $sg-1;
    min-width: 0;
  }

  .steam-guard__stack--fill {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
  }

  .steam-guard__icon {
    flex: none;
    width: 0.95em;
    height: 0.95em;
    fill: currentColor;
  }

  .steam-guard__center {
    min-height: 9rem;
    place-content: center;
    text-align: center;
  }

  .steam-guard__stack > :global(p),
  .steam-guard__center > :global(p) {
    margin: 0;
    line-height: 1.45;
  }

  .steam-guard__heading,
  .steam-guard__account {
    margin: 0;
    color: inherit;
    font-size: 1rem;
    font-weight: 650;
    text-align: center;
  }

  .steam-guard__heading:focus,
  .steam-guard__account:focus {
    outline: none;
  }

  .steam-guard__field {
    display: grid;
    gap: 0.35rem;
    width: 100%;
    padding: 0;
    text-align: left;
  }

  /* Beats the global `input[type="password"]` rules so inputs span the form, not their content. */
  .steam-guard__field :global(.modal-input) {
    width: 100%;
    min-width: 0;
    margin: 0;
  }

  .steam-guard__check {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    width: fit-content;
    justify-self: start;
    text-align: left;
  }

  /*
   * .form-check is an inline-block wrapping the drawn 20px box, so its line box
   * adds descender space underneath. Centring then aligned that taller box rather
   * than the checkbox, leaving the label sitting low against it.
   */
  .steam-guard__check > :global(.form-check) {
    display: inline-flex;
    align-items: center;
  }

  .steam-guard__check > :global(label) {
    padding: 0;
  }

  /*
   * Marks the action the account actually needs. Colour is not the only signal:
   * the button keeps its label and icon, so this only adds emphasis.
   */
  .steam-guard__suggested {
    border-color: var(--green, #4caf50);
    background: color-mix(in srgb, var(--green, #4caf50) 22%, transparent);
  }

  .steam-guard__suggested:hover:not(:disabled) {
    background: color-mix(in srgb, var(--green, #4caf50) 34%, transparent);
  }

  .steam-guard__success {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.6rem;
    padding: 1.2rem 0;
  }

  .steam-guard__success-mark {
    width: 1.6rem;
    height: 1.6rem;
    flex: none;
    fill: none;
    stroke: var(--green, #4caf50);
    stroke-width: 2.5;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .steam-guard__success p {
    margin: 0;
    font-weight: 600;
  }

  .steam-guard__unlock-form {
    justify-items: stretch;
    text-align: left;
  }

  .steam-guard__code {
    position: relative;
    width: min(18rem, 100%);
    margin: 0.3rem auto 0;
    padding: 1rem 1.25rem 1.1rem;
    border: 1px solid var(--modal-border, var(--border-bar-bg));
    border-radius: 0.25rem;
    /* Role surfaces, not "lighten toward white": that mix was invisible on the
       light themes, where the content background already is nearly white. */
    background: var(--role-tile-bg, rgb(255 255 255 / 8%));
    color: inherit;
    font: 700 clamp(1.8rem, 7vw, 2.7rem) / 1 "Consolas", "Cascadia Mono", monospace;
    letter-spacing: 0.16em;
    text-align: center;
    cursor: pointer;
  }

  .steam-guard__code:hover {
    background: var(--role-dropdown-hover-bg, rgb(255 255 255 / 12%));
  }

  .steam-guard__code:focus-visible,
  .steam-guard__accounts button:focus-visible,
  .steam-guard__link:focus-visible {
    outline: 2px solid var(--role-focus-ring, var(--white, #fff));
    outline-offset: 2px;
  }

  .steam-guard__countdown {
    position: absolute;
    left: 0;
    bottom: 0;
    width: var(--steam-guard-code-progress);
    height: 0.22rem;
    background: var(--accent, #66c0f4);
    transition: width 250ms linear;
  }

  .steam-guard__expiry {
    color: var(--whiteSecondary, #d7d7d7);
    font-size: 0.78rem;
    text-align: center;
  }

  .steam-guard__actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: $sg-1;
  }

  /* Explicit width/height: the global `button { width: max-content }` rule cannot be trusted. */
  .steam-guard__actions :global(button) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: $sg-1;
    width: auto;
    min-height: 2.75rem;
  }

  .steam-guard__actions--end {
    justify-content: flex-end;
  }

  /* Remember Me left, primary action right. */
  .steam-guard__actions--split {
    flex-wrap: nowrap;
    justify-content: space-between;
    gap: $sg-2;
  }

  .steam-guard__actions--stretch :global(button) {
    flex: 1;
  }

  .steam-guard__actions--column {
    align-items: stretch;
    flex-direction: column;
  }

  .steam-guard__actions--column :global(button) {
    width: 100%;
  }

  /* Equal-width secondary actions on one row. */
  .steam-guard__grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: $sg-1;
  }

  .steam-guard__grid :global(button) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: $sg-half;
    width: 100%;
    min-width: 0;
    min-height: 2.75rem;
    padding-inline: $sg-1;
  }

  .steam-guard__footer {
    display: flex;
    justify-content: center;
    padding-top: $sg-half;
  }

  .steam-guard__link {
    display: inline-flex;
    align-items: center;
    gap: $sg-1;
    width: auto;
    min-height: 2.75rem;
    padding: $sg-1 $sg-1;
    border: 0;
    border-radius: 0.25rem;
    background: transparent;
    color: var(--whiteSecondary, #d7d7d7);
    text-decoration: underline;
    cursor: pointer;
  }

  .steam-guard__link:hover:not(:disabled) {
    background: var(--role-dropdown-hover-bg, rgb(255 255 255 / 12%));
    color: var(--white, #fff);
  }

  .steam-guard__link:disabled {
    cursor: default;
    opacity: 0.55;
  }

  /*
   * Capped at four rows so a large vault does not turn the whole modal into a
   * scroller; this list gets its own instead. The cap is a fixed rem value, not
   * a percentage or anything frame-relative, so the pre-ready measuring pass
   * still sees a deterministic height.
   */
  .steam-guard__accounts {
    display: flex;
    flex-direction: column;
    gap: $sg-1;
    margin: 0;
    padding: 0.15rem;
    max-height: calc(5 * 3.5rem + 4 * #{$sg-1} + 2 * 0.15rem);
    overflow-y: auto;
    list-style: none;
  }

  .steam-guard__accounts button {
    display: flex;
    min-height: 3.5rem;
    align-items: center;
    gap: 0.75rem;
    width: 100%;
    padding: $sg-1 $sg-1;
    border: 1px solid var(--modal-border, var(--border-bar-bg));
    border-radius: 0.25rem;
    background: transparent;
    color: inherit;
    text-align: left;
    cursor: pointer;
  }

  .steam-guard__accounts button:hover:not(:disabled) {
    border-color: var(--accent, #66c0f4);
    background: var(--role-dropdown-hover-bg, rgb(255 255 255 / 12%));
  }

  .steam-guard__accounts-avatar {
    display: flex;
    flex: none;
    width: 3.25rem;
    height: 3.25rem;
  }

  .steam-guard__accounts-avatar :global(img),
  .steam-guard__accounts-avatar :global(video) {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  /*
   * Username and display name sit tightly together (wrapping to a second line only when the row
   * is narrow); the spare width lives between this cell and the state, never between the names.
   */
  .steam-guard__accounts-name {
    display: flex;
    flex: 1 1 auto;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0.1rem 0.4rem;
    min-width: 0;
  }

  .steam-guard__accounts-username,
  .steam-guard__accounts-display {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .steam-guard__accounts-username {
    font-weight: 650;
  }

  .steam-guard__accounts-state {
    flex: none;
    margin-left: auto;
    padding-left: $sg-1;
  }

  .steam-guard__accounts-display,
  .steam-guard__accounts-state {
    color: var(--whiteSecondary, #d7d7d7);
    font-size: 0.82rem;
  }

  .steam-guard__identity {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    align-items: center;
    gap: $sg-2;
  }

  .steam-guard__identity-avatar {
    display: flex;
    flex: none;
    width: 3.25rem;
    height: 3.25rem;
  }

  .steam-guard__identity-avatar :global(img),
  .steam-guard__identity-avatar :global(video) {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .steam-guard__identity-name {
    display: grid;
    gap: 0.25rem;
    min-width: 0;
  }

  .steam-guard__identity-display {
    font-weight: 650;
    font-size: 1rem;
  }

  .steam-guard__identity-name small {
    color: var(--whiteSecondary, #d7d7d7);
    font-size: 0.9rem;
    display: block;
  }

  .steam-guard__error {
    margin: 0;
    color: var(--role-error, #ff6b6b);
  }

	  .steam-guard__warning {
    margin: 0;
    color: var(--role-warning, #ffd166);
    font-size: 0.82rem;
    text-align: center;
	  }

	.steam-guard__hint {
		color: var(--whiteSecondary, #d7d7d7);
		font-size: 0.82rem;
		text-align: center;
	}

	.steam-guard__challenge-options {
		display: flex;
		flex-wrap: wrap;
		gap: 0.55rem 1rem;
		margin: 0;
		padding: 0.75rem;
		border: 1px solid var(--modal-border, var(--border-bar-bg));
	}

	.steam-guard__challenge-options legend {
		padding: 0 0.3rem;
	}

	.steam-guard__challenge-options label {
		display: flex;
		align-items: center;
		gap: 0.4rem;
	}

	.steam-guard__recovery-code {
		display: block;
		padding: 0.8rem;
		border: 1px solid var(--role-warning, #ffd166);
		border-radius: 0.25rem;
		font: 700 1.25rem / 1.2 "Consolas", "Cascadia Mono", monospace;
		letter-spacing: 0.08em;
		text-align: center;
		user-select: text;
	}

	.steam-guard__qr-approval {
		display: grid;
		gap: 0.8rem;
		padding: 1rem;
		border: 1px solid var(--modal-border, var(--border-bar-bg));
		border-radius: 0.65rem;
		background: var(--role-tile-bg, rgb(255 255 255 / 8%));
	}

	.steam-guard__qr-approval h3,
	.steam-guard__qr-approval dl,
	.steam-guard__qr-approval dd {
		margin: 0;
	}

	.steam-guard__qr-approval dl {
		display: grid;
		gap: 0.45rem;
	}

	.steam-guard__qr-approval dl > div {
		display: grid;
		grid-template-columns: minmax(5rem, 0.35fr) minmax(0, 1fr);
		gap: 0.65rem;
	}

	.steam-guard__qr-approval dt {
		color: var(--whiteSecondary, #d7d7d7);
	}

	.steam-guard__qr-approval dd {
		overflow-wrap: anywhere;
	}

	.steam-guard__qr-status {
		margin: 0;
		min-height: 1.3rem;
		text-align: center;
	}

	.steam-guard__qr-progress {
		width: 100%;
		height: 0.2rem;
		overflow: hidden;
		border-radius: 999px;
		background: var(--modal-border, var(--border-bar-bg));
	}

	.steam-guard__qr-progress::after {
		display: block;
		width: 45%;
		height: 100%;
		border-radius: inherit;
		background: var(--accent, #4da3ff);
		content: "";
		animation: steam-guard-qr-progress 1.1s ease-in-out infinite alternate;
	}

	@keyframes steam-guard-qr-progress {
		from { transform: translateX(-15%); }
		to { transform: translateX(135%); }
	}

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  @media (max-width: 640px) {
    /* Width itself is frame-driven; only the button rows need the narrow-viewport fallback. */
    :global(.modalFG:not(.modalFG--ready)) .steam-guard {
      width: 100%;
    }

    .steam-guard__grid {
      grid-template-columns: minmax(0, 1fr);
    }

    .steam-guard__actions--split {
      flex-wrap: wrap;
    }
  }

  @media (prefers-reduced-motion: reduce) {
		.steam-guard__countdown,
		.steam-guard__qr-progress::after {
      transition: none;
			animation: none;
    }
  }

  @media (forced-colors: active) {
    .steam-guard__code,
    .steam-guard__accounts button {
      border-color: CanvasText;
      background: Canvas;
      color: CanvasText;
    }

    .steam-guard__countdown {
      background: Highlight;
    }
  }
</style>
