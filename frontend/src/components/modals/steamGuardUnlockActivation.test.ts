import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const source = readFileSync(new URL("./SteamGuardModalBody.svelte", import.meta.url), "utf8");

function fragmentAfter(marker: string, length = 1_000): string {
  const index = source.indexOf(marker);
  expect(index).toBeGreaterThanOrEqual(0);
  return source.slice(index, index + length);
}

describe("Steam Guard unlock activation", () => {
  it("directly handles Enter and click for account unlock", () => {
    const password = fragmentAfter('id="steam-guard-password"');
    expect(password).toContain("runOnEnter(event, unlockAccount)");

    const actions = fragmentAfter("steam-guard__actions steam-guard__actions--split");
    expect(actions).toContain('type="button"');
    expect(actions).toContain("on:click={unlockAccount}");
    expect(actions).toContain('busy ? $t("SteamGuard_Unlocking") : $t("SteamGuard_Unlock")');
  });

  it("keeps Remember Me and Unlock on the same row", () => {
    const row = fragmentAfter("steam-guard__actions steam-guard__actions--split");
    const remember = row.indexOf('for="steam-guard-remember-session"');
    const unlock = row.indexOf("on:click={unlockAccount}");
    expect(remember).toBeGreaterThanOrEqual(0);
    expect(unlock).toBeGreaterThan(remember);
  });

  it("directly handles Enter and click for vault unlock", () => {
    const password = fragmentAfter('id="steam-enrollment-vault-password"');
    expect(password).toContain("runOnEnter(event, submitVaultPreparation)");

    const error = fragmentAfter('{#if inlineError}<p class="steam-guard__error"', 1_400);
    expect(error).toContain('type="button"');
    expect(error).toContain("on:click={submitVaultPreparation}");
    expect(error).toContain('$t("SteamGuard_Unlocking")');
  });
});
