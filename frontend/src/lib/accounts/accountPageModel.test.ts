import { describe, expect, it } from "vitest";
import type { AccountRowProjection } from "../../components/PlatformAccountAdapter";
import { hasActiveAccountTags, mergeGameStatsByAccount } from "./accountPageModel";

type Row = {
  id: string;
  tags?: { id: string; name: string; color: string }[];
};

const rows = {
  tags: (row: Row) => row.tags,
} as AccountRowProjection<Row>;

describe("accountPageModel", () => {
  it("reports active tags only when a loaded account has at least one tag", () => {
    expect(hasActiveAccountTags([], rows)).toBe(false);
    expect(hasActiveAccountTags([{ id: "a" }, { id: "b", tags: [] }], rows)).toBe(false);
    expect(hasActiveAccountTags([{ id: "a", tags: [{ id: "tag1", name: "Tagged", color: "#333333" }] }], rows)).toBe(true);
  });

  it("retains other accounts when one account's game stats refresh", () => {
    const current = {
      accountA: { Game: { wins: { statValue: "1", indicatorMarkup: "" } } },
      accountB: { Game: { wins: { statValue: "2", indicatorMarkup: "" } } },
    };
    const patch = {
      accountA: { Game: { wins: { statValue: "3", indicatorMarkup: "" } } },
    };

    expect(mergeGameStatsByAccount(current, patch)).toEqual({
      accountA: patch.accountA,
      accountB: current.accountB,
    });
  });
});
