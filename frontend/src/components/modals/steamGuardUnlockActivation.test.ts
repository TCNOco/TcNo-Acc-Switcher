import { readFileSync } from "node:fs";
import { describe, expect, it, vi } from "vitest";
import { render } from "svelte/server";
import type { ComponentProps } from "svelte";
import AuthenticatorUnlockForm from "./AuthenticatorUnlockForm.svelte";

vi.mock("../../stores/i18n", async () => {
  const { readable } = await import("svelte/store");
  return { t: readable((key: string) => key) };
});

const source = readFileSync(new URL("./SteamGuardModalBody.svelte", import.meta.url), "utf8").replace(/\r\n/g, "\n");

function fragmentAfter(marker: string, length = 1_000): string {
  const index = source.indexOf(marker);
  expect(index).toBeGreaterThanOrEqual(0);
  return source.slice(index, index + length);
}

/** A function's body up to a marker inside it, rather than a hand-sized window. */
function functionBodyBefore(signature: string, end: string): string {
  const start = source.indexOf(signature);
  expect(start).toBeGreaterThanOrEqual(0);
  const stop = source.indexOf(end, start);
  expect(stop).toBeGreaterThan(start);
  return source.slice(start, stop);
}

/** The headerbar back action's branch for one screen, bounded by the next case. */
function headerBackCase(screen: string): string {
  const start = source.indexOf(`case "${screen}":`, source.indexOf("headerBackAction"));
  expect(start).toBeGreaterThanOrEqual(0);
  const end = source.indexOf("\t\t\tcase ", start + 1);
  expect(end).toBeGreaterThan(start);
  return source.slice(start, end);
}

/** The vault create/unlock form, bounded by its own closing tag. */
function vaultPreparationForm(): string {
  const start = source.indexOf('on:submit|preventDefault={submitVaultPreparation}');
  expect(start).toBeGreaterThanOrEqual(0);
  const end = source.indexOf("</form>", start);
  expect(end).toBeGreaterThan(start);
  return source.slice(start, end);
}

describe("Steam Guard unlock activation", () => {
  const form = (props: Partial<ComponentProps<typeof AuthenticatorUnlockForm>> = {}) => render(AuthenticatorUnlockForm, {
    props: {
      idPrefix: "steam-guard",
      onUnlock: () => {},
      pickKeyfile: async () => "",
      ...props,
    },
  }).body;

  function primaryDisabled(html: string): boolean {
    const button = html.match(/<button[^>]*class="[^"]*modal-primary[^>]*>/)?.[0];
    expect(button).toBeDefined();
    return /\sdisabled(?:[\s=>])/.test(button!);
  }

  it("requires credentials while allowing backup keys and keyfiles without a password", () => {
    expect(primaryDisabled(form())).toBe(true);
    for (const credentials of [{ password: "password" }, { keyfilePath: "C:/keys/vault.key" }, { backupKey: "recovery-key" }]) {
      expect(primaryDisabled(form(credentials))).toBe(false);
    }
    expect(primaryDisabled(form({ backupKey: "   " }))).toBe(true);
    expect(primaryDisabled(form({ password: "password", busy: true }))).toBe(true);
    expect(primaryDisabled(form({ password: "password", available: false }))).toBe(true);
  });

  it("offers the same factor picker and session controls for either provider", () => {
    for (const idPrefix of ["steam-guard", "battlenet-authenticator"]) {
      const html = form({ idPrefix });
      expect(html).toContain(`id="${idPrefix}-password"`);
      expect(html).toContain(`id="${idPrefix}-backup-key"`);
      expect(html).toContain("SteamGuard_Unlock_ChooseKeyfile");
      expect(html).toContain(`for="${idPrefix}-remember-session"`);
      expect(html).toContain("SteamGuard_Unlock_SecurityKey");
      expect(html).toContain("SteamGuard_Unlock");
    }
  });

  it("disables unavailable security keys and displays accessible unlock errors", () => {
    const html = form({ hasSecurityKey: false, error: "Wrong password" });
    expect(html).toMatch(/<button[^>]*disabled[^>]*>\s*SteamGuard_Unlock_SecurityKey/);
    expect(html).toContain('aria-describedby="steam-guard-unlock-error"');
    expect(html).toContain('role="alert">Wrong password');
  });

  it("keeps recovery controls out of vault creation", () => {
    const html = form({ creating: true });
    expect(html).toContain('autocomplete="new-password"');
    expect(html).not.toContain("SteamGuard_Unlock_ChooseKeyfile");
    expect(html).not.toContain("SteamGuard_Unlock_SecurityKey");
    expect(html).not.toContain("SteamGuard_RememberMe");
  });

  // Dismissing without navigating left the header button clearing the QR status
  // line and then doing nothing at all on every click after that.
  it("leaves the QR screen from the headerbar back button", () => {
    const qr = headerBackCase("qr");
    expect(qr).toContain("void dismissQRLogin().then(backToAccount)");
    // The approval stage still steps back to the scan view, mirroring its Cancel.
    expect(qr).toContain('qrStage === "approval" && qrApproval');
    expect(qr).toContain('$t("SteamGuard_Cancel")');
  });

  // Every way the setup page offers to store an account needs an open vault.
  // Asking after the choice meant picking an maFile, choosing the file, and then
  // being sent back to do both again once the vault was finally open.
  it("opens or creates the vault before the setup page's choice", () => {
    const check = fragmentAfter("async function ensureVaultForSetup", 700);
    expect(check).toContain("if (status.configured && status.unlocked) return;");
    expect(check).toContain("vaultSetupOnly = true");
    expect(check).toContain('enrollmentStage = "vault"');
    expect(check).toContain('transition({ type: "show-enrollment"');

    // And hands the page back once the vault exists, rather than continuing into
    // an enrollment the user never asked for. Which page depends on where the
    // errand came from: adding an account is not an account's setup page.
    const submitted = fragmentAfter("if (vaultSetupOnly) {", 700);
    expect(submitted).toContain('transition({ type: "show-add-account"');
    expect(submitted).toContain('transition({ type: "show-setup"');
  });

  it("directly handles Enter and click for vault unlock", () => {
    const password = fragmentAfter('id="steam-enrollment-vault-password"');
    expect(password).toContain("runOnEnter(event, submitVaultPreparation)");

    // Anchored inside the vault form rather than on the first inline-error block
    // in the file: other screens show inline errors too, and whichever appears
    // earliest would otherwise be asserted instead of this one.
    const form = vaultPreparationForm();
    expect(form).toContain('{#if inlineError}<p class="steam-guard__error"');
    expect(form).toContain('type="button"');
    expect(form).toContain("on:click={submitVaultPreparation}");
    expect(form).toContain('$t("SteamGuard_Unlocking")');
  });
});

