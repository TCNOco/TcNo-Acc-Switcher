import { describe, expect, it } from "vitest";
import {
  chunkOwnedGames,
  filterOwnedGames,
  gameOwnerAccounts,
  ownerDisplayName,
  ownersTooltipText,
  sortOwnedGames,
  splitGameOwners,
  type OwnedGameRow,
} from "./ownedGames";

function game(name: string, owners: string[] = []): OwnedGameRow {
  return { appId: name, name, iconUrl: "", owners };
}

describe("sortOwnedGames", () => {
  const games = [game("banana"), game("Apple"), game("cherry")];

  it("sorts case-insensitively both ways", () => {
    expect(sortOwnedGames(games, "alpha_asc").map((g) => g.name)).toEqual(["Apple", "banana", "cherry"]);
    expect(sortOwnedGames(games, "alpha_desc").map((g) => g.name)).toEqual(["cherry", "banana", "Apple"]);
  });

  it("falls back to the game name for Steam's by-username kinds", () => {
    expect(sortOwnedGames(games, "steam_user_asc").map((g) => g.name)).toEqual(["Apple", "banana", "cherry"]);
    expect(sortOwnedGames(games, "steam_user_desc").map((g) => g.name)).toEqual(["cherry", "banana", "Apple"]);
  });

  it("leaves the order alone for kinds a game has no data for", () => {
    expect(sortOwnedGames(games, "lastused_new_old")).toBe(games);
  });
});

describe("sortOwnedGames by owner count", () => {
  const games = [
    game("delta", ["a"]),
    game("alpha", ["a", "b", "c"]),
    game("Charlie", ["a", "b"]),
    game("bravo", ["a", "b"]),
    game("zulu"),
    game("echo"),
  ];
  const names = (kind: "owned_count_asc" | "owned_count_desc") =>
    sortOwnedGames(games, kind).map((g) => g.name);

  it("orders by how many accounts own the game", () => {
    expect(names("owned_count_asc")).toEqual(["delta", "bravo", "Charlie", "alpha", "echo", "zulu"]);
    expect(names("owned_count_desc")).toEqual(["alpha", "bravo", "Charlie", "delta", "echo", "zulu"]);
  });

  it("breaks equal counts by name, case-insensitively and the same way in both directions", () => {
    for (const kind of ["owned_count_asc", "owned_count_desc"] as const) {
      const order = names(kind);
      expect(order.indexOf("bravo")).toBeLessThan(order.indexOf("Charlie"));
    }
  });

  it("keeps ownership-unknown games in one block at the end of both directions", () => {
    for (const kind of ["owned_count_asc", "owned_count_desc"] as const) {
      expect(names(kind).slice(-2)).toEqual(["echo", "zulu"]);
    }
  });

  // Unknown is not zero: an owned game still outranks it when counts run low-to-high.
  it("does not rank unknown ownership as zero owners", () => {
    const rows = [game("unknown"), game("owned", ["a"])];
    expect(sortOwnedGames(rows, "owned_count_asc").map((g) => g.name)).toEqual(["owned", "unknown"]);
  });
});

describe("filterOwnedGames", () => {
  const games = [game("Half-Life 2"), game("Portal 2"), game("Team Fortress 2")];

  it("returns the same list for a blank query", () => {
    expect(filterOwnedGames(games, "   ")).toBe(games);
  });

  it("matches every word anywhere in the name", () => {
    expect(filterOwnedGames(games, "2 port").map((g) => g.name)).toEqual(["Portal 2"]);
  });
});

describe("splitGameOwners", () => {
  it("shows everything when it fits", () => {
    expect(splitGameOwners(["a", "b"], 3)).toEqual({ shown: ["a", "b"], overflow: 0 });
  });

  it("counts the remainder past the cap", () => {
    expect(splitGameOwners(["a", "b", "c", "d"], 2)).toEqual({ shown: ["a", "b"], overflow: 2 });
  });

  it("hides every owner when there is no room", () => {
    expect(splitGameOwners(["a", "b"], 0)).toEqual({ shown: [], overflow: 2 });
  });
});

describe("gameOwnerAccounts", () => {
  const accounts = new Map([
    ["a", { steamId64: "a" }],
    ["b", { steamId64: "b" }],
    ["c", { steamId64: "c" }],
  ]);

  it("keeps the owner order the row shows", () => {
    expect(gameOwnerAccounts(game("x", ["c", "a"]), accounts).map((v) => v.steamId64)).toEqual(["c", "a"]);
  });

  it("offers nothing when no game is selected", () => {
    expect(gameOwnerAccounts(null, accounts)).toEqual([]);
  });

  // Empty owners means ownership was never resolved, not that every account qualifies.
  it("offers nothing for a game whose ownership is unknown", () => {
    expect(gameOwnerAccounts(game("x"), accounts)).toEqual([]);
  });

  it("drops owners with no account behind them", () => {
    expect(gameOwnerAccounts(game("x", ["a", "gone", "b"]), accounts).map((v) => v.steamId64)).toEqual(["a", "b"]);
  });
});

describe("ownerDisplayName", () => {
  const accounts = new Map([["76561198000000001", { displayName: "Wesley" }]]);

  it("names a known owner", () => {
    expect(ownerDisplayName(accounts, "76561198000000001")).toBe("Wesley");
  });

  // Games load before accounts do, so an unresolved id is the normal first paint.
  it("falls back to the id while the account list is still loading", () => {
    expect(ownerDisplayName(new Map(), "76561198000000001")).toBe("76561198000000001");
    expect(ownerDisplayName(new Map([["76561198000000001", { displayName: "" }]]), "76561198000000001"))
      .toBe("76561198000000001");
  });
});

describe("ownersTooltipText", () => {
  const accounts = new Map([
    ["a", { displayName: "Ann" }],
    ["b", { displayName: "Bob" }],
  ]);

  it("joins every owner's name with the given separator", () => {
    expect(ownersTooltipText(accounts, ["a", "b"], "unknown", "\n")).toBe("Ann\nBob");
    expect(ownersTooltipText(accounts, ["a", "b"], "unknown", ", ")).toBe("Ann, Bob");
  });

  it("uses the unknown text when ownership was never resolved", () => {
    expect(ownersTooltipText(accounts, [], "unknown", ", ")).toBe("unknown");
  });

  it("keeps unresolved ids in place alongside resolved names", () => {
    expect(ownersTooltipText(accounts, ["a", "c"], "unknown", ", ")).toBe("Ann, c");
  });
});

describe("chunkOwnedGames", () => {
  const rows = Array.from({ length: 7 }, (_, i) => game("g" + i));

  it("keeps every row, in order, across the blocks", () => {
    const chunks = chunkOwnedGames(rows, 3);
    expect(chunks.map((c) => c.rows.length)).toEqual([3, 3, 1]);
    expect(chunks.flatMap((c) => c.rows.map((r) => r.name))).toEqual(rows.map((r) => r.name));
  });

  // The block index is the keyed-each key, so it has to count from zero regardless of size.
  it("indexes blocks from zero", () => {
    expect(chunkOwnedGames(rows, 3).map((c) => c.index)).toEqual([0, 1, 2]);
  });

  it("has nothing to render for an empty library", () => {
    expect(chunkOwnedGames([], 50)).toEqual([]);
  });

  // A zero or negative size would otherwise loop forever building empty blocks.
  it("falls back to one block when the size is not positive", () => {
    expect(chunkOwnedGames(rows, 0)).toEqual([{ index: 0, rows }]);
    expect(chunkOwnedGames(rows, -5)).toEqual([{ index: 0, rows }]);
  });
});
