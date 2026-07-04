#!/usr/bin/env python3
"""Validate locale JSON files against frontend/src/Resources/en-US.json."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
RESOURCES = ROOT / "frontend" / "src" / "Resources"
SOURCE_LOCALE = "en-US"
SOURCE_FILE = RESOURCES / f"{SOURCE_LOCALE}.json"
PLACEHOLDER_RE = re.compile(r"\{([A-Za-z_][A-Za-z0-9_]*)\}")
TAG_RE = re.compile(r"</?([A-Za-z][A-Za-z0-9]*)\b[^>]*>|<br\s*/?>", re.IGNORECASE)


def load_json(path: Path) -> dict[str, str]:
    with path.open(encoding="utf-8-sig") as f:
        data = json.load(f)
    if not isinstance(data, dict):
        raise ValueError(f"{path} is not a JSON object")
    return {str(k): str(v) for k, v in data.items()}


def placeholders(value: str) -> list[str]:
    return PLACEHOLDER_RE.findall(value)


def html_tags(value: str) -> list[str]:
    tags: list[str] = []
    for match in TAG_RE.finditer(value):
        raw = match.group(0).lower()
        if raw.startswith("<br"):
            tags.append("br")
        else:
            prefix = "/" if raw.startswith("</") else ""
            tags.append(prefix + match.group(1).lower())
    return tags


def validate_locale(path: Path, source: dict[str, str]) -> list[str]:
    errors: list[str] = []
    try:
        data = load_json(path)
    except Exception as exc:  # noqa: BLE001 - report all parse/load failures.
        return [f"{path.name}: cannot load JSON: {exc}"]

    source_keys = list(source)
    keys = list(data)
    if keys != source_keys:
        missing = [key for key in source_keys if key not in data]
        extra = [key for key in keys if key not in source]
        if missing:
            errors.append(f"{path.name}: missing {len(missing)} key(s): {', '.join(missing[:10])}")
        if extra:
            errors.append(f"{path.name}: extra {len(extra)} key(s): {', '.join(extra[:10])}")
        if not missing and not extra:
            errors.append(f"{path.name}: key order differs from {SOURCE_FILE.name}")

    for key, source_value in source.items():
        if key not in data:
            continue
        translated = data[key]
        src_placeholders = sorted(placeholders(source_value))
        dst_placeholders = sorted(placeholders(translated))
        if dst_placeholders != src_placeholders:
            errors.append(
                f"{path.name}:{key}: placeholders {dst_placeholders} != {src_placeholders}"
            )

        src_tags = html_tags(source_value)
        if src_tags:
            dst_tags = html_tags(translated)
            if dst_tags != src_tags:
                errors.append(f"{path.name}:{key}: html tags {dst_tags} != {src_tags}")

    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("locales", nargs="*", help="Locale codes to validate. Defaults to every non-English JSON.")
    args = parser.parse_args()

    source = load_json(SOURCE_FILE)
    if args.locales:
        paths = [RESOURCES / f"{locale}.json" for locale in args.locales]
    else:
        paths = sorted(path for path in RESOURCES.glob("*.json") if path.stem != SOURCE_LOCALE)

    if not paths:
        print("No locale files to validate.")
        return 0

    all_errors: list[str] = []
    for path in paths:
        if not path.exists():
            all_errors.append(f"{path.name}: file does not exist")
            continue
        all_errors.extend(validate_locale(path, source))

    if all_errors:
        for error in all_errors:
            print(error, file=sys.stderr)
        print(f"FAILED: {len(all_errors)} issue(s) across {len(paths)} locale file(s).", file=sys.stderr)
        return 1

    print(f"OK: {len(paths)} locale file(s) match {SOURCE_FILE.name}.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
