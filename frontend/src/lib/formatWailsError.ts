/**
 * Formats Wails/JS errors: `message` lines first, then other fields as "Key: value" lines
 * (e.g. `cause`, `kind`). Plain objects, JSON strings, and `{ message, cause, kind }` shapes.
 */
export type FormatWailsErrorOptions = {
  /** If the first line of `message` is in this set, replace with `translateMessage` (i18n). */
  i18nFirstLineKeys?: Set<string>;
  translateMessage?: (key: string) => string;
};

function isEmptyPlainObject(v: unknown): boolean {
  return v !== null && typeof v === "object" && !Array.isArray(v) && Object.keys(v as object).length === 0;
}

/**
 * @param err - Wails/runtime error
 * @param options - optional i18n for the first `message` line (when it is a known key; see Steam userdata ops)
 */
export function formatWailsError(err: unknown, options?: FormatWailsErrorOptions): string {
  const obj = extractErrorObject(err);
  if (!obj) {
    if (err == null) return "";
    return String(err);
  }

  const lines: string[] = [];
  const msg = obj.message;
  if (typeof msg === "string" && msg.trim()) {
    const messageLines = msg
      .split("\n")
      .map((s) => s.replace(/\r$/, "").trimEnd())
      .filter((s) => s.length > 0);
    if (messageLines.length > 0) {
      const first = messageLines[0].trim();
      const keys = options?.i18nFirstLineKeys;
      const tr = options?.translateMessage;
      const head = tr && keys && keys.has(first) ? tr(first) : first;
      lines.push(head, ...messageLines.slice(1));
    }
  }

  const otherKeys = Object.keys(obj)
    .filter((k) => k !== "message")
    .sort((a, b) => a.localeCompare(b));
  for (const k of otherKeys) {
    const v = obj[k];
    if (v === undefined) continue;
    if (k === "cause" && isEmptyPlainObject(v)) {
      lines.push("Cause: <>");
      continue;
    }
    lines.push(`${formatKeyLabel(k)}: ${formatValue(v)}`);
  }

  if (lines.length === 0) {
    return String(err);
  }
  return lines.join("\n");
}

/** Prefix + blank line + formatted error body (same options as [formatWailsError]). */
export function formatToastWithError(
  prefix: string,
  err: unknown,
  wailsOptions?: FormatWailsErrorOptions,
): string {
  const body = formatWailsError(err, wailsOptions);
  const p = prefix.trim();
  if (!p) return body;
  if (!body) return p;
  return `${p}\n\n${body}`;
}

/**
 * Wails often rejects with `Error` whose `.message` is a whole JSON object string, or `{ message: "{...}" }`.
 * Unwrap so we format like plain `{ message, cause, kind }`.
 */
function tryParseJSONMessageObject(msg: string): Record<string, unknown> | null {
  const t = msg.trim();
  if (!t.startsWith("{")) return null;
  try {
    const parsed = JSON.parse(t) as unknown;
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
  } catch {
    /* keep null */
  }
  return null;
}

function extractErrorObject(err: unknown): Record<string, unknown> | null {
  if (err == null) return null;

  if (typeof err === "object" && !Array.isArray(err)) {
    const o = err as Record<string, unknown>;
    if (typeof o.message === "string") {
      const inner = tryParseJSONMessageObject(o.message);
      if (inner && typeof inner.message === "string") {
        return { ...o, ...inner };
      }
    }
    if ("message" in o || "cause" in o || "kind" in o) {
      return o;
    }
  }

  if (typeof err === "string") {
    return tryParseJSONMessageObject(err) ?? null;
  }

  if (err instanceof Error) {
    const fromMsg = err.message != null ? tryParseJSONMessageObject(String(err.message)) : null;
    if (fromMsg) return fromMsg;
    return { message: err.message };
  }

  return null;
}

function formatKeyLabel(k: string): string {
  if (k.length === 0) return k;
  return k.charAt(0).toUpperCase() + k.slice(1);
}

function formatValue(v: unknown): string {
  if (v == null) return String(v);
  if (typeof v === "string") {
    return splitCamelCaseErrors(v);
  }
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  if (typeof v === "bigint") return v.toString();
  return JSON.stringify(v);
}

/** e.g. RuntimeError → Runtime Error for display */
function splitCamelCaseErrors(s: string): string {
  if (!/[a-z][A-Z]/.test(s)) return s;
  return s.replace(/([a-z])([A-Z])/g, "$1 $2");
}
