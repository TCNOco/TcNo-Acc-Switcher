import { get } from "svelte/store";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
import { route, type Route } from "../stores/nav";

/** Stable path key for Statistics.json PageStats (matches server normPageKey). */
export function routeToStatsPath(r: Route): string {
  switch (r.page) {
    case "home":
      return "/";
    case "settings":
      return "/settings";
    case "preview-css":
      return "/preview-css";
    case "manage-platforms":
      return "/manage-platforms";
    case "platform":
      return "/platform/" + r.platformName;
    case "platform-settings":
      return "/platform-settings/" + r.platformName;
    case "steam-advanced-clearing":
      return "/steam/advanced-clearing";
    default:
      return "/";
  }
}

const tickMs = 60_000;

/**
 * Records SPA page visits and dwell time via the platform service (Statistics.json).
 * Call once after the route store is initialised.
 */
export function installPageStatsTracking(): () => void {
  let path = "";
  let started = 0;

  const flushSeconds = (usePath: string, minSeconds: number): void => {
    const elapsed = Math.max(0, Math.floor((Date.now() - started) / 1000));
    if (usePath && elapsed >= minSeconds) {
      void PlatformService.StatsAddPageTime(usePath, elapsed);
      started = Date.now();
    }
  };

  const initPath = routeToStatsPath(get(route));
  path = initPath;
  started = Date.now();
  void PlatformService.StatsRecordPageVisit(path);

  const tick = setInterval(() => {
    flushSeconds(path, 5);
  }, tickMs);

  const onVisibility = (): void => {
    if (document.visibilityState === "hidden") {
      flushSeconds(path, 1);
    }
  };
  const onPageHide = (): void => {
    flushSeconds(path, 1);
  };
  document.addEventListener("visibilitychange", onVisibility);
  window.addEventListener("pagehide", onPageHide);

  const unsub = route.subscribe((r) => {
    const next = routeToStatsPath(r);
    if (next === path) {
      return;
    }
    flushSeconds(path, 1);
    path = next;
    started = Date.now();
    void PlatformService.StatsRecordPageVisit(next);
  });

  return () => {
    clearInterval(tick);
    document.removeEventListener("visibilitychange", onVisibility);
    window.removeEventListener("pagehide", onPageHide);
    flushSeconds(path, 1);
    unsub();
  };
}
