import type { PlatformStartup } from "../../bindings/TcNo-Acc-Switcher/internal/platform/models.js";

export type Route =
  | { page: "home" }
  | { page: "settings" }
  | { page: "preview-css" }
  | { page: "manage-platforms" }
  | { page: "platform"; platformName: string }
  | { page: "platform-settings"; platformName: string }
  | { page: "steam-advanced-clearing" };

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

type RouteParser = (parts: string[]) => Route | null;

const ROUTE_PARSERS: Record<string, RouteParser> = {
  "":                  () => ({ page: "home" }),
  home:                () => ({ page: "home" }),
  settings:            () => ({ page: "settings" }),
  "preview-css":       () => ({ page: "preview-css" }),
  test:                () => ({ page: "preview-css" }),
  "manage-platforms":   () => ({ page: "manage-platforms" }),
  platform:             (p) => p[1] ? { page: "platform", platformName: decodeURIComponent(p[1]) } : null,
  "platform-settings":  (p) => p[1] ? { page: "platform-settings", platformName: decodeURIComponent(p[1]) } : null,
  steam:                (p) => p[1]?.toLowerCase() === "advanced-clearing" ? { page: "steam-advanced-clearing" } : null,
};

export function parseHash(hash: string): Route | null {
  let h = (hash.startsWith("#") ? hash.slice(1) : hash).trim();
  if (!h || h === "/") return { page: "home" };

  const parts = h.split("/").filter((p) => p.length > 0);
  const head = decodeURIComponent(parts[0] || "").toLowerCase();

  const parser = ROUTE_PARSERS[head];
  return parser ? parser(parts) : null;
}

export function validateRoute(r: Route, startup: PlatformStartup): Route {
  if (startup.platformsFileMissing) return { page: "home" };

  const disabled = new Set(startup.disabledPlatformNames || []);

  const nameOk = (name: string) => {
    const n = name.trim();
    if (!n) return false;
    if (!(startup.allPlatformNames || []).includes(n)) return false;
    if (disabled.has(n)) return false;
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
