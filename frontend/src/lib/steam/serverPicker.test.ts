import { describe, expect, it } from "vitest";
import {
  DEFAULT_SORT,
  ariaSort,
  availableRegions,
  bulkAction,
  filterGroups,
  groupPing,
  groupSearchText,
  isGroupPending,
  isPopPending,
  lossQuality,
  nextSort,
  pingQuality,
  shouldCaptureTypedKey,
  sortGroups,
  type PingMap,
} from "./serverPicker";
import type { ServerGroupDTO } from "../../../bindings/TcNo-Acc-Switcher/internal/serverpicker/models";

function group(over: Partial<ServerGroupDTO> = {}): ServerGroupDTO {
  return {
    id: "sto",
    label: "Stockholm (Sweden)",
    country: "se",
    countryName: "Sweden",
    region: "europe",
    blocked: false,
    members: [
      { id: "sto", desc: "Stockholm - Kista (Sweden)", relays: 2 },
      { id: "sto2", desc: "Stockholm - Bromma (Sweden)", relays: 1 },
    ],
    ...over,
  } as ServerGroupDTO;
}

const regionLabels = {
  asia: "Asia",
  europe: "Europe",
  northAmerica: "North America",
  southAmerica: "South America",
  oceania: "Oceania",
  africa: "Africa",
  middleEast: "Middle East",
};

const hongKong = group({
  id: "hkg",
  label: "Hong Kong",
  country: "hk",
  countryName: "Hong Kong",
  region: "asia",
  members: [{ id: "hkg", desc: "Hong Kong", relays: 10 }],
});

describe("pingQuality", () => {
  it("buckets at the thresholds the reference picker uses", () => {
    expect(pingQuality(0)).toBe("good");
    expect(pingQuality(75)).toBe("good");
    expect(pingQuality(76)).toBe("fair");
    expect(pingQuality(150)).toBe("fair");
    expect(pingQuality(151)).toBe("poor");
  });

  it("treats a missing measurement as unknown rather than as a fast server", () => {
    expect(pingQuality(null)).toBe("unknown");
    expect(pingQuality(undefined)).toBe("unknown");
    expect(pingQuality(-1)).toBe("unknown");
  });
});

describe("lossQuality", () => {
  it("buckets loss separately from ping", () => {
    expect(lossQuality(0)).toBe("good");
    expect(lossQuality(4.9)).toBe("good");
    expect(lossQuality(5)).toBe("fair");
    expect(lossQuality(20)).toBe("fair");
    expect(lossQuality(25)).toBe("poor");
    expect(lossQuality(null)).toBe("unknown");
  });
});

describe("groupPing", () => {
  it("reports the best member, because that is the route Steam would take", () => {
    const pings: PingMap = {
      sto: { reachable: true, rttMs: 120, loss: 25 },
      sto2: { reachable: true, rttMs: 40, loss: 0 },
    };
    expect(groupPing(group(), pings)).toEqual({ rttMs: 40, loss: 0 });
  });

  it("ignores members that did not answer", () => {
    const pings: PingMap = {
      sto: { reachable: false, rttMs: -1, loss: -1 },
      sto2: { reachable: true, rttMs: 90, loss: 50 },
    };
    expect(groupPing(group(), pings)).toEqual({ rttMs: 90, loss: 50 });
  });

  it("is null until something answers", () => {
    expect(groupPing(group(), {})).toBeNull();
  });
});

describe("groupSearchText", () => {
  it("includes the member POP ids so a row is reachable by any of them", () => {
    expect(groupSearchText(group(), "Europe")).toContain("sto2");
  });
});

describe("filterGroups", () => {
  const groups = [group(), hongKong];

  it("matches a region name typed into the search box", () => {
    const got = filterGroups(groups, { region: "", query: "asia", regionLabels });
    expect(got.map((g) => g.id)).toEqual(["hkg"]);
  });

  it("matches a country name that appears nowhere in the label", () => {
    const got = filterGroups(groups, { region: "", query: "sweden", regionLabels });
    expect(got.map((g) => g.id)).toEqual(["sto"]);
  });

  it("matches a POP id hidden inside a grouped row", () => {
    const got = filterGroups(groups, { region: "", query: "sto2", regionLabels });
    expect(got.map((g) => g.id)).toEqual(["sto"]);
  });

  it("folds case and diacritics, so a mistyped umlaut still finds the row", () => {
    expect(filterGroups(groups, { region: "", query: "STOCKHOLM", regionLabels })).toHaveLength(1);
    expect(filterGroups(groups, { region: "", query: "STOCKHÖLM", regionLabels })).toHaveLength(1);
  });

  it("combines the region dropdown with the search box", () => {
    expect(filterGroups(groups, { region: "asia", query: "stockholm", regionLabels })).toEqual([]);
    expect(filterGroups(groups, { region: "europe", query: "stockholm", regionLabels })).toHaveLength(1);
  });
});

describe("availableRegions", () => {
  it("lists only regions with servers, in canonical order", () => {
    expect(availableRegions([hongKong, group()])).toEqual(["asia", "europe"]);
  });
});

