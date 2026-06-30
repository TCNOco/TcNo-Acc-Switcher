import { Browser } from "@wailsio/runtime";
import { get } from "svelte/store";
import { formatToastWithError } from "./formatWailsError";
import { t } from "../stores/i18n";
import { offlineMode } from "../stores/offlineMode";
import { pushToast } from "../stores/toast";

const defaultAllowedHosts = new Set([
  "crowdin.com",
  "github.com",
  "ko-fi.com",
  "patreon.com",
  "tcno.co",
]);

export interface OpenExternalUrlOptions {
  allowAnyHttps?: boolean;
  allowedHosts?: Iterable<string>;
  respectOfflineMode?: boolean;
}

function hostMatches(host: string, allowedHost: string): boolean {
  const h = host.toLowerCase();
  const allowed = allowedHost.toLowerCase();
  return h === allowed || h.endsWith(`.${allowed}`);
}

function parseAllowedUrl(rawUrl: string, opts: OpenExternalUrlOptions): URL | null {
  let parsed: URL;
  try {
    parsed = new URL(rawUrl);
  } catch {
    return null;
  }
  if (parsed.protocol !== "https:") {
    return null;
  }
  if (opts.allowAnyHttps) {
    return parsed;
  }
  const allowedHosts = opts.allowedHosts ? Array.from(opts.allowedHosts) : Array.from(defaultAllowedHosts);
  return allowedHosts.some((host) => hostMatches(parsed.hostname, host)) ? parsed : null;
}

export async function openExternalUrl(rawUrl: string, opts: OpenExternalUrlOptions = {}): Promise<boolean> {
  if ((opts.respectOfflineMode ?? true) && get(offlineMode)) {
    pushToast({
      type: "info",
      message: get(t)("Toast_OfflineModeNoLinks"),
      duration: 5000,
    });
    return false;
  }

  const parsed = parseAllowedUrl(rawUrl.trim(), opts);
  if (!parsed) {
    pushToast({
      type: "error",
      message: get(t)("Toast_LaunchFailed"),
      duration: 5000,
    });
    return false;
  }

  try {
    await Browser.OpenURL(parsed.toString());
    return true;
  } catch (e) {
    pushToast({
      type: "error",
      message: formatToastWithError(get(t)("Toast_LaunchFailed"), e),
      duration: 8000,
    });
    return false;
  }
}
