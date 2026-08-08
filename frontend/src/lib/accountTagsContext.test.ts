import { describe, expect, it, vi } from "vitest";
import type { MenuItemDef } from "../stores/contextMenu";

vi.mock("../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js", () => ({
  AddTagToAccount: vi.fn(),
  ApplySpecialTag: vi.fn(),
  CreateTagAndAddToAccount: vi.fn(),
  RemoveTagFromAccount: vi.fn(),
  RemoveTagFromAllAccounts: vi.fn(),
  SetTagExpiry: vi.fn(),
}));

vi.mock("../stores/modal", () => ({
  openTagExpiryModal: vi.fn(),
}));

import {
  buildTagsSectionMenuItem,
  formatTagExpiryCountdown,
  MANAGED_TAG_CS2_COOLDOWN_NAME,
  MANAGED_TAG_CS2_PRIME_NAME,
  SPECIAL_TAG_CS2_DROP_CLAIMED_NAME,
  tagHasExpiry,
  type TagDefRow,
} from "./accountTagsContext";

const tr = (key: string) => key;

function buildMenu(assignedTags: TagDefRow[], tagDefs: TagDefRow[] = assignedTags): MenuItemDef {
  return buildTagsSectionMenuItem({
    platformKey: "Steam",
    uniqueId: "76561199141170487",
    assignedTags,
    tagDefs,
    tr,
    afterChange: vi.fn(),
    onSuccess: vi.fn(),
    onError: vi.fn(),
  });
}

function childByLabel(item: MenuItemDef, label: string): MenuItemDef {
  const child = item.children?.find((c) => c.label === label);
  if (!child) {
    throw new Error(`Missing menu child: ${label}`);
  }
  return child;
}

describe("buildTagsSectionMenuItem", () => {
  it("excludes special tags from the modify submenu", () => {
    const menu = buildMenu([
      { id: "normal", name: "Trade", color: "#336699" },
      { id: "special-random-id", name: SPECIAL_TAG_CS2_DROP_CLAIMED_NAME, color: "#993333" },
    ]);

    const modify = childByLabel(menu, "Tags_Modify");
    expect(modify.children?.map((item) => item.label)).toEqual(["Trade"]);

    const remove = childByLabel(menu, "Tags_Remove");
    expect(remove.children?.map((item) => item.label)).toContain(SPECIAL_TAG_CS2_DROP_CLAIMED_NAME);
  });

  it("hides the modify submenu when only special tags are assigned", () => {
    const menu = buildMenu([
      { id: "special-random-id", name: SPECIAL_TAG_CS2_DROP_CLAIMED_NAME, color: "#993333" },
    ]);

    expect(menu.children?.map((item) => item.label)).not.toContain("Tags_Modify");
    expect(menu.children?.map((item) => item.label)).toContain("Tags_Remove");
  });

  it("never offers to add an app-managed tag", () => {
    // Only the Steam settings checkbox applies these. Offering one under Add
    // would create a tag the next sweep immediately takes back.
    const menu = buildMenu([], [
      { id: "normal", name: "Trade", color: "#336699" },
      { id: "managed", name: MANAGED_TAG_CS2_COOLDOWN_NAME, color: "#c94f4f" },
      { id: "managed-prime", name: MANAGED_TAG_CS2_PRIME_NAME, color: "#c8a11e" },
    ]);

    const add = childByLabel(menu, "Tags_Add");
    const labels = add.children?.filter((item) => item.type !== "search").map((item) => item.label);
    expect(labels).toEqual(["Trade"]);
  });

  it("never offers to remove the app-managed cooldown tag", () => {
    // The sweep reapplies it, so removing it by hand only hides it until the
    // next refresh. It is turned off in Steam settings instead.
    const menu = buildMenu([
      { id: "normal", name: "Trade", color: "#336699" },
      { id: "managed", name: MANAGED_TAG_CS2_COOLDOWN_NAME, color: "#c94f4f" },
    ]);

    const remove = childByLabel(menu, "Tags_Remove");
    expect(remove.children?.map((item) => item.label)).toEqual(["Trade"]);
  });

  it("hides the remove submenu when only the managed tag is assigned", () => {
    const menu = buildMenu([
      { id: "managed", name: MANAGED_TAG_CS2_COOLDOWN_NAME, color: "#c94f4f" },
    ]);

    expect(menu.children?.map((item) => item.label)).not.toContain("Tags_Remove");
  });

  it("hides both modify and remove when no tags are assigned", () => {
    const menu = buildMenu([]);

    expect(menu.children?.map((item) => item.label)).toEqual(["Tags_Add", "Tags_AddSpecial"]);
  });
});

describe("tag expiry countdown helpers", () => {
  const now = Date.parse("2026-07-07T10:00:00Z");

  it("formats remaining expiry time as compact countdown units", () => {
    expect(formatTagExpiryCountdown("2026-07-09T13:04:05Z", now)).toBe("2d 3h 4m 5s");
    expect(formatTagExpiryCountdown("2026-07-07T11:30:00Z", now)).toBe("1h 30m 0s");
    expect(formatTagExpiryCountdown("2026-07-07T10:01:30Z", now)).toBe("1m 30s");
    expect(formatTagExpiryCountdown("2026-07-07T10:00:30Z", now)).toBe("30s");
  });

  it("handles expired or missing expiry values", () => {
    expect(formatTagExpiryCountdown("2026-07-07T09:59:59Z", now)).toBe("");
    expect(formatTagExpiryCountdown("", now)).toBe("");
    expect(formatTagExpiryCountdown("not-a-date", now)).toBe("");
    expect(tagHasExpiry({ expiresAt: "2026-07-07T11:00:00Z" })).toBe(true);
    expect(tagHasExpiry({ expiresAt: "not-a-date" })).toBe(false);
  });
});
