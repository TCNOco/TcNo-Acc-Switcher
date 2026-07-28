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


def load_source() -> dict[str, str]:
    with (RESOURCES / f"{SOURCE_LOCALE}.json").open(encoding="utf-8-sig") as f:
        return {str(k): str(v) for k, v in json.load(f).items()}


def load_patch(source: str) -> dict[str, str]:
    text = sys.stdin.read() if source == "-" else Path(source).read_text(encoding="utf-8")
    data = json.loads(text)
    if not isinstance(data, dict):
        raise ValueError("patch must be a JSON object of {key: translated value}")
    return {str(k): str(v) for k, v in data.items()}


def line_pattern(key: str) -> re.Pattern[str]:
    return re.compile(rf'^(\s*){re.escape(json.dumps(key))}:\s*".*"(,?)$')


def key_line_index(lines: list[str], key: str) -> int | None:
    pattern = line_pattern(key)
    for index, line in enumerate(lines):
        if pattern.match(line):
            return index
    return None


def insert_key(lines: list[str], key: str, value: str, source_order: list[str]) -> None:
    """Insert a key the locale does not have yet, at the position en-US puts it."""
    anchor = None
    for previous in reversed(source_order[: source_order.index(key)]):
        anchor = key_line_index(lines, previous)
        if anchor is not None:
            break

    if anchor is None:  # nothing precedes it in this file; go straight after the brace.
        anchor = next(i for i, line in enumerate(lines) if line.lstrip().startswith("{"))

    indent = "  "
    for line in lines[anchor : anchor + 2]:
        stripped = line.lstrip()
        if stripped.startswith('"'):
            indent = line[: len(line) - len(stripped)]
            break

    # The last entry carries no trailing comma, so inserting after it moves the comma.
    anchor_is_last = not lines[anchor].rstrip().endswith(",") and lines[anchor].lstrip().startswith('"')
    if anchor_is_last:
        lines[anchor] = lines[anchor].rstrip() + ","
        comma = ""
    else:
        comma = "" if lines[anchor + 1].lstrip().startswith("}") else ","

    lines.insert(anchor + 1, f"{indent}{json.dumps(key)}: {json.dumps(value, ensure_ascii=False)}{comma}")


def apply_patch(
    text: str,
    patch: dict[str, str],
    source_order: list[str],
) -> tuple[str, list[str], list[str], list[str]]:
    lines = text.split("\n")
    applied: list[str] = []
    inserted: list[str] = []
    unknown: list[str] = []

    for key, value in patch.items():
        if key not in source_order:
            unknown.append(key)
            continue
        index = key_line_index(lines, key)
        if index is None:
            insert_key(lines, key, value, source_order)
            inserted.append(key)
            continue
        indent, comma = line_pattern(key).match(lines[index]).groups()
        lines[index] = f"{indent}{json.dumps(key)}: {json.dumps(value, ensure_ascii=False)}{comma}"
        applied.append(key)

    return "\n".join(lines), applied, inserted, unknown


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

    source_order = list(load_source())
    original = path.read_text(encoding="utf-8")
    before = json.loads(original)
    updated, applied, inserted, unknown = apply_patch(original, patch, source_order)

    after = json.loads(updated)
    expected_order = [key for key in source_order if key in after]
    if list(after) != expected_order:
        print(f"Refusing to write: key order would not match {SOURCE_LOCALE}.json.", file=sys.stderr)
        return 1
    dropped = [key for key in before if key not in after]
    if dropped:
        print(f"Refusing to write: {len(dropped)} key(s) would be lost: {', '.join(dropped[:5])}", file=sys.stderr)
        return 1
    unexpected = [k for k in after if k in before and after[k] != before[k] and k not in patch]
    if unexpected:
        print(f"Refusing to write: {len(unexpected)} unrelated key(s) changed: {', '.join(unexpected[:5])}", file=sys.stderr)
        return 1
    wrong = [k for k in applied + inserted if after[k] != patch[k]]
    if wrong:
        print(f"Refusing to write: {len(wrong)} key(s) did not round-trip: {', '.join(wrong[:5])}", file=sys.stderr)
        return 1

    if unknown:
        print(f"{len(unknown)} key(s) not in {SOURCE_LOCALE}.json, skipped: {', '.join(unknown[:10])}", file=sys.stderr)

    unchanged = [k for k in applied if before[k] == patch[k]]
    changed = len(applied) - len(unchanged)
    summary = f"{changed} key(s) updated, {len(inserted)} inserted in {path.name} ({len(unchanged)} already identical)."
    if args.check:
        print(f"Would apply: {summary}")
        return 1 if unknown else 0

    path.write_text(updated, encoding="utf-8", newline="\n")
    print(f"Applied: {summary}")
    return 1 if unknown else 0


if __name__ == "__main__":
    raise SystemExit(main())
