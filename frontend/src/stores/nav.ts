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

export const route = writable<Route>({ page: "home" });
export const previousPage = writable<Route | null>(null);
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
