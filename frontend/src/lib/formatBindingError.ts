// Improve errors from Wails for displying in Modal dialog
export function simplifyBindingErrorText(text: string): string {
  const t = text.trim();
  if (!t.startsWith("{") || !t.includes('"message"')) {
    return text;
  }
  try {
    const o = JSON.parse(t) as { message?: unknown };
    if (typeof o.message === "string" && o.message.trim()) {
      return o.message.trim();
    }
  } catch {

  }
  return text;
}

export function formatUnknownError(e: unknown): string {
  if (e instanceof Error && e.message) {
    return simplifyBindingErrorText(e.message);
  }
  return simplifyBindingErrorText(String(e));
}
