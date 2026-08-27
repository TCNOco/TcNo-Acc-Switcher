/**
 * Zips a locale's positional value array back onto the shared key index.
 *
 * Runs on the startup path, so it stays a plain indexed loop. A null slot is
 * left absent rather than stored, which is what lets the en-US spread underneath
 * supply the string for a key that locale is missing.
 */
export function zipMessages(
  keys: readonly string[],
  values: readonly (string | null)[],
): Record<string, string> {
  // Null prototype so a key like __proto__ becomes an own property, which is
  // what the JSON.parse this replaced would have produced. Plain assignment on
  // a normal object would set the prototype instead.
  const messages: Record<string, string> = Object.create(null);
  for (let i = 0; i < keys.length; i++) {
    const value = values[i];
    if (value != null) messages[keys[i]] = value;
  }
  return messages;
}
