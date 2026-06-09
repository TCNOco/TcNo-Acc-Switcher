import type { ComponentType, SvelteComponent } from "svelte";
import type { Route } from "../stores/routeCodec";

type PageModule = { default: ComponentType<SvelteComponent> };

function cacheKey(route: Route): string {
  if ("platformName" in route && route.platformName) {
    if (route.page === "platform" && route.platformName === "Steam") {
      return "platform:Steam";
    }
    return `${route.page}:${route.platformName}`;
  }
  return route.page;
}

function loaderFor(route: Route): () => Promise<PageModule> {
  switch (route.page) {
    case "home":
      return () => import("../pages/Home.svelte");
    case "settings":
      return () => import("../pages/Settings.svelte");
    case "preview-css":
      return () => import("../pages/PreviewCss.svelte");
    case "platform":
      return route.platformName === "Steam"
        ? () => import("../pages/PlatformSteam.svelte")
        : () => import("../pages/Platform.svelte");
    case "platform-settings":
      return () => import("../pages/PlatformSettings.svelte");
    case "steam-advanced-clearing":
      return () => import("../pages/SteamAdvancedClearing.svelte");
    case "manage-platforms":
      return () => import("../pages/ManagePlatforms.svelte");
  }
}

const pageCache = new Map<string, Promise<PageModule>>();

/** Load a page module, deduplicating concurrent requests. */
export function loadPageModule(route: Route): Promise<PageModule> {
  const key = cacheKey(route);
  let cached = pageCache.get(key);
  if (!cached) {
    cached = loaderFor(route)();
    pageCache.set(key, cached);
  }
  return cached;
}

/** Warm the cache for a route without navigating. */
export function prefetchPage(route: Route): void {
  void loadPageModule(route);
}

/** Prefetch routes users open often from the home screen. */
export function prefetchCommonPages(): void {
  prefetchPage({ page: "settings" });
  prefetchPage({ page: "manage-platforms" });
  void import("../components/GeneralSettingsBlock.svelte");
}

/** Warm platform account pages after the home screen has rendered. */
export function prefetchPlatformPages(): void {
  prefetchPage({ page: "platform", platformName: "Steam" });
  void import("../pages/PlatformSteam.svelte");
  void import("../pages/Platform.svelte");
}
