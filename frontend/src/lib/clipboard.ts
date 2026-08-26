/**
 * Copy text to the clipboard, falling back for webviews without the async API.
 *
 * `navigator.clipboard` needs a secure context, which the app's asset server is
 * not guaranteed to be on every platform - it is absent in WebKitGTK. Throws
 * when the copy did not happen, so a caller can say so instead of pretending it
 * worked.
 */
export async function copyText(text: string): Promise<void> {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }

  // execCommand needs a focused, selected element that is actually in the
  // document, so the textarea is real but kept out of view and out of the
  // accessibility tree. Positioned rather than display:none, which cannot be
  // selected.
  const el = document.createElement("textarea");
  el.value = text;
  el.setAttribute("readonly", "");
  el.setAttribute("aria-hidden", "true");
  el.tabIndex = -1;
  el.style.position = "fixed";
  el.style.top = "-1000px";
  el.style.opacity = "0";
  document.body.appendChild(el);
  try {
    el.select();
    if (!document.execCommand?.("copy")) {
      throw new Error("clipboard unavailable");
    }
  } finally {
    el.remove();
  }
}
