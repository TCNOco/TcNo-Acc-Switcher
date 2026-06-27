#!/usr/bin/env python3
"""
Remove unused i18n keys from frontend/src/Resources/*.json.

Usage:
  python other/tools/remove_unused_i18n.py              # dry-run (default)
  python other/tools/remove_unused_i18n.py --apply      # write changes
  python other/tools/remove_unused_i18n.py --apply --prefix Installer_ --prefix Updater_
  python other/tools/remove_unused_i18n.py --apply --prefix Installer_,Updater_,Installed_
  python other/tools/remove_unused_i18n.py --apply --key Settings_WindowsAccent
  python other/tools/remove_unused_i18n.py --apply --key Foo_,Bar_   # comma-separated

Keys are "unused" when the exact key string does not appear in:
  - frontend/src/**/*.{svelte,ts,js}
  - **/*.go
(excluding Resources JSON and generated bindings)
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
RESOURCES = ROOT / "frontend" / "src" / "Resources"
LOCALE_GLOB = "*.json"

SKIP_PARTS = {"Resources", "bindings", "node_modules", ".git"}


def should_scan(path: Path) -> bool:
    return not any(part in path.parts for part in SKIP_PARTS)


def collect_source_blob() -> str:
    patterns = [
        "frontend/src/**/*.svelte",
        "frontend/src/**/*.ts",
        "frontend/src/**/*.js",
        "**/*.go",
    ]
    chunks: list[str] = []
    for pattern in patterns:
        for path in ROOT.glob(pattern):
            if not should_scan(path):
                continue
            try:
                chunks.append(path.read_text(encoding="utf-8", errors="replace"))
            except OSError:
                pass
    return "\n".join(chunks)


def load_keys(path: Path) -> list[str]:
    with path.open(encoding="utf-8") as f:
        data = json.load(f)
    if not isinstance(data, dict):
        raise ValueError(f"{path} is not a JSON object")
    return list(data.keys())


def keys_to_remove(
    canonical_keys: list[str],
    source_blob: str,
    prefixes: tuple[str, ...] | None,
    extra_keys: set[str],
    exclude_keys: set[str],
) -> list[str]:
    unused: list[str] = []
    for key in canonical_keys:
        if key in exclude_keys:
            continue
        if prefixes and not any(key.startswith(p) for p in prefixes):
            continue
        if key in extra_keys or key not in source_blob:
            unused.append(key)
    return unused


def strip_keys(path: Path, remove: set[str]) -> tuple[dict, int]:
    with path.open(encoding="utf-8") as f:
        data = json.load(f)
    if not isinstance(data, dict):
        raise ValueError(f"{path} is not a JSON object")

    removed = 0
    cleaned: dict[str, str] = {}
    for key, value in data.items():
        if key in remove:
            removed += 1
            continue
        cleaned[key] = value
    return cleaned, removed


def write_locale(path: Path, data: dict) -> None:
    text = json.dumps(data, ensure_ascii=False, indent=2)
    path.write_text(text + "\n", encoding="utf-8", newline="\n")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--apply",
        action="store_true",
        help="Write cleaned JSON files (default is dry-run only)",
    )
    parser.add_argument(
        "--prefix",
        metavar="PREFIX",
        action="append",
        default=[],
        help="Only remove keys starting with PREFIX (repeatable). Omit to remove all unused keys.",
    )
    parser.add_argument(
        "--also-remove",
        metavar="KEY",
        action="append",
        default=[],
        help="Force-remove KEY even if referenced in source (repeatable)",
    )
    parser.add_argument(
        "--keep",
        metavar="KEY",
        action="append",
        default=[],
        help="Never remove KEY even if unused (repeatable)",
    )
    parser.add_argument(
        "--key",
        metavar="KEY",
        action="append",
        default=[],
        help="Remove exact KEY from all locale files, even if still referenced in source (repeatable)",
    )
    args = parser.parse_args()

    en_us = RESOURCES / "en-US.json"
    if not en_us.exists():
        print(f"Missing {en_us}", file=sys.stderr)
        return 1

    canonical_keys = load_keys(en_us)
    explicit_keys: set[str] = set()
    for item in args.key:
        explicit_keys.update(k.strip() for k in item.split(",") if k.strip())
    explicit_keys -= set(args.keep)

    prefixes: tuple[str, ...] | None = None
    if args.prefix:
        expanded: list[str] = []
        for item in args.prefix:
            expanded.extend(p.strip() for p in item.split(",") if p.strip())
        prefixes = tuple(expanded)

    remove_set: set[str] = set(explicit_keys)
    remove_list: list[str] = sorted(explicit_keys)

    scan_unused = prefixes is not None or args.also_remove or not explicit_keys
    if scan_unused:
        source_blob = collect_source_blob()
        unused_list = keys_to_remove(
            canonical_keys,
            source_blob,
            prefixes,
            set(args.also_remove),
            set(args.keep),
        )
        remove_set.update(unused_list)
        remove_list = sorted(remove_set, key=lambda k: (k not in explicit_keys, k))

    if not remove_set:
        print("No keys to remove.")
        return 0

    missing_from_en = sorted(explicit_keys - set(canonical_keys))
    if missing_from_en:
        print("Warning: key(s) not in en-US.json:", ", ".join(missing_from_en))

    locale_files = sorted(RESOURCES.glob(LOCALE_GLOB))
    print(f"Mode: {'APPLY' if args.apply else 'DRY-RUN'}")
    if explicit_keys:
        print(f"Explicit key(s): {', '.join(sorted(explicit_keys))}")
    if prefixes:
        print(f"Prefix filter: {', '.join(prefixes)}")
    print(f"Keys to remove: {len(remove_set)}")
    print()

    total_removed = 0
    for path in locale_files:
        cleaned, removed = strip_keys(path, remove_set)
        total_removed += removed
        print(f"  {path.name}: would remove {removed} key(s)")
        if args.apply and removed:
            write_locale(path, cleaned)

    print()
    if args.apply:
        print(f"Done. Removed {total_removed} key occurrence(s) across {len(locale_files)} file(s).")
    else:
        print(f"Dry-run complete ({total_removed} key occurrence(s)). Re-run with --apply to write files.")
        print()
        print("Keys that would be removed:")
        for key in remove_list:
            print(f"  {key}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
