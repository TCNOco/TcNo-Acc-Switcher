#!/usr/bin/env python3
"""Apply translated values to a locale file without rewriting the whole file.

Takes a JSON object of {key: translated value} and rewrites only those keys'
lines, so every untouched key stays byte-identical and key order cannot drift.
Lets a translation pass edit a locale it never had to read.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
RESOURCES = ROOT / "frontend" / "src" / "Resources"
SOURCE_LOCALE = "en-US"


def load_patch(source: str) -> dict[str, str]:
    text = sys.stdin.read() if source == "-" else Path(source).read_text(encoding="utf-8")
    data = json.loads(text)
    if not isinstance(data, dict):
        raise ValueError("patch must be a JSON object of {key: translated value}")
    return {str(k): str(v) for k, v in data.items()}


def line_pattern(key: str) -> re.Pattern[str]:
    return re.compile(rf'^(\s*){re.escape(json.dumps(key))}:\s*".*"(,?)$')


def apply_patch(text: str, patch: dict[str, str]) -> tuple[str, list[str], list[str]]:
    lines = text.split("\n")
    applied: list[str] = []
    missing: list[str] = []

    for key, value in patch.items():
        pattern = line_pattern(key)
        for index, line in enumerate(lines):
            match = pattern.match(line)
            if match:
                indent, comma = match.group(1), match.group(2)
                lines[index] = f"{indent}{json.dumps(key)}: {json.dumps(value, ensure_ascii=False)}{comma}"
                applied.append(key)
                break
        else:
            missing.append(key)

    return "\n".join(lines), applied, missing


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("locale", help="Locale code to update, e.g. es-ES.")
    parser.add_argument("patch", nargs="?", default="-", help="JSON file of {key: value}, or - for stdin (default).")
    parser.add_argument("--check", action="store_true", help="Report what would change without writing.")
    args = parser.parse_args()

    if args.locale == SOURCE_LOCALE:
        print(f"Refusing to patch the source locale {SOURCE_LOCALE}.", file=sys.stderr)
        return 1

    path = RESOURCES / f"{args.locale}.json"
    if not path.exists():
        print(f"No such locale file: {path}", file=sys.stderr)
        return 1

    patch = load_patch(args.patch)
    if not patch:
        print("Empty patch; nothing to do.")
        return 0

    original = path.read_text(encoding="utf-8")
    before = json.loads(original)
    updated, applied, missing = apply_patch(original, patch)

    after = json.loads(updated)
    if list(after) != list(before):
        print("Refusing to write: key order or key set changed.", file=sys.stderr)
        return 1
    unexpected = [k for k in after if after[k] != before[k] and k not in patch]
    if unexpected:
        print(f"Refusing to write: {len(unexpected)} unrelated key(s) changed: {', '.join(unexpected[:5])}", file=sys.stderr)
        return 1
    wrong = [k for k in applied if after[k] != patch[k]]
    if wrong:
        print(f"Refusing to write: {len(wrong)} key(s) did not round-trip: {', '.join(wrong[:5])}", file=sys.stderr)
        return 1

    if missing:
        print(f"{len(missing)} key(s) not found in {path.name}: {', '.join(missing[:10])}", file=sys.stderr)

    unchanged = [k for k in applied if before[k] == patch[k]]
    if args.check:
        print(f"Would update {len(applied) - len(unchanged)} key(s) in {path.name} ({len(unchanged)} already identical).")
        return 1 if missing else 0

    path.write_text(updated, encoding="utf-8", newline="\n")
    print(f"Updated {len(applied) - len(unchanged)} key(s) in {path.name} ({len(unchanged)} already identical).")
    return 1 if missing else 0


if __name__ == "__main__":
    raise SystemExit(main())
