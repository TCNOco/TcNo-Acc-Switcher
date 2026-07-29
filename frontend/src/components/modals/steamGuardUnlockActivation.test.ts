import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const source = readFileSync(new URL("./SteamGuardModalBody.svelte", import.meta.url), "utf8");

function fragmentAfter(marker: string, length = 1_000): string {
  const index = source.indexOf(marker);
  expect(index).toBeGreaterThanOrEqual(0);
  return source.slice(index, index + length);
}

/**
 * The whole actions row, to its closing tag rather than a fixed number of
 * characters: a window sized by hand fails the next time a button is added,
 * which says nothing about whether the row still works.
 */
function unlockActionsRow(): string {
  const start = source.indexOf("steam-guard__actions steam-guard__actions--split");
  expect(start).toBeGreaterThanOrEqual(0);
  const end = source.indexOf("</form>", start);
  expect(end).toBeGreaterThan(start);
  return source.slice(start, end);
}

describe("Steam Guard unlock activation", () => {
  it("directly handles Enter and click for account unlock", () => {
    const password = fragmentAfter('id="steam-guard-password"');
    expect(password).toContain("runOnEnter(event, unlockAccount)");

    const actions = unlockActionsRow();
    expect(actions).toContain('type="button"');
    expect(actions).toContain("on:click={unlockAccount}");
    expect(actions).toContain('busy ? $t("SteamGuard_Unlocking") : $t("SteamGuard_Unlock")');
  });

  // A security key needs nothing typed in, which is the one thing the Unlock
  // button cannot allow without letting an empty form through. It gets its own
  // button, and that button is not hidden behind knowing a key is enrolled.
  it("offers the security key beside Unlock, on its own handler", () => {
    const actions = unlockActionsRow();
    expect(actions).toContain("on:click={() => void unlockWithSecurityKey()}");
    expect(actions).toContain('$t("SteamGuard_Unlock_SecurityKey")');

    // Unlock still requires something typed: the empty case is the other button.
    expect(actions).toContain(
      'disabled={busy || (password.length === 0 && unlockKeyfilePath === "" && unlockBackupKey.trim() === "")}',
    );
  });

  it("keeps Remember Me and Unlock on the same row", () => {
    const row = unlockActionsRow();
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
