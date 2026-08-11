import { writable } from "svelte/store";

/**
 * Paints the window's own frame as a warning.
 *
 * It goes through a store because the frame is not the page's to draw: the
 * border bars belong to the application shell, above every route. The Steam
 * Guard session browser is the one thing that raises it, when its content view
 * is showing a site outside the trusted list - and that is also why it has to be
 * the frame. The page area there is covered by a native view this window cannot
 * draw over, so the strip around it is the only surface left to warn on.
 */
export const pageFrameAlert = writable(false);
