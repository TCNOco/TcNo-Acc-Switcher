import { writable } from "svelte/store";

/**
 * A modal body's current way out, surfaced as the back button in the modal
 * headerbar. Bodies publish it rather than the shell guessing, because only the
 * body knows whether the current screen goes back, closes, or returns to a list.
 */
export interface ModalBackAction {
	/** Tooltip and accessible name. Mirrors the label the screen's own control uses. */
	label: string;
	run: () => void;
}

export const modalBackAction = writable<ModalBackAction | null>(null);

export function setModalBackAction(action: ModalBackAction | null): void {
	modalBackAction.set(action);
}

/** Bodies must call this when they unmount, or a stale action outlives its screen. */
export function clearModalBackAction(): void {
	modalBackAction.set(null);
}
