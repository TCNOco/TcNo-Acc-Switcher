import { get } from "svelte/store";
import { beforeEach, describe, expect, it, vi } from "vitest";

const fixture = vi.hoisted(() => ({
  listeners: new Set<(event: { data: unknown }) => void>(),
  call: vi.fn(),
}));
vi.mock("@wailsio/runtime", async (importOriginal) => ({
  ...await importOriginal<typeof import("@wailsio/runtime")>(),
  Call: { ByName: fixture.call },
  Events: { On: (_name: string, callback: (event: { data: unknown }) => void) => { fixture.listeners.add(callback); return () => fixture.listeners.delete(callback); } },
}));
import { battleNetAuthenticators, refreshBattleNetAuthenticators, withBattleNetView } from "./battleNetAuthenticatorBridge";

beforeEach(() => {
  fixture.listeners.clear();
  fixture.call.mockReset();
  fixture.call.mockImplementation(async (name: string, ...args: unknown[]) => {
    if (name.endsWith(".RequestBattleNetView")) {
      for (const callback of fixture.listeners) callback({ data: { requestId: args[0], accountId: "battlenet-authenticators", capability: "fixture-token", lease: "fixture-lease" } });
    }
  });
});

describe("Battle.net context menu authorization", () => {
  it("keeps the lease for the code copy and releases it afterwards", async () => {
    const action = vi.fn(async (token: string) => {
      expect(token).toBe("fixture-token");
      expect(fixture.call).not.toHaveBeenCalledWith(expect.stringContaining("EndSensitiveView"), expect.anything(), expect.anything());
    });
    await withBattleNetView(action);
    expect(action).toHaveBeenCalledOnce();
    expect(fixture.call).toHaveBeenLastCalledWith(expect.stringContaining("EndSensitiveView"), "fixture-token", "fixture-lease");
    expect(fixture.listeners.size).toBe(0);
  });
  it("releases the lease on failure", async () => {
    await expect(withBattleNetView(async () => { throw new Error("copy failed"); })).rejects.toThrow("copy failed");
    expect(fixture.call).toHaveBeenLastCalledWith(expect.stringContaining("EndSensitiveView"), "fixture-token", "fixture-lease");
  });
  it("clears cached menu accounts without acquiring a lease when locked", async () => {
    fixture.call.mockResolvedValue({ unlocked: false });
    await refreshBattleNetAuthenticators();
    expect(get(battleNetAuthenticators)).toEqual({ unlocked: false, accounts: [] });
    expect(fixture.call).toHaveBeenCalledTimes(1);
  });
});
