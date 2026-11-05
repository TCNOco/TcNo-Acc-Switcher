import { describe, expect, it, vi } from "vitest";
import { render } from "svelte/server";
import type { ComponentProps } from "svelte";
import { emptyBattleNetForm, type BattleNetActions } from "../../lib/battleNetAuthenticator";
import BattleNetAuthenticatorPanel from "./BattleNetAuthenticatorPanel.svelte";

vi.mock("../../stores/i18n", async () => {
  const { readable } = await import("svelte/store");
  return { t: readable((key: string) => key) };
});

const account = { serial: "EU-1234-5678-9012", label: "Test account", pending: false, hasRestoreCode: true, accountLink: "" };
const actions: BattleNetActions = {
  navigate: vi.fn(), selectAccount: vi.fn(), pickImport: vi.fn(), previewImport: vi.fn(), importSelected: vi.fn(),
  enroll: vi.fn(), cancelEnrollment: vi.fn(), restore: vi.fn(), recover: vi.fn(), exportSelected: vi.fn(),
  openEnrollmentAccount: vi.fn(),
  showRecovery: vi.fn(), acknowledgeBackup: vi.fn(), copyCode: vi.fn(), copyRecovery: vi.fn(), browse: vi.fn(), rename: vi.fn(), link: vi.fn(),
  pickKeyfile: async () => "",
};

function view(props: Partial<ComponentProps<typeof BattleNetAuthenticatorPanel>> = {}): string {
  return render(BattleNetAuthenticatorPanel, { props: { screen: "home", form: emptyBattleNetForm(), actions, ...props } }).body;
}

