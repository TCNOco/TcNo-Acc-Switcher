/**
 * How long after the field appears the caret is still claimed for it, and when
 * inside that window it is tried. The screen it belongs to is opened by a click
 * elsewhere - a context menu item, a button - and several things settle focus
 * over the frames that follow: the modal's focus trap choosing a first element,
 * the menu handing focus back to the row it was opened from, the screen's own
 * transition. All of them are done well inside this window, and none of them is
 * the user.
 */
const ATTEMPT_DELAYS_MS = [40, 100, 200, 350];
const WINDOW_MS = 500;

/**
 * Puts the caret in a field as the screen it belongs to appears, and keeps it
 * there for the moment it takes everything else to settle.
 *
 * Focusing once, from the screen transition, means guessing when the field will
 * exist, when it will stop being disabled, and whether anything will move focus
 * afterwards - and a guess that loses leaves the user typing into nothing. This
 * waits for the field, checks that the caret actually arrived, and tries again
 * for a few hundred milliseconds if it did not.
 *
 * It stops the instant the user does anything - a key, a pointer - so it can
 * never take the caret off something they chose themselves. That is the whole
 * safety rule: no window, no heuristic about which element is "allowed" to hold
 * focus, just "if they have not touched anything yet, this field is still what
 * they are about to type into".
 *
 * The parameter is whether the field can take focus yet; pass `!busy` for one
 * that is disabled while work is in flight.
 */
export function focusOnShow(node: HTMLElement, enabled = true) {
  const doc = node.ownerDocument;
  let armed = enabled;
  let running = false;
  let stopped = false;
  let frame = 0;
  const timers: ReturnType<typeof setTimeout>[] = [];

  function stop(): void {
    stopped = true;
    running = false;
    cancelAnimationFrame(frame);
    for (const timer of timers) clearTimeout(timer);
    timers.length = 0;
    doc.removeEventListener("pointerdown", stop, true);
    doc.removeEventListener("keydown", stop, true);
  }

  function attempt(): void {
    if (stopped || !armed) return;
    if (!node.isConnected || (node as Partial<HTMLInputElement>).disabled) return;
    if (doc.activeElement === node) return;
    node.focus({ preventScroll: true });
    // Duck-typed rather than instanceof: the field is normally empty, but after a
    // rejected attempt it still holds what was typed, and selecting it means the
    // next keystroke replaces it instead of appending.
    const selectable = node as Partial<HTMLInputElement>;
    if (typeof selectable.select === "function") selectable.select();
  }

  function start(): void {
    if (stopped || running || !armed) return;
    running = true;
    doc.addEventListener("pointerdown", stop, true);
    doc.addEventListener("keydown", stop, true);
    frame = requestAnimationFrame(attempt);
    for (const delay of ATTEMPT_DELAYS_MS) timers.push(setTimeout(attempt, delay));
    timers.push(setTimeout(stop, WINDOW_MS));
  }

  start();

  return {
    update(next: boolean) {
      armed = next;
      if (armed) start();
    },
    destroy() {
      stop();
    },
  };
}
