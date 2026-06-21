/** Svelte action that replaces children of `node` with `clone`. Used for drag ghost mirroring. */
export function ghostCloneMount(
  node: HTMLElement,
  clone: HTMLElement | null,
) {
  function apply(c: HTMLElement | null) {
    node.replaceChildren();
    if (c) node.appendChild(c);
  }
  apply(clone);
  return {
    update(next: HTMLElement | null) {
      apply(next);
    },
    destroy() {
      node.replaceChildren();
    },
  };
}
