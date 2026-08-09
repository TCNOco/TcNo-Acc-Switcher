import { afterEach, describe, expect, it } from "vitest";
import { get } from "svelte/store";
import { buildFilterMenuItems } from "./filterMenu";
import { platformListSort } from "../stores/platformListSort";
import { resetSteamPageTab, steamPageTab } from "../stores/steamPageTab";

/** Untranslated `t` echoes the key back, so labels assert as resource keys. */
function labels(items: { label?: string }[]): string[] {
  return items.map((item) => item.label ?? "");
}

function sortChildren(platformName: string) {
  const items = buildFilterMenuItems(platformName);
  return items[1]?.children ?? [];
}

describe("buildFilterMenuItems", () => {
  afterEach(() => {
    resetSteamPageTab();
  });

  it("always leads with Search then Sort By", () => {
    for (const platformName of ["", "Steam", "Epic Games"]) {
      expect(labels(buildFilterMenuItems(platformName))).toEqual([
        "Filter_Search",
        "Filter_SortBy",
      ]);
    }
  });

  it("sorts the home grid by name only", () => {
    expect(labels(sortChildren(""))).toEqual(["Filter_Sort_AZ", "Filter_Sort_ZA"]);
  });

  it("offers last-used ordering on platform pages", () => {
    expect(labels(sortChildren("Epic Games"))).toEqual([
      "Filter_Sort_AZ",
      "Filter_Sort_ZA",
      "Filter_Sort_LastUsed",
    ]);
  });

  it("keeps alphabetical entries leaf rows off Steam", () => {
    const alpha = sortChildren("Epic Games")[0];
    expect(alpha?.children).toBeUndefined();
    expect(alpha?.action).toBeTypeOf("function");
  });

  // Steam rows have a display name and an account name, so both orderings nest a
  // by-username variant while the parent row stays clickable for display-name sort.
  it("nests username ordering under each alphabetical entry on Steam", () => {
    const [az, za] = sortChildren("Steam");
    expect(labels(az?.children ?? [])).toEqual(["Filter_Sort_AZ", "Filter_Sort_Username"]);
    expect(labels(za?.children ?? [])).toEqual(["Filter_Sort_ZA", "Filter_Sort_Username"]);
    expect(az?.action).toBeTypeOf("function");
    expect(za?.action).toBeTypeOf("function");
  });

  // Only game rows have owners to count, and Steam's two tabs share this one menu.
  it("offers owner-count ordering on Steam's games tab only", () => {
    expect(labels(sortChildren("Steam"))).not.toContain("Filter_Sort_OwnedCount");

    steamPageTab.set("games");
    expect(labels(sortChildren("Steam"))).toEqual([
      "Filter_Sort_AZ",
      "Filter_Sort_ZA",
      "Filter_Sort_OwnedCount",
    ]);
    expect(labels(sortChildren("Epic Games"))).not.toContain("Filter_Sort_OwnedCount");
  });

  // A game has one name, so there is no second ordering to nest under either direction.
  it("flattens the alphabetical entries on the games tab", () => {
    steamPageTab.set("games");
    const [az, za] = sortChildren("Steam");
    expect(az?.children).toBeUndefined();
    expect(za?.children).toBeUndefined();

    az?.action?.();
    expect(get(platformListSort)?.kind).toBe("alpha_asc");
    za?.action?.();
    expect(get(platformListSort)?.kind).toBe("alpha_desc");
  });

  // sortOwnedGames has no timestamp to read, so a last-used entry would do nothing.
  it("drops last-used ordering on the games tab", () => {
    expect(labels(sortChildren("Steam"))).toContain("Filter_Sort_LastUsed");

    steamPageTab.set("games");
    expect(labels(sortChildren("Steam"))).not.toContain("Filter_Sort_LastUsed");
  });

  // The games tab must not reach back into the account list's menu shape.
  it("leaves the account variant nested when the tab flips back", () => {
    steamPageTab.set("games");
    steamPageTab.set("switcher");
    const [az, za] = sortChildren("Steam");
    expect(labels(az?.children ?? [])).toEqual(["Filter_Sort_AZ", "Filter_Sort_Username"]);
    expect(labels(za?.children ?? [])).toEqual(["Filter_Sort_ZA", "Filter_Sort_Username"]);
  });

  it("fires both owner-count directions", () => {
    steamPageTab.set("games");
    const owned = sortChildren("Steam").at(-1)?.children ?? [];
    expect(labels(owned)).toEqual(["Filter_Sort_Ascending", "Filter_Sort_Descending"]);

    owned[0]?.action?.();
    expect(get(platformListSort)?.kind).toBe("owned_count_asc");
    owned[1]?.action?.();
    expect(get(platformListSort)?.kind).toBe("owned_count_desc");
  });
});
