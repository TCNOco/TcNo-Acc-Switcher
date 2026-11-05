import { describe, expect, it } from "vitest";
import { get } from "svelte/store";
import { authenticatorAction, publishAuthenticatorAction, clearAuthenticatorAction } from "./authenticatorAction";

describe("authenticator action", () => {
  it("publishes a descriptor-selected provider for the current platform", () => {
    publishAuthenticatorAction("BattleNet", "battlenet");
    expect(get(authenticatorAction)).toEqual({ platformKey: "BattleNet", provider: "battlenet" });
    clearAuthenticatorAction("BattleNet");
    expect(get(authenticatorAction).provider).toBe("");
  });
  it("does not let a departing page clear the next platform's action", () => {
    publishAuthenticatorAction("BattleNet", "battlenet");
    clearAuthenticatorAction("Other");
    expect(get(authenticatorAction).provider).toBe("battlenet");
    clearAuthenticatorAction("BattleNet");
  });
});
