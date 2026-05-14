import { writable, type Writable } from "svelte/store";

/**
 * When set, this interceptor runs BEFORE fileDropInterceptor in FileDropOverlay.
 * It handles drops that land on the background drop zones.
 * Return true to consume the drop; false to pass it on.
 */
export type BgZoneDropInterceptor = (paths: string[]) => Promise<boolean>;

export const backgroundZoneInterceptor: Writable<BgZoneDropInterceptor | null> = writable(null);
