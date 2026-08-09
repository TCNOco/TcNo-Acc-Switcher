import { writable } from "svelte/store";

export type SteamPageTab = "switcher" | "games";

/** Which half of the Steam platform page is showing. Read by the title bar tab and the page. */
export const steamPageTab = writable<SteamPageTab>("switcher");

export function toggleSteamPageTab(): void {
  steamPageTab.update((tab) => (tab === "games" ? "switcher" : "games"));
}

export function resetSteamPageTab(): void {
  steamPageTab.set("switcher");
}
