export type SubmenuVerticalLayout = {
  naturalTop: number;
  naturalBottom: number;
  viewportHeight: number;
  padding: number;
};

export function submenuTopOffset(layout: SubmenuVerticalLayout): number {
  const bottomLimit = Math.max(layout.padding, layout.viewportHeight - layout.padding);
  const overflow = Math.max(0, layout.naturalBottom - bottomLimit);
  if (overflow === 0) {
    return 0;
  }

  const availableShift = Math.max(0, layout.naturalTop - layout.padding);
  return -Math.ceil(Math.min(overflow, availableShift));
}
