import { get } from "svelte/store";
import { describe, expect, it, vi } from "vitest";
import type { SteamGuardModalController } from "../lib/steamGuardModal";
import {
  activeModal,
  cancelActiveModal,
  openSteamGuardEnrollment,
  openSteamGuardForAccount,
  openSteamGuardImport,
  openSteamGuardQrLogin,
} from "./modal";

const account = { id: "76561198000000001", username: "test_user" };

function controller(): SteamGuardModalController {
  return {
    getCode: vi.fn().mockResolvedValue(null),
    unlock: vi.fn().mockRejectedValue(new Error("not connected")),
  };
}

describe("Steam Guard modal integration", () => {
  it("stores adapter functions and non-secret metadata only", async () => {
    const pending = openSteamGuardForAccount(account, controller());
    const modal = get(activeModal);

    expect(modal).toMatchObject({
      kind: "steamGuard",
      title: "Steam Guard",
      account,
      entry: "account",
    });
    expect(modal && Object.keys(modal)).not.toContain("password");
    expect(modal && Object.keys(modal)).not.toContain("code");

    cancelActiveModal();
    await expect(pending).resolves.toBeUndefined();
  });

  it.each([
    [openSteamGuardImport, "import"],
    [openSteamGuardEnrollment, "enrollment"],
    [openSteamGuardQrLogin, "qr"],
  ] as const)("opens the requested %s entry", async (open, entry) => {
    const pending = open(account, controller());

    expect(get(activeModal)).toMatchObject({ kind: "steamGuard", entry });

    cancelActiveModal();
    await pending;
  });
});
