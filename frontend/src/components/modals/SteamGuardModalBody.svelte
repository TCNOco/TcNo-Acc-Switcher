<script lang="ts">
  import { onDestroy, onMount, tick } from "svelte";
  import { Events } from "@wailsio/runtime";
  import {
    clearModalBackAction,
    setModalBackAction,
    type ModalBackAction,
  } from "../../stores/modalBack";
  import {
	    initialSteamGuardModalState,
	    acknowledgeSteamRevocationThenRefresh,
		closeSteamGuardEnrollment,
    reduceSteamGuardModal,
    SteamGuardCapabilityError,
    SteamGuardContentProtectionLease,
    isStaleCapabilityError,
	    steamGuardAccountForState,
	    steamGuardListingAnchor,
	    steamGuardRowState,
	    steamGuardCodeCanAutoRefresh,
		    steamGuardCodeProgress,
			steamGuardQRFailureMessage,
			steamCredentialStep,
			isPendingAccountId,
			steamEnrollmentStep,
			steamLoginAgainNextStep,
	    type SteamGuardAccountRef,
	    type SteamGuardAccountKind,
	    type SteamGuardAccountSummary,
	    type SteamGuardSessionStatus,
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
	  import { dismissModal, setModalBusy } from "../../stores/modal";
	  import { passwordPolicyMessage, validateNewPassword } from "../../lib/passwordPolicy";
	  import { pushToast } from "../../stores/toast";
	  import { formatToastWithError, formatUnknownError } from "../../lib/formatWailsError";
	  import { requestModalAutoFit } from "../../lib/modalFrame";
	  import { controllerSpatialNavigation } from "../../lib/actions/controllerSpatialNavigation";
	  import { focusOnShow } from "../../lib/actions/focusOnShow";
	  import SteamAccountAvatar from "../SteamAccountAvatar.svelte";
  import SteamGuardVaultFactors from "./SteamGuardVaultFactors.svelte";
	  import { loadSteamGuardSwitcherProfile } from "../../lib/steamGuardBridge";
	  import type { SteamAccountRow } from "../../lib/steam/types";
	  import type { SteamBrowserSite } from "../../lib/steam/steamBrowserSites";

	/** Font Awesome Free v5.15.4 solid glyphs, inlined like the rest of the app's icons. */
	const ICONS = {
		shield: { box: "0 0 512 512", path: "M466.5 83.7l-192-80a48.15 48.15 0 0 0-36.9 0l-192 80C27.7 91.1 16 108.6 16 128c0 198.5 114.5 335.7 221.5 380.3 11.8 4.9 25.1 4.9 36.9 0C360.1 472.6 496 349.3 496 128c0-19.4-11.7-36.9-29.5-44.3z" },
		key: { box: "0 0 512 512", path: "M512 176.001C512 273.203 433.202 352 336 352c-11.22 0-22.19-1.062-32.827-3.069l-24.012 27.014A23.999 23.999 0 0 1 261.223 384H224v40c0 13.255-10.745 24-24 24h-40v40c0 13.255-10.745 24-24 24H24c-13.255 0-24-10.745-24-24v-78.059c0-6.365 2.529-12.47 7.029-16.971l161.802-161.802C163.108 213.814 160 195.271 160 176 160 78.798 238.797.001 335.999 0 433.488-.001 512 78.511 512 176.001zM336 128c0 26.51 21.49 48 48 48s48-21.49 48-48-21.49-48-48-48-48 21.49-48 48z" },
		qrcode: { box: "0 0 448 512", path: "M0 224h192V32H0v192zM64 96h64v64H64V96zm192-64v192h192V32H256zm128 128h-64V96h64v64zM0 480h192V288H0v192zm64-128h64v64H64v-64zm352-64h32v128h-96v-32h-32v96h-64V288h96v32h64v-32zm0 160h32v32h-32v-32zm-64 0h32v32h-32v-32z" },
		fileExport: { box: "0 0 576 512", path: "M384 121.941V128H256V0h6.059c6.365 0 12.47 2.529 16.971 7.029l97.941 97.941A24.005 24.005 0 0 1 384 121.941zM248 160c-13.2 0-24-10.8-24-24V0H24C10.745 0 0 10.745 0 24v464c0 13.255 10.745 24 24 24h336c13.255 0 24-10.745 24-24V160H248zm189.75 42.938l-96 96c-15.121 15.12-40.971 4.393-40.971-16.971V240h-64v-64h64v-41.967c0-21.346 25.833-32.104 40.971-16.971l96 96c9.373 9.373 9.373 24.569 0 33.938z" },
		image: { box: "0 0 512 512", path: "M464 448H48c-26.51 0-48-21.49-48-48V112c0-26.51 21.49-48 48-48h416c26.51 0 48 21.49 48 48v288c0 26.51-21.49 48-48 48zM112 120c-30.928 0-56 25.072-56 56s25.072 56 56 56 56-25.072 56-56-25.072-56-56-56zM64 384h384V272l-87.515-87.515c-4.686-4.686-12.284-4.686-16.971 0L208 320l-55.515-55.515c-4.686-4.686-12.284-4.686-16.971 0L64 336v48z" },
		crop: { box: "0 0 512 512", path: "M488 352h-40V96c0-17.67-14.33-32-32-32H192v64h192v256h-32V128H160V0H96v64H24C10.745 64 0 74.745 0 88v48c0 13.255 10.745 24 24 24h72v264c0 13.255 10.745 24 24 24h232v64h64v-64h72c13.255 0 24-10.745 24-24v-48c0-13.255-10.745-24-24-24z" },
		globe: { box: "0 0 496 512", path: "M336.5 160C322 70.7 287.8 8 248 8s-74 62.7-88.5 152h177zM152 256c0 22.2 1.2 43.5 3.3 64h185.3c2.1-20.5 3.3-41.8 3.3-64s-1.2-43.5-3.3-64H155.3c-2.1 20.5-3.3 41.8-3.3 64zm324.7-96c-28.6-67.9-86.5-120.4-158-141.6 24.4 33.8 41.2 84.7 50 141.6h108zM177.2 18.4C105.8 39.6 47.8 92.1 19.3 160h108c8.7-56.9 25.5-107.8 49.9-141.6zM487.4 192H372.7c2.1 21 3.3 42.5 3.3 64s-1.2 43-3.3 64h114.6c5.5-20.5 8.6-41.8 8.6-64s-3.1-43.5-8.5-64zM120 256c0-21.5 1.2-43 3.3-64H8.6C3.2 212.5 0 233.8 0 256s3.2 43.5 8.6 64h114.6c-2-21-3.2-42.5-3.2-64zm39.5 96c14.5 89.3 48.7 152 88.5 152s74-62.7 88.5-152h-177zm159.3 141.6c71.4-21.2 129.4-73.7 158-141.6h-108c-8.8 56.9-25.6 107.8-50 141.6zM19.3 352c28.6 67.9 86.5 120.4 158 141.6-24.4-33.8-41.2-84.7-50-141.6h-108z" },
		list: { box: "0 0 512 512", path: "M48 48a48 48 0 1 0 0 96 48 48 0 1 0 0-96zm448 32H176c-8.8 0-16-7.2-16-16V48c0-8.8 7.2-16 16-16h320c8.8 0 16 7.2 16 16v16c0 8.8-7.2 16-16 16zM48 208a48 48 0 1 0 0 96 48 48 0 1 0 0-96zm448 32H176c-8.8 0-16-7.2-16-16v-16c0-8.8 7.2-16 16-16h320c8.8 0 16 7.2 16 16v16c0 8.8-7.2 16-16 16zM48 368a48 48 0 1 0 0 96 48 48 0 1 0 0-96zm448 32H176c-8.8 0-16-7.2-16-16v-16c0-8.8 7.2-16 16-16h320c8.8 0 16 7.2 16 16v16c0 8.8-7.2 16-16 16z" },
		plus: { box: "0 0 448 512", path: "M416 208H272V64c0-17.67-14.33-32-32-32h-32c-17.67 0-32 14.33-32 32v144H32c-17.67 0-32 14.33-32 32v32c0 17.67 14.33 32 32 32h144v144c0 17.67 14.33 32 32 32h32c17.67 0 32-14.33 32-32V304h144c17.67 0 32-14.33 32-32v-32c0-17.67-14.33-32-32-32z" },
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
  /**
   * The account whose setup page sent the user into an add flow, or null when
   * that flow was reached some other way. Backing out of an add flow returns
   * here rather than closing the modal.
   */
  let setupReturn: SteamGuardAccountRef | null = null;
  // Extra unlock factors. Only the keyfile's path is held here; Go reads it.
  let unlockKeyfilePath = "";
  let unlockBackupKey = "";
  let busy = false;
  // Which browse button has a window opening. Separate from busy on purpose:
  // busy shuts the whole modal, close button included, and a page opening in
  // its own window has nothing in here to protect.
  let openingBrowserSite: SteamBrowserSite | null = null;
  let refreshing = false;
  let inlineError = "";
  let statusMessage = "";
  let statusTimer: ReturnType<typeof setTimeout> | undefined;
  let now = Date.now();
  let clockTimer: ReturnType<typeof setInterval> | undefined;
  let offRevoked: (() => void) | undefined;
  let offAccountUpdated: (() => void) | undefined;
  /** Whose capability the open picker was listed under. Once the picker is
   *  showing, the state holds no account of its own to re-derive it from. */
  let pickerIdentity: SteamGuardAccountRef | null = null;
  let pickerRefreshTimer: ReturnType<typeof setTimeout> | undefined;
	let qrStage: "idle" | "scanning" | "approval" | "authorizing" | "authorized" | "error" = "idle";
	let qrMessage = "";
	let qrAttempt = "";
	let qrApproval: SteamGuardQRApproval | null = null;
	let qrRegionSelecting = false;
	let qrNeedsLogin = false;
	/**
	 * The scan-to-sign-in code shown beside the password form. Its own state,
	 * because it is its own sign-in: the two are live at the same time and either
	 * can finish first, so sharing authStage/authHandle would have one wipe the
	 * other's session out from under it.
	 */
	let qrLoginStage: "idle" | "starting" | "waiting" | "unavailable" = "idle";
	let qrLoginHandle = "";
	let qrLoginImage = "";
	let qrLoginPollTimer: ReturnType<typeof setTimeout> | undefined;
	let qrLoginStartedFor = "";
	let qrLoginAccount: SteamGuardAccountRef | null = null;
	/** The close of the previous code, which the next one has to wait for. */
	let qrLoginClosing: Promise<void> | null = null;
	let authPurpose: SteamAuthPurpose = "login_again";
	let authStage: "idle" | "refreshing" | "credentials" | "challenge" | "polling" | "success" | "error" = "idle";
	let authAccountName = account.username;
	let authPassword = "";
	let authCode = "";
	let authHandle = "";
	let authChallenge = "";
	let authMessage = "";
	/** Why the sign-in form is being shown, when it is not the usual reason. */
	let credentialsHint = "";
	/** The enrollment screen is promoting a login-only account, not adding a new one. */
	let promotingLoginOnly = false;
	/** The vault form is being shown for its own sake, before the setup page's choice. */
	let vaultSetupOnly = false;
	/** The vault errand came from the add-account screen, which is not an
	 *  account's setup page, so that is where preparing it hands back. */
	let vaultSetupForAddAccount = false;
	let authResult: SteamCredentialResult | null = null;
	/** One stored-code attempt per sign-in: a code Steam rejected must not be resubmitted. */
	let storedDeviceCodeTried = false;
	let lastPageKey = "";
	/**
	 * What the account's stored Steam session turned out to be. "unknown" until
	 * the check answers, and it stays there when the check could not reach one:
	 * a failure to ask is not a verdict.
	 */
	let sessionVerdict: SteamGuardSessionStatus = "unknown";
	/** Highlights Login Again when the stored Steam session will not work. */
	$: sessionNeedsLogin = sessionVerdict === "needs-login";
	/**
	 * Browsing rides on that session, so it is offered only once something has
	 * said the session works. An undecided one hides it rather than opening a
	 * window that would land on a signed-out page.
	 */
	$: canBrowse = sessionVerdict === "valid";
	/** Second click of the inline Remove confirmation; reset on every transition. */
	let removeConfirming = false;
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
	/** Extra ways into an existing vault, offered beside the password. */
	let vaultKeyfilePath = "";
	let vaultBackupKey = "";
	let revocationCode = "";
	let revocationConfirmation = "";
	let confirmationCode = "";
	// Kept apart from authMessage, which the confirmation screen also uses for
	// ordinary instructions. A rejection rendered through the same paragraph is
	// indistinguishable from the guidance it replaces.
	let confirmationError = "";
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
	/**
	 * Fallback summary for an account we have no vault listing for. "authenticator"
	 * is the safe default: it is what every account was before login-only existed,
	 * and this only builds an identity for display, never a decision about which
	 * screen to open — that reads the real listing via knownKindOf. With no listing
	 * there is no session verdict either, hence "unknown".
	 */
	function refSummary(target: SteamGuardAccountRef, locked: boolean): SteamGuardAccountSummary {
		return { ...target, locked, kind: "authenticator", sessionStatus: "unknown" };
	}

	function summaryOf(
		target: SteamGuardAccountRef,
		locked: boolean,
		summaries: SteamGuardAccountSummary[],
		profiles: Record<string, SteamGuardAccountSummary>,
	): SteamGuardAccountSummary {
		const candidates = [
			summaries.find((summary) => summary.id === target.id),
			profiles[target.id],
			refSummary(target, locked),
		];
		return candidates.find(hasAvatar) ?? candidates.find(Boolean) ?? refSummary(target, locked);
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

	/**
	 * Runs a call that needs the capability, re-acquiring once if the backend
	 * refuses the one in hand.
	 *
	 * The capability is bound to the vault's generation, and every vault write
	 * rotates it - including writes this modal never asked for. The owned-games
	 * sweep renews lapsed sessions in the background, on accounts the user may
	 * not even have open, and that one batch invalidates the capability of every
	 * window holding one (see refreshLapsedOwnedGamesSessions in Go, which says
	 * as much). Nothing tells the modal, so the first sign of it is a call
	 * rejected for a capability that worked a second earlier - which is what left
	 * the browse buttons dead until the modal was reopened.
	 *
	 * Retrying the call itself is safe for the ones that use this: each checks the
	 * capability before it does anything, so a rejected attempt did nothing to
	 * repeat.
	 */
	async function withCapability<T>(
		currentAccount: SteamGuardAccountRef,
		run: (capability: string) => Promise<T>,
	): Promise<T> {
		try {
			return await run(await ensureCapability(currentAccount));
		} catch (error) {
			if (!isStaleCapabilityError(error)) throw error;
			console.warn("Steam Guard: the capability was superseded; acquiring a new one", error);
			await contentProtection.acquire(currentAccount.id);
			const refreshed = contentProtection.capabilityFor(currentAccount.id);
			if (!refreshed) throw new SteamGuardCapabilityError();
			return await run(refreshed);
		}
	}

	/**
	 * Which set of sign-in endpoints the running flow belongs to. The add-account
	 * screen drives the very same state machine, but against a pending id that
	 * only the add endpoints accept - Steam has not yet said which account the
	 * credentials belong to, so there is no SteamID64 to key on.
	 */
	function credentialApi(currentAccount: SteamGuardAccountRef) {
		return isPendingAccountId(currentAccount.id)
			? {
				begin: controller.beginAddAccountLogin,
				submit: controller.submitAddAccountCode,
				poll: controller.pollAddAccountLogin,
				cancel: controller.cancelAddAccountLogin,
			}
			: {
				begin: controller.beginCredentialLogin,
				submit: controller.submitCredentialCode,
				poll: controller.pollCredentialLogin,
				cancel: controller.cancelCredentialLogin,
			};
	}

  $: codeProgress = state.screen === "account-code"
    ? steamGuardCodeProgress(state.view.expiresAt, now)
    : 0;
	$: remainingSeconds = state.screen === "account-code"
    ? Math.max(0, Math.ceil((state.view.expiresAt - now) / 1_000))
	    : 0;
	$: oneOperationCode = state.screen === "account-code" && state.view.unlockPersistence === "one_operation";
	// The headerbar back button mirrors whichever way out the current screen
	// already offers, so there is one consistent place to leave any screen
	// instead of a control whose meaning shifts. Suppressed while busy, and on
	// the one-operation code screen, matching the controls it mirrors.
	$: headerBackAction = ((): ModalBackAction | null => {
		if (busy) return null;
		switch (state.screen) {
			case "account-code":
				return oneOperationCode
					? null
					: { label: $t("SteamGuard_Code_ShowAllAccounts"), run: showAllAccounts };
			case "all-accounts":
				return {
					label: pickerReturnAccount ? $t("SteamGuard_Back") : $t("Button_Close"),
					run: leaveAllAccounts,
				};
			case "enrollment":
				// Once Steam holds a pending authenticator, leaving is "close and
				// resume" and nothing behind this screen can undo it. Before that it
				// is an ordinary back, to whichever page sent the user here.
				return enrollmentStage === "recovery" || enrollmentStage === "confirmation"
					? { label: $t("SteamGuard_CloseAndResume"), run: cancelEnrollment }
					: { label: $t("SteamGuard_Back"), run: () => { void cancelSetup(); } };
			case "login-again":
				return {
					label: $t("SteamGuard_Back"),
					run: () => { void cancelCredentialLogin().then(backToAccount); },
				};
			case "login-only":
				return { label: $t("SteamGuard_Code_ShowAllAccounts"), run: showAllAccounts };
			case "login-only-setup":
				return { label: $t("SteamGuard_Back"), run: () => { void cancelSetup(); } };
			case "setup":
				return { label: $t("SteamGuard_Code_ShowAllAccounts"), run: showAllAccounts };
			case "qr":
				// Two ways out, matching the two controls the screen shows: an
				// approval waiting to be answered is cancelled back to the scan
				// view, anything else leaves for the account screen. Dismissing
				// alone stranded the header button on the scan view, where it
				// cleared the status line and then had nothing left to do.
				return qrStage === "approval" && qrApproval
					? { label: $t("SteamGuard_Cancel"), run: () => { void dismissQRLogin(); } }
					: { label: $t("SteamGuard_Back"), run: () => { void dismissQRLogin().then(backToAccount); } };
			case "import":
				// Reachable without an account, from the picker, and that way in
				// needs a way out too.
				return state.account
					? { label: $t("SteamGuard_Back"), run: backFromImport }
					: { label: $t("SteamGuard_Code_ShowAllAccounts"), run: backFromImport };
			case "export-authorize":
			case "recovery":
			case "error":
				return steamGuardAccountForState(state)
					? { label: $t("SteamGuard_Back"), run: backToAccount }
					: null;
			default:
				return null;
		}
	})();
	$: setModalBackAction(headerBackAction);
	// Escape, the backdrop and the close button mean the same thing as that back
	// button, so they answer to the same busy state: a pending unlock can be
	// waiting on a security-key prompt this modal has no way to call off.
	$: setModalBusy(busy);
	onDestroy(() => {
		clearModalBackAction();
		setModalBusy(false);
	});
	// Whose vault record the picker is listed under, and so also whether there is
	// a picker to go back to at all: with nothing to anchor a listing on, the
	// screens that offer it are leaving the modal instead.
	$: listingAnchor = steamGuardListingAnchor(
		[steamGuardAccountForState(state), pickerReturnAccount, account],
		knownSummaries,
	);
	$: enrollmentRetrySeconds = Math.max(0, Math.ceil((enrollmentRetryAt - now) / 1_000));
	$: exportAccount = steamGuardAccountForState(state) ?? account;
	$: exportAccountSummary = summaryOf(exportAccount, false, knownSummaries, switcherProfiles);
	// The unlock screen names one account only when that is the account being
	// unlocked. Narrowed inline on each use: state is a union keyed on screen.
	$: lockedAccountSummary = state.screen === "locked" && !openAllAccountsOnReady
		? summaryOf(state.account, true, knownSummaries, switcherProfiles)
		: null;
	$: if (state.screen === "locked" && !openAllAccountsOnReady) void ensureSwitcherProfile(state.account);

	// The unlock screen has to know a security key is enrolled before it can
	// offer to use one, and only the enrolment path loaded this before.
	let unlockStatusChecked = false;
	$: if (state.screen === "locked" && !unlockStatusChecked) {
		unlockStatusChecked = true;
		void loadVaultStatusForUnlock();
	}

	// Signing in by scanning is offered on the login-only form only. That screen
	// exists to store a session, which is exactly what a scan produces; the
	// enrollment form beside it needs the password itself, to add an
	// authenticator. Started once per account, because re-running it would throw
	// away a code the user may already have their phone pointed at.
	$: showQRLogin = state.screen === "login-only-setup" && qrLoginStage !== "unavailable";
	$: if (state.screen === "login-only-setup" && authStage === "credentials" && state.account) {
		if (qrLoginStartedFor !== state.account.id) {
			qrLoginStartedFor = state.account.id;
			void startQRLogin(state.account, "login_only");
		}
	} else if (qrLoginStartedFor) {
		void stopQRLogin(qrLoginAccount ?? account);
	}

	// Opening straight onto the add screen - from the account list's background
	// menu or the toolbar - skips showAddAccount, which is what mints the attempt
	// and makes sure there is a vault to store anything in. Once prepared the
	// screen carries an account, so this cannot re-enter.
	let addAccountPrepared = false;
	$: if (state.screen === "add-account" && !state.account && !addAccountPrepared) {
		addAccountPrepared = true;
		void showAddAccount();
	}

	async function loadVaultStatusForUnlock(): Promise<void> {
		if (!controller.getSteamGuardVaultStatus) return;
		try {
			vaultStatus = await controller.getSteamGuardVaultStatus();
		} catch (error) {
			// Not knowing only costs the extra affordance; the password still works.
			console.error("Steam Guard: vault status unavailable", error);
		}
	}

	// The code screen shows the same identity as every other screen.
	$: codeAccountSummary = state.screen === "account-code"
		? summaryOf(state.view.account, false, knownSummaries, switcherProfiles)
		: refSummary(account, false);
	$: if (state.screen === "account-code") void ensureSwitcherProfile(state.view.account);

	// The setup screen shows the same identity block as a stored account, from the
	// row that opened it: nothing about this account is in the vault to read.
	$: setupAccountSummary = state.screen === "setup"
		? summaryOf(state.account, false, knownSummaries, switcherProfiles)
		: refSummary(account, false);
	$: if (state.screen === "setup") void ensureSwitcherProfile(state.account);
	// Once per modal: after the vault is created this lands back here, and asking
	// again would only confirm what the creation already settled.
	let setupVaultChecked = false;
	$: if (state.screen === "setup" && !setupVaultChecked) {
		setupVaultChecked = true;
		void ensureVaultForSetup(state.account);
	}
	$: loginOnlyAccountSummary = state.screen === "login-only"
		? summaryOf(state.account, false, knownSummaries, switcherProfiles)
		: refSummary(account, false);
	$: if (state.screen === "login-only") void ensureSwitcherProfile(state.account);
	// Login again is the only recovery this screen offers, so it earns the same
	// highlight it gets on the code screen when the stored session has lapsed.
	$: if (state.screen === "login-only") void checkSessionNeedsLogin(state.account);
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
    removeConfirming = false;
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
    const list = controller.listAccounts;
    const summaries = list
      ? await withCapability(currentAccount, (capability) => list(currentAccount.id, capability))
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

  /** The vault listing's verdict on a record's shape, defaulting to authenticator. */
  function knownKindOf(accountId: string): SteamGuardAccountKind {
    return knownSummaries.find((summary) => summary.id === accountId)?.kind ?? "authenticator";
  }

  /**
   * The single way to land on an account.
   *
   * A login-only record has no shared secret, so getCode fails on it. Every path
   * that opens an account goes through here rather than calling loadAccount, so
   * a new screen never has to remember which of them still take the code path.
   */
  async function openAccountScreen(target: SteamGuardAccountRef): Promise<void> {
    let kind = knownKindOf(target.id);
    // An account we have never listed is not "an authenticator", it is unknown -
    // and defaulting to authenticator sends a secret-less record to getCode,
    // which fails with a dead end. This is the ordinary case straight after
    // adding an account, since that flow never had a reason to list them.
    let listed = knownSummaries.some((summary) => summary.id === target.id);
    if (!listed) {
      try {
        await accountSummaries(target);
        kind = knownKindOf(target.id);
        listed = knownSummaries.some((summary) => summary.id === target.id);
      } catch (error) {
        console.error("Steam Guard: account list could not be refreshed", error);
        // Unknown because the list could not be read, not because the vault is
        // without it. The code path below has its own recovery for that.
        listed = true;
      }
    }
    // Nothing is stored for it, so there is no account screen to open. This is
    // also what makes the setup flows' Back work: they lead here, and an account
    // still not in the vault lands back on the page that offers to add it.
    if (!listed) {
      transition({ type: "show-setup", account: target });
      return;
    }
    // The vault holds it now, so the setup page it may have come from is no
    // longer a place to go back to.
    setupReturn = null;
    if (kind === "login-only") {
      await loadLoginOnlyAccount(target);
      return;
    }
    await loadAccount(target);
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
      // A record with no authenticator cannot answer getCode. Re-read the
      // listing before giving up: if that is all this was, it belongs on the
      // login-only screen, not on an error one.
      let reroute = false;
      try {
        await accountSummaries(nextAccount);
        reroute = knownKindOf(nextAccount.id) === "login-only";
      } catch (listError) {
        console.error("Steam Guard: account list could not be refreshed", listError);
      }
      if (reroute) {
        busy = false;
        await loadLoginOnlyAccount(nextAccount);
        return;
      }
      transition({
        type: "fail",
        account: nextAccount,
        message: $t("SteamGuard_Error_AccountNotLoaded"),
      });
    } finally {
      busy = false;
    }
  }

  /** The login-only counterpart of loadAccount: no code to fetch, so none is asked for. */
  async function loadLoginOnlyAccount(target: SteamGuardAccountRef): Promise<void> {
    if (busy) return;
    transition({ type: "load-account", account: target });
    busy = true;
    try {
      await contentProtection.acquire(target.id);
      await ensureCapability(target);
      const status = await controller.getSteamGuardVaultStatus?.();
      if (status && !status.unlocked) {
        transition({ type: "lock-account", account: target });
        return;
      }
      transition({ type: "show-login-only", account: target });
    } catch (error) {
      console.error("Steam Guard: login-only account could not be opened", error);
      transition({
        type: "fail",
        account: target,
        message: $t("SteamGuard_Error_AccountNotLoaded"),
      });
    } finally {
      busy = false;
    }
  }

  async function pickUnlockKeyfile(): Promise<void> {
    if (!controller.pickKeyfile || busy) return;
    try {
      const chosen = await controller.pickKeyfile();
      if (chosen) unlockKeyfilePath = chosen;
    } catch (error) {
      console.error("Steam Guard: keyfile could not be read", error);
      inlineError = withFailureReason($t("SteamGuard_Unlock_KeyfileRejected"), error);
    }
  }

  /** Unlock with what is filled in. The backend still asks the device if some
   *  enrolled way in needs one, so a key used with a password works from here. */
  async function unlockAccount(): Promise<void> {
    return runUnlock(false);
  }

  /** Unlock by security key alone, which needs nothing typed in. */
  async function unlockWithSecurityKey(): Promise<void> {
    return runUnlock(true);
  }

  async function runUnlock(useSecurityKey: boolean): Promise<void> {
    // A backup key opens the vault on its own, and a security key needs nothing
    // typed at all, so an empty password is valid.
    const hasFactor = unlockKeyfilePath !== "" || unlockBackupKey.trim() !== "" || useSecurityKey;
    if (busy || state.screen !== "locked" || (password.length === 0 && !hasFactor)) return;
    const lockedAccount = state.account;
    busy = true;
    inlineError = "";

    // A login-only record has no code, so controller.unlock - which resolves to a
    // code view - has nothing to return for it. Unlock the vault itself instead.
    //
    // An account we have not listed takes the same path: while the vault is
    // locked its kind is unknowable, and guessing "authenticator" would report a
    // correct password as rejected. Opening the vault first costs one extra step
    // and then openAccountScreen can read the real kind.
    const kindUnknown = !knownSummaries.some((summary) => summary.id === lockedAccount.id);
    if ((kindUnknown || knownKindOf(lockedAccount.id) === "login-only") && controller.unlockSteamGuardVault) {
      try {
        const capability = await ensureCapability(lockedAccount);
        await controller.unlockSteamGuardVault(
          lockedAccount.id, password, rememberForSession, capability, unlockKeyfilePath, unlockBackupKey.trim(),
        );
        password = "";
        unlockBackupKey = "";
        unlockKeyfilePath = "";
        busy = false;
        await openAccountScreen(lockedAccount);
        return;
      } catch (error) {
        console.error("Steam Guard: vault unlock was rejected", error);
        password = "";
        inlineError = $t("SteamGuard_Error_PasswordRejected");
        focusCurrentScreen();
      } finally {
        busy = false;
      }
      return;
    }

    let pending: Promise<import("../../lib/steamGuardModal").SteamGuardCodeView>;
    try {
		const capability = await ensureCapability(lockedAccount);
		pending = hasFactor && controller.unlockWithFactors
			? controller.unlockWithFactors(
				lockedAccount.id, password, unlockKeyfilePath, unlockBackupKey.trim(), rememberForSession, capability,
			)
			: controller.unlock(lockedAccount.id, password, rememberForSession, capability);
    } catch (error) {
      console.error("Steam Guard: unlock could not start", error);
      password = "";
      busy = false;
      inlineError = $t("SteamGuard_Error_UnlockStartFailed");
      focusCurrentScreen();
      return;
    }
    password = "";
    unlockBackupKey = "";

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
		// This one runs itself, every thirty seconds, so a superseded capability
		// here does not wait for the user to do anything: it drops a working code
		// screen to an error screen on its own.
		const view = await withCapability(currentAccount,
			(capability) => controller.getCode(currentAccount.id, capability));
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
      const copy = controller.copyCode;
      if (!copy) throw new Error("Secure clipboard unavailable");
      const target = state.view.account;
      await withCapability(target, async (capability) => { await copy(target.id, capability); });
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

  /**
   * Picker rows carry the switcher's avatar and community name, which for an
   * account added seconds ago are still downloading. The account list emits a
   * patch when they land, so the picker follows instead of showing the login
   * name against a blank image until it is reopened.
   */
  async function refreshPickerRows(): Promise<void> {
    if (state.screen !== "all-accounts" || busy || !pickerIdentity) return;
    try {
      const rows = await accountSummaries(pickerIdentity);
      if (state.screen === "all-accounts") transition({ type: "show-all", accounts: rows });
    } catch (error) {
      // What is on screen is still correct, only its images are stale.
      console.error("Steam Guard: account rows could not be refreshed", error);
    }
  }

  /** Coalesced: a refreshed account emits several patches as each asset lands. */
  function schedulePickerRefresh(): void {
    if (pickerRefreshTimer) clearTimeout(pickerRefreshTimer);
    pickerRefreshTimer = setTimeout(() => {
      pickerRefreshTimer = undefined;
      void refreshPickerRows();
    }, 400);
  }

  async function showAllAccounts(): Promise<void> {
    if (busy) return;
    // Not the screen's own account: the add screen's is a pending attempt and
    // the setup page's is an account the vault does not hold, and listing under
    // either is refused. With no anchor at all there is no list to show, and
    // nothing behind this screen either - the way out is out of the modal.
    const anchor = listingAnchor;
    if (!anchor) {
      dismissModal();
      return;
    }
    busy = true;
    try {
      // Opened from an account, so leaving the picker goes back to that account.
      // Screens holding no account of their own are not places to go back to:
      // the setup page is a placeholder, an import started from the picker has
      // nothing behind it but the picker itself, and the add screen holds either
      // a pending attempt or an account created seconds ago.
      pickerReturnAccount = state.screen === "setup" || state.screen === "add-account"
        ? null
        : steamGuardAccountForState(state) ?? null;
      pickerIdentity = anchor;
      transition({ type: "show-all", accounts: await accountSummaries(anchor) });
    } catch (error) {
      console.error("Steam Guard: account list could not be loaded", error);
      transition({ type: "fail", message: $t("SteamGuard_Error_AccountsNotLoaded") });
    } finally {
      busy = false;
    }
  }

  /**
   * Deletes a login-only vault record. Only ever offered for that kind: an
   * authenticator's secrets exist nowhere else, and Go refuses one regardless.
   * The account's stored CS2 cooldown is left behind on purpose - it describes
   * the account, not the vault record.
   */
  async function confirmRemoveLoginOnly(): Promise<void> {
    if (state.screen !== "login-only" || busy || !controller.removeLoginOnlyAccount) return;
    const target = state.account;
    busy = true;
    try {
      const capability = await ensureCapability(target);
      await controller.removeLoginOnlyAccount(target.id, capability);
      removeConfirming = false;
      announce($t("SteamGuard_LoginOnly_Removed"));
      // The record is gone, so returning to it would fail; show what is left.
      // The controller republishes toolbar availability for us.
      pickerReturnAccount = null;
      busy = false;
      await showAllAccounts();
      return;
    } catch (error) {
      console.error("Steam Guard: login-only account could not be removed", error);
      removeConfirming = false;
      inlineError = $t("SteamGuard_Error_LoginOnlyRemoveFailed");
      busy = false;
      return;
    }
    busy = false;
  }

  /** Leaves the account picker the way the user entered it. */
  function leaveAllAccounts(): void {
    if (pickerReturnAccount) {
      void openAccountScreen(pickerReturnAccount);
      return;
    }
    dismissModal();
  }

  async function showConfirmations(): Promise<void> {
		const openWindow = controller.openConfirmations;
		if (state.screen !== "account-code" || !openWindow) return;
		const currentAccount = state.view.account;
		try {
			await withCapability(currentAccount, async (capability) => {
				await openWindow(currentAccount.id, capability);
			});
    } catch (error) {
      reportFailure($t("SteamGuard_Error_ConfirmationsOpenFailed"), error);
    }
  }

  // Opens a browser window signed in as the account this modal is showing. It
  // works from both the authenticator and the session-only screen, which hold
  // the same tokens; only the id differs by screen.
  async function openBrowser(site: SteamBrowserSite): Promise<void> {
    const open = controller.openSteamBrowser;
    if (!open || busy || openingBrowserSite) return;
    const account =
      state.screen === "account-code" ? state.view.account :
      state.screen === "login-only" ? loginOnlyAccountSummary : null;
    if (!account?.id) return;
    // Set before the first await, so a double-click is refused by the guard
    // above rather than by a disabled attribute Svelte has not flushed yet.
    openingBrowserSite = site;
    try {
      const result = await withCapability(account, (capability) => open(account.id, site, capability));
      // Opening a window renews a lapsed session, and that write rotates the
      // vault generation this modal's capability is bound to. Without this the
      // next thing the user clicked failed instead.
      await refreshCapabilityIfRequired(account, result?.capabilityRefreshRequired === true);
      // A session too old to renew is not an error to read and dismiss; the
      // only thing to do about it is sign in, so go straight there - but the
      // modal stayed usable throughout, so only take it over if it is still on
      // the account this open started from and nothing else is running.
      // startLoginAgain bails while busy, which would strand its refreshing
      // screen with no work behind it.
      if (result?.needsLogin) {
        if (!busy && steamGuardAccountForState(state)?.id === account.id) showLoginAgainState();
        return;
      }
    } catch (error) {
      reportFailure($t("SteamGuard_Error_BrowserOpenFailed"), error);
    } finally {
      openingBrowserSite = null;
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
      // These actions close this modal to run their own dialogs, so a failure
      // has no screen of ours left to land on.
      reportFailure($t("SteamGuard_Error_ActionFailed"), error);
    } finally {
      busy = false;
    }
  }

  function backToAccount(): void {
    const currentAccount = steamGuardAccountForState(state) ?? account;
    void openAccountScreen(currentAccount);
  }

	/** One click: refresh the saved session, and only ask for a password if Steam rejects it. */
	function showLoginAgainState(): void {
		if (state.screen !== "account-code" && state.screen !== "login-only") return;
		const currentAccount = steamGuardAccountForState(state);
		if (!currentAccount) return;
		transition({ type: "show-login-again", account: currentAccount }, false);
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

  /**
   * The three ways to start holding an account, from the setup screen. Each one
   * hands off to the flow that already exists for it; this screen only chooses.
   */
  function startSetupLoginOnly(): void {
    if (state.screen !== "setup" || busy) return;
    setupReturn = state.account;
    transition({ type: "show-login-only-setup", account: state.account });
    void startLoginOnlySetup();
  }

  function startSetupImport(): void {
    if (state.screen !== "setup" || busy) return;
    setupReturn = state.account;
    transition({ type: "show-import", account: state.account });
  }

  function startSetupEnrollment(): void {
    if (state.screen !== "setup" || busy) return;
    setupReturn = state.account;
    transition({ type: "show-enrollment", account: state.account });
    void startEnrollment();
  }

  /**
   * Back to the page that offered this flow, when that is where it came from.
   *
   * Remembered rather than worked out on the way back: working it out means
   * listing the vault, and these screens are reachable with the vault still
   * locked - the one state a listing cannot answer in.
   */
  function returnToSetup(): boolean {
    const target = setupReturn;
    if (!target) return false;
    setupReturn = null;
    transition({ type: "show-setup", account: target });
    return true;
  }

  /**
   * Adding an account by signing in. The picker only renders with the vault
   * open, so this needs no vault gate of its own; the entry points that can be
   * reached without one prepare the vault before the modal opens.
   */
  async function showAddAccount(): Promise<void> {
    if (busy || !controller.newAddAccountAttempt) return;
    setupReturn = null;
    clearAuthTimer();
    clearAuthSecrets();
    authAccountName = "";
    authHandle = "";
    authChallenge = "";
    authResult = null;
    storedDeviceCodeTried = false;
    authStage = "idle";
    authMessage = "";
    inlineError = "";
    busy = true;
    try {
      // Minted once per visit rather than per button press: it is the identity
      // this screen runs under, including for the vault form below, which needs
      // something a capability can be bound to and has no account to use.
      const pendingId = await controller.newAddAccountAttempt();
      const pendingRef = { id: pendingId, username: "" };
      // A Steam password, and possibly a vault password, are about to be typed
      // here, so the capture guard goes up before the form does. The pending id
      // is what it binds to; there is no account yet.
      await contentProtection.acquire(pendingId);
      const status = await controller.getSteamGuardVaultStatus?.();
      if (status && (!status.configured || !status.unlocked)) {
        // This is the one screen reachable with no vault at all - it is how the
        // first account gets in - so it has to be able to create or open one.
        vaultSetupOnly = true;
        vaultSetupForAddAccount = true;
        vaultStatus = status;
        rememberForSession = status.rememberForSession;
        enrollmentStage = "vault";
        transition({ type: "show-enrollment", account: pendingRef });
        return;
      }
      transition({ type: "show-add-account", account: pendingRef });
    } catch (error) {
      console.error("Steam Guard: add-account screen could not be opened", error);
      transition({ type: "fail", message: withFailureReason($t("SteamGuard_Error_SignInStartFailed"), error) });
    } finally {
      busy = false;
      focusCurrentScreen();
    }
  }

  /**
   * Both buttons run one sign-in; the purpose only decides what is stored at the
   * end of it. A fresh attempt id is minted per press because Go issues one per
   * sign-in, and a second press must not reuse the first one id.
   */
  async function beginAddAccount(purpose: SteamAuthPurpose): Promise<void> {
    if (busy || state.screen !== "add-account") return;
    if (!authAccountName.trim() || !authPassword) return;
    if (!controller.newAddAccountAttempt) return;
    busy = true;
    inlineError = "";
    authPurpose = purpose;
    let pendingId = state.account?.id ?? "";
    try {
      // Normally the attempt from showAddAccount; a fresh one only when a
      // previous sign-in on this screen already spent it.
      if (!pendingId) pendingId = await controller.newAddAccountAttempt();
    } catch (error) {
      console.error("Steam Guard: add-account attempt could not be started", error);
      authStage = "error";
      authMessage = withFailureReason($t("SteamGuard_Error_SignInStartFailed"), error);
      busy = false;
      return;
    }
    const named = authAccountName.trim();
    // From here the flow is identical to any other credential sign-in; the
    // pending id stands in for the SteamID64 nobody knows yet.
    transition({ type: "show-add-account", account: { id: pendingId, username: named, displayName: named } });
    authStage = "credentials";
    busy = false;
    await beginCredentialLogin();
  }

  /** An maFile names its own account, so this one needs no account chosen first. */
  function importFromAllAccounts(): void {
    if (state.screen !== "all-accounts" || busy) return;
    setupReturn = null;
    transition({ type: "show-import" });
  }

  /** Import has three ways in - the setup page, an account, the picker. */
  function backFromImport(): void {
    if (state.screen !== "import" || busy) return;
    if (returnToSetup()) return;
    const target = state.account;
    if (target) {
      void openAccountScreen(target);
      return;
    }
    void showAllAccounts();
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

	function clearQRLoginTimer(): void {
		if (qrLoginPollTimer) clearTimeout(qrLoginPollTimer);
		qrLoginPollTimer = undefined;
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
		vaultBackupKey = "";
		vaultKeyfilePath = "";
		exportPassword = "";
	}

	function showCredentialForm(purpose: SteamAuthPurpose, currentAccount: SteamGuardAccountRef): void {
		clearAuthTimer();
		credentialsHint = "";
		authPurpose = purpose;
		authStage = "credentials";
		authAccountName = currentAccount.username;
		authHandle = "";
		authChallenge = "";
		authMessage = "";
		authResult = null;
		storedDeviceCodeTried = false;
		// Login-only shares the enrollment screen's credential stages but none of
		// its authenticator stages, so it must not inherit a stale enrollmentStage.
		if (purpose === "add_authenticator" || purpose === "login_only") enrollmentStage = "idle";
		clearAuthSecrets();
		focusCurrentScreen();
	}

	/**
	 * Sign-in that stores the session and nothing else. It reuses the enrollment
	 * screen's vault and credential stages; the authenticator stages never fire
	 * because enrollmentStage stays idle for this purpose.
	 */
	async function startLoginOnlySetup(): Promise<void> {
		if (state.screen !== "login-only-setup" || busy) return;
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
			enrollmentStage = "idle";
			authMessage = "";
			showCredentialForm("login_only", currentAccount);
		} catch (error) {
			console.error("Steam Guard: login-only sign-in could not be started", error);
			enrollmentStage = "error";
			authMessage = $t("SteamGuard_Error_SetupResumeFailed");
		} finally {
			busy = false;
		}
	}

	/**
	 * Back out of whichever setup screen is showing, to the page that offered it.
	 * Only for the stages before Steam has been asked for anything - once an
	 * enrollment is pending, leaving means "close and resume", not "back".
	 */
	async function cancelSetup(): Promise<void> {
		if (busy) return;
		await cancelCredentialLogin();
		clearAuthSecrets();
		enrollmentStatus = null;
		vaultStatus = null;
		enrollmentStage = "idle";
		authStage = "idle";
		authMessage = "";
		promotingLoginOnly = false;
		vaultSetupOnly = false;
		if (returnToSetup()) return;
		// Nothing behind it, which is every way in that is not the setup page.
		dismissModal();
	}

	function restartSetup(): void {
		if (state.screen === "login-only-setup") {
			void startLoginOnlySetup();
			return;
		}
		void startEnrollment();
	}

	/**
	 * Turns a login-only account into a full Steam Guard one.
	 *
	 * The record already holds the session a fresh sign-in would produce, so Go
	 * starts the enrollment from it and this lands straight on the screen asking
	 * for Steam's confirmation code — no password, no second 2FA ceremony. Only a
	 * session Steam will no longer accept falls back to the credential form, and
	 * that is the same form the ordinary add flow uses, with the name filled in.
	 */
	async function promoteLoginOnly(): Promise<void> {
		if (state.screen !== "login-only" || busy || !controller.promoteLoginOnlyAccount) return;
		const currentAccount = state.account;
		transition({ type: "show-enrollment", account: currentAccount });
		promotingLoginOnly = true;
		busy = true;
		inlineError = "";
		authStage = "idle";
		enrollmentStage = "checking";
		authMessage = $t("SteamGuard_Promote_Starting");
		try {
			// The vault can have relocked since this account was opened, and the
			// stored session cannot be read through a locked one. Asking here keeps
			// the unlock inline instead of failing the promotion outright.
			const status = await controller.getSteamGuardVaultStatus?.();
			if (status && (!status.configured || !status.unlocked)) {
				vaultStatus = status;
				rememberForSession = status.rememberForSession;
				enrollmentStage = "vault";
				authMessage = "";
				focusCurrentScreen();
				return;
			}
			await runLoginOnlyPromotion(currentAccount);
		} catch (error) {
			failLoginOnlyPromotion(error);
		} finally {
			busy = false;
		}
	}

	async function runLoginOnlyPromotion(currentAccount: SteamGuardAccountRef): Promise<void> {
		if (!controller.promoteLoginOnlyAccount) return;
		authMessage = $t("SteamGuard_Promote_Starting");
		const capability = await ensureCapability(currentAccount);
		const promotion = await controller.promoteLoginOnlyAccount(currentAccount.id, capability);
		promotingLoginOnly = false;
		await refreshCapabilityIfRequired(currentAccount, promotion.capabilityRefreshRequired);
		if (promotion.needsLogin) {
			showCredentialForm("add_authenticator", currentAccount);
			credentialsHint = $t("SteamGuard_Promote_SignInAgain");
			return;
		}
		if (!promotion.enrollment) throw new Error("Enrollment status unavailable");
		await prepareEnrollment(currentAccount, promotion.enrollment);
	}

	/**
	 * Leaves the flag cleared, so Try Again takes the ordinary add-authenticator
	 * route. That route enrolls over a login-only record too, and asking for a
	 * password beats retrying whatever Steam just refused.
	 */
	function failLoginOnlyPromotion(error: unknown): void {
		console.error("Steam Guard: login-only account could not be promoted", error);
		promotingLoginOnly = false;
		enrollmentStage = "error";
		authMessage = withFailureReason($t("SteamGuard_Error_PromoteFailed"), error);
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

	/**
	 * Every way the setup page offers to store an account needs an open vault, so
	 * one that is missing or locked is dealt with before the choice rather than
	 * after it. Picking an maFile, choosing the file, and only then being asked
	 * for the vault password means doing the whole thing twice.
	 *
	 * The same form covers both: it creates a vault or opens one, per its status.
	 */
	async function ensureVaultForSetup(currentAccount: SteamGuardAccountRef): Promise<void> {
		if (busy || !controller.getSteamGuardVaultStatus) return;
		busy = true;
		try {
			const status = await controller.getSteamGuardVaultStatus();
			if (status.configured && status.unlocked) return;
			vaultSetupOnly = true;
			vaultStatus = status;
			rememberForSession = status.rememberForSession;
			authStage = "idle";
			enrollmentStage = "vault";
			authMessage = "";
			transition({ type: "show-enrollment", account: currentAccount });
		} catch (error) {
			// Not knowing costs nothing here: the page still works, and the flow
			// the user picks checks the vault for itself.
			console.error("Steam Guard: vault status unavailable for setup", error);
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
		if (busy || enrollmentStage !== "vault" || !vaultStatus) return;
		if (state.screen !== "enrollment" && state.screen !== "login-only-setup") return;
		// A backup key opens the vault alone and a security key needs nothing
		// typed at all, so an empty password is valid when unlocking. Creating a
		// vault still requires one.
		const hasVaultFactor = vaultKeyfilePath !== "" || vaultBackupKey.trim() !== "";
		if (!vaultPassword && (!vaultStatus.configured || !hasVaultFactor)) return;
		const currentAccount = steamGuardAccountForState(state) ?? account;
		const forLoginOnly = state.screen === "login-only-setup";
		if (!vaultStatus.configured) {
			const policyError = validateNewPassword(vaultPassword);
			if (policyError) {
				inlineError = passwordPolicyMessage(policyError, $t);
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
		// Creating a vault moves it to a new generation, and capabilities are
		// bound to the generation they were issued against. This modal takes a
		// capability when it opens, which for a first-time setup is before any
		// vault exists, so that one is stale the moment the vault is created.
		const vaultWasCreated = !vaultStatus.configured;
		let pending: Promise<void>;
		try {
			if (vaultStatus.configured) {
				if (!controller.unlockSteamGuardVault) throw new Error("Steam Guard vault unlock unavailable");
				pending = controller.unlockSteamGuardVault(
					currentAccount.id,
					vaultPassword,
					rememberForSession,
					await ensureCapability(currentAccount),
					vaultKeyfilePath,
					vaultBackupKey.trim(),
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
		if (vaultWasCreated) {
			try {
				await contentProtection.acquire(currentAccount.id);
			} catch (error) {
				console.error("Steam Guard: capability could not be refreshed after vault creation", error);
				inlineError = $t("SteamGuard_Error_SetupContinueFailed");
				busy = false;
				return;
			}
		}
		vaultStatus = null;
		// The vault is open; continue into whichever flow was started. Login
		// only stores a session and adds no authenticator, so resuming
		// enrollment here would drop the user into the wrong ceremony.
		if (forLoginOnly) {
			enrollmentStage = "idle";
			authMessage = "";
			busy = false;
			showCredentialForm("login_only", currentAccount);
			return;
		}
		// The vault was the whole errand - it was missing or locked, and nothing
		// has been chosen for this account yet. Hand back the page that asks.
		if (vaultSetupOnly) {
			vaultSetupOnly = false;
			enrollmentStage = "idle";
			authMessage = "";
			busy = false;
			if (vaultSetupForAddAccount) {
				vaultSetupForAddAccount = false;
				// currentAccount is the pending attempt this screen runs under, so
				// the sign-in below continues on the identity the vault was just
				// opened against.
				transition({ type: "show-add-account", account: currentAccount });
				return;
			}
			transition({ type: "show-setup", account: currentAccount });
			return;
		}
		if (promotingLoginOnly) {
			enrollmentStage = "checking";
			try {
				await runLoginOnlyPromotion(currentAccount);
			} catch (error) {
				failLoginOnlyPromotion(error);
			} finally {
				busy = false;
			}
			return;
		}
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
		confirmationError = "";
		// The sign-in is finished once enrollment takes over. The template
		// tests authStage before enrollmentStage, so leaving it on "polling"
		// keeps the sign-in progress screen on top of whichever enrollment
		// screen is chosen below - including the one asking for the code Steam
		// has just sent. Branches that genuinely need a sign-in screen set this
		// again themselves.
		authStage = "idle";
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
		// These are refusals, not guidance, so they go to the error line rather
		// than replacing the instructions that tell the user what to type.
		if (status.state === "confirmation_code_rejected") {
			confirmationError = $t("SteamGuard_Error_ConfirmationCodeRejected");
		} else if (status.state === "authenticator_code_retry") {
			confirmationError = $t("SteamGuard_Error_AuthenticatorRetry");
		} else if (status.state === "rate_limited") {
			confirmationError = $t("SteamGuard_Error_RateLimitedRetry");
		}
		enrollmentStage = "confirmation";
		focusCurrentScreen();
	}

	async function beginCredentialLogin(): Promise<void> {
		if (busy || authAccountName.trim().length === 0 || authPassword.length === 0) return;
		const currentAccount = steamGuardAccountForState(state) ?? account;
		const begin = credentialApi(currentAccount).begin;
		if (!begin) return;
		busy = true;
		authMessage = $t("SteamGuard_Challenge_SigningIn");
		let pending: Promise<SteamCredentialResult>;
		try {
			pending = begin(
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
	 * a definite answer moves the verdict, and every failure leaves it undecided.
	 *
	 * The verdict is published after the local step rather than at the end, so what
	 * it gates does not wait on a request to Steam for an answer already known
	 * offline. The probe can still turn it down afterwards.
	 */
	async function checkSessionNeedsLogin(currentAccount: SteamGuardAccountRef): Promise<void> {
		if (!currentAccount.id || sessionCheckedAccount === currentAccount.id) return;
		sessionCheckedAccount = currentAccount.id;
		sessionVerdict = "unknown";

		let capability = "";
		try {
			capability = await ensureCapability(currentAccount);
		} catch (error) {
			console.warn("Steam Guard: session state needs a capability", error);
			return;
		}
		try {
			// Renew before judging. An account left alone for a day has a lapsed
			// access token and a refresh token still good for months, so asking the
			// stored session alone reports a sign-in the user does not need.
			const renewed = controller.ensureFreshSession
				? await controller.ensureFreshSession(currentAccount.id, capability)
				: undefined;
			if (renewed?.capabilityRefreshRequired) {
				// The renewal wrote to the vault, so the generation moved and this
				// capability with it; the probe below needs the new one.
				await refreshCapabilityIfRequired(currentAccount, true);
				capability = capabilityFor(currentAccount);
			}
			const local = renewed ?? await controller.steamSessionLocalState?.(currentAccount.id, capability);
			if (local) sessionVerdict = local.needsLogin ? "needs-login" : "valid";
		} catch (error) {
			console.warn("Steam Guard: stored session could not be read", error);
		}
		if (sessionVerdict === "needs-login") return;
		try {
			const probed = await controller.probeSteamSession?.(currentAccount.id, capability);
			if (probed) sessionVerdict = probed.needsLogin ? "needs-login" : "valid";
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
		sessionVerdict = "unknown";
		authStage = "success";
		successCountdown = LOGIN_SUCCESS_SECONDS;
		successTimer = setInterval(() => {
			successCountdown -= 1;
			if (successCountdown > 0) return;
			clearAuthTimer();
			authStage = "idle";
			void openAccountScreen(currentAccount);
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
			// The add ran under a pending id. Everything from here is keyed on the
			// account Steam named - including the enrollment steps below, which
			// only ever accept a real SteamID64 - so swap identity and take a
			// capability for it, whether or not the result asked for a refresh.
			const rekeyed = isPendingAccountId(currentAccount.id) && !!result.steamId64;
			if (rekeyed) {
				const named = authAccountName.trim();
				currentAccount = { id: result.steamId64 as string, username: named, displayName: named };
			}
			await refreshCapabilityIfRequired(currentAccount, result.capabilityRefreshRequired || rekeyed);
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
			if (rekeyed) {
				// A freshly added account: the list is where it can be seen to have
				// arrived, and where another can be added straight after. Re-point
				// the screen at the named account first - the pending attempt was
				// spent by the poll, so nothing can be authorised under it again.
				clearAuthSecrets();
				authStage = "idle";
				transition({ type: "show-add-account", account: currentAccount });
				await showAllAccounts();
				return;
			}
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

	/**
	 * Opens the scan-to-sign-in code for the account whose form is on screen.
	 *
	 * Failure is deliberately quiet. This is the second of two ways to sign in on
	 * the same screen, so a Steam that will not open a QR session, or a build
	 * whose backend has no such call, should leave the user with the password
	 * form rather than an error over the top of it.
	 */
	async function startQRLogin(currentAccount: SteamGuardAccountRef, purpose: SteamAuthPurpose): Promise<void> {
		const begin = controller.beginQRLogin;
		if (!begin || !currentAccount.id) {
			qrLoginStage = "unavailable";
			return;
		}
		clearQRLoginTimer();
		qrLoginAccount = currentAccount;
		qrLoginStage = "starting";
		qrLoginImage = "";
		qrLoginHandle = "";
		// Steam holds one scan session per account, so the previous code has to
		// be closed before the next can open. Leaving and coming straight back
		// used to race that cancel and be refused as a conflict, which left the
		// screen with no code on it at all.
		if (qrLoginClosing) {
			await qrLoginClosing;
			qrLoginClosing = null;
		}
		try {
			const result = await begin(currentAccount.id, await ensureCapability(currentAccount), purpose);
			handleQRResult(currentAccount, result);
		} catch (error) {
			console.warn("Steam Guard: the sign-in QR code could not be opened", error);
			qrLoginStage = "unavailable";
		}
	}

	function handleQRResult(currentAccount: SteamGuardAccountRef, result: SteamCredentialResult): void {
		qrLoginHandle = result.handle || qrLoginHandle;
		// Steam replaces the code while it waits, so every poll can bring a new
		// one. Only overwrite when there is a replacement: a poll that carries no
		// image must not blank the one on screen.
		if (result.qrImage) qrLoginImage = result.qrImage;
		const step = steamCredentialStep(result);
		if (step === "complete") {
			clearQRLoginTimer();
			qrLoginStage = "idle";
			qrLoginHandle = "";
			qrLoginImage = "";
			void handleCredentialResult(currentAccount, result);
			return;
		}
		if (step === "failed" || (!result.canPoll && !result.qrImage && !qrLoginImage)) {
			clearQRLoginTimer();
			qrLoginStage = "unavailable";
			qrLoginHandle = "";
			return;
		}
		qrLoginStage = "waiting";
		const delay = Math.max(750, Math.min(result.pollAfterMillis || 2_000, 10_000));
		qrLoginPollTimer = setTimeout(() => void pollQRLogin(currentAccount), delay);
	}

	async function pollQRLogin(currentAccount: SteamGuardAccountRef): Promise<void> {
		const poll = controller.pollQRLogin;
		if (!poll || !qrLoginHandle) return;
		const handle = qrLoginHandle;
		try {
			const result = await poll(currentAccount.id, await ensureCapability(currentAccount), handle);
			// The screen moved on while this was in flight - the password form
			// finished, or the user left - so this answer is about a session that
			// no longer matters.
			if (qrLoginHandle !== handle) return;
			handleQRResult(currentAccount, result);
		} catch (error) {
			console.warn("Steam Guard: the sign-in QR code could not be checked", error);
			if (qrLoginHandle !== handle) return;
			clearQRLoginTimer();
			qrLoginStage = "unavailable";
			qrLoginHandle = "";
		}
	}

	/** Ends the scan session, so a code nobody used does not outlive its screen. */
	async function stopQRLogin(currentAccount: SteamGuardAccountRef): Promise<void> {
		clearQRLoginTimer();
		const handle = qrLoginHandle;
		qrLoginHandle = "";
		qrLoginImage = "";
		qrLoginStage = "idle";
		qrLoginStartedFor = "";
		qrLoginAccount = null;
		const cancel = controller.cancelQRLogin;
		if (!handle || !cancel) return;
		const capability = capabilityFor(currentAccount);
		if (!capability) return;
		qrLoginClosing = cancel(currentAccount.id, capability, handle).catch((error: unknown) => {
			console.warn("Steam Guard: the sign-in QR code could not be closed", error);
		});
		await qrLoginClosing;
	}

	async function submitCredentialCode(): Promise<void> {
		if (busy || !authHandle || !authChallenge || !authCode) return;
		const currentAccount = steamGuardAccountForState(state) ?? account;
		const submit = credentialApi(currentAccount).submit;
		if (!submit) return;
		busy = true;
		let pending: Promise<SteamCredentialResult>;
		try {
			pending = submit(
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
		if (busy || !authHandle) return;
		const currentAccount = steamGuardAccountForState(state) ?? account;
		const poll = credentialApi(currentAccount).poll;
		if (!poll) return;
		busy = true;
		try {
			await handleCredentialResult(
				currentAccount,
				await poll(
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
		await stopQRLogin(qrLoginAccount ?? currentAccount);
		const handle = authHandle;
		authHandle = "";
		clearAuthSecrets();
		const cancel = credentialApi(currentAccount).cancel;
		if (handle && cancel) {
			const capability = capabilityFor(currentAccount);
			if (capability) {
				await cancel(currentAccount.id, capability, handle).catch((error: unknown) => {
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
		confirmationError = "";
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
			confirmationError = withFailureReason($t("SteamGuard_Error_ConfirmationSubmitFailed"), error);
			return;
		}
		confirmationCode = "";
		try {
			const status = await pending;
			await refreshCapabilityIfRequired(currentAccount, status.capabilityRefreshRequired);
			// No backup dialog from here: it replaces this modal, so it destroyed
			// the completion screen the moment that screen appeared. The screen
			// carries the reminder itself and offers the folder on demand.
			await prepareEnrollment(currentAccount, status);
		} catch (error) {
			console.error("Steam Guard: confirmation code was rejected", error);
			confirmationError = withFailureReason($t("SteamGuard_Error_ConfirmationRejected"), error);
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
				hasSecurityKey: false,
				passwordOpens: true,
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
			const scan = controller.captureQrFromSteam;
			await withCapability(currentAccount, (capability) =>
				scan(currentAccount.id, capability).then((result) =>
					handleQRScanResult(currentAccount, capability, result)));
		} catch (error) {
			console.error("Steam Guard: Steam could not be scanned for a QR code", error);
			qrStage = "error";
			qrMessage = $t("SteamGuard_QR_ScanFailed");
		} finally {
			busy = false;
		}
  }

  async function chooseQrScreenshot(): Promise<void> {
    if (state.screen !== "qr" || busy || !controller.pickQrScreenshot || !controller.decodeQrScreenshot) return;
		const currentAccount = state.account;
		const decode = controller.decodeQrScreenshot;
		busy = true;
		qrStage = "scanning";
		qrMessage = $t("SteamGuard_QR_ReadingScreenshot");
		try {
			// The picker runs outside withCapability so a retry re-reads the file
			// already chosen rather than asking for it again. It needs no
			// capability of its own: the image never leaves Go until the decode,
			// which is the call that checks one.
			const path = await controller.pickQrScreenshot();
			if (!path) {
				resetQRState();
				return;
			}
			await withCapability(currentAccount, (capability) =>
				decode(currentAccount.id, path, capability).then((result) =>
					handleQRScanResult(currentAccount, capability, result)));
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
		const decode = controller.decodeQrScreenshot;
		busy = true;
		qrStage = "scanning";
		qrMessage = $t("SteamGuard_QR_ReadingDropped");
		try {
			await withCapability(currentAccount, (capability) =>
				decode(currentAccount.id, path, capability).then((result) =>
					handleQRScanResult(currentAccount, capability, result)));
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
			// The only QR flow that does not go through withCapability. Its capability
			// is checked again after the drag, so a retry would put the overlay back up
			// and ask for the same box a second time - worse than saying so. Sweeps no
			// longer rotate a live capability out from under it (carryCapabilitiesAcross
			// in Go), which is what used to land here.
			const capability = await ensureCapability(currentAccount);
			await handleQRScanResult(
				currentAccount,
				capability,
				await controller.selectQrRegion(currentAccount.id, capability),
			);
		} catch (error) {
			console.error("Steam Guard: screen region could not be scanned", error);
			qrStage = "error";
			// Not the same failure: the region was read fine, the vault moved under it.
			// "Could not be scanned safely" sends the user off inspecting their screen.
			qrMessage = isStaleCapabilityError(error)
				? $t("SteamGuard_QR_VaultChanged")
				: $t("SteamGuard_QR_RegionFailed");
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
	 * One click: renew the saved session, and only ask for a password if that fails.
	 *
	 * Steam's access token lapses in about a day while the refresh token stored
	 * beside it lasts months, so most arrivals here need no password at all. Going
	 * straight to the form asked for one every day and told the user their refresh
	 * token had been rejected without ever having offered it to Steam.
	 *
	 * A renewal can still mint a token for a session Steam goes on to refuse, so
	 * success is confirmed against Steam before it is reported.
	 */
	async function startLoginAgain(): Promise<void> {
		if (state.screen !== "login-again" || busy) return;
		const currentAccount = state.account;
		const renew = controller.loginAgain;
		if (!renew) {
			showCredentialForm("login_again", currentAccount);
			authMessage = $t("SteamGuard_LoginAgain_TokenRejected");
			return;
		}
		busy = true;
		authStage = "refreshing";
		authMessage = $t("SteamGuard_LoginAgain_Refreshing");
		try {
			const result = await renew(currentAccount.id, await ensureCapability(currentAccount));
			await refreshCapabilityIfRequired(currentAccount, result.capabilityRefreshRequired);
			if (steamLoginAgainNextStep(result) === "done" && await renewedSessionWorks(currentAccount)) {
				announce($t("SteamGuard_LoginAgain_RefreshedAnnounce"));
				startLoginSuccessCountdown(currentAccount);
				return;
			}
			showCredentialForm("login_again", currentAccount);
			// Steam refusing the renewed session is a different fact from Steam
			// refusing the refresh token, and the user can act on the difference.
			authMessage = $t(result.state === "refreshed"
				? "SteamGuard_LoginAgain_RefreshFailed"
				: "SteamGuard_LoginAgain_TokenRejected");
		} catch (error) {
			console.error("Steam Guard: the saved Steam session could not be renewed", error);
			showCredentialForm("login_again", currentAccount);
			authMessage = $t("SteamGuard_LoginAgain_RefreshFailed");
		} finally {
			busy = false;
		}
	}

	/**
	 * A renewed access token is not proof the session works — Steam can issue one
	 * and still refuse to use it. Only a definite refusal counts against it: an
	 * unavailable or failing probe leaves the renewal standing rather than sending
	 * the user to a password form on no evidence.
	 */
	async function renewedSessionWorks(currentAccount: SteamGuardAccountRef): Promise<boolean> {
		const probe = controller.probeSteamSession;
		if (!probe) return true;
		try {
			const probed = await probe(currentAccount.id, capabilityFor(currentAccount));
			return probed?.needsLogin !== true;
		} catch (error) {
			console.warn("Steam Guard: the renewed session could not be checked with Steam", error);
			return true;
		}
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
			await openAccountScreen(currentAccount);
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
			offAccountUpdated = Events.On("steam-account-updated", () => {
				if (state.screen !== "all-accounts") return;
				schedulePickerRefresh();
			});
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
				void openAccountScreen(account);
			} else if (entry === "add-account") {
				// Nothing to bind a capability to yet: the id arrives with the
				// attempt, which the reactive gate mints. Acquiring here with the
				// empty id the menu request carries is what failed, and it failed
				// before any lease existed - so the capture guard never went up.
				focusCurrentScreen();
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
		offAccountUpdated?.();
		if (pickerRefreshTimer) clearTimeout(pickerRefreshTimer);
    setSteamGuardDropTarget("none");
    if (clockTimer) clearInterval(clockTimer);
    if (statusTimer) clearTimeout(statusTimer);
    clearQRLoginTimer();
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
            <SteamAccountAvatar account={avatarRow(lockedAccountSummary ?? refSummary(state.account, true))} fallback={PROFILE_FALLBACK} scope="steam-guard" />
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
          use:focusOnShow={!busy}
          aria-invalid={inlineError ? "true" : undefined}
          aria-describedby={inlineError ? "steam-guard-unlock-error" : undefined}
          on:keydown={(event) => runOnEnter(event, unlockAccount)}
        />
      </label>
      <!-- A vault can have a keyfile or a backup key enrolled, and then the
           password alone will not open it. Offered on every unlock rather than
           only after a failure: the user knows what they enrolled, and a
           password-only vault simply leaves these empty. -->
      <SteamGuardVaultFactors
        idPrefix="steam-guard"
        bind:backupKey={unlockBackupKey}
        bind:keyfilePath={unlockKeyfilePath}
        {busy}
        pickKeyfile={controller.pickKeyfile}
        onSubmit={unlockAccount}
      />
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
        <div class="steam-guard__actions-group">
          <!-- Its own button because it needs nothing typed in, which is the one
               thing Unlock cannot allow without letting an empty form through.
               Always shown, so the option is not a secret; disabled only once
               the vault has said it has no key to ask for. -->
          <button
            class="btnicontext"
            type="button"
            disabled={busy || vaultStatus?.hasSecurityKey === false}
            title={vaultStatus?.hasSecurityKey === false ? $t("SteamGuard_Unlock_NoSecurityKey") : ""}
            on:click={() => void unlockWithSecurityKey()}
          >
            {$t("SteamGuard_Unlock_SecurityKey")}
          </button>
          <!-- A backup key opens the vault on its own, so requiring a password
               here would make that factor unusable. -->
          <button
            class="btnicontext modal-primary"
            type="button"
            disabled={busy || (password.length === 0 && unlockKeyfilePath === "" && unlockBackupKey.trim() === "")}
            on:click={unlockAccount}
          >
            <svg class="steam-guard__icon" viewBox={ICONS.key.box} aria-hidden="true"><path d={ICONS.key.path} /></svg>
            {busy ? $t("SteamGuard_Unlocking") : $t("SteamGuard_Unlock")}
          </button>
        </div>
      </div>
    </form>
  {:else if state.screen === "account-code"}
    <section class="steam-guard__stack">
      <div class="steam-guard__identity" data-steamguard-focus tabindex="-1">
        <span class="steam-guard__identity-avatar">
          <SteamAccountAvatar account={avatarRow(codeAccountSummary)} fallback={PROFILE_FALLBACK} scope="steam-guard" />
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
				<!-- Only once the session has been found to work: a window opened on a
				     lapsed one lands on a signed-out page, and Login Again above is
				     already the thing to do about that. -->
				{#if controller.openSteamBrowser && canBrowse}
					<div class="steam-guard__browse" use:controllerSpatialNavigation>
						<button type="button" class="btnicontext" aria-busy={openingBrowserSite === "store"} disabled={busy || openingBrowserSite !== null} on:click={() => openBrowser("store")}>
							{#if openingBrowserSite === "store"}<span class="steam-guard__spinner" aria-hidden="true"></span>{:else}<svg class="steam-guard__icon" viewBox={ICONS.globe.box} aria-hidden="true"><path d={ICONS.globe.path} /></svg>{/if}
							{$t("SteamGuard_Browse_Store")}
						</button>
						<button type="button" class="btnicontext" aria-busy={openingBrowserSite === "community"} disabled={busy || openingBrowserSite !== null} on:click={() => openBrowser("community")}>
							{#if openingBrowserSite === "community"}<span class="steam-guard__spinner" aria-hidden="true"></span>{:else}<svg class="steam-guard__icon" viewBox={ICONS.globe.box} aria-hidden="true"><path d={ICONS.globe.path} /></svg>{/if}
							{$t("SteamGuard_Browse_Community")}
						</button>
						<button type="button" class="btnicontext" aria-busy={openingBrowserSite === "chat"} disabled={busy || openingBrowserSite !== null} on:click={() => openBrowser("chat")}>
							{#if openingBrowserSite === "chat"}<span class="steam-guard__spinner" aria-hidden="true"></span>{:else}<svg class="steam-guard__icon" viewBox={ICONS.globe.box} aria-hidden="true"><path d={ICONS.globe.path} /></svg>{/if}
							{$t("SteamGuard_Browse_Chat")}
						</button>
					</div>
				{/if}
			{/if}
      <div class="steam-guard__footer">
        <button
          type="button"
          class="fancyLink steam-guard__link"
	          disabled={busy || oneOperationCode}
          on:click={showAllAccounts}
        >
          <svg class="steam-guard__icon" viewBox={ICONS.list.box} aria-hidden="true"><path d={ICONS.list.path} /></svg>
          {$t("SteamGuard_Code_ShowAllAccounts")}
        </button>
      </div>
    </section>
  {:else if state.screen === "setup"}
    <!-- A placeholder page: this account is in no vault record, so there is no
         code, no session and nothing to manage. It exists to name the account
         being added and offer the three ways to start holding it. -->
    <section class="steam-guard__stack">
      <div class="steam-guard__identity" data-steamguard-focus tabindex="-1">
        <span class="steam-guard__identity-avatar">
          <SteamAccountAvatar account={avatarRow(setupAccountSummary)} fallback={PROFILE_FALLBACK} scope="steam-guard" />
        </span>
        <span class="steam-guard__identity-name">
          <span class="steam-guard__identity-display">{setupAccountSummary.username}</span>
          {#if setupAccountSummary.displayName && setupAccountSummary.displayName !== setupAccountSummary.username}
            <small>{setupAccountSummary.displayName}</small>
          {/if}
        </span>
      </div>
      <p class="steam-guard__hint">{$t("SteamGuard_Setup_Body")}</p>
      {#if inlineError}<p class="steam-guard__error" role="alert">{inlineError}</p>{/if}
      <div class="steam-guard__grid" use:controllerSpatialNavigation>
        <button type="button" class="btnicontext" disabled={busy} on:click={startSetupLoginOnly}>
          <svg class="steam-guard__icon" viewBox={ICONS.key.box} aria-hidden="true"><path d={ICONS.key.path} /></svg>
          {$t("SteamGuard_Setup_JustLogin")}
        </button>
        <button
          type="button"
          class="btnicontext"
          disabled={busy || !controller.importMaFile}
          on:click={startSetupImport}
        >
          <svg class="steam-guard__icon" viewBox={ICONS.fileExport.box} aria-hidden="true"><path d={ICONS.fileExport.path} /></svg>
          {$t("SteamGuard_Import_Title")}
        </button>
        <button
          type="button"
          class="btnicontext"
          disabled={busy || !controller.resumeSteamGuardEnrollment}
          on:click={startSetupEnrollment}
        >
          <svg class="steam-guard__icon" viewBox={ICONS.shield.box} aria-hidden="true"><path d={ICONS.shield.path} /></svg>
          <!-- Just the feature's name here: the three buttons are already read as
               ways to add this account, so "Add" on one of them says nothing. -->
          {$t("SteamGuard_Title")}
        </button>
      </div>
      <div class="steam-guard__footer">
        <button type="button" class="fancyLink steam-guard__link" disabled={busy} on:click={showAllAccounts}>
          <svg class="steam-guard__icon" viewBox={ICONS.list.box} aria-hidden="true"><path d={ICONS.list.path} /></svg>
          {$t("SteamGuard_Code_ShowAllAccounts")}
        </button>
      </div>
    </section>
  {:else if state.screen === "login-only"}
    <!-- No code tile, and no Confirmations, QR or Export: this record holds a
         session and nothing else. They are absent rather than disabled, because
         they are not unavailable right now, they do not apply to this account. -->
    <section class="steam-guard__stack">
      <div class="steam-guard__identity" data-steamguard-focus tabindex="-1">
        <span class="steam-guard__identity-avatar">
          <SteamAccountAvatar account={avatarRow(loginOnlyAccountSummary)} fallback={PROFILE_FALLBACK} scope="steam-guard" />
        </span>
        <span class="steam-guard__identity-name">
          <span class="steam-guard__identity-display">{loginOnlyAccountSummary.username}</span>
          {#if loginOnlyAccountSummary.displayName && loginOnlyAccountSummary.displayName !== loginOnlyAccountSummary.username}
            <small>{loginOnlyAccountSummary.displayName}</small>
          {/if}
        </span>
      </div>
      <p class="steam-guard__hint">{$t("SteamGuard_LoginOnly_Body")}</p>
      {#if inlineError}<p class="steam-guard__error" role="alert">{inlineError}</p>{/if}
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
        <!-- The way out of being login-only. Not offered when the backend cannot
             promote: the stored session is what makes this one click instead of
             a second sign-in. -->
        <button
          type="button"
          class="btnicontext"
          disabled={busy || !controller.promoteLoginOnlyAccount}
          on:click={promoteLoginOnly}
        >
          <svg class="steam-guard__icon" viewBox={ICONS.shield.box} aria-hidden="true"><path d={ICONS.shield.path} /></svg>
          {$t("SteamGuard_Enrollment_Add")}
        </button>
        <!-- Two-step inline rather than a confirm dialog: openConfirm takes the
             single modal slot and would tear this modal down mid-flow. -->
        {#if removeConfirming}
          <button type="button" class="btnicontext modal-danger" disabled={busy} on:click={confirmRemoveLoginOnly}>
            {$t("SteamGuard_LoginOnly_RemoveConfirm")}
          </button>
          <button type="button" class="btnicontext" disabled={busy} on:click={() => (removeConfirming = false)}>
            {$t("SteamGuard_Cancel")}
          </button>
        {:else}
          <button
            type="button"
            class="btnicontext"
            disabled={busy || !controller.removeLoginOnlyAccount}
            on:click={() => (removeConfirming = true)}
          >
            {$t("SteamGuard_LoginOnly_Remove")}
          </button>
        {/if}
      </div>
      <!-- A session-only record holds the same tokens an authenticator does, so
           it browses identically. This is the one capability it does not lack -
           as long as the session is still one Steam accepts. -->
      {#if controller.openSteamBrowser && canBrowse}
        <div class="steam-guard__browse" use:controllerSpatialNavigation>
          <button type="button" class="btnicontext" aria-busy={openingBrowserSite === "store"} disabled={busy || openingBrowserSite !== null} on:click={() => openBrowser("store")}>
            {#if openingBrowserSite === "store"}<span class="steam-guard__spinner" aria-hidden="true"></span>{:else}<svg class="steam-guard__icon" viewBox={ICONS.globe.box} aria-hidden="true"><path d={ICONS.globe.path} /></svg>{/if}
            {$t("SteamGuard_Browse_Store")}
          </button>
          <button type="button" class="btnicontext" aria-busy={openingBrowserSite === "community"} disabled={busy || openingBrowserSite !== null} on:click={() => openBrowser("community")}>
            {#if openingBrowserSite === "community"}<span class="steam-guard__spinner" aria-hidden="true"></span>{:else}<svg class="steam-guard__icon" viewBox={ICONS.globe.box} aria-hidden="true"><path d={ICONS.globe.path} /></svg>{/if}
            {$t("SteamGuard_Browse_Community")}
          </button>
          <button type="button" class="btnicontext" aria-busy={openingBrowserSite === "chat"} disabled={busy || openingBrowserSite !== null} on:click={() => openBrowser("chat")}>
            {#if openingBrowserSite === "chat"}<span class="steam-guard__spinner" aria-hidden="true"></span>{:else}<svg class="steam-guard__icon" viewBox={ICONS.globe.box} aria-hidden="true"><path d={ICONS.globe.path} /></svg>{/if}
            {$t("SteamGuard_Browse_Chat")}
          </button>
        </div>
      {/if}
      <div class="steam-guard__footer">
        <button type="button" class="fancyLink steam-guard__link" disabled={busy} on:click={showAllAccounts}>
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
            {@const rowState = steamGuardRowState(listedAccount)}
            <li>
              <button type="button" disabled={busy} on:click={() => openAccountScreen(listedAccount)}>
                <span class="steam-guard__accounts-avatar">
                  <SteamAccountAvatar account={avatarRow(listedAccount)} fallback={PROFILE_FALLBACK} scope="steam-guard" />
                </span>
                <span class="steam-guard__accounts-name">
                  <span
                    class="steam-guard__accounts-username"
                    class:acc_name--vac={listedAccount.vac}
                    class:acc_name--limited={!listedAccount.vac && listedAccount.limited}
                    title={listedAccount.vac
                      ? $t("Steam_Status_VacBanned")
                      : listedAccount.limited
                        ? $t("Steam_Status_Limited")
                        : undefined}
                  >{listedAccount.username}</span>
                  {#if listedAccount.displayName && listedAccount.displayName !== listedAccount.username}
                    <small class="steam-guard__accounts-display">{listedAccount.displayName}</small>
                  {/if}
                </span>
                <!-- A locked login-only row still reads "Locked": that is a
                     state the user has to act on, and it outranks the kind. So
                     does a lapsed session, which is why "Login" comes before
                     the kind too. "Ready" is only claimed for a session
                     this build could actually read; an unreadable one keeps the
                     plain, unbadged word rather than promising a live session. -->
                {#if rowState === "locked"}
                  <small class="steam-guard__accounts-state">{$t("SteamGuard_AllAccounts_Locked")}</small>
                {:else if rowState === "login-again"}
                  <small
                    class="steam-guard__accounts-state steam-guard__accounts-state--badge
                           steam-guard__accounts-state--login-again"
                    >{$t("SteamGuard_AllAccounts_LoginAgain")}</small
                  >
                {:else if rowState === "login-only"}
                  <small class="steam-guard__accounts-state steam-guard__accounts-state--login-only"
                    >{$t("SteamGuard_AllAccounts_LoginOnly")}</small
                  >
                {:else}
                  <small
                    class="steam-guard__accounts-state"
                    class:steam-guard__accounts-state--badge={rowState === "ready"}
                    class:steam-guard__accounts-state--ready={rowState === "ready"}
                    >{$t("SteamGuard_AllAccounts_Ready")}</small
                  >
                {/if}
              </button>
            </li>
          {/each}
        </ul>
      {:else}
        <p>{$t("SteamGuard_AllAccounts_Empty")}</p>
      {/if}
      <div class="steam-guard__footer">
        <button type="button" class="fancyLink steam-guard__link" disabled={busy} on:click={leaveAllAccounts}>
          <svg class="steam-guard__icon" viewBox={ICONS.back.box} aria-hidden="true"><path d={ICONS.back.path} /></svg>
          {pickerReturnAccount ? $t("SteamGuard_Back") : $t("Button_Close")}
        </button>
        <!-- The one way into the vault that needs no account picked first: an
             maFile carries its own identity. -->
        <button type="button" class="fancyLink steam-guard__link" disabled={busy} on:click={importFromAllAccounts}>
          <svg class="steam-guard__icon" viewBox={ICONS.fileExport.box} aria-hidden="true"><path d={ICONS.fileExport.path} /></svg>
          {$t("SteamGuard_Import_Title")}
        </button>
        <!-- The other way in that needs no account picked first: signing in
             names the account, the same way an maFile carries its own name. -->
        <button type="button" class="fancyLink steam-guard__link" disabled={busy} on:click={() => void showAddAccount()}>
          <svg class="steam-guard__icon" viewBox={ICONS.plus.box} aria-hidden="true"><path d={ICONS.plus.path} /></svg>
          {$t("SteamGuard_AddAccount_Link")}
        </button>
      </div>
    </section>
  {:else if state.screen === "add-account"}
    <section class="steam-guard__stack">
      <h2 class="steam-guard__heading" data-steamguard-focus tabindex="-1">{$t("SteamGuard_AddAccount_Title")}</h2>
      <!-- Which account is being signed in. By the code step the form is gone,
           and without this there is nothing on screen saying what was typed. -->
      {#if state.account?.username}<p class="steam-guard__account">{state.account.username}</p>{/if}
      {#if authStage === "idle"}
        <form class="steam-guard__stack" on:submit|preventDefault={() => void beginAddAccount(authPurpose)}>
          <p>{$t("SteamGuard_AddAccount_Body")}</p>
          <label class="steam-guard__field" for="steam-add-account">
            <span>{$t("SteamGuard_Field_SteamAccountName")}</span>
            <input id="steam-add-account" class="modal-input" bind:value={authAccountName} autocomplete="username" disabled={busy} data-steamguard-autofocus use:focusOnShow={!busy} />
          </label>
          <label class="steam-guard__field" for="steam-add-password">
            <span>{$t("SteamGuard_Field_SteamPassword")}</span>
            <input id="steam-add-password" class="modal-input" bind:value={authPassword} type="password" autocomplete="current-password" disabled={busy} />
          </label>
          {#if inlineError}<p class="steam-guard__error" role="alert">{inlineError}</p>{/if}
          <p class="steam-guard__hint">{$t("SteamGuard_AddAccount_As")}</p>
          <div class="steam-guard__actions steam-guard__actions--split">
            <!-- The add screen is reachable with nothing behind it - the toolbar
                 and the list's background menu both open it directly - and with
                 no account list to return to this button leaves the modal. -->
            <button type="button" class="btnicontext" disabled={busy} on:click={showAllAccounts}>
              <svg class="steam-guard__icon" viewBox={ICONS.back.box} aria-hidden="true"><path d={ICONS.back.path} /></svg>
              {listingAnchor ? $t("SteamGuard_Back") : $t("Button_Close")}
            </button>
            <div class="steam-guard__actions-group">
              <button type="button" class="btnicontext modal-primary" aria-busy={busy} disabled={busy || !authAccountName.trim() || !authPassword} on:click={() => void beginAddAccount("login_only")}>
                {#if busy}<span class="steam-guard__spinner" aria-hidden="true"></span>{:else}<svg class="steam-guard__icon" viewBox={ICONS.key.box} aria-hidden="true"><path d={ICONS.key.path} /></svg>{/if}{$t("SteamGuard_AddAccount_AsLoginOnly")}
              </button>
              <button type="button" class="btnicontext modal-primary" aria-busy={busy} disabled={busy || !authAccountName.trim() || !authPassword} on:click={() => void beginAddAccount("add_authenticator")}>
                {#if busy}<span class="steam-guard__spinner" aria-hidden="true"></span>{:else}<svg class="steam-guard__icon" viewBox={ICONS.shield.box} aria-hidden="true"><path d={ICONS.shield.path} /></svg>{/if}{$t("SteamGuard_AddAccount_AsSteamGuard")}
              </button>
            </div>
          </div>
        </form>
      {:else if authStage === "credentials"}
        <p role="status">{authMessage || $t("SteamGuard_Challenge_SigningIn")}</p>
        <div class="steam-guard__qr-progress" aria-hidden="true"></div>
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
          <label class="steam-guard__field" for="steam-add-code">
            <span>{authChallenge === "email_code" ? $t("SteamGuard_Challenge_EmailCode") : $t("SteamGuard_Challenge_DeviceCode")}</span>
            <input id="steam-add-code" class="modal-input" bind:value={authCode} autocomplete="one-time-code" disabled={busy} data-steamguard-autofocus use:focusOnShow={!busy} on:keydown={(event) => runOnEnter(event, submitCredentialCode)} />
          </label>
          <div class="steam-guard__actions steam-guard__actions--end">
            <button type="button" class="btnicontext modal-primary" aria-busy={busy} disabled={busy || !authCode} on:click={submitCredentialCode}>{#if busy}<span class="steam-guard__spinner" aria-hidden="true"></span>{/if}{busy ? $t("SteamGuard_Challenge_Submitting") : $t("SteamGuard_Challenge_Submit")}</button>
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
      {:else}
        <p class="steam-guard__error" role="alert">{authMessage}</p>
        <div class="steam-guard__actions steam-guard__actions--end">
          <button type="button" class="btnicontext modal-primary" disabled={busy} on:click={() => void showAddAccount()}>{$t("SteamGuard_TryAgain")}</button>
          <button type="button" class="btnicontext" disabled={busy} on:click={showAllAccounts}>{listingAnchor ? $t("SteamGuard_Back") : $t("Button_Close")}</button>
        </div>
      {/if}
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
        <button type="button" class="btnicontext" disabled={busy} on:click={backFromImport}>
          {state.account ? $t("SteamGuard_Back") : $t("SteamGuard_Code_ShowAllAccounts")}
        </button>
      </div>
    </section>
  {:else if state.screen === "enrollment" || state.screen === "login-only-setup"}
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
					<input id="steam-enrollment-vault-password" class="modal-input" bind:value={vaultPassword} type="password" autocomplete={vaultStatus.configured ? "current-password" : "new-password"} disabled={busy} data-steamguard-autofocus use:focusOnShow={!busy} on:keydown={(event) => runOnEnter(event, submitVaultPreparation)} />
				</label>
				{#if vaultStatus.configured}
					<div class="steam-guard__check">
						<span class="form-check">
							<input id="steam-enrollment-remember-session" class="form-check-input" bind:checked={rememberForSession} type="checkbox" disabled={busy} />
							<label class="form-check-label" for="steam-enrollment-remember-session" aria-hidden="true"></label>
						</span>
						<label for="steam-enrollment-remember-session">{$t("SteamGuard_RememberMe")}</label>
					</div>
					<!-- Same ways in as the account unlock screen. Without these a vault
					     protected by a keyfile or a backup key could not be opened from
					     the screens that add an account. -->
					<SteamGuardVaultFactors
						idPrefix="steam-enrollment"
						bind:backupKey={vaultBackupKey}
						bind:keyfilePath={vaultKeyfilePath}
						{busy}
						pickKeyfile={controller.pickKeyfile}
						onSubmit={submitVaultPreparation}
					/>
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
					{#if vaultStatus.configured}
						<!-- Its own button: a security key needs nothing typed, which the
						     primary button cannot allow without accepting an empty form. -->
						<button
							type="button"
							class="btnicontext"
							disabled={busy || vaultStatus.hasSecurityKey === false}
							title={vaultStatus.hasSecurityKey === false ? $t("SteamGuard_Unlock_NoSecurityKey") : ""}
							on:click={() => void submitVaultPreparation()}
						>{$t("SteamGuard_Unlock_SecurityKey")}</button>
					{/if}
					<button type="button" class="btnicontext modal-primary" aria-busy={busy} disabled={busy || (!vaultPassword && !vaultKeyfilePath && !vaultBackupKey.trim()) || (!vaultStatus.configured && !vaultPasswordConfirmation) || (!vaultStatus.configured && vaultStatus.savedAccountDataEncrypted && !vaultAppPassword)} on:click={submitVaultPreparation}>{#if busy}<span class="steam-guard__spinner" aria-hidden="true"></span>{/if}{vaultStatus.configured ? (busy ? $t("SteamGuard_Unlocking") : $t("SteamGuard_Unlock")) : (busy ? $t("SteamGuard_Vault_CreatingVault") : $t("SteamGuard_Vault_CreateVault"))}</button>
					<button type="button" class="btnicontext" disabled={busy} on:click={cancelSetup}>{$t("SteamGuard_Back")}</button>
				</div>
			</form>
		{:else if authStage === "credentials"}
			<form class="steam-guard__stack" on:submit|preventDefault={beginCredentialLogin}>
				<!-- Its own field rather than authMessage: that one carries the
				     sign-in progress text, which would replace this the moment the
				     form is submitted. -->
				<p>{credentialsHint || $t("SteamGuard_Enrollment_CredentialsBody")}</p>
				<!-- Steam's own login page puts the code beside the password box, and it
				     is the quicker way in for anyone whose phone is already signed in.
				     Beside rather than above: stacked, the two together made the screen
				     tall enough to scroll. Hidden entirely when it is not on offer, so
				     the fields simply take the whole width back. -->
				<div class="steam-guard__signin" class:steam-guard__signin--split={showQRLogin}>
					{#if showQRLogin}
						<div class="steam-guard__qr-login">
							<div class="steam-guard__qr-login-code" aria-busy={qrLoginStage === "starting"}>
								{#if qrLoginImage}
									<img src={qrLoginImage} alt={$t("SteamGuard_QRLogin_CodeAlt")} draggable="false" />
								{:else}
									<span class="steam-guard__spinner" aria-hidden="true"></span>
								{/if}
							</div>
							<p class="steam-guard__qr-login-caption">{$t("SteamGuard_QRLogin_Body")}</p>
							<p class="sr-only" role="status">
								{qrLoginStage === "waiting" ? $t("SteamGuard_QRLogin_Waiting") : $t("SteamGuard_QRLogin_Preparing")}
							</p>
						</div>
					{/if}
					<div class="steam-guard__signin-fields">
						<label class="steam-guard__field" for="steam-enrollment-account">
							<span>{$t("SteamGuard_Field_SteamAccountName")}</span>
							<input id="steam-enrollment-account" class="modal-input" bind:value={authAccountName} autocomplete="username" disabled={busy} on:keydown={(event) => runOnEnter(event, beginCredentialLogin)} />
						</label>
						<label class="steam-guard__field" for="steam-enrollment-password">
							<span>{$t("SteamGuard_Field_SteamPassword")}</span>
							<input id="steam-enrollment-password" class="modal-input" bind:value={authPassword} type="password" autocomplete="current-password" disabled={busy} data-steamguard-autofocus use:focusOnShow={!busy} on:keydown={(event) => runOnEnter(event, beginCredentialLogin)} />
						</label>
					</div>
				</div>
				<div class="steam-guard__actions steam-guard__actions--end">
					<button type="button" class="btnicontext modal-primary" aria-busy={busy} disabled={busy || !authAccountName.trim() || !authPassword} on:click={beginCredentialLogin}>{#if busy}<span class="steam-guard__spinner" aria-hidden="true"></span>{/if}{busy ? $t("SteamGuard_Challenge_SigningIn") : $t("SteamGuard_SignIn")}</button>
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
					<input id="steam-enrollment-code" class="modal-input" bind:value={authCode} autocomplete="one-time-code" disabled={busy} data-steamguard-autofocus use:focusOnShow={!busy} on:keydown={(event) => runOnEnter(event, submitCredentialCode)} />
				</label>
				<div class="steam-guard__actions steam-guard__actions--end">
					<button type="button" class="btnicontext modal-primary" aria-busy={busy} disabled={busy || !authCode} on:click={submitCredentialCode}>{#if busy}<span class="steam-guard__spinner" aria-hidden="true"></span>{/if}{busy ? $t("SteamGuard_Challenge_Submitting") : $t("SteamGuard_Challenge_Submit")}</button>
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
					<input id="steam-recovery-confirmation" class="modal-input" bind:value={revocationConfirmation} autocomplete="off" spellcheck="false" disabled={busy} data-steamguard-autofocus use:focusOnShow={!busy} on:keydown={(event) => runOnEnter(event, acknowledgeRevocationCode)} />
				</label>
				{#if authMessage}<p class="steam-guard__error" role="alert">{authMessage}</p>{/if}
				<div class="steam-guard__actions steam-guard__actions--end">
					<button type="button" class="btnicontext modal-primary" disabled={busy || !revocationConfirmation} on:click={acknowledgeRevocationCode}>{$t("SteamGuard_RecoveryCode_Saved")}</button>
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
				{#if confirmationError}<p class="steam-guard__error" role="alert">{confirmationError}</p>{/if}
				<label class="steam-guard__field" for="steam-enrollment-confirmation">
					<span>{$t("SteamGuard_Field_ConfirmationCode")}</span>
					<input id="steam-enrollment-confirmation" class="modal-input" bind:value={confirmationCode} autocomplete="one-time-code" disabled={busy || enrollmentRetrySeconds > 0} data-steamguard-autofocus use:focusOnShow={!busy && enrollmentRetrySeconds === 0} on:keydown={(event) => runOnEnter(event, finalizeEnrollment)} />
				</label>
				<div class="steam-guard__actions steam-guard__actions--end">
					<button type="button" class="btnicontext modal-primary" aria-busy={busy} disabled={busy || !confirmationCode || enrollmentRetrySeconds > 0} on:click={finalizeEnrollment}>{#if busy}<span class="steam-guard__spinner" aria-hidden="true"></span>{/if}{busy ? $t("SteamGuard_Enrollment_Finishing") : $t("SteamGuard_Enrollment_Finish")}</button>
					<button type="button" class="btnicontext" disabled={busy} on:click={cancelEnrollment}>{$t("SteamGuard_CloseAndResume")}</button>
				</div>
			</form>
		{:else if enrollmentStage === "complete"}
			<p>{$t("SteamGuard_Enrollment_Complete")}</p>
			<!-- The reminder is shown here rather than in a dialog of its own: that
			     dialog takes the modal slot, which meant closing this screen before
			     it could be read. The folder path is one button away. -->
			<p class="steam-guard__warning">{$t("SteamGuard_Enrollment_BackupReminder")}</p>
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
				<button type="button" class="btnicontext modal-primary" disabled={busy} on:click={restartSetup}>{$t("SteamGuard_TryAgain")}</button>
				<button type="button" class="btnicontext" disabled={busy} on:click={cancelSetup}>{$t("SteamGuard_Back")}</button>
			</div>
		{:else if state.screen === "login-only-setup"}
			<p>{$t("SteamGuard_LoginOnly_SetupBody")}</p>
			<div class="steam-guard__actions steam-guard__actions--end">
				<button type="button" class="btnicontext modal-primary" disabled={busy} on:click={startLoginOnlySetup}>{$t("SteamGuard_SignIn")}</button>
				<button type="button" class="btnicontext" disabled={busy} on:click={cancelSetup}>{$t("SteamGuard_Back")}</button>
			</div>
		{:else}
			<p>{$t("SteamGuard_Enrollment_IntroBody")}</p>
			<div class="steam-guard__actions steam-guard__actions--end">
				<button type="button" class="btnicontext modal-primary" disabled={!controller.resumeSteamGuardEnrollment || busy} on:click={startEnrollment}>{$t("SteamGuard_Enrollment_Add")}</button>
				<button type="button" class="btnicontext" disabled={busy} on:click={showQrState}>{$t("SteamGuard_LoginWithQR")}</button>
				<button type="button" class="btnicontext" disabled={busy} on:click={cancelSetup}>{$t("SteamGuard_Back")}</button>
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
					disabled={!controller.pickQrScreenshot || !controller.decodeQrScreenshot || busy}
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
				<label class="steam-guard__field" for="steam-login-password"><span>{$t("SteamGuard_Field_SteamPassword")}</span><input id="steam-login-password" class="modal-input" bind:value={authPassword} type="password" autocomplete="current-password" disabled={busy} data-steamguard-autofocus use:focusOnShow={!busy} on:keydown={(event) => runOnEnter(event, beginCredentialLogin)} /></label>
				<!-- Cancel leaves the login-again flow entirely: the renewal that
				     precedes this form has already finished by the time it shows, so
				     staying on this screen would strand the user on its "Refreshing…"
				     fallback with nothing running. -->
				<div class="steam-guard__actions steam-guard__actions--end"><button type="button" class="btnicontext modal-primary" disabled={busy || !authAccountName.trim() || !authPassword} on:click={beginCredentialLogin}>{$t("SteamGuard_SignIn")}</button><button type="button" class="btnicontext" disabled={busy} on:click={() => { void cancelCredentialLogin().then(backToAccount); }}>{$t("SteamGuard_Cancel")}</button></div>
			</div>
		{:else if authStage === "challenge" && authResult}
			<form class="steam-guard__stack" on:submit|preventDefault={submitCredentialCode}>
				<p>{authMessage}</p>
				{#if authResult.canSubmitEmailCode && authResult.canSubmitDeviceCode}<fieldset class="steam-guard__challenge-options"><legend>{$t("SteamGuard_Challenge_CodeSource")}</legend><label><input type="radio" bind:group={authChallenge} value="email_code" /> {$t("SteamGuard_Challenge_EmailCode")}</label><label><input type="radio" bind:group={authChallenge} value="device_code" /> {$t("SteamGuard_Challenge_DeviceCode")}</label></fieldset>{/if}
				<label class="steam-guard__field" for="steam-login-code"><span>{authChallenge === "email_code" ? $t("SteamGuard_Challenge_EmailCode") : $t("SteamGuard_Challenge_DeviceCode")}</span><input id="steam-login-code" class="modal-input" bind:value={authCode} autocomplete="one-time-code" disabled={busy} data-steamguard-autofocus use:focusOnShow={!busy} on:keydown={(event) => runOnEnter(event, submitCredentialCode)} /></label>
				<div class="steam-guard__actions steam-guard__actions--end"><button type="button" class="btnicontext modal-primary" disabled={busy || !authCode} on:click={submitCredentialCode}>{$t("SteamGuard_Challenge_Submit")}</button><button type="button" class="btnicontext" disabled={busy} on:click={() => { void cancelCredentialLogin().then(backToAccount); }}>{$t("SteamGuard_Cancel")}</button></div>
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
				<button type="button" class="fancyLink steam-guard__link" disabled={busy} on:click={backToAccount}>
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
          <SteamAccountAvatar account={avatarRow(exportAccountSummary)} fallback={PROFILE_FALLBACK} scope="steam-guard" />
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
            data-steamguard-autofocus use:focusOnShow={!busy}
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
            type="button"
            class="btnicontext modal-primary"
            disabled={busy || !exportPassword || !controller.exportMaFile}
            on:click={submitExport}
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

  /* Centred with grid alignment, not auto margins: `.steam-guard :global(button)`
     above carries one more element in its selector, so its `margin: 0` won and
     dropped the code to the start of its column. */
  .steam-guard__code {
    position: relative;
    justify-self: center;
    width: min(18rem, 100%);
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

  /* Buttons travel together against the split row's space-between. */
  .steam-guard__actions-group {
    display: flex;
    align-items: center;
    gap: $sg-1;
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

  // The browsing pair sits under the three-up grid. They are sized to their
  // labels and centred rather than stretched across columns, which made two
  // buttons in a three-column grid look oversized and off to one side.
  .steam-guard__browse {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: $sg-1;
  }

  .steam-guard__browse :global(button) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: $sg-half;
    min-height: 2.75rem;
    padding-inline: $sg-2;
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
    /* Wraps rather than squeezing: a translated pair of links is easily wider
       than a narrow modal, and a clipped one cannot be clicked. */
    flex-wrap: wrap;
    gap: $sg-half;
    padding-top: $sg-half;
  }

  /* A link with a touch target: fancyLink strips the button chrome the themes
     paint on, the size and ink are this row's own. */
  .steam-guard__link {
    display: inline-flex;
    align-items: center;
    gap: $sg-1;
    min-height: 2.75rem;
    padding: $sg-1 $sg-1;
    border-radius: 0.25rem;
    color: var(--whiteSecondary, #d7d7d7);
    text-decoration: underline;
  }

  /* fancyLink forces the fill off so no theme can paint a button here; taking
     the highlight back needs the same force. */
  .steam-guard__link:hover:not(:disabled) {
    background: var(--role-dropdown-hover-bg, rgb(255 255 255 / 12%)) !important;
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

  /*
   * Two lines by construction, not by a hard-coded break: min-content is the
   * narrowest box that still fits the longest word, so every locale wraps at its
   * own word boundaries instead of at one guessed for English. Two lines at
   * 0.82rem x 1.15 fit the row's 3.5rem min-height, so nothing grows.
   */
  .steam-guard__accounts-state--login-only {
    width: min-content;
    text-align: center;
    white-space: normal;
    line-height: 1.15;
  }

  .steam-guard__accounts-display,
  .steam-guard__accounts-state {
    color: var(--whiteSecondary, #d7d7d7);
    font-size: 0.82rem;
  }

  /*
   * Below the shared colour rule on purpose: a single-class variant has the same
   * specificity, so a badge colour declared above it would be overridden.
   *
   * A state worth acting on reads as a badge rather than as a word. Its own
   * symmetric padding replaces the base rule's padding-left - the row's 0.75rem
   * gap already separates it from the name - and the min-width keeps every badge
   * the same size, so the column does not jitter as rows change state. The width
   * is sized for the one-word labels; a longer translation grows the badge.
   */
  .steam-guard__accounts-state--badge {
    min-width: 3.5rem;
    padding: 0.15rem 0.45rem;
    border: 1px solid transparent;
    border-radius: 0.25rem;
    color: var(--white, #fff);
    text-align: center;
  }

  .steam-guard__accounts-state--ready {
    border-color: var(--green, #4caf50);
    background: color-mix(in srgb, var(--green, #4caf50) 22%, transparent);
  }

  .steam-guard__accounts-state--login-again {
    border-color: var(--role-warning, #ffd166);
    background: color-mix(in srgb, var(--role-warning, #ffd166) 22%, transparent);
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

	/* Shown inside a primary button while its action is in flight. The vault
	   steps derive a key and the Steam steps wait on the network, both long
	   enough that a button which only greys out reads as a click that did
	   nothing. currentColor keeps it legible on any button in any theme. */
	/*
	 * The scan-to-sign-in code, drawn beside the sign-in fields rather than above
	 * them: stacked, the two together made the screen tall enough to scroll.
	 */
	.steam-guard__signin {
		display: flex;
		flex-direction: column;
		gap: $sg-1;
	}

	.steam-guard__signin--split {
		flex-direction: row;
		align-items: center;
		gap: $sg-2;
	}

	.steam-guard__signin-fields {
		display: flex;
		flex: 1 1 auto;
		flex-direction: column;
		gap: $sg-1;
		/* Without this the fields refuse to shrink below their content and push the
		   code off the edge of a narrow frame. */
		min-width: 0;
	}

	.steam-guard__qr-login {
		display: flex;
		flex: 0 0 auto;
		flex-direction: column;
		align-items: center;
		gap: $sg-half;
		width: 8.5rem;
	}

	.steam-guard__qr-login-code {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 8.5rem;
		height: 8.5rem;
		border-radius: $sg-half;
		/* The SVG paints its own white quiet zone, so this only rounds the corners
		   and stands in while the code is still being fetched. */
		background: #fff;
		overflow: hidden;
	}

	.steam-guard__qr-login-code img {
		width: 100%;
		height: 100%;
		/* A QR code is a grid of hard squares, and smoothing it on the way to the
		   panel's pixel grid is what makes a camera hunt for focus. */
		image-rendering: pixelated;
	}

	.steam-guard__qr-login-caption {
		margin: 0;
		font-size: 0.8em;
		opacity: 0.75;
		text-align: center;
	}

	.steam-guard__spinner {
		display: inline-block;
		width: 0.9em;
		height: 0.9em;
		margin-right: 0.45em;
		border: 2px solid currentColor;
		border-top-color: transparent;
		border-radius: 50%;
		vertical-align: -0.1em;
		animation: steam-guard-spin 0.8s linear infinite;
	}

	@keyframes steam-guard-spin {
		to { transform: rotate(360deg); }
	}

	@media (prefers-reduced-motion: reduce) {
		.steam-guard__spinner { animation-duration: 2.4s; }
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
    .steam-guard__accounts button,
    /* The tint would otherwise survive on a row repainted to Canvas. The words
       still tell the states apart. */
    .steam-guard__accounts-state--badge {
      border-color: CanvasText;
      background: Canvas;
      color: CanvasText;
    }

    .steam-guard__countdown {
      background: Highlight;
    }
  }
</style>
