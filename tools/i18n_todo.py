#!/usr/bin/env python3
"""Report the outstanding i18n work per locale: missing keys and values that are
still verbatim English.

Prints only the keys that need attention, each with the context needed to
translate it, so a translation pass never has to open a whole locale file.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

sys.dont_write_bytecode = True  # importing a sibling tool must not litter tools/__pycache__.
sys.path.insert(0, str(Path(__file__).resolve().parent))

# Windows consoles default to cp1252, which mangles the ellipses and curly quotes in
# the English source. Callers that cannot trust this output go looking for a way to
# dump everything at once instead of translating in batches.
for _stream in (sys.stdout, sys.stderr):
    if hasattr(_stream, "reconfigure"):
        _stream.reconfigure(encoding="utf-8")

from generate_i18n_plan import (  # noqa: E402 - path shim above must run first.
    PLACEHOLDER_RE,
    RESOURCES,
    SOURCE_LOCALE,
    SOURCE_FILE,
    collect_usage,
    load_source_strings,
    role_for,
)

DEFAULT_MAX_ITEMS = 60
# A hard ceiling, not a default: one call must not be able to dump the whole backlog.
# Batching is the only thing keeping a locale with 300 outstanding keys from landing
# in a caller's context in one piece.
MAX_ITEMS_CEILING = 60
IGNORE_FILE = Path(__file__).resolve().parent / "i18n_untranslated.json"
GLOBAL_SCOPE = "*"

# There is deliberately no "identical in most locales, so it must be a proper noun"
# heuristic here. It cannot tell a proper noun from a key that simply has not been
# translated anywhere yet, and it hid 15 real keys by scoring them 37/37 identical.
# Untranslatable keys are recorded explicitly in IGNORE_FILE instead.


def load_ignored() -> dict[str, list[str]]:
    if not IGNORE_FILE.exists():
        return {}
    with IGNORE_FILE.open(encoding="utf-8") as f:
        data = json.load(f)
    return {k: list(v) for k, v in data.items() if isinstance(v, list)}


def ignored_for(ignored: dict[str, list[str]], locale: str | None) -> set[str]:
    keys = set(ignored.get(GLOBAL_SCOPE, []))
    if locale:
        keys |= set(ignored.get(locale, []))
    return keys


def add_ignored(keys: list[str], scope: str, source: dict[str, str]) -> None:
    unknown = [key for key in keys if key not in source]
    if unknown:
        print(f"Not in {SOURCE_LOCALE}.json, refusing to add: {', '.join(unknown)}", file=sys.stderr)
        keys = [key for key in keys if key in source]
    if not keys:
        return

    raw = json.loads(IGNORE_FILE.read_text(encoding="utf-8")) if IGNORE_FILE.exists() else {}
    existing = set(raw.get(scope, []))
    added = sorted(set(keys) - existing)
    raw[scope] = sorted(existing | set(keys))
    IGNORE_FILE.write_text(json.dumps(raw, indent=2, ensure_ascii=False) + "\n", encoding="utf-8", newline="\n")
    print(f"Ignoring {len(added)} new key(s) under {scope!r}: {', '.join(added) or '(none new)'}")


def load_locale(path: Path) -> dict[str, str]:
    with path.open(encoding="utf-8-sig") as f:
        data = json.load(f)
    return {str(k): str(v) for k, v in data.items()}


def locale_paths() -> list[Path]:
    return sorted(path for path in RESOURCES.glob("*.json") if path.stem != SOURCE_LOCALE)


def looks_translatable(value: str) -> bool:
    return any(char.isalpha() for char in value)


def work_for(
    source: dict[str, str],
    data: dict[str, str],
    ignored: set[str],
    missing_only: bool,
) -> tuple[list[str], list[str], int]:
    """Missing keys, still-English keys, and how many the whitelist suppressed."""
    missing = [key for key in source if key not in data]
    if missing_only:
        return missing, [], 0
    verbatim = [
        key
        for key, english in source.items()
        if key in data and data[key] == english and looks_translatable(english)
    ]
    kept = [key for key in verbatim if key not in ignored]
    return missing, kept, len(verbatim) - len(kept)


def format_placeholders(value: str) -> str:
    names = PLACEHOLDER_RE.findall(value)
    return ", ".join(f"{{{name}}}" for name in names) if names else "none"


def emit_summary(rows: list[tuple[str, int, int, int]]) -> None:
    width = max(len(name) for name, _, _, _ in rows)
    print(f"{'locale'.ljust(width)}  missing  english")
    for name, missing, verbatim, _ in rows:
        if missing or verbatim:
            print(f"{name.ljust(width)}  {missing:>7}  {verbatim:>7}")
    total_missing = sum(missing for _, missing, _, _ in rows)
    total_verbatim = sum(verbatim for _, _, verbatim, _ in rows)
    total_ignored = sum(suppressed for _, _, _, suppressed in rows)
    clean = sum(1 for _, missing, verbatim, _ in rows if not missing and not verbatim)
    print()
    print(f"{total_missing} missing key(s), {total_verbatim} still-English value(s) across {len(rows)} locale(s).")
    print(f"{clean} locale(s) need no work.")
    if total_ignored:
        print(f"{total_ignored} value(s) suppressed by {IGNORE_FILE.name}.")
    print("Run this tool with a locale code to list that locale's items with translation context.")


def emit_items(
    locale: str,
    source: dict[str, str],
    keys: list[tuple[str, str]],
    usage: dict[str, dict[str, object]] | None,
    limit: int,
    suppressed: int,
) -> None:
    shown = keys[:limit]
    print(f"# {locale}: {len(keys)} item(s) to translate")
    print(f"# source: frontend/src/Resources/{SOURCE_LOCALE}.json")
    if suppressed:
        print(f"# {suppressed} value(s) intentionally left in English, suppressed by {IGNORE_FILE.name}")
    print()
    for key, reason in shown:
        english = source[key]
        print(f"## {key}  [{reason}]")
        print(f"English: {english}")
        print(f"Placeholders: {format_placeholders(english)}")
        if usage is not None:
            info = usage.get(key, {})
            locations = info.get("locations") or []
            role = locations[0]["role"] if locations else role_for(key, english, "")
            print(f"Role: {role}")
            if locations:
                spots = ", ".join(f"{loc['path']}:{loc['line']}" for loc in locations[:3])
                print(f"Used at: {spots}")
            else:
                print("Used at: not found in scanned source (runtime-only or stale)")
            nearby = sorted(info.get("nearby_buttons") or [])
            if nearby:
                print(f"Nearby buttons: {', '.join(nearby[:8])}")
            siblings = sorted(info.get("nearby") or [])
            if siblings:
                print(f"Nearby strings: {', '.join(siblings[:8])}")
        print()
    if len(keys) > limit:
        print(f"# {len(keys) - limit} more item(s) not shown; raise --max or rerun after translating these.")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("locale", nargs="?", help="Locale code to detail. Omit for a summary of every locale.")
    parser.add_argument("--max", type=int, default=DEFAULT_MAX_ITEMS, help=f"Items to print (default {DEFAULT_MAX_ITEMS}).")
    parser.add_argument("--missing-only", action="store_true", help="Skip values that are present but still English.")
    parser.add_argument("--no-context", action="store_true", help="Skip the source scan that supplies usage context.")
    parser.add_argument("--json", action="store_true", help="Emit JSON instead of text.")
    parser.add_argument(
        "--ignore",
        nargs="+",
        metavar="KEY",
        help="Record keys as intentionally left in English so they stop being reported. "
        "Scoped to the given locale, or to every locale when no locale is given.",
    )
    parser.add_argument("--show-ignored", action="store_true", help="List the suppressed keys and exit.")
    args = parser.parse_args()

    if args.max > MAX_ITEMS_CEILING:
        print(
            f"Refusing --max {args.max}: batches are capped at {MAX_ITEMS_CEILING}.\n"
            "Translate this batch, apply it with tools/i18n_apply.py, then run again — the\n"
            "list shrinks each time. Dumping the whole backlog in one call is what this\n"
            "tool exists to prevent.",
            file=sys.stderr,
        )
        return 2

    source = load_source_strings()
    all_locales = {path.stem: load_locale(path) for path in locale_paths()}

    if args.ignore:
        add_ignored(args.ignore, args.locale or GLOBAL_SCOPE, source)

    ignored_map = load_ignored()
    if args.show_ignored:
        for scope in sorted(ignored_map):
            if args.locale and scope not in (GLOBAL_SCOPE, args.locale):
                continue
            for key in ignored_map[scope]:
                print(f"{scope}\t{key}")
        return 0

    if not args.locale:
        rows = []
        for name, data in all_locales.items():
            missing, verbatim, suppressed = work_for(
                source, data, ignored_for(ignored_map, name), args.missing_only
            )
            rows.append((name, len(missing), len(verbatim), suppressed))
        if args.json:
            print(json.dumps([{"locale": n, "missing": m, "english": e, "ignored": i} for n, m, e, i in rows], indent=2))
        else:
            emit_summary(rows)
        return 0

    if args.locale not in all_locales:
        print(f"Unknown locale {args.locale!r}. Known: {', '.join(sorted(all_locales))}", file=sys.stderr)
        return 1

    missing, verbatim, suppressed = work_for(
        source, all_locales[args.locale], ignored_for(ignored_map, args.locale), args.missing_only
    )
    keys = [(key, "missing") for key in missing]
    if not args.missing_only:
        keys += [(key, "still English") for key in verbatim]

    usage = None if args.no_context else collect_usage(source)

    if args.json:
        payload = []
        for key, reason in keys[: args.max]:
            entry: dict[str, object] = {"key": key, "reason": reason, "english": source[key]}
            if usage is not None:
                locations = usage.get(key, {}).get("locations") or []
                entry["role"] = locations[0]["role"] if locations else role_for(key, source[key], "")
                entry["used_at"] = [f"{loc['path']}:{loc['line']}" for loc in locations[:3]]
                entry["nearby_buttons"] = sorted(usage[key]["nearby_buttons"])[:8]
            payload.append(entry)
        print(
            json.dumps(
                {"locale": args.locale, "total": len(keys), "ignored": suppressed, "items": payload},
                indent=2,
                ensure_ascii=False,
            )
        )
        return 0

    emit_items(args.locale, source, keys, usage, args.max, suppressed)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
