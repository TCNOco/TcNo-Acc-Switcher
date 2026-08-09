import { describe, expect, it } from "vitest";
import { buildFilterMenuItems } from "./filterMenu";

/** Untranslated `t` echoes the key back, so labels assert as resource keys. */
function labels(items: { label?: string }[]): string[] {
  return items.map((item) => item.label ?? "");
}

function sortChildren(platformName: string) {
  const items = buildFilterMenuItems(platformName);
  return items[1]?.children ?? [];
}

describe("buildFilterMenuItems", () => {
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
});
