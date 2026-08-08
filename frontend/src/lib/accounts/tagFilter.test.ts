import { describe, expect, it } from "vitest";
import type { TagDefRow } from "../accountTagsContext";
import { buildAccountMap, displayIdsForTagFilter } from "./accountPageModel";

type Row = { id: string; tags: TagDefRow[] };

const COOLDOWN: TagDefRow = { id: "cooldown", name: "CS2 Cooldown", color: "#c94f4f" };
const TRADE: TagDefRow = { id: "trade", name: "Trade", color: "#336699" };

const accounts: Row[] = [
  { id: "a", tags: [COOLDOWN] },
  { id: "b", tags: [TRADE] },
  { id: "c", tags: [] },
  { id: "d", tags: [COOLDOWN, TRADE] },
];

const rows = {
  id: (r: Row) => r.id,
  tags: (r: Row) => r.tags,
} as Parameters<typeof displayIdsForTagFilter<Row>>[2];

const ids = accounts.map((a) => a.id);
const map = buildAccountMap(accounts, rows);

function filtered(mode: Parameters<typeof displayIdsForTagFilter<Row>>[3]): string[] {
  return displayIdsForTagFilter(ids, map, rows, mode);
}

describe("displayIdsForTagFilter", () => {
  it("lists only accounts carrying the tag", () => {
    expect(filtered({ kind: "tag", id: "cooldown", name: "CS2 Cooldown" })).toEqual(["a", "d"]);
  });

  it("lists every account not carrying the tag, tagged or not", () => {
    // The point of the negative filter: "which accounts can I queue on" includes
    // untagged accounts and accounts carrying only other tags.
    expect(filtered({ kind: "notTag", id: "cooldown", name: "CS2 Cooldown" })).toEqual(["b", "c"]);
  });

  it("keeps an unresolvable id under a negative filter", () => {
    // "not tagged X" is the inclusive reading; dropping unknown rows would
    // silently shrink the list.
    const withGhost = [...ids, "ghost"];
    expect(displayIdsForTagFilter(withGhost, map, rows, { kind: "notTag", id: "cooldown", name: "CS2 Cooldown" }))
      .toEqual(["b", "c", "ghost"]);
  });

  it("still handles all and untagged", () => {
    expect(filtered({ kind: "all" })).toEqual(ids);
    expect(filtered({ kind: "untagged" })).toEqual(["c"]);
  });
});
