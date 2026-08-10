import { escapeHtml } from "./html";

/**
 * Renders GitHub release-notes markdown to HTML for the update modal.
 *
 * Escape-first: the input is HTML-escaped before any transform, so tags can
 * only come from the transforms below, and callers must still pass the result
 * through sanitizeHtml(..., "inline") at the render site. Covers the subset
 * GitHub's generated notes actually use — headings, bold, italic, inline
 * code, links, bare URLs and lists; anything else stays literal text.
 */
export function renderReleaseNotes(markdown: string): string {
  const lines = markdown.replaceAll("\r\n", "\n").split("\n");
  const out: string[] = [];
  let para: string[] = [];
  let list: { tag: "ul" | "ol"; items: string[] } | null = null;

  const flushPara = () => {
    if (para.length) {
      out.push(`<p>${para.join("<br>")}</p>`);
      para = [];
    }
  };
  const flushList = () => {
    if (list) {
      out.push(`<${list.tag}>${list.items.map((i) => `<li>${i}</li>`).join("")}</${list.tag}>`);
      list = null;
    }
  };
  const pushListItem = (tag: "ul" | "ol", item: string) => {
    flushPara();
    if (list?.tag !== tag) {
      flushList();
      list = { tag, items: [] };
    }
    list.items.push(inlineMarkdown(item));
  };

  for (const rawLine of lines) {
    const line = rawLine.replace(/^>\s?/, "").trimEnd();
    const heading = /^#{1,6}\s+(.+)$/.exec(line);
    const bullet = /^\s*[-*+]\s+(.+)$/.exec(line);
    const ordered = /^\s*\d+[.)]\s+(.+)$/.exec(line);

    if (!line.trim() || /^\s*(-{3,}|_{3,}|\*{3,})\s*$/.test(line)) {
      flushPara();
      flushList();
    } else if (heading) {
      flushPara();
      flushList();
      out.push(`<p><strong>${inlineMarkdown(heading[1])}</strong></p>`);
    } else if (bullet) {
      pushListItem("ul", bullet[1]);
    } else if (ordered) {
      pushListItem("ol", ordered[1]);
    } else {
      flushList();
      para.push(inlineMarkdown(line.trim()));
    }
  }
  flushPara();
  flushList();
  return out.join("");
}

function inlineMarkdown(text: string): string {
  let s = escapeHtml(text);
  s = s.replace(/`([^`]+)`/g, "<code>$1</code>");
  s = s.replace(/\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g, '<a href="$2">$1</a>');
  s = s.replace(/(^|[\s(])(https?:\/\/[^\s]+)/g, (_, lead: string, url: string) => {
    // Trailing punctuation belongs to the sentence, not the URL.
    const trimmed = url.replace(/[).,;:!?]+$/, "");
    return `${lead}<a href="${trimmed}">${trimmed}</a>${url.slice(trimmed.length)}`;
  });
  s = s.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  s = s.replace(/(^|[\s>])\*([^*\n]+)\*(?=[\s<.,;:!?)]|$)/g, "$1<em>$2</em>");
  s = s.replace(/(^|[\s>])_([^_\n]+)_(?=[\s<.,;:!?)]|$)/g, "$1<em>$2</em>");
  return s;
}
