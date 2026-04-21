/**
 * Formats Wails/JS errors for toasts: primary `message`, then other fields as "Key: value" lines.
 * Handles JSON strings and plain objects like `{ message, cause, kind }`.
 */
export function formatWailsError(err: unknown): string {
  const obj = extractErrorObject(err);
  if (!obj) {
    if (err == null) return "";
    return String(err);
  }

  const lines: string[] = [];
  const msg = obj.message;
  if (typeof msg === "string" && msg.trim()) {
    lines.push(msg.trim());
  }

  const keys = Object.keys(obj)
    .filter((k) => k !== "message")
    .sort((a, b) => a.localeCompare(b));
  for (const k of keys) {
    const v = obj[k];
    if (v === undefined) continue;
    lines.push(`${formatKeyLabel(k)}: ${formatValue(v)}`);
  }

  if (lines.length === 0) {
    return String(err);
  }
  return lines.join("\n");
}

/** Prefix string + blank line + formatted error (no trailing duplication of prefix). */
export function formatToastWithError(prefix: string, err: unknown): string {
  const body = formatWailsError(err);
  const p = prefix.trim();
  if (!p) return body;
  if (!body) return p;
  return `${p}\n\n${body}`;
}

function extractErrorObject(err: unknown): Record<string, unknown> | null {
  if (err == null) return null;

  if (typeof err === "object" && !Array.isArray(err)) {
    const o = err as Record<string, unknown>;
    if ("message" in o || "cause" in o || "kind" in o) {
      return o;
    }
  }

  if (typeof err === "string") {
    const t = err.trim();
    if (t.startsWith("{")) {
      try {
        const parsed = JSON.parse(t) as unknown;
        if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
          return parsed as Record<string, unknown>;
        }
      } catch {
        return null;
      }
    }
    return null;
  }

  if (err instanceof Error) {
    return { message: err.message };
  }

  return null;
}

function formatKeyLabel(k: string): string {
  if (k.length === 0) return k;
  return k.charAt(0).toUpperCase() + k.slice(1);
}

function formatValue(v: unknown): string {
  if (v === null || v === undefined) return String(v);
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
