import { writable } from "svelte/store";

/** Footer status line; set from the Steam account list, Go (e.g. switching), or cleared on navigate away. */
export const actionBarStatus = writable("");