describe("Battle.net authenticator tasks", () => {
  it("enables direct Restore activation only with all credentials and no pending request", () => {
    const form = { ...emptyBattleNetForm(), accountIdentifier: "test@example.invalid", restoreSerial: account.serial, restoreCode: "ABCD123456" };
    const button = (html: string) => html.match(/<button\b[^>]*>Authenticator_Restore<\/button>/)?.[0];
    const enabled = button(view({ screen: "restore", form }));
    expect(enabled).toContain('type="button"');
    expect(enabled).not.toContain("disabled");
    for (const key of ["accountIdentifier", "restoreSerial", "restoreCode"] as const) {
      expect(button(view({ screen: "restore", form: { ...form, [key]: "   " } }))).toContain("disabled");
    }
    const pending = view({ screen: "restore", form, busy: true });
    expect(pending).toMatch(/<button\b[^>]*aria-busy="true"[^>]*disabled[^>]*>Authenticator_Restoring<\/button>/);
    expect(view({ screen: "restore", form, error: "Restore failed" })).toContain('role="alert">Restore failed');
  });

  it.each([
    ["import", "Authenticator_Preview", { source: "backup.json" }],
    ["export", "Authenticator_Export", { exportPassword: "export password" }],
    ["rename", "Authenticator_SaveLabel", { accountLabel: "Renamed" }],
  ] as const)("keeps %s activation independent of native submission", (screen, label, fields) => {
    const html = view({ screen, form: { ...emptyBattleNetForm(), ...fields } });
    expect(html).toMatch(new RegExp(`<button\\b[^>]*type="button"[^>]*>${label}</button>`));
    expect(html).not.toContain('type="submit"');
  });

  it("offers recovery without another enrollment after an uncertain response", () => {
    const html = view({ screen: "enroll", enrollment: "handle", enrollmentState: "recovery-required" });
    expect(html).toContain("Authenticator_RecoveryRequired");
    expect(html).toContain("Authenticator_RecoverResponse");
    expect(html).toContain("Authenticator_Browse");
    expect(html).not.toContain("Authenticator_RetrySetup");
    expect(html).not.toContain("Authenticator_OpenBrowser");
  });
  it("offers account setup instead of another sign-in when verification is required", () => {
    const html = view({ screen: "enroll", enrollment: "handle", enrollmentState: "verification-required" });
    expect(html).toContain("Authenticator_AccountSetupRequired");
    expect(html).toContain("Authenticator_AccountSetupHint");
    const steps = [...html.matchAll(/<li\b[^>]*>([^<]+)<\/li>/g)].map(match => match[1]);
    expect(steps).toEqual([
      "Authenticator_AccountSetupStepQuestion",
      "Authenticator_AccountSetupStepVerification",
      "Authenticator_AccountSetupStepAuthenticator",
      "Authenticator_AccountSetupStepRetry",
    ]);
    expect(html).toContain("Authenticator_Browse");
    expect(html).toContain("Authenticator_RetrySetup");
    expect(html).not.toContain("Authenticator_Waiting");
    expect(html).not.toContain("Authenticator_OpenBrowser");
    expect(html).not.toContain("Authenticator_RecoverHint");
  });

  it("keeps recovery focused on backup even with an account setup notice", () => {
    const html = view({ screen: "recovery", accounts: [{ ...account, pending: true, requiresAccountSetup: true }], serial: account.serial });
    expect(html).toContain("Authenticator_AccountSetupNotice");
    expect(html).toContain("Authenticator_ShowRecovery");
    expect(html).not.toContain("Authenticator_Browse");
    expect(html).not.toContain("Authenticator_RetrySetup");
  });
  it("starts with one action when no authenticators are saved", () => {
    const html = view();
    expect(html).toContain("Authenticator_EmptyHint");
    expect(html).toContain("Authenticator_Add");
    expect(html.match(/<button\b/g)).toHaveLength(1);
    expect(html).not.toContain("<form");
    expect(html).not.toContain("Authenticator_Recover");
  });

  it("shows the saved account and copyable code without task forms", () => {
    const html = view({ accounts: [account], serial: account.serial, code: "12345678", remaining: 23 });
    expect(html).toContain("Test account");
    expect(html).toContain("Test account (EU-1234-5678-9012)");
    expect(html).toContain("12345678");
    expect(html).toContain("Authenticator_Copy");
    expect(html).not.toContain("<form");
    expect(html).not.toContain("Authenticator_OnlineNotice");
    expect(html).not.toContain("Authenticator_SourcePassword");
    expect(html).not.toContain("Authenticator_RestoreCode");
    expect(html).not.toContain("Authenticator_ExportPassword");
  });

  it("explains all three ways to add, with local recovery collapsed", () => {
    const html = view({ screen: "add" });
    for (const key of ["Authenticator_EnrollHint", "Authenticator_ImportHint", "Authenticator_RestoreHint", "Authenticator_RecoverHint"]) expect(html).toContain(key);
    expect(html).toMatch(/<details[^>]*>\s*<summary[^>]*>Authenticator_Recover/);
    expect(html).not.toMatch(/<details[^>]*\sopen/);
    expect(html).not.toContain("<input");
  });

  it("keeps import, restore, and export credentials on separate screens", () => {
    const imports = view({ screen: "import" });
    expect(imports).toContain("Authenticator_SourcePassword");
    expect(imports).not.toContain("Authenticator_RestoreCode");
    expect(imports).not.toContain("Authenticator_ExportPassword");
    const restore = view({ screen: "restore" });
    expect(restore).toContain("Authenticator_RestoreCode");
    expect(restore).not.toContain("Authenticator_SourcePassword");
    expect(restore).not.toContain("Authenticator_ExportPassword");
    const exports = view({ screen: "export", accounts: [account], serial: account.serial });
    expect(exports).toContain("Authenticator_ExportPassword");
    expect(exports).toContain("SteamGuard_Unlock_ChooseKeyfile");
    expect(exports).not.toContain("Authenticator_RestoreCode");
  });

  it("asks before displaying recovery secrets and offers backup acknowledgement only for pending accounts", () => {
    const hidden = view({ screen: "recovery", accounts: [{ ...account, pending: true }], serial: account.serial });
    expect(hidden).toContain("Authenticator_ShowRecovery");
    expect(hidden).not.toContain("<textarea");
    const recovery = { serial: account.serial, restoreCode: "RESTORE123", uri: "otpauth://example", qrImage: "data:image/png;base64,AAAA" };
    const revealed = view({ screen: "recovery", accounts: [{ ...account, pending: true }], serial: account.serial, recovery });
    expect(revealed).toContain("RESTORE123");
    for (const label of ["Authenticator_Serial", "Authenticator_RestoreCode"]) {
      expect(revealed).toMatch(new RegExp(`<input[^>]*aria-label="${label}"[^>]*readonly[^>]*value="[^"]+"`));
    }
    expect(revealed).toMatch(/<textarea[^>]*class="[^"]*recovery-value[^>]*readonly/);
    expect(revealed).toContain("Authenticator_BackupAcknowledged");
    expect(revealed).toMatch(/<details[^>]*>\s*<summary[^>]*>Authenticator_ShowQR/);
    expect(view({ screen: "recovery", accounts: [account], serial: account.serial, recovery })).not.toContain("Authenticator_BackupAcknowledged");
  });

  it("marks new authenticators for recovery backup on the code screen", () => {
    expect(view({ accounts: [{ ...account, pending: true }], serial: account.serial })).toContain("Authenticator_BackupNeeded");
    expect(view({ accounts: [account], serial: account.serial })).not.toContain("Authenticator_BackupNeeded");
  });
  it("does not ask users to sign in again on the recovery screen", () => {
    const html = view({ screen: "recovery", accounts: [{ ...account, label: account.serial }], serial: account.serial });
    expect(html).not.toContain("Authenticator_IdentifyAccount");
    expect(html).not.toContain("Authenticator_Browse");
  });

  it("keeps cancellation available during sign-in and blocks it while saving", () => {
    const waiting = view({ screen: "enroll", enrollment: "handle", enrollmentState: "waiting" });
    expect(waiting).toContain("Authenticator_Waiting");
    expect(waiting).not.toMatch(/<button[^>]*disabled[^>]*>Authenticator_Cancel/);
    const saving = view({ screen: "enroll", enrollment: "handle", enrollmentState: "saving" });
    expect(saving).toContain("Authenticator_Saving");
    expect(saving).toMatch(/<button[^>]*disabled[^>]*>Authenticator_Cancel/);
    expect(saving).not.toContain("Authenticator_OpenBrowser");
  });

  it("disables matching imports until replacement is explicitly enabled", () => {
    const props = { screen: "import" as const, accounts: [account], preview: [account] };
    expect(view(props)).toMatch(/<input[^>]*type="checkbox"[^>]*disabled/);
    expect(view({ ...props, form: { ...emptyBattleNetForm(), replaceExisting: true } })).not.toMatch(/<input[^>]*type="checkbox"[^>]*disabled/);
  });
});