describe("adding an account", () => {
  // The list's background menu and the toolbar open this screen directly, so
  // nothing has minted an attempt or checked there is a vault to store into.
  // Without the mount gate the first sign-in failed against a missing vault.
  it("prepares the attempt and the vault when opened directly", () => {
    const mount = fragmentAfter('state.screen === "add-account" && !state.account', 200);
    expect(mount).toContain("addAccountPrepared = true");
    expect(mount).toContain("showAddAccount()");

    const prepare = fragmentAfter("async function showAddAccount", 1_800);
    expect(prepare).toContain("newAddAccountAttempt()");
    expect(prepare).toContain("!status.configured || !status.unlocked");
    expect(prepare).toContain("vaultSetupForAddAccount = true");
    // The capture guard has to be up before either password is typed, and the
    // attempt is the only identity there is to bind it to.
    expect(prepare).toContain("contentProtection.acquire(pendingId)");
  });

  // Acquiring on mount used the id from the menu request, which for an add is
  // empty: it threw, and no lease meant no capture protection either.
  it("does not take a capability before an attempt exists", () => {
    const mount = functionBodyBefore('} else if (entry === "add-account") {', "} else {");
    expect(mount).not.toContain("contentProtection.acquire(account.id)");
    expect(mount).toContain("focusCurrentScreen()");
  });

  // Polling spends the attempt, so anything afterwards has to be authorised
  // under the account Steam named or it has no usable identity left.
  it("re-points the screen at the named account before leaving it", () => {
    const done = fragmentAfter("// A freshly added account", 600);
    expect(done).toContain('transition({ type: "show-add-account", account: currentAccount })');
    expect(done).toContain("showAllAccounts()");
  });

  // A pending attempt, and an account created seconds ago, are not where the
  // user came from, so the picker offers Close rather than Back to them.
  it("is not a place the account picker returns to", () => {
    const picker = fragmentAfter("pickerReturnAccount = state.screen", 200);
    expect(picker).toContain('state.screen === "add-account"');
  });

  // Back listed the vault under the screen's own account, which here is the
  // pending attempt: Go refuses an anchor it holds no record for, and the
  // refusal landed on an error screen with no way off it.
  it("leaves through an anchor the vault actually holds", () => {
    const back = functionBodyBefore("async function showAllAccounts", "} catch (error)");
    expect(back).toContain("const anchor = listingAnchor;");
    expect(back).toContain("if (!anchor) {\n      dismissModal();");
    expect(back).toContain("accountSummaries(anchor)");
    expect(back).not.toContain("steamGuardAccountForState(state) ?? account");
  });
});
