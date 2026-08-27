export type Span = { top: number; bottom: number };

/**
 * How far a container must scroll for `row` to be in sight, or 0 when it
 * already is. Positive scrolls down. The result is the smallest movement that
 * works, so a row just off the edge does not jump to the middle.
 */
export function scrollDeltaIntoView(row: Span, view: Span, margin = 8): number {
  // A row taller than the view can never fit; showing its top is the useful half.
  if (row.top < view.top + margin) {
    return row.top - (view.top + margin);
  }
  if (row.bottom > view.bottom - margin) {
    return row.bottom - (view.bottom - margin);
  }
  return 0;
}
