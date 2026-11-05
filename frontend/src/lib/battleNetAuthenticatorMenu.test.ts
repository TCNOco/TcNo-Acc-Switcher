import { describe, expect, it, vi } from "vitest";
import { battleNetAccountLabel, matchingBattleNetAuthenticators, type BattleNetAccount } from "./battleNetAuthenticator";
import { buildBattleNetAuthenticatorMenu } from "./battleNetAuthenticatorMenu";

const account: BattleNetAccount = { serial: "EU-1234-5678-9012", label: "Example#1234", accountLink: "test@example.invalid", pending: false, hasRestoreCode: true };
const tr = (key: string) => key;
const deps = () => ({ open: vi.fn(), copy: vi.fn(), browse: vi.fn() });

describe("Battle.net account authenticator menus", () => {
  it("displays the account name and serial without repeating a serial-only name", () => {
    expect(battleNetAccountLabel(account)).toBe("Example#1234 (EU-1234-5678-9012)");
    expect(battleNetAccountLabel({ ...account, label: account.serial })).toBe(account.serial);
    expect(battleNetAccountLabel({ ...account, label: "" })).toBe(account.serial);
  });
  it("matches only the linked identifier, with whitespace and case normalized", () => {
    expect(matchingBattleNetAuthenticators([account], " TEST@example.invalid ")).toEqual([account]);
    expect(matchingBattleNetAuthenticators([account], "Example#1234")).toEqual([]);
    expect(matchingBattleNetAuthenticators([account], "")).toEqual([]);
  });
  it("offers copying, browsing, recovery, export, and rename for the matching account", () => {
    const actions = deps();
    const item = buildBattleNetAuthenticatorMenu(account.accountLink, { unlocked: true, accounts: [account] }, tr, actions);
    expect(item.children?.map(row => row.label)).toEqual(["Authenticator_Copy", "Authenticator_Browse", "Authenticator_Recovery", "Authenticator_Export", "Authenticator_Rename"]);
    item.children?.[0].action?.();
    expect(actions.copy).toHaveBeenCalledWith(account.serial);
    expect(item.children?.[0].progress).toBeDefined();
    item.children?.[1].action?.();
    expect(actions.browse).toHaveBeenCalledWith(account.serial);
    item.children?.[2].action?.();
    expect(actions.open).toHaveBeenCalledWith(account.serial, "recovery");
  });
  it.each([false, true])("does not offer another account's code when unlocked=%s", unlocked => {
    const item = buildBattleNetAuthenticatorMenu("other@example.invalid", { unlocked, accounts: [account] }, tr, deps());
    expect(item.children).toBeUndefined();
    expect(buildBattleNetAuthenticatorMenu(account.accountLink, { unlocked: false, accounts: [account] }, tr, deps()).children).toBeUndefined();
  });
  it("keeps multiple matching authenticators separate by serial", () => {
    const second = { ...account, serial: "US-5555-6666-7777" };
    const actions = deps();
    const item = buildBattleNetAuthenticatorMenu(account.accountLink, { unlocked: true, accounts: [account, second] }, tr, actions);
    expect(item.children?.map(row => row.label)).toEqual([battleNetAccountLabel(account), battleNetAccountLabel(second)]);
    item.children?.[1].children?.[0].action?.();
    expect(actions.copy).toHaveBeenCalledWith(second.serial);
    item.action?.();
    expect(actions.open).toHaveBeenCalledWith(undefined);
  });
});
