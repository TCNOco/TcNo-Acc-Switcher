import { get } from "svelte/store";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { SteamGuardModalController } from "../steamGuardModal";
import { activeModal, cancelActiveModal } from "../../stores/modal";
import { emitSteamGuardMenuRequest } from "./steamGuardMenuRequest";
import { bindSteamGuardMenuToModal } from "./steamGuardModalHost";

const controller: SteamGuardModalController = {
  getCode: vi.fn().mockResolvedValue(null),
  unlock: vi.fn().mockRejectedValue(new Error("not connected")),
};

let unsubscribe: (() => void) | undefined;

afterEach(() => {
  unsubscribe?.();
  unsubscribe = undefined;
  if (get(activeModal)) cancelActiveModal();
});

describe("Steam Guard menu modal host", () => {
  it.each([
    ["open", "account"],
    ["all", "all-accounts"],
    ["add", "enrollment"],
    ["import", "import"],
  ] as const)("maps %s requests to the %s entry", (action, entry) => {
    unsubscribe = bindSteamGuardMenuToModal(controller);

    emitSteamGuardMenuRequest({
      action,
      steamId64: "76561198000000001",
      accountName: "test_user",
      displayName: "Test User",
      pending: false,
    });

    expect(get(activeModal)).toMatchObject({
      kind: "steamGuard",
      entry,
      account: {
        id: "76561198000000001",
        username: "test_user",
        displayName: "Test User",
      },
    });
  });

  it("hands the request's avatar to the modal account", () => {
    unsubscribe = bindSteamGuardMenuToModal(controller);

    emitSteamGuardMenuRequest({
      action: "open",
      steamId64: "76561198000000001",
      accountName: "test_user",
      displayName: "Test User",
      pending: false,
      imageUrl: "/img/a.jpg",
      staticImageUrl: "/img/a_static.jpg",
    });

    expect(get(activeModal)).toMatchObject({
      account: { imageUrl: "/img/a.jpg", staticImageUrl: "/img/a_static.jpg" },
    });
  });
});
