import { describe, expect, it } from "vitest";
import { extraFactorsNeeded, passwordIsUsed } from "./steamGuardFactors";

const summary = (factors: string[][], passwordOpens: boolean) => ({
  factors: factors.map((requires) => ({ requires })),
  passwordOpens,
});

describe("extraFactorsNeeded", () => {
  it("asks for nothing while a password-only way in exists", () => {
    // A keyfile enrolled alongside the password is an alternative, not an
    // addition, so the password on its own still opens the vault.
    expect(extraFactorsNeeded(summary([["password"], ["keyfile"]], true))).toEqual([]);
  });

  it("asks for the keyfile once the password alone no longer opens the vault", () => {
    expect(extraFactorsNeeded(summary([["password", "keyfile"], ["recovery"]], false)))
      .toEqual(["keyfile"]);
  });

  it("prefers the way in that uses the password over one that does not", () => {
    expect(extraFactorsNeeded(summary([["recovery"], ["password", "keyfile"]], false)))
      .toEqual(["keyfile"]);
  });

  it("falls back to the only way in when none uses the password", () => {
    expect(extraFactorsNeeded(summary([["keyfile"]], false))).toEqual(["keyfile"]);
    expect(passwordIsUsed(summary([["keyfile"]], false))).toBe(false);
  });

  it("leaves security keys to the backend, which asks the device", () => {
    expect(extraFactorsNeeded(summary([["password", "securitykey"]], false))).toEqual([]);
  });

  it("treats an unknown vault as password-only rather than blocking on a prompt", () => {
    expect(extraFactorsNeeded(null)).toEqual([]);
    expect(passwordIsUsed(null)).toBe(true);
  });
});
