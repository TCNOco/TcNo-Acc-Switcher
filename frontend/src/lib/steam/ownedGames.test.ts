import { describe, expect, it } from "vitest";
import { filterOwnedGames, sortOwnedGames, splitGameOwners, type OwnedGameRow } from "./ownedGames";

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
