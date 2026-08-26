import { derived, get, type Readable } from "svelte/store";
import { homeScreenData } from "./homeScreenData";
import type { OSCapabilities } from "../../bindings/TcNo-Acc-Switcher/internal/platform/models.js";

/**
 * What this build can actually do, as reported by the backend.
 *
 * The values ride along on the startup payload the app already awaits before
 * rendering, so there is no separate request and no race. Wails' own
 * `window._wails.environment.OS` is injected on the page-load hook, after module
 * scripts run, so reading it at component init can see `undefined`.
 */

/** Every capability off, used until startup data arrives. */
const NONE: OSCapabilities = {
  shortcuts: false,
  elevation: false,
  processControl: false,
  closingMethods: false,
  registry: false,
  protocolHandler: false,
  broadcastDetection: false,
  screenCaptureExclusion: false,
  controllerInput: false,
  qrCapture: false,
  secureClipboard: false,
  steamBrowser: false,
  serverPicker: false,
  autostart: false,
} as OSCapabilities;

/**
 * Capabilities of the running build. Every capability reads false until startup
 * data lands, so a control is briefly hidden rather than briefly offered and
 * then withdrawn.
 */
export const capabilities: Readable<OSCapabilities> = derived(
  homeScreenData,
  ($startup) => $startup?.capabilities ?? NONE,
);

/**
 * A gamescope session: Steam's Game Mode on a handheld, where the app is the
 * only thing on screen. False until startup data arrives: unlike
 * `capabilities`, false here means shown, so a control is briefly present
 * rather than briefly missing.
 */
export const gameMode: Readable<boolean> = derived(
  homeScreenData,
  ($startup) => $startup?.gameMode ?? false,
);

/** "windows" | "linux" | "darwin", or "" before startup data arrives. */
export const currentOS: Readable<string> = derived(
  homeScreenData,
  ($startup) => $startup?.os ?? "",
);

/** Non-reactive read, for callers outside a component. */
export function getCapabilities(): OSCapabilities {
  return get(capabilities);
}

/** Non-reactive read of the OS. */
export function getCurrentOS(): string {
  return get(currentOS);
}
