type HtmlPolicyName = "inline" | "gameStats" | "miniProfile";

const policies: Record<HtmlPolicyName, Set<string>> = {
  inline: new Set(["a", "b", "br", "code", "em", "i", "li", "ol", "p", "span", "strong", "sub", "sup", "ul"]),
  gameStats: new Set([
    "b",
    "br",
    "defs",
    "div",
    "em",
    "g",
    "h6",
    "i",
    "img",
    "path",
    "span",
    "strong",
    "sup",
    "svg",
    "use",
  ]),
  miniProfile: new Set(["b", "br", "div", "em", "i", "img", "p", "source", "span", "strong", "video"]),
};

const svgTags = new Set(["defs", "g", "path", "svg", "use"]);
const globalAttrs = new Set(["aria-hidden", "aria-label", "class", "role", "title"]);
const gameStatsAttrs = new Set([
  "alt",
  "d",
  "draggable",
  "fill",
  "fill-opacity",
  "fill-rule",
  "height",
  "id",
  "stroke-linejoin",
  "stroke-miterlimit",
  "style",
  "viewBox",
  "viewbox",
  "width",
  "xmlns",
]);
const miniProfileAttrs = new Set(["alt", "autoplay", "loop", "muted", "playsinline", "src", "srcset", "type"]);

function safeClass(value: string): string {
  return value
    .split(/\s+/)
    .filter((token) => /^[A-Za-z0-9_-]+$/.test(token))
    .join(" ");
}

function safeStyle(value: string): string {
  const v = value.trim();
  if (
    v.length > 600 ||
    /url\s*\(/i.test(v) ||
    /expression\s*\(/i.test(v) ||
    /@import/i.test(v) ||
    /behavior\s*:/i.test(v) ||
    /-moz-binding/i.test(v)
  ) {
    return "";
  }
  return v;
}

function safeUrl(value: string, { image = false, svgUse = false }: { image?: boolean; svgUse?: boolean } = {}): string {
  const v = value.trim();
  if (!v) {
    return "";
  }
  if (svgUse && (/^#[A-Za-z0-9_-]+$/.test(v) || /^\/?img\/icons\/[A-Za-z0-9_./-]+#[A-Za-z0-9_-]+$/.test(v))) {
    return v;
  }
  if (image && /^data:image\/(?:png|jpe?g|webp|gif);base64,[A-Za-z0-9+/=]+$/i.test(v)) {
    return v;
  }
  if (/^\/?img\/[A-Za-z0-9_./%-]+$/i.test(v)) {
    return v;
  }
  try {
    const parsed = new URL(v);
    return parsed.protocol === "https:" ? parsed.toString() : "";
  } catch {
    return "";
  }
}

function safeSrcset(value: string): string {
  const kept: string[] = [];
  for (const part of value.split(",")) {
    const fields = part.trim().split(/\s+/).filter(Boolean);
    if (fields.length === 0) {
      continue;
    }
    const url = safeUrl(fields[0], { image: true });
    if (!url) {
      continue;
    }
    kept.push([url, ...fields.slice(1)].join(" "));
  }
  return kept.join(", ");
}

function allowAttr(policy: HtmlPolicyName, tag: string, attr: Attr): string | null {
  const name = attr.name;
  const key = name.toLowerCase();
  if (key.startsWith("on") || key === "srcdoc") {
    return null;
  }
  if (key === "class") {
    return safeClass(attr.value);
  }
  if (globalAttrs.has(key)) {
    return attr.value.trim();
  }
  if (key === "href") {
    return safeUrl(attr.value, { svgUse: tag === "use" });
  }
  if (key === "src") {
    return safeUrl(attr.value, { image: tag === "img" });
  }
  if (key === "srcset") {
    return safeSrcset(attr.value);
  }
  if (policy === "miniProfile" && miniProfileAttrs.has(key)) {
    return attr.value.trim();
  }
  if (policy === "gameStats" && gameStatsAttrs.has(name)) {
    return key === "style" ? safeStyle(attr.value) : attr.value.trim();
  }
  if (policy === "gameStats" && gameStatsAttrs.has(key)) {
    return key === "style" ? safeStyle(attr.value) : attr.value.trim();
  }
  return null;
}

function cleanAttrName(name: string): string {
  return name.toLowerCase() === "viewbox" ? "viewBox" : name;
}

function sanitizeNode(node: Node, policy: HtmlPolicyName): Node | DocumentFragment | null {
  if (node.nodeType === Node.TEXT_NODE) {
    return document.createTextNode(node.textContent ?? "");
  }
  if (node.nodeType !== Node.ELEMENT_NODE) {
    return null;
  }

  const src = node as Element;
  const tag = src.tagName.toLowerCase();
  const fragment = document.createDocumentFragment();
  if (!policies[policy].has(tag)) {
    for (const child of Array.from(src.childNodes)) {
      const clean = sanitizeNode(child, policy);
      if (clean) {
        fragment.appendChild(clean);
      }
    }
    return fragment;
  }

  const clean = svgTags.has(tag)
    ? document.createElementNS("http://www.w3.org/2000/svg", tag)
    : document.createElement(tag);
  for (const attr of Array.from(src.attributes)) {
    const val = allowAttr(policy, tag, attr);
    if (val) {
      clean.setAttribute(cleanAttrName(attr.name), val);
    }
  }
  if (tag === "a") {
    clean.setAttribute("rel", "noreferrer noopener");
    clean.setAttribute("target", "_blank");
  }
  for (const child of Array.from(src.childNodes)) {
    const cleanChild = sanitizeNode(child, policy);
    if (cleanChild) {
      clean.appendChild(cleanChild);
    }
  }
  return clean;
}

export function sanitizeHtml(html: string | null | undefined, policy: HtmlPolicyName = "inline"): string {
  if (typeof document === "undefined") {
    return "";
  }
  const raw = String(html ?? "").trim();
  if (!raw) {
    return "";
  }
  const template = document.createElement("template");
  template.innerHTML = raw;
  const out = document.createElement("template");
  for (const child of Array.from(template.content.childNodes)) {
    const clean = sanitizeNode(child, policy);
    if (clean) {
      out.content.appendChild(clean);
    }
  }
  return out.innerHTML;
}