describe("bulkAction", () => {
  it("offers to block when everything visible is allowed", () => {
    expect(bulkAction([group(), hongKong])).toBe("disable");
  });

  it("offers to allow as soon as anything visible is blocked", () => {
    expect(bulkAction([group({ blocked: true }), hongKong])).toBe("enable");
    expect(bulkAction([group({ blocked: true }), group({ id: "hkg", blocked: true })])).toBe("enable");
  });
});

describe("nextSort", () => {
  it("starts a new column ascending, so ping sorts fastest-first on the first click", () => {
    expect(nextSort({ key: "server", dir: "desc" }, "ping")).toEqual({ key: "ping", dir: "asc" });
  });

  it("flips the direction when the active column is clicked again", () => {
    expect(nextSort({ key: "ping", dir: "asc" }, "ping")).toEqual({ key: "ping", dir: "desc" });
    expect(nextSort({ key: "ping", dir: "desc" }, "ping")).toEqual({ key: "ping", dir: "asc" });
  });
});

describe("ariaSort", () => {
  it("marks only the active column, with its direction", () => {
    expect(ariaSort({ key: "ping", dir: "asc" }, "ping")).toBe("ascending");
    expect(ariaSort({ key: "ping", dir: "desc" }, "ping")).toBe("descending");
    expect(ariaSort({ key: "ping", dir: "asc" }, "server")).toBe("none");
  });
});

describe("sortGroups", () => {
  const fast = group({ id: "ams", label: "Amsterdam (Netherlands)", members: [{ id: "ams", desc: "Amsterdam", relays: 4 }] });
  const slow = group({ id: "syd", label: "Sydney (Australia)", members: [{ id: "syd", desc: "Sydney", relays: 4 }] });
  const dead = group({ id: "gru", label: "Sao Paulo (Brazil)", members: [{ id: "gru", desc: "Sao Paulo", relays: 4 }] });
  const rows = [slow, dead, fast];
  const pings: PingMap = {
    ams: { reachable: true, rttMs: 20, loss: 0 },
    syd: { reachable: true, rttMs: 300, loss: 50 },
  };

  it("sorts by name in both directions", () => {
    expect(sortGroups(rows, pings, DEFAULT_SORT).map((g) => g.id)).toEqual(["ams", "gru", "syd"]);
    expect(sortGroups(rows, pings, { key: "server", dir: "desc" }).map((g) => g.id)).toEqual(["syd", "gru", "ams"]);
  });

  it("sorts by ping fastest-first", () => {
    expect(sortGroups(rows, pings, { key: "ping", dir: "asc" }).map((g) => g.id)).toEqual(["ams", "syd", "gru"]);
  });

  it("keeps unmeasured servers last even when sorting descending", () => {
    expect(sortGroups(rows, pings, { key: "ping", dir: "desc" }).map((g) => g.id)).toEqual(["syd", "ams", "gru"]);
    expect(sortGroups(rows, pings, { key: "loss", dir: "desc" }).map((g) => g.id)).toEqual(["syd", "ams", "gru"]);
  });

  it("does not mutate the input", () => {
    const original = [...rows];
    sortGroups(rows, pings, { key: "ping", dir: "asc" });
    expect(rows).toEqual(original);
  });
});

describe("pending state", () => {
  it("is false when no sweep is running, so idle rows show a dash not a spinner", () => {
    expect(isPopPending("sto", {}, false)).toBe(false);
    expect(isGroupPending(group(), {}, false)).toBe(false);
  });

  it("holds a grouped row at loading until every member has reported", () => {
    const partial: PingMap = { sto: { reachable: true, rttMs: 20, loss: 0 } };
    expect(isGroupPending(group(), partial, true)).toBe(true);
    expect(isGroupPending(group(), { ...partial, sto2: { reachable: false, rttMs: -1, loss: -1 } }, true)).toBe(false);
  });

  it("clears for a POP that reported unreachable, which is an answer", () => {
    expect(isPopPending("sto", { sto: { reachable: false, rttMs: -1, loss: -1 } }, true)).toBe(false);
  });
});

describe("shouldCaptureTypedKey", () => {
  const key = (over: Partial<Parameters<typeof shouldCaptureTypedKey>[0]>) => ({
    key: "a",
    ctrlKey: false,
    altKey: false,
    metaKey: false,
    ...over,
  });

  it("captures plain characters so typing anywhere starts a search", () => {
    expect(shouldCaptureTypedKey(key({}))).toBe(true);
  });

  it("lets shortcuts and navigation keys through", () => {
    expect(shouldCaptureTypedKey(key({ ctrlKey: true }))).toBe(false);
    expect(shouldCaptureTypedKey(key({ altKey: true }))).toBe(false);
    expect(shouldCaptureTypedKey(key({ metaKey: true }))).toBe(false);
    expect(shouldCaptureTypedKey(key({ key: "ArrowDown" }))).toBe(false);
  });

  it("ignores space, which filters nothing and would steal button activation", () => {
    expect(shouldCaptureTypedKey(key({ key: " " }))).toBe(false);
  });
});
