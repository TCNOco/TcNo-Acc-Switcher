import { describe, expect, it, vi } from "vitest";
import type { SharedMenuItems } from "../../components/PlatformAccountAdapter";
import type { MenuItemDef } from "../../stores/contextMenu";
import { buildSteamExtraMenu } from "./contextMenuBuilder";
import type { SteamMenuDeps } from "./menuCommands";
import { buildSteamGuardMenuItem } from "./steamGuardMenu";
import type { SteamAccountRow, SteamGuardMenuRequest } from "./types";

vi.mock("./menuCommands", () => ({ createSteamMenuCommands: () => ({}) }));

const labels: Record<string, string> = {
  Context_2Factor: "2-Factor",
  Context_AddSteamGuard: "Add Steam Guard",
  Context_ImportMaFile: "Import maFile",
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
  it("shows the exact 2-Factor submenu for an account without an authenticator", () => {
    const open = vi.fn<(request: SteamGuardMenuRequest) => void>();
    const item = buildSteamGuardMenuItem(account({ steamGuardPending: true }), open, tr);

    expect(item.label).toBe("2-Factor");
    expect(item.children?.map(({ label }) => label)).toEqual(["Add Steam Guard", "Import maFile"]);

    child(item, "Add Steam Guard").action?.();
    child(item, "Import maFile").action?.();
    expect(open.mock.calls.map(([request]) => request.action)).toEqual(["add", "import"]);
    expect(open.mock.calls[0]?.[0]).toEqual({
      action: "add",
      steamId64: "76561198000000001",
      accountName: "login",
      displayName: "Display",
      pending: true,
    });
  });

  it("shows one Steam Guard action for an account with an authenticator", () => {
    const open = vi.fn<(request: SteamGuardMenuRequest) => void>();
    const item = buildSteamGuardMenuItem(account({ hasSteamGuard: true }), open, tr);

    expect(item.label).toBe("Steam Guard");
    expect(item.children).toBeUndefined();
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
    };

    const menu = buildSteamExtraMenu(account(), shared, deps);
    const manage = menu.find((candidate) => candidate.children?.includes(shared.changeImage));

    expect(menu).not.toContain(shared.notes);
    expect(menu).not.toContain(shared.forget);
    expect(manage?.children?.slice(-2)).toEqual([shared.notes, shared.forget]);
  });
});
