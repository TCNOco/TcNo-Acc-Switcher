export type ViewportDropdownLayout = {
  maxHeight: number;
  /** Physical px to pull the menu back from the viewport's right edge. */
  shift: number;
  /** How far to scroll the page so a long menu has room to open downward. */
  scrollBy: number;
};

const VIEWPORT_MARGIN = 8;
const MAX_VIEWPORT_HEIGHT_RATIO = 0.6;
/** Below this a menu is too short to be worth opening in place. */
const MIN_MENU_HEIGHT = 144;

export function computeViewportDropdownLayout(
  trigger: Pick<DOMRect, "top" | "bottom" | "left">,
  options: { viewportHeight: number; viewportWidth: number; menuHeight: number; menuWidth: number },
): ViewportDropdownLayout {
  const heightCap = Math.floor(options.viewportHeight * MAX_VIEWPORT_HEIGHT_RATIO);
  const wanted = Math.min(options.menuHeight, heightCap);
  const spaceBelow = Math.max(0, Math.floor(options.viewportHeight - trigger.bottom - VIEWPORT_MARGIN));

  /* Always downward. Flipping a menu above its trigger moves the options away
     from where the pointer already is and buries the control they belong to;
     scrolling the page is the cheaper way to reach a long list. */
  const scrollBy = Math.max(0, wanted - spaceBelow);
  const maxHeight = Math.min(wanted, Math.max(spaceBelow, MIN_MENU_HEIGHT));

  /* Pull back from the right edge, but never past the left one — a menu shoved
     off the near side is no more usable than one off the far side. */
  const overflowEnd = Math.max(0, trigger.left + options.menuWidth - (options.viewportWidth - VIEWPORT_MARGIN));
  // `|| 0` collapses the -0 that negating zero would otherwise write as "-0px".
  const shift = -Math.min(overflowEnd, Math.max(0, trigger.left - VIEWPORT_MARGIN)) || 0;

  return { maxHeight, shift, scrollBy };
}

export function applyViewportDropdownLayout(
  style: Pick<CSSStyleDeclaration, "setProperty">,
  layout: ViewportDropdownLayout,
): void {
  // `important`: some themes pin their own `top` on `.custom-dropdown-menu`.
  style.setProperty("top", "100%", "important");
  style.setProperty("bottom", "auto", "important");
  style.setProperty("left", `${layout.shift}px`, "important");
  style.setProperty("max-height", `${layout.maxHeight}px`);
}

function scrollableAncestor(node: HTMLElement): HTMLElement | null {
  let el = node.parentElement;
  while (el) {
    const { overflowY } = getComputedStyle(el);
    if ((overflowY === "auto" || overflowY === "scroll") && el.scrollHeight > el.clientHeight) {
      return el;
    }
    el = el.parentElement;
  }
  return null;
}

export function viewportDropdown(node: HTMLElement) {
  let frame = 0;
  // One nudge per open, or re-measuring after the scroll would nudge again.
  let nudged = false;

  function measure(): void {
    frame = 0;
    const trigger = node.parentElement?.querySelector<HTMLElement>(".dropdown-toggle");
    if (!trigger) return;

    const rect = node.getBoundingClientRect();
    const layout = computeViewportDropdownLayout(trigger.getBoundingClientRect(), {
      viewportHeight: window.innerHeight || document.documentElement.clientHeight,
      viewportWidth: window.innerWidth || document.documentElement.clientWidth,
      menuHeight: Math.max(node.scrollHeight, rect.height),
      menuWidth: Math.max(node.scrollWidth, rect.width),
    });

    applyViewportDropdownLayout(node.style, layout);

    if (!nudged && layout.scrollBy > 0) {
      nudged = true;
      const scroller = scrollableAncestor(node);
      if (scroller) {
        const before = scroller.scrollTop;
        scroller.scrollTop = before + layout.scrollBy;
        /* Re-measure inline, not on the next frame: the menu would otherwise
           paint once at the cramped height and jump. `nudged` bounds it to a
           single retry. */
        if (scroller.scrollTop !== before) measure();
      }
    }
  }

  function schedule(): void {
    if (frame) cancelAnimationFrame(frame);
    frame = requestAnimationFrame(measure);
  }

  measure();
  window.addEventListener("resize", schedule);
  window.addEventListener("scroll", schedule, true);

  return {
    destroy(): void {
      if (frame) cancelAnimationFrame(frame);
      window.removeEventListener("resize", schedule);
      window.removeEventListener("scroll", schedule, true);
    },
  };
}
