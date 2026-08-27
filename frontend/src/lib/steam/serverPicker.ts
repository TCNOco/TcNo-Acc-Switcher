import { matchesQuery, queryTokens } from "../settingsFilter";
import type { ServerGroupDTO } from "../../../bindings/TcNo-Acc-Switcher/internal/serverpicker/models";

/** Region ids the Go side derives from each POP's coordinates. */
export const REGIONS = [
  "asia",
  "europe",
  "northAmerica",
  "southAmerica",
  "oceania",
  "africa",
  "middleEast",
] as const;

export type Region = (typeof REGIONS)[number];

export const REGION_LABEL_KEYS: Record<string, string> = {
  asia: "ServerPicker_Region_Asia",
  europe: "ServerPicker_Region_Europe",
  northAmerica: "ServerPicker_Region_NorthAmerica",
  southAmerica: "ServerPicker_Region_SouthAmerica",
  oceania: "ServerPicker_Region_Oceania",
  africa: "ServerPicker_Region_Africa",
  middleEast: "ServerPicker_Region_MiddleEast",
};

/** One POP's live measurement, keyed by POP id. */
export type PingMap = Record<string, { reachable: boolean; rttMs: number; loss: number }>;

export type Quality = "good" | "fair" | "poor" | "unknown";

/**
 * Thresholds match the reference server picker so a user moving between the two
 * reads the same colours.
 */
export function pingQuality(rttMs: number | null | undefined): Quality {
  if (rttMs === null || rttMs === undefined || !Number.isFinite(rttMs) || rttMs < 0) {
    return "unknown";
  }
  if (rttMs <= 75) return "good";
  if (rttMs <= 150) return "fair";
  return "poor";
}

export function lossQuality(loss: number | null | undefined): Quality {
  if (loss === null || loss === undefined || !Number.isFinite(loss) || loss < 0) {
    return "unknown";
  }
  if (loss < 5) return "good";
  if (loss <= 20) return "fair";
  return "poor";
}

/**
 * A group's ping is its best member's: Steam will route you through whichever
 * POP in the city answers fastest, so the best one is what you would actually
 * get if the group stays enabled.
 */
export function groupPing(group: ServerGroupDTO, pings: PingMap): { rttMs: number; loss: number } | null {
  let best: { rttMs: number; loss: number } | null = null;
  for (const member of group.members ?? []) {
    const p = pings[member.id];
    if (!p || !p.reachable) continue;
    if (!best || p.rttMs < best.rttMs) {
      best = { rttMs: p.rttMs, loss: p.loss };
    }
  }
  return best;
}

/**
 * Search text for one row. Region and country are folded in so typing "asia"
 * surfaces Hong Kong and Tokyo, and "sweden" finds Stockholm — the POP ids and
 * Valve's descriptions alone would not.
 */
export function groupSearchText(group: ServerGroupDTO, regionLabel: string): string {
  const memberIds = (group.members ?? []).map((m) => m.id).join(" ");
  const memberNames = (group.members ?? []).map((m) => m.desc).join(" ");
  return [group.label, group.id, memberIds, memberNames, group.countryName, regionLabel, group.region]
    .filter(Boolean)
    .join(" ");
}

export type SortKey = "server" | "id" | "ping" | "loss";
export type SortDir = "asc" | "desc";
export type Sort = { key: SortKey; dir: SortDir };

export const DEFAULT_SORT: Sort = { key: "server", dir: "asc" };

/**
 * Clicking the active column flips it; clicking a new one starts ascending —
 * which for ping and loss means best-first, the order people actually want.
 */
export function nextSort(current: Sort, clicked: SortKey): Sort {
  if (current.key !== clicked) return { key: clicked, dir: "asc" };
  return { key: clicked, dir: current.dir === "asc" ? "desc" : "asc" };
}

/**
 * aria-sort for one column header. Takes the whole sort so callers pass it as
 * an argument — a Svelte template only re-evaluates an expression when a value
 * it names changes, and reading `sort` inside a helper would not count.
 */
export function ariaSort(sort: Sort, key: SortKey): "ascending" | "descending" | "none" {
  if (sort.key !== key) return "none";
  return sort.dir === "asc" ? "ascending" : "descending";
}

/** True while a sweep is running and this POP has not reported yet. */
export function isPopPending(popId: string, pings: PingMap, sweeping: boolean): boolean {
  return sweeping && !pings[popId];
}

/** A grouped row is still measuring until every member has reported. */
export function isGroupPending(group: ServerGroupDTO, pings: PingMap, sweeping: boolean): boolean {
  if (!sweeping) return false;
  return (group.members ?? []).some((m) => !pings[m.id]);
}

/**
 * Sorts a copy of the rows. Unmeasured servers sink to the bottom in both
 * directions: they are not "the worst ping", they are an absence of one, and
 * floating them to the top on a descending sort would bury the real results.
 */
export function sortGroups(groups: ServerGroupDTO[], pings: PingMap, sort: Sort): ServerGroupDTO[] {
  const sign = sort.dir === "asc" ? 1 : -1;

  const numeric = (g: ServerGroupDTO): number | null => {
    const best = groupPing(g, pings);
    if (!best) return null;
    return sort.key === "ping" ? best.rttMs : best.loss;
  };

  return [...groups].sort((a, b) => {
    if (sort.key === "server") {
      return sign * a.label.localeCompare(b.label);
    }
    if (sort.key === "id") {
      return sign * a.id.localeCompare(b.id);
    }
    const av = numeric(a);
    const bv = numeric(b);
    if (av === null && bv === null) return a.label.localeCompare(b.label);
    if (av === null) return 1;
    if (bv === null) return -1;
    if (av === bv) return a.label.localeCompare(b.label);
    return sign * (av - bv);
  });
}

export type FilterOptions = {
  region: string;
  query: string;
  /** Region id -> translated label, so search matches what the user sees. */
  regionLabels: Record<string, string>;
};

export function filterGroups(groups: ServerGroupDTO[], opts: FilterOptions): ServerGroupDTO[] {
  const tokens = queryTokens(opts.query ?? "");
  const region = (opts.region ?? "").trim();
  return groups.filter((g) => {
    if (region && g.region !== region) return false;
    if (tokens.length === 0) return true;
    return matchesQuery(groupSearchText(g, opts.regionLabels[g.region] ?? ""), tokens);
  });
}

/** Regions present in the list, in the canonical order, for the dropdown. */
export function availableRegions(groups: ServerGroupDTO[]): Region[] {
  const present = new Set(groups.map((g) => g.region));
  return REGIONS.filter((r) => present.has(r));
}

/**
 * The bulk button offers whichever action is not already the state of every
 * visible row: all allowed means the only useful action is to block them.
 */
export function bulkAction(visible: ServerGroupDTO[]): "disable" | "enable" {
  if (visible.length === 0) return "disable";
  return visible.every((g) => !g.blocked) ? "disable" : "enable";
}

/**
 * Decides whether a keystroke that landed on the page should be replayed into
 * the search box. Mirrors the settings-page search so the two behave alike.
 */
export function shouldCaptureTypedKey(e: {
  key: string;
  ctrlKey: boolean;
  altKey: boolean;
  metaKey: boolean;
}): boolean {
  if (e.ctrlKey || e.altKey || e.metaKey) return false;
  // Single characters only, so shortcuts and navigation keys pass through.
  // Space is excluded: it filters nothing and would steal button activation.
  return e.key.length === 1 && e.key !== " ";
}
