/**
 * Explorer's "Copy as path" wraps the path in double quotes. Nothing downstream
 * takes them off - the Go side only trims and cleans - so a pasted path is
 * rejected, and the quote also hides the drive letter from the Windows checks
 * below. Only a matched pair is removed, so a lone quote is left as typed.
 */
export function stripSurroundingQuotes(p: string): string {
  const s = p.trim();
  if (s.length >= 2 && s.startsWith('"') && s.endsWith('"')) {
    return s.slice(1, -1).trim();
  }
  return s;
}

export function normalizeDisplayPath(p: string): string {
  if (!p) return p;
  let s = stripSurroundingQuotes(p);
  const isWin = /^[a-zA-Z]:[\\/]/.test(s) || s.startsWith("\\\\");
  if (isWin) {
    s = s.replace(/\//g, "\\");
    while (s.includes("\\\\")) {
      s = s.replace(/\\\\/g, "\\");
    }
  } else {
    while (s.includes("//")) {
      s = s.replace(/\/\//g, "/");
    }
  }
  return s;
}

/**
 * Identifies a row in the path picker. A shortcut root points at a folder that
 * also appears under its drive, so path alone does not tell the two rows apart:
 * the prefix does, and keeps focus and scrolling on the row inside the tree.
 */
export function treeRowKey(path: string): string {
  return "tree:" + normalizePathKey(path);
}

export function shortcutRowKey(path: string): string {
  return "link:" + normalizePathKey(path);
}

function normalizePathKey(p: string): string {
  const d = normalizeDisplayPath(p);
  if (/^[a-zA-Z]:\\/.test(d) || d.startsWith("\\\\")) {
    return d.toLowerCase();
  }
  return d;
}

export function sameFsPath(a: string, b: string): boolean {
  if (!a && !b) return true;
  if (!a || !b) return false;
  return normalizePathKey(a) === normalizePathKey(b);
}

export function parentDisplayPath(p: string): string {
  const s = normalizeDisplayPath(p);
  if (!s) return "";

  if (/^[a-zA-Z]:\\/.test(s)) {
    const m = s.match(/^([a-zA-Z]:\\)(.*)$/i);
    if (!m) return "";
    const root = m[1];
    const rest = m[2].replace(/\\+$/, "");
    if (!rest) return "";
    const parts = rest.split("\\").filter(Boolean);
    if (parts.length === 0) return "";
    if (parts.length === 1) return root;
    parts.pop();
    return root + parts.join("\\");
  }

  if (s.startsWith("\\\\")) {
    const parts = s.split(/\\+/).filter((x) => x.length > 0);
    if (parts.length <= 2) return "";
    parts.pop();
    return "\\\\" + parts.join("\\");
  }

  const t = s.replace(/\/+$/, "");
  const parts = t.split("/").filter(Boolean);
  if (parts.length <= 1) return "";
  parts.pop();
  return "/" + parts.join("/");
}

function pathPrefix(f: string, s: string): boolean {
  if (!f || !s) return false;
  if (f === s) return true;
  const sep = f.includes("\\") ? "\\" : "/";
  if (f === sep) return s.startsWith(sep);
  const prefix = f.endsWith(sep) ? f : f + sep;
  return s.startsWith(prefix);
}

export function folderCoversSelected(folder: string, selected: string): boolean {
  const f = normalizePathKey(folder);
  const s = normalizePathKey(selected);
  return pathPrefix(f, s);
}

export function isStrictAncestorFolder(folder: string, selected: string): boolean {
  const f = normalizePathKey(folder);
  const s = normalizePathKey(selected);
  if (!f || !s || f === s) return false;
  const sep = f.includes("\\") ? "\\" : "/";
  if (f === sep) return s.length > 1 && s.startsWith(sep);
  const prefix = f.endsWith(sep) ? f : f + sep;
  return s.startsWith(prefix);
}
