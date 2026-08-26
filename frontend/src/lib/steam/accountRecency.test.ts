import { describe, expect, it } from "vitest";
import { orderByAccountRank, rankAccountsByRecency, type AccountRecency } from "./accountRecency";

function account(steamId64: string, lastLogin = ""): AccountRecency {
  return { steamId64, lastLogin };
}

function rankMap(order: string[]): Map<string, number> {
  return new Map(order.map((id, index) => [id, index]));
}

describe("rankAccountsByRecency", () => {
  it("puts the newest login first", () => {
    const ranked = rankAccountsByRecency(
      [
        account("a", "2024-03-01T10:00:00Z"),
        account("b", "2026-01-05T10:00:00Z"),
        account("c", "2025-07-20T10:00:00Z"),
      ],
      false,
    );
    expect(ranked).toEqual(["b", "c", "a"]);
  });

  it("keeps accounts Steam has no login date for behind every account that has one", () => {
    const ranked = rankAccountsByRecency(
      [account("never"), account("old", "2020-01-01T00:00:00Z"), account("blank", "   ")],
      false,
    );
    expect(ranked).toEqual(["old", "never", "blank"]);
  });

  it("falls back to the account list's order for equal dates and for the undated block", () => {
    const same = "2025-05-05T00:00:00Z";
    expect(rankAccountsByRecency([account("z", same), account("a", same)], false)).toEqual(["z", "a"]);
    expect(rankAccountsByRecency([account("z"), account("a")], false)).toEqual(["z", "a"]);
  });

  it("leaves an order the user arranged alone, however old its logins are", () => {
    const accounts = [account("chosen-first"), account("chosen-second", "2026-01-05T10:00:00Z")];
    expect(rankAccountsByRecency(accounts, true)).toEqual(["chosen-first", "chosen-second"]);
  });

  it("survives an unparseable date without dropping the account", () => {
    const ranked = rankAccountsByRecency(
      [account("junk", "not a date"), account("real", "2025-01-01T00:00:00Z")],
      false,
    );
    expect(ranked).toEqual(["real", "junk"]);
  });
});

describe("orderByAccountRank", () => {
  it("hands the pinned slots to the most recently used owners of the game", () => {
    const ranked = rankAccountsByRecency(
      [
        account("stale", "2019-01-01T00:00:00Z"),
        account("newest", "2026-02-01T00:00:00Z"),
        account("recent", "2026-01-01T00:00:00Z"),
        account("older", "2024-01-01T00:00:00Z"),
        account("unused"),
        account("middling", "2025-01-01T00:00:00Z"),
      ],
      false,
    );
    // Owners arrive in whatever order the games list built them in.
    const owners = ["unused", "stale", "middling", "newest", "older", "recent"].map((steamId64) => ({
      steamId64,
    }));
    const tiles = orderByAccountRank(owners, rankMap(ranked)).map((t) => t.steamId64);
    expect(tiles.slice(0, 4)).toEqual(["newest", "recent", "middling", "older"]);
    expect(tiles.slice(4)).toEqual(["stale", "unused"]);
  });

  it("does not reorder the caller's array", () => {
    const owners = [{ steamId64: "b" }, { steamId64: "a" }];
    orderByAccountRank(owners, rankMap(["a", "b"]));
    expect(owners.map((o) => o.steamId64)).toEqual(["b", "a"]);
  });

  it("trails owners the ranking has not caught up with", () => {
    const owners = [{ steamId64: "unranked" }, { steamId64: "ranked" }];
    expect(orderByAccountRank(owners, rankMap(["ranked"])).map((o) => o.steamId64)).toEqual([
      "ranked",
      "unranked",
    ]);
  });
});
