import { describe, expect, it, vi } from "vitest";
import type { SharedMenuItems } from "../../components/PlatformAccountAdapter";
import type { MenuItemDef } from "../../stores/contextMenu";
import { buildSteamExtraMenu } from "./contextMenuBuilder";
import type { SteamMenuDeps } from "./menuCommands";
import { buildSteamGuardMenuItem } from "./steamGuardMenu";
import type { SteamAccountRow, SteamGuardMenuRequest } from "./types";

const setBanStatusHidden = vi.hoisted(() => vi.fn<(hidden: boolean) => Promise<void>>());

vi.mock("./menuCommands", () => ({
  createSteamMenuCommands: () => ({ setBanStatusHidden }),
}));

const labels: Record<string, string> = {
  Context_SteamGuard: "Steam Guard",
};

const tr = (key: string) => labels[key] ?? key;

function account(overrides: Partial<SteamAccountRow> = {}): SteamAccountRow {
  return {
    steamId64: "76561198000000001",
    personaName: "Persona",
    displayName: "Display",
    accountName: " login ",
    currentSession: false,
    showShortNotes: false,
    note: "",
    hasSteamGuard: false,
    steamGuardPending: false,
    ...overrides,
  } as SteamAccountRow;
}

function child(item: MenuItemDef, label: string): MenuItemDef {
  const found = item.children?.find((candidate) => candidate.label === label);
  if (!found) throw new Error(`missing menu item: ${label}`);
  return found;
}

describe("Steam Guard context menu", () => {
  // One entry, whatever the account's state: the submenu that named three flows
  // up front is gone, and the setup page offers them instead.
  it("sends an account the vault does not hold to the setup page", () => {
    const open = vi.fn<(request: SteamGuardMenuRequest) => void>();
    const item = buildSteamGuardMenuItem(account(), open, tr);

    expect(item.label).toBe("Steam Guard");
    expect(item.children).toBeUndefined();
    item.action?.();
    expect(open).toHaveBeenCalledWith({
      action: "setup",
      steamId64: "76561198000000001",
      accountName: "login",
      displayName: "Display",
      pending: false,
    });
  });

  // Every shape the vault stores opens on the account itself. A half-finished
  // enrollment and a session-only record are records too, and offering to add
  // one that is already there is how a pending enrollment got orphaned.
  it.each([
    ["an authenticator", { hasSteamGuard: true }],
    ["a pending enrollment", { steamGuardPending: true }],
    ["a login-only record", { steamGuardLoginOnly: true }],
  ] as const)("opens %s directly", (_label, overrides) => {
    const open = vi.fn<(request: SteamGuardMenuRequest) => void>();
    const item = buildSteamGuardMenuItem(account(overrides), open, tr);

    expect(item.label).toBe("Steam Guard");
    item.action?.();
    expect(open).toHaveBeenCalledWith(expect.objectContaining({
      action: "open",
      steamId64: "76561198000000001",
    }));
  });

  // The unlock screen runs while the vault is locked, so it cannot look the avatar
  // up itself: the already-loaded row has to carry it.
  it("carries the row's avatar so the locked unlock screen shows no placeholder", () => {
    const open = vi.fn<(request: SteamGuardMenuRequest) => void>();
    buildSteamGuardMenuItem(
      account({ hasSteamGuard: true, imageUrl: " /img/a.jpg ", staticImageUrl: " /img/a_static.jpg " }),
      open,
      tr,
    ).action?.();

    expect(open).toHaveBeenCalledWith(expect.objectContaining({
      imageUrl: "/img/a.jpg",
      staticImageUrl: "/img/a_static.jpg",
    }));
  });
});

