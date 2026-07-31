#!/usr/bin/env python3
"""Retire keys from every locale file, line by line.

The counterpart to i18n_apply.py, which deliberately refuses to drop a key so a
translation pass can never lose one. Removing a string the app no longer shows
is the one case where dropping is the point, and doing it by hand across 38
files is how a stray comma or a reordered key gets in.

Only the named keys' lines are touched; everything else stays byte-identical.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
RESOURCES = ROOT / "frontend" / "src" / "Resources"


def line_pattern(key: str) -> re.Pattern[str]:
    return re.compile(rf'^\s*{re.escape(json.dumps(key))}:\s*".*"(,?)$')


def remove_keys(text: str, keys: list[str]) -> tuple[str, list[str]]:
    lines = text.split("\n")
    removed: list[str] = []
    for key in keys:
        pattern = line_pattern(key)
        for index, line in enumerate(lines):
            if pattern.match(line):
                del lines[index]
                removed.append(key)
                break

    # Removing the final entry leaves the one before it carrying a comma that
    # now precedes the closing brace, which is not valid JSON.
    for index in range(len(lines) - 1, -1, -1):
        stripped = lines[index].strip()
        if not stripped or stripped.startswith("}"):
            continue
        if stripped.endswith(","):
            lines[index] = lines[index].rstrip()[:-1]
        break

    return "\n".join(lines), removed


def process(path: Path, keys: list[str], check: bool) -> tuple[bool, str]:
    original = path.read_text(encoding="utf-8")
    before = json.loads(original.lstrip("﻿"))
    updated, removed = remove_keys(original, keys)
    if not removed:
        return True, f"{path.name}: none of the keys present"

    after = json.loads(updated.lstrip("﻿"))
    expected = {k: v for k, v in before.items() if k not in removed}
    if after != expected:
        return False, f"{path.name}: refusing to write, content other than the named keys changed"
    if list(after) != [k for k in before if k not in removed]:
        return False, f"{path.name}: refusing to write, key order would drift"

    if not check:
        path.write_text(updated, encoding="utf-8", newline="\n")
    return True, f"{path.name}: {len(removed)} key(s) removed"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("keys", nargs="+", help="Keys to remove from every locale file.")
    parser.add_argument("--check", action="store_true", help="Report what would change without writing.")
    args = parser.parse_args()

    failures = 0
    for path in sorted(RESOURCES.glob("*.json")):
        ok, message = process(path, args.keys, args.check)
        print(("Would apply: " if args.check and ok else "") + message)
        failures += 0 if ok else 1
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
