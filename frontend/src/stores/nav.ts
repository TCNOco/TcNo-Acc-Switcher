import { get, writable } from "svelte/store";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
import { setPlatformAccountCounts, setPlatformTagCounts } from "./platformAccountsCache";
import {
  applySinglePlatformStartupRoute,
  type Route,
  serializeRoute,
  parseHash,
  validateRoute,
} from "./routeCodec";
import type { PlatformStartup } from "../../bindings/TcNo-Acc-Switcher/internal/platform/models.js";
import { homeScreenData } from "./homeScreenData";

export type { Route };

const _route = writable<Route>({ page: "home" });

/**
 * Writes that do not change the route are dropped.
 *
 * Pages set their own route on mount, and App re-evaluates
 * `loadPageModule($route)` inside an `{#await}` whenever the store changes. A
 * fresh object for the same route therefore remounted the page, whose `onMount`
 * set the route again — an unbounded remount loop under Svelte 5. Comparing by
 * serialised form keeps identity stable whenever the destination is unchanged,
 * which every consumer of this store relies on.
 */
export const route = {
  subscribe: _route.subscribe,
  set: (v: Route): void => {
    if (serializeRoute(get(_route)) !== serializeRoute(v)) {
      _route.set(v);
    }
  },
  update: (fn: (v: Route) => Route): void => {
    _route.update((cur) => {
      const next = fn(cur);
      return serializeRoute(cur) === serializeRoute(next) ? cur : next;
    });
  },
};
const _previousPage = writable<Route | null>(null);

/** Same no-op guard as `route`: pages set this from reactive blocks too. */
export const previousPage = {
  subscribe: _previousPage.subscribe,
  set: (v: Route | null): void => {
    const cur = get(_previousPage);
    const same =
      cur === v ||
      (cur !== null && v !== null && serializeRoute(cur) === serializeRoute(v));
    if (!same) {
      _previousPage.set(v);
    }
  },
};
export const appBarTitle = writable("TcNo Account Switcher");
let historyIndex = 0;
let historyMaxIndex = 0;


function applyCliHint(startup: PlatformStartup, base: Route): Route {
  const hint = startup.cliNavigateHint?.trim();
  if (!hint) return base;
  try {
    const parsed = JSON.parse(hint) as Route;
    if (parsed && typeof parsed === "object" && "page" in parsed) return validateRoute(parsed, startup);
  } catch {}
  return base;
}

function syncRouteUrl(next: Route): void {
  const url = serializeRoute(next);
  if (window.location.hash !== url || window.history.state?.idx !== historyIndex) {
    history.replaceState({ idx: historyIndex }, "", url);
  }
}

/** Call after i18n init; sets route from hash + startup validation and optional CLI hint. */
export async function resolveInitialRoute(): Promise<void> {
  let fromHash = parseHash(window.location.hash || "#/") || { page: "home" };
  try {
    const startup = await PlatformService.GetStartup();
    homeScreenData.set(startup);
    setPlatformAccountCounts(startup.platformAccountCounts ?? {});
    setPlatformTagCounts(startup.platformTagCounts);
    let next = validateRoute(fromHash, startup);
    next = applySinglePlatformStartupRoute(next, startup);
    next = applyCliHint(startup, next);
    route.set(next);
    syncRouteUrl(next);
  } catch {
    route.set(fromHash);
  }
}

let syncing = false;

/** Replace current history entry + route (does not push — avoids orphan stack entries on logical back). */
function replaceCurrentHistoryRoute(next: Route): void {
  syncing = true;
  try {
    const url = serializeRoute(next);
    const st = window.history.state as { idx?: unknown } | null;
    const idx =
      typeof st?.idx === "number" && Number.isFinite(st.idx) ? st.idx : historyIndex;
    window.history.replaceState({ idx }, "", url);
    route.set(next);
  } finally {
    syncing = false;
  }
}

/** Sync store → location.hash without adding history entries. */
export function installHashSync(): () => void {
  const unsub = route.subscribe((r) => {
    if (syncing) {
      return;
    }
    const url = serializeRoute(r);
    if (window.location.hash !== url) {
      syncing = true;
      if (historyIndex < historyMaxIndex) {
        // Truncate the virtual forward stack after diverging navigation.
        historyMaxIndex = historyIndex;
      }
      historyIndex += 1;
      historyMaxIndex = historyIndex;
      history.pushState({ idx: historyIndex }, "", url);
      syncing = false;
    }
  });

  const onPop = (ev: PopStateEvent): void => {
    const next = parseHash(window.location.hash || "#/");
    if (next) {
      const idx = ev.state?.idx;
      if (typeof idx === "number" && Number.isFinite(idx)) {
        historyIndex = idx;
      }
      syncing = true;
      route.set(next);
      syncing = false;
    }
  };
  window.addEventListener("popstate", onPop);

  return () => {
    unsub();
    window.removeEventListener("popstate", onPop);
  };
}

/** Apply a route from external JSON (same shape as Route). */
export function applyNavigateJSON(json: string): void {
  const s = json.trim();
  if (!s) {
    return;
  }
  try {
    const obj = JSON.parse(s) as Route;
    if (!obj || typeof obj !== "object" || !("page" in obj)) {
      return;
    }
    void PlatformService.GetStartup()
      .then((startup) => {
        homeScreenData.set(startup);
        const v = validateRoute(obj, startup);
        route.set(v);
      })
      .catch(() => {
        route.set(obj);
      });
  } catch {
    /* ignore */
  }
}

function canNavigateBack(): boolean {
  return historyIndex > 0;
}

function canNavigateForward(): boolean {
  return historyIndex < historyMaxIndex;
}

function navigateBack(): boolean {
  if (!canNavigateBack()) {
    return false;
  }
  history.back();
  return true;
}

export function navigateForward(): boolean {
  if (!canNavigateForward()) {
    return false;
  }
  history.forward();
  return true;
}

export function navigateBackLikeButton(): void {
  const r = get(route);
  if (r.page === "home") {
    return;
  }
  if (historyIndex > 0) {
    history.back();
    return;
  }
  if (typeof window !== "undefined" && window.history.length > 1) {
    history.back();
    return;
  }
  const prev = get(previousPage);
  replaceCurrentHistoryRoute(prev ?? { page: "home" });
}
