import { derived, get, type Readable } from "svelte/store";
import { homeScreenData } from "./homeScreenData";
import type { OSCapabilities } from "../../bindings/TcNo-Acc-Switcher/internal/platform/models.js";

/**
 * What this build can actually do, as reported by the backend.
 *
 * Replaces sniffing navigator.userAgent for "windows", which could only answer
 * "is this Windows" — and the distinction that matters more often is Linux
 * versus macOS, since process control exists on one and not the other.
 *
 * The values ride along on the startup payload the app already awaits before
 * rendering, so there is no separate request and no race. Wails also injects
 * `window._wails.environment.OS`, but it does so on the page-load hook, after
 * module scripts run — reading it at component init can see `undefined`, which
 * for a Windows-only gate would hide the control on Windows.
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
 * Capabilities of the running build.
 *
 * Before startup data lands every capability reads false, so a control is
 * briefly hidden rather than briefly offered and then withdrawn — and a failure
 * to load startup data hides Windows-only controls instead of showing ones that
 * would throw when used.
 */
export const capabilities: Readable<OSCapabilities> = derived(
  homeScreenData,
  ($startup) => $startup?.capabilities ?? NONE,
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
