type ScrollbarWidthVarOptions = {
  enabled?: boolean;
  targetSelector?: string;
  variable?: string;
};

const DEFAULT_VAR = "--shortcut-scrollbar-width";

function measureScrollbarWidth(node: HTMLElement): number {
  if (node.scrollHeight <= node.clientHeight + 1) return 0;
  return Math.max(0, node.offsetWidth - node.clientWidth);
}

export function scrollbarWidthVar(
  node: HTMLElement,
  options: ScrollbarWidthVarOptions = {},
) {
  let opts = options;
  let raf = 0;
  let target: HTMLElement = node;

  function resolveTarget(): HTMLElement {
    const selector = opts.targetSelector;
    if (!selector) return node;
    return (node.closest(selector) as HTMLElement | null) ?? node;
  }

  function setWidth(px: number): void {
    target.style.setProperty(opts.variable ?? DEFAULT_VAR, `${px}px`);
  }

  function clearWidth(): void {
    target.style.removeProperty(opts.variable ?? DEFAULT_VAR);
  }

  function updateTarget(): void {
    const next = resolveTarget();
    if (next === target) return;
    clearWidth();
    target = next;
  }

  function measure(): void {
    raf = 0;
    updateTarget();
    if (opts.enabled === false) {
      clearWidth();
      return;
    }
    setWidth(measureScrollbarWidth(node));
  }

  function schedule(): void {
    if (raf) cancelAnimationFrame(raf);
    raf = requestAnimationFrame(measure);
  }

  const resizeObserver =
    typeof ResizeObserver !== "undefined" ? new ResizeObserver(schedule) : null;
  const mutationObserver =
    typeof MutationObserver !== "undefined"
      ? new MutationObserver(schedule)
      : null;

  resizeObserver?.observe(node);
  mutationObserver?.observe(node, { childList: true, subtree: true });
  schedule();

  return {
    update(next: ScrollbarWidthVarOptions = {}) {
      opts = next;
      schedule();
    },
    destroy() {
      if (raf) cancelAnimationFrame(raf);
      resizeObserver?.disconnect();
      mutationObserver?.disconnect();
      clearWidth();
    },
  };
}