// Copying a code needs an open vault and an authenticator, and the row carries a
// countdown so the user can see whether the code is about to roll over.
describe("Copy Steam Guard code row", () => {
  const shared = (): SharedMenuItems => {
    const item = (label: string): MenuItemDef => ({ label, action: vi.fn() });
    return {
      swapTo: item("Swap"),
      changeName: item("Rename"),
      createShortcut: item("Shortcut"),
      changeImage: item("Image"),
      forget: item("Forget"),
      notes: item("Notes"),
      tags: item("Tags"),
      gameStats: null,
    };
  };

  const deps = (steamGuardVaultUnlocked: boolean): SteamMenuDeps => ({
    name: "Steam",
    installedGames: [],
    gameDataBySteamId: {},
    steamIds: [],
    refreshGameDataAppSets: async () => {},
    openSteamGuard: vi.fn(),
    steamGuardVaultUnlocked,
  });

  const copyRow = (acc: SteamAccountRow, unlocked: boolean): MenuItemDef | undefined => {
    const menu = buildSteamExtraMenu(acc, shared(), deps(unlocked));
    const copy = menu.find((candidate) => candidate.label === "Context_CopySubmenu");
    return copy?.children?.find((candidate) => candidate.label === "Context_CopySteamGuardCode");
  };

  it("offers a counted-down code only for an authenticator in an open vault", () => {
    expect(copyRow(account({ hasSteamGuard: true }), true)?.progress).toBeDefined();
    expect(copyRow(account({ hasSteamGuard: true }), false)).toBeUndefined();
    expect(copyRow(account({ hasSteamGuard: false }), true)).toBeUndefined();
    expect(copyRow(account({ steamGuardPending: true }), true)).toBeUndefined();
  });
});

describe("Steam account Manage submenu", () => {
  it("contains Notes and Forget instead of exposing them at the root", () => {
    const item = (label: string): MenuItemDef => ({ label, action: vi.fn() });
    const shared: SharedMenuItems = {
      swapTo: item("Swap"),
      changeName: item("Rename"),
      createShortcut: item("Shortcut"),
      changeImage: item("Image"),
      forget: item("Forget"),
      notes: item("Notes"),
      tags: item("Tags"),
      gameStats: null,
    };
    const deps: SteamMenuDeps = {
      name: "Steam",
      installedGames: [],
      gameDataBySteamId: {},
      steamIds: [account().steamId64],
      refreshGameDataAppSets: async () => {},
      openSteamGuard: vi.fn(),
      steamGuardVaultUnlocked: false,
    };

    const menu = buildSteamExtraMenu(account(), shared, deps);
    const manage = menu.find((candidate) => candidate.children?.includes(shared.changeImage));

    expect(menu).not.toContain(shared.notes);
    expect(menu).not.toContain(shared.forget);
    expect(manage?.children?.slice(-2)).toEqual([shared.notes, shared.forget]);
  });
});

// The switcher paints VAC and limited status on the account name. A ban on one
// account is not something the owner necessarily wants on screen every time, so
// it can be hidden per account - but only where it would otherwise be shown.
describe("Steam hide ban status menu item", () => {
  const shared = (): SharedMenuItems => {
    const item = (label: string): MenuItemDef => ({ label, action: vi.fn() });
    return {
      swapTo: item("Swap"),
      changeName: item("Rename"),
      createShortcut: item("Shortcut"),
      changeImage: item("Image"),
      forget: item("Forget"),
      notes: item("Notes"),
      tags: item("Tags"),
      gameStats: null,
    };
  };

  const deps = (): SteamMenuDeps => ({
    name: "Steam",
    installedGames: [],
    gameDataBySteamId: {},
    steamIds: [],
    refreshGameDataAppSets: async () => {},
    openSteamGuard: vi.fn(),
    steamGuardVaultUnlocked: false,
  });

  const manageOf = (acc: SteamAccountRow, items: SharedMenuItems): MenuItemDef => {
    const menu = buildSteamExtraMenu(acc, items, deps());
    const manage = menu.find((candidate) => candidate.children?.includes(items.changeImage));
    if (!manage) throw new Error("missing Manage submenu");
    return manage;
  };

  const labelsOf = (item: MenuItemDef): (string | undefined)[] =>
    (item.children ?? []).map((candidate) => candidate.label);

  it("offers nothing for an account with no ban the settings would show", () => {
    const manage = manageOf(account({ hasVisibleBan: false }), shared());

    expect(labelsOf(manage)).not.toContain("Context_Steam_HideBanStatus");
    expect(labelsOf(manage)).not.toContain("Context_Steam_ShowBanStatus");
  });

  it("hides a shown ban, and offers to show it again once hidden", () => {
    setBanStatusHidden.mockClear();
    child(manageOf(account({ hasVisibleBan: true, banStatusHidden: false }), shared()),
      "Context_Steam_HideBanStatus").action?.();
    expect(setBanStatusHidden).toHaveBeenCalledWith(true);

    setBanStatusHidden.mockClear();
    child(manageOf(account({ hasVisibleBan: true, banStatusHidden: true }), shared()),
      "Context_Steam_ShowBanStatus").action?.();
    expect(setBanStatusHidden).toHaveBeenCalledWith(false);
  });
});
