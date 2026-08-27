import { get } from "svelte/store";
import { afterEach, describe, expect, it } from "vitest";
import {
  clearSteamGuardActionAccount,
  publishSteamGuardActionAccounts,
  steamGuardActionAccount,
  steamGuardActionAccountFrom,
} from "./steamGuardAction";

afterEach(clearSteamGuardActionAccount);

describe("Steam Guard action-bar account", () => {
  it("stays hidden when no active authenticator exists", () => {
    expect(steamGuardActionAccountFrom([
      {
        steamId64: "76561198000000001",
        accountName: "pending_user",
        hasSteamGuard: false,
      },
    ])).toBeNull();
  });

  it("selects the first active authenticator without retaining secrets", () => {
    expect(steamGuardActionAccountFrom([
      {
        steamId64: "76561198000000001",
        accountName: "plain_user",
        hasSteamGuard: false,
      },
      {
        steamId64: "76561198000000002",
        accountName: "guarded_user",
        personaName: "Guarded User",
        hasSteamGuard: true,
      },
    ])).toEqual({
      steamId64: "76561198000000002",
      accountName: "guarded_user",
      displayName: "Guarded User",
    });
  });

  it("carries the switcher avatar so the action-bar entry needs no locked-vault lookup", () => {
    expect(steamGuardActionAccountFrom([
      {
        steamId64: "76561198000000004",
        accountName: "guarded_user",
        hasSteamGuard: true,
        imageUrl: " /img/a.jpg ",
        staticImageUrl: " /img/a_static.jpg ",
      },
    ])).toMatchObject({
      imageUrl: "/img/a.jpg",
      staticImageUrl: "/img/a_static.jpg",
    });
  });

  it("offers a login-only account when it is the only one in the vault", () => {
    // The toolbar entry opens the vault, and a login-only record is in the vault
    // just as much as an authenticator is.
    expect(steamGuardActionAccountFrom([
      {
        steamId64: "76561198000000005",
        accountName: "session_only",
        hasSteamGuard: false,
        steamGuardLoginOnly: true,
      },
    ])).toMatchObject({ steamId64: "76561198000000005" });
  });

  it("prefers an authenticator over a login-only account", () => {
    expect(steamGuardActionAccountFrom([
      {
        steamId64: "76561198000000005",
        accountName: "session_only",
        steamGuardLoginOnly: true,
      },
      {
        steamId64: "76561198000000006",
        accountName: "guarded_user",
        hasSteamGuard: true,
      },
    ])).toMatchObject({ steamId64: "76561198000000006" });
  });

  it("publishes and clears action-bar availability", () => {
    publishSteamGuardActionAccounts([
      {
        steamId64: "76561198000000003",
        accountName: "third_user",
        hasSteamGuard: true,
      },
    ]);

    expect(get(steamGuardActionAccount)?.steamId64).toBe("76561198000000003");
    clearSteamGuardActionAccount();
    expect(get(steamGuardActionAccount)).toBeNull();
  });
});
