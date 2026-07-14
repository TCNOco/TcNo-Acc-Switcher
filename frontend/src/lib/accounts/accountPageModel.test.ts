import { describe, expect, it } from "vitest";
import type { AccountRowProjection } from "../../components/PlatformAccountAdapter";
import { hasActiveAccountTags } from "./accountPageModel";

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
});
