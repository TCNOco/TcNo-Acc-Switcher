import { writable } from "svelte/store";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
import type { PlatformStartup } from "../../bindings/TcNo-Acc-Switcher/internal/platform/models.js";

export type Route =
  | { page: "home" }
  | { page: "settings" }
  | { page: "preview-css" }
  | { page: "manage-platforms" }
  | { page: "platform"; platformName: string }
  | { page: "platform-settings"; platformName: string }
  | { page: "steam-advanced-clearing" };

export const route = writable<Route>({ page: "home" });
export const previousPage = writable<Route | null>(null);
export const appBarTitle = writable("TcNo Account Switcher");

export function serializeRoute(r: Route): string {
  switch (r.page) {
    case "home":
      return "#/";
    case "settings":
      return "#/settings";
    case "preview-css":
      return "#/preview-css";
    case "manage-platforms":
      return "#/manage-platforms";
    case "platform":
      return "#/platform/" + encodeURIComponent(r.platformName);
    case "platform-settings":
      return "#/platform-settings/" + encodeURIComponent(r.platformName);
    case "steam-advanced-clearing":
      return "#/steam/advanced-clearing";
    default:
      return "#/";
  }
}

export function parseHash(hash: string): Route | null {
  let h = (hash.startsWith("#") ? hash.slice(1) : hash).trim();
  if (!h || h === "/") {
    return { page: "home" };
  }
  const parts = h.split("/").filter((p) => p.length > 0);
  const head = decodeURIComponent(parts[0] || "").toLowerCase();

  if (head === "" || head === "home") {
    return { page: "home" };
  }
  if (head === "settings") {
    return { page: "settings" };
  }
  if (head === "preview-css" || head === "test") {
    return { page: "preview-css" };
  }
  if (head === "manage-platforms") {
    return { page: "manage-platforms" };
  }
  if (head === "platform" && parts[1]) {
    return { page: "platform", platformName: decodeURIComponent(parts[1]) };
  }
  if (head === "platform-settings" && parts[1]) {
    return { page: "platform-settings", platformName: decodeURIComponent(parts[1]) };
  }
  if (head === "steam" && parts[1]?.toLowerCase() === "advanced-clearing") {
    return { page: "steam-advanced-clearing" };
  }
  return null;
}

function validateRoute(r: Route, startup: PlatformStartup): Route {
  if (startup.platformsFileMissing) {
    return { page: "home" };
  }
  const disabled = new Set(startup.disabledPlatformNames || []);

  const nameOk = (name: string) => {
    const n = name.trim();
    if (!n) {
      return false;
    }
    if (!(startup.allPlatformNames || []).includes(n)) {
      return false;
    }
    if (disabled.has(n)) {
      return false;
    }
    return true;
  };

  switch (r.page) {
    case "platform":
    case "platform-settings":
      return nameOk(r.platformName) ? r : { page: "home" };
    case "steam-advanced-clearing":
      return nameOk("Steam") ? r : { page: "home" };
    default:
      return r;
  }
}

/** Call after i18n init; sets route from hash + startup validation and optional CLI hint. */
export async function resolveInitialRoute(): Promise<void> {
  let fromHash = parseHash(window.location.hash || "#/") || { page: "home" };
  try {
    const startup = await PlatformService.GetStartup();
    let next = validateRoute(fromHash, startup);

    const hint = startup.cliNavigateHint?.trim();
    if (hint) {
      try {
        const parsed = JSON.parse(hint) as Route;
        if (parsed && typeof parsed === "object" && "page" in parsed) {
          next = validateRoute(parsed, startup);
        }
      } catch {
        /* ignore */
      }
    }

    route.set(next);
    const url = serializeRoute(next);
    if (window.location.hash !== url) {
      history.replaceState(null, "", url);
    }
  } catch {
    route.set(fromHash);
  }
}

let syncing = false;

/** Sync store → location.hash without adding history entries. */
export function installHashSync(): () => void {
  const unsub = route.subscribe((r) => {
    if (syncing) {
      return;
    }
    const url = serializeRoute(r);
    if (window.location.hash !== url) {
      syncing = true;
      history.replaceState(null, "", url);
      syncing = false;
    }
  });

  const onPop = (): void => {
    const next = parseHash(window.location.hash || "#/");
    if (next) {
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
        const v = validateRoute(obj, startup);
        route.set(v);
        history.replaceState(null, "", serializeRoute(v));
      })
      .catch(() => {
        route.set(obj);
      });
  } catch {
    /* ignore */
  }
}
