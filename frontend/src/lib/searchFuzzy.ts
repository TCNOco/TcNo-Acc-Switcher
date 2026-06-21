/** Match if every whitespace-separated word in `query` appears in `text` (case-insensitive). */
export function fuzzyWordsMatch(query: string, text: string): boolean {
  const queryWords = query
    .toLowerCase()
    .split(/\s+/)
    .filter((w) => w.length > 0);
  if (queryWords.length === 0) {
    return true;
  }
  const hay = text.toLowerCase();
  return queryWords.every((word) => hay.includes(word));
}
