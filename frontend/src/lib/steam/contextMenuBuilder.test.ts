import { describe, expect, it, vi } from "vitest";
import type { SharedMenuItems } from "../../components/PlatformAccountAdapter";
import type { MenuItemDef } from "../../stores/contextMenu";
import { buildSteamExtraMenu } from "./contextMenuBuilder";
import type { SteamMenuDeps } from "./menuCommands";
import { buildSteamGuardMenuItem } from "./steamGuardMenu";
import type { SteamAccountRow, SteamGuardMenuRequest } from "./types";
import type { SteamBrowserSite } from "./steamBrowserSites";

const setBanStatusHidden = vi.hoisted(() => vi.fn<(hidden: boolean) => Promise<void>>());
const copySteamId = vi.hoisted(() => vi.fn<(format: string) => Promise<void>>());
const copyTradeLink = vi.hoisted(() => vi.fn<() => Promise<void>>());

vi.mock("./menuCommands", () => ({
  createSteamMenuCommands: () => ({ setBanStatusHidden, copySteamId, copyTradeLink }),
}));

const labels: Record<string, string> = {
  Context_SteamGuard: "Steam Guard",
  Context_Steam_OpenStore: "Steam Store",
  Context_Steam_OpenCommunity: "Steam Community",
  Context_Steam_OpenChat: "Steam Chat",
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
    const item = buildSteamGuardMenuItem(account(), { openSteamGuard: open, vaultUnlocked: false }, tr);

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
    const item = buildSteamGuardMenuItem(account(overrides), { openSteamGuard: open, vaultUnlocked: false }, tr);

    expect(item.label).toBe("Steam Guard");
    item.action?.();
    expect(open).toHaveBeenCalledWith(expect.objectContaining({
      action: "open",
      steamId64: "76561198000000001",
    }));
  });

  // The browsing entries hang off the same row rather than replacing its click,
  // so the usual Steam Guard flow still opens on it.
  it("offers the sites without taking over the row's own action", () => {
    const open = vi.fn<(request: SteamGuardMenuRequest) => void>();
    const openBrowser = vi.fn<(account: SteamAccountRow, site: SteamBrowserSite) => void>();
    const item = buildSteamGuardMenuItem(
      account({ hasSteamGuard: true }),
      { openSteamGuard: open, openBrowser, vaultUnlocked: true },
      tr,
    );

    expect(item.children?.map((c) => c.label)).toEqual(["Steam Store", "Steam Community", "Steam Chat"]);
    item.action?.();
    expect(open).toHaveBeenCalledWith(expect.objectContaining({ action: "open" }));

    const named = expect.objectContaining({ steamId64: "76561198000000001" });
    for (const [index, site] of (["store", "community", "chat"] as const).entries()) {
      item.children?.[index].action?.();
      expect(openBrowser).toHaveBeenCalledWith(named, site);
    }
  });

  // A session-only record holds the same tokens an authenticator does, so it
  // browses identically.
  it("offers browsing for a login-only record", () => {
    const item = buildSteamGuardMenuItem(
      account({ steamGuardLoginOnly: true }),
      { openSteamGuard: vi.fn(), openBrowser: vi.fn(), vaultUnlocked: true },
      tr,
    );
    expect(item.children).toHaveLength(3);
  });

  // Minting a session needs the vault open, and an account it does not hold has
  // no session at all, so neither can offer browsing.
  it.each([
    ["the vault is locked", { hasSteamGuard: true }, { vaultUnlocked: false, withBrowser: true }],
    ["the account is not held", {}, { vaultUnlocked: true, withBrowser: true }],
    ["the build cannot browse", { hasSteamGuard: true }, { vaultUnlocked: true, withBrowser: false }],
  ] as const)("hides browsing when %s", (_label, overrides, opts) => {
    const item = buildSteamGuardMenuItem(
      account(overrides),
      {
        openSteamGuard: vi.fn(),
        openBrowser: opts.withBrowser ? vi.fn() : undefined,
        vaultUnlocked: opts.vaultUnlocked,
      },
      tr,
    );
    expect(item.children).toBeUndefined();
  });

  // The unlock screen runs while the vault is locked, so it cannot look the avatar
  // up itself: the already-loaded row has to carry it.
  it("carries the row's avatar so the locked unlock screen shows no placeholder", () => {
    const open = vi.fn<(request: SteamGuardMenuRequest) => void>();
    buildSteamGuardMenuItem(
      account({ hasSteamGuard: true, imageUrl: " /img/a.jpg ", staticImageUrl: " /img/a_static.jpg " }),
      { openSteamGuard: open, vaultUnlocked: false },
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

// Steam only shows an account its own trade URL, and only over that account's
// session, so the row is worth offering exactly when the vault can supply one.
describe("Copy Trade Link", () => {
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

  const copySubmenu = (acc: SteamAccountRow, unlocked: boolean): MenuItemDef => {
    const menu = buildSteamExtraMenu(acc, shared(), deps(unlocked));
    const copy = menu.find((candidate) => candidate.label === "Context_CopySubmenu");
    if (!copy) throw new Error("missing Copy submenu");
    return copy;
  };

  const tradeRow = (acc: SteamAccountRow, unlocked: boolean): MenuItemDef | undefined =>
    copySubmenu(acc, unlocked).children?.find((c) => c.label === "Context_Steam_TradeLink");

  it.each([
    ["a full authenticator", { hasSteamGuard: true }, true, true],
    ["a login-only record", { steamGuardLoginOnly: true }, true, true],
    // In the vault, but whether it holds a usable session is not guaranteed.
    ["a half-finished enrollment", { steamGuardPending: true }, true, false],
    ["an account the vault does not hold", {}, true, false],
    ["a locked vault", { hasSteamGuard: true }, false, false],
  ] as const)("%s: offered = %o", (_name, overrides, unlocked, offered) => {
    expect(tradeRow(account(overrides), unlocked) !== undefined).toBe(offered);
  });

  it("sits last in the Copy submenu, directly above the SteamID rows", () => {
    const labels = (copySubmenu(account({ hasSteamGuard: true }), true).children ?? [])
      .map((c) => c.label);
    expect(labels.slice(-2)).toEqual(["Context_Steam_TradeLink", "Context_CopySteamIdSubmenu"]);
  });

  it("fetches on click rather than carrying a remembered link", () => {
    copyTradeLink.mockClear();
    // Building the menu must not reach Steam: menus are built on every
    // right-click, and the link is only wanted when the row is actually used.
    const row = tradeRow(account({ hasSteamGuard: true }), true);
    expect(copyTradeLink).not.toHaveBeenCalled();
    row?.action?.();
    expect(copyTradeLink).toHaveBeenCalledTimes(1);
  });
});

// Every row here reads one field off the Go SteamIDFormats struct, so a renamed
// or dropped field turns into a silently empty clipboard rather than an error.
describe("Copy SteamID submenu", () => {
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

  const deps: SteamMenuDeps = {
    name: "Steam",
    installedGames: [],
    gameDataBySteamId: {},
    steamIds: [],
    refreshGameDataAppSets: async () => {},
    openSteamGuard: vi.fn(),
    steamGuardVaultUnlocked: false,
  };

  const idSubmenu = (): MenuItemDef => {
    const menu = buildSteamExtraMenu(account(), shared(), deps);
    const copy = menu.find((candidate) => candidate.label === "Context_CopySubmenu");
    if (!copy) throw new Error("missing Copy submenu");
    return child(copy, "Context_CopySteamIdSubmenu");
  };

  it.each([
    ["Context_Steam_Id64", "ID64"],
    ["Context_Steam_Id3", "ID3"],
    ["Context_Steam_Id32", "ID32"],
    ["Context_Steam_FriendCode", "FriendCode"],
    ["Context_Steam_Cs2FriendCode", "CS2FriendCode"],
  ])("copies %s as %s", (label, format) => {
    copySteamId.mockClear();
    child(idSubmenu(), label).action?.();
    expect(copySteamId).toHaveBeenCalledWith(format);
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

// Steam holds a Personal Game Data page for a few of its own titles, under the
// account's profile. Reaching one means opening it as the account, so it rides
// on the same session the Steam Guard browsing entries do.
describe("Personal Game Data rows", () => {
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

  const held = account({ hasSteamGuard: true });

  const deps = (overrides: Partial<SteamMenuDeps> = {}): SteamMenuDeps => ({
    name: "Steam",
    installedGames: [
      { appId: "730", name: "Counter-Strike 2" },
      { appId: "440", name: "Team Fortress 2" },
      { appId: "570", name: "Dota 2" },
      { appId: "252950", name: "Rocket League" },
    ],
    gameDataBySteamId: {},
    steamIds: [],
    refreshGameDataAppSets: async () => {},
    openSteamGuard: vi.fn(),
    openSteamBrowser: vi.fn(),
    steamGuardVaultUnlocked: true,
    ...overrides,
  });

  const gameDataMenu = (acc: SteamAccountRow, d: SteamMenuDeps): MenuItemDef => {
    const menu = buildSteamExtraMenu(acc, shared(), d);
    const manage = menu.find((candidate) => candidate.label === "Context_ManageSubmenu");
    if (!manage) throw new Error("missing Manage submenu");
    return child(manage, "Context_GameDataSubmenu");
  };

  const gameNames = (item: MenuItemDef): (string | undefined)[] =>
    (item.children ?? []).filter((c) => c.type !== "search").map((c) => c.label);

  it.each([
    ["Counter-Strike 2", "gamedata-730"],
    ["Team Fortress 2", "gamedata-440"],
    ["Dota 2", "gamedata-570"],
  ] as const)("opens %s on its own page", (game, site) => {
    const openSteamBrowser = vi.fn<(acc: SteamAccountRow, site: SteamBrowserSite) => void>();
    const menu = gameDataMenu(held, deps({ openSteamBrowser }));

    child(child(menu, game), "Context_Steam_PersonalGameData").action?.();
    expect(openSteamBrowser).toHaveBeenCalledWith(held, site);
  });

  // The page belongs to Steam, not to userdata, so a game with no local files
  // and no backup still has one - but nothing else to offer.
  it("lists a game with a page even when it has no local data", () => {
    const menu = gameDataMenu(held, deps());
    expect(gameNames(menu)).toEqual(["Counter-Strike 2", "Team Fortress 2", "Dota 2"]);
    expect((child(menu, "Dota 2").children ?? []).map((c) => c.label))
      .toEqual(["Context_Steam_PersonalGameData"]);
  });

  // Same gate as the browsing entries: a session is needed to open the page as
  // the account, and without one there is nothing to show.
  it.each([
    ["the vault is locked", held, { steamGuardVaultUnlocked: false }],
    ["the build cannot browse", held, { openSteamBrowser: undefined }],
    ["the account is not held", account(), {}],
  ] as const)("offers nothing when %s", (_label, acc, overrides) => {
    const menu = gameDataMenu(acc, deps(overrides));
    // Down to the empty placeholder the submenu shows when it has no rows.
    expect(menu.children?.map((c) => c.label)).toEqual(["Context_GameData_NoFolders"]);
  });

  // A game Steam has no page for is still listed only when it has local data,
  // which is what the submenu was for before any of this.
  it("leaves a game with neither a page nor local data out", () => {
    const menu = gameDataMenu(held, deps({
      gameDataBySteamId: {
        [held.steamId64]: { userdata: new Set(["252950"]), backup: new Set() },
      },
    }));
    expect(gameNames(menu)).toContain("Rocket League");
    expect((child(menu, "Rocket League").children ?? []).map((c) => c.label))
      .toEqual(["Open folder", "Context_Game_CopySettingsFrom", "Context_Game_BackupData"]);
  });
});
