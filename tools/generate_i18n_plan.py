#!/usr/bin/env python3
"""
Generate a Markdown checklist for documenting and translating i18n strings.

The generated file is intentionally separate from runtime locale JSON files.
It uses frontend/src/Resources/en-US.json as the source of truth, scans source
files for translation key usage, and preserves the locale list that existed
before rebuilding translations.
"""

from __future__ import annotations

import argparse
import json
import re
from datetime import date
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
RESOURCES = ROOT / "frontend" / "src" / "Resources"
SOURCE_LOCALE = "en-US"
SOURCE_FILE = RESOURCES / f"{SOURCE_LOCALE}.json"

DEFAULT_TARGET_LOCALES = [
    "af-ZA",
    "ar-SA",
    "bg-BG",
    "ca-ES",
    "cs-CZ",
    "da-DK",
    "de-CH",
    "de-DE",
    "el-GR",
    "en-PT",
    "es-ES",
    "fa-IR",
    "fi-FI",
    "fr-FR",
    "he-IL",
    "hi-IN",
    "hu-HU",
    "id-ID",
    "it-IT",
    "ja-JP",
    "ko-KR",
    "nl-NL",
    "no-NO",
    "pl-PL",
    "pt-BR",
    "pt-PT",
    "ro-RO",
    "ru-RU",
    "sk-SK",
    "sr-SP",
    "sv-SE",
    "th-TH",
    "tr-TR",
    "uk-UA",
    "vi-VN",
    "zh-CN",
    "zh-TW",
]

SOURCE_GLOBS = [
    "frontend/src/**/*.svelte",
    "frontend/src/**/*.ts",
    "frontend/src/**/*.js",
    "internal/**/*.go",
    "cmd/**/*.go",
]

SKIP_PARTS = {"Resources", "bindings", "node_modules", ".git", ".svelte-kit", "dist", "build"}
STRING_LITERAL_RE = re.compile(r"""["']([A-Za-z][A-Za-z0-9_]{2,})["']""")
PLACEHOLDER_RE = re.compile(r"\{([A-Za-z_][A-Za-z0-9_]*)\}")
HTML_RE = re.compile(r"<[A-Za-z][^>]*>|</[A-Za-z][^>]*>|<br\s*/?>", re.IGNORECASE)


def should_scan(path: Path) -> bool:
    return not any(part in SKIP_PARTS for part in path.parts)


def load_source_strings() -> dict[str, str]:
    with SOURCE_FILE.open(encoding="utf-8-sig") as f:
        data = json.load(f)
    if not isinstance(data, dict):
        raise ValueError(f"{SOURCE_FILE} is not a JSON object")
    return {str(k): str(v) for k, v in data.items()}


def target_locales() -> list[str]:
    found = sorted(
        path.stem for path in RESOURCES.glob("*.json") if path.stem != SOURCE_LOCALE
    )
    return found or DEFAULT_TARGET_LOCALES


def iter_source_files() -> list[Path]:
    files: list[Path] = []
    for glob in SOURCE_GLOBS:
        files.extend(path for path in ROOT.glob(glob) if path.is_file() and should_scan(path))
    return sorted(set(files))


def role_for(key: str, source: str, usage_context: str) -> str:
    lower_context = usage_context.lower()
    if "aria-label" in lower_context or key.startswith("Aria_"):
        return "accessibility label"
    if "<button" in lower_context or key.startswith("Button_"):
        return "button label"
    if "use:tooltip" in lower_context or key.startswith("Tooltip_"):
        return "tooltip/help text"
    if "pushToast" in usage_context or key.startswith("Toast_"):
        return "toast notification"
    if key.startswith("Modal_"):
        return "modal text"
    if key.startswith("Settings_Header_"):
        return "settings section heading"
    if key.startswith("Settings_"):
        return "setting label/help text"
    if key.startswith("Command_"):
        return "command palette text"
    if key.startswith("Search_"):
        return "search UI text"
    if key.startswith("Title_"):
        return "window/page title"
    if key.startswith("Tags_"):
        return "tag management text"
    if key.startswith("Context_"):
        return "context menu text"
    if key.startswith("Steam_"):
        return "Steam platform setting/account text"
    if key.startswith("Preview_"):
        return "theme/CSS preview text"
    if key.startswith("Feedback_"):
        return "feedback form text"
    if key.startswith("Stability_"):
        return "post-switch stability prompt text"
    if source.endswith(":"):
        return "label ending with colon"
    return "UI text"


def collect_usage(strings: dict[str, str]) -> dict[str, dict[str, object]]:
    keys = set(strings)
    usage: dict[str, dict[str, object]] = {
        key: {"locations": [], "nearby": set(), "nearby_buttons": set(), "html_usage": False}
        for key in keys
    }

    for path in iter_source_files():
        try:
            lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
        except OSError:
            continue

        rel = path.relative_to(ROOT).as_posix()
        for index, line in enumerate(lines):
            present = [match.group(1) for match in STRING_LITERAL_RE.finditer(line) if match.group(1) in keys]
            if not present:
                continue

            start = max(0, index - 6)
            end = min(len(lines), index + 7)
            context_lines = lines[start:end]
            context = "\n".join(context_lines)
            context_keys = {
                match.group(1)
                for ctx_line in context_lines
                for match in STRING_LITERAL_RE.finditer(ctx_line)
                if match.group(1) in keys
            }

            for key in present:
                entry = usage[key]
                locations = entry["locations"]
                assert isinstance(locations, list)
                locations.append(
                    {
                        "path": rel,
                        "line": index + 1,
                        "role": role_for(key, strings[key], context),
                    }
                )
                nearby = entry["nearby"]
                nearby_buttons = entry["nearby_buttons"]
                assert isinstance(nearby, set)
                assert isinstance(nearby_buttons, set)
                nearby.update(k for k in context_keys if k != key)
                nearby_buttons.update(k for k in context_keys if k != key and k.startswith("Button_"))
                if "{@html" in context or "@html" in line:
                    entry["html_usage"] = True

    return usage


def fenced(value: str) -> str:
    fence = "```"
    while fence in value:
        fence += "`"
    return f"{fence}text\n{value}\n{fence}"


def format_list(values: list[str], empty: str = "none") -> str:
    if not values:
        return empty
    return ", ".join(f"`{value}`" for value in values)


def context_note(key: str, role: str, locations: list[dict[str, object]]) -> str:
    if locations:
        first = locations[0]
        path = first["path"]
        line = first["line"]
        return f"Used as {role} at {path}:{line}; review nearby strings for screen-level context."
    if key.startswith("Installer_"):
        return "Installer-facing text; keep setup/update wording concise and action-oriented."
    if key.startswith("Updater_"):
        return "Updater-facing text; preserve technical update terminology and progress meaning."
    return "No scanned usage found; verify whether this is runtime-only, platform-injected, or stale before translating."


def build_markdown(strings: dict[str, str], usage: dict[str, dict[str, object]], locales: list[str]) -> str:
    out: list[str] = []
    out.append("# TcNo Account Switcher i18n Translation Plan")
    out.append("")
    out.append(f"Generated: {date.today().isoformat()}")
    out.append(f"Source locale: `{SOURCE_LOCALE}`")
    out.append(f"Source file: `frontend/src/Resources/{SOURCE_LOCALE}.json`")
    out.append(f"Source strings: {len(strings)}")
    out.append(f"Target locales: {len(locales)}")
    out.append("")
    out.append("## Target Locales")
    out.append("")
    for locale in locales:
        out.append(f"- [ ] `{locale}`")
    out.append("")
    out.append("## Workflow")
    out.append("")
    out.append("- [ ] Keep `en-US.json` as the source of truth.")
    out.append("- [ ] For each string, document the short function/context before translating.")
    out.append("- [ ] Translate each reviewed string into every target locale in the same pass.")
    out.append("- [ ] Preserve keys, placeholders, HTML tags, and brand/product names unless a locale convention requires otherwise.")
    out.append("- [ ] After locale files are rebuilt, validate JSON parse, key parity, placeholders, and HTML-bearing strings.")
    out.append("")
    out.append("## Strings")
    out.append("")

    for number, (key, source) in enumerate(strings.items(), start=1):
        info = usage[key]
        locations = info["locations"]
        assert isinstance(locations, list)
        nearby = sorted(info["nearby"])
        nearby_buttons = sorted(info["nearby_buttons"])
        placeholders = [f"{{{name}}}" for name in PLACEHOLDER_RE.findall(source)]
        html = bool(HTML_RE.search(source) or info["html_usage"])
        role = locations[0]["role"] if locations else role_for(key, source, "")

        out.append(f"### [ ] {number:03d}. `{key}`")
        out.append("")
        out.append("- English:")
        out.append("")
        out.append(fenced(source))
        out.append("")
        out.append(f"- Inferred role: {role}")
        out.append(f"- Function/context note: {context_note(key, str(role), locations)}")
        out.append(f"- Placeholders: {format_list(placeholders)}")
        out.append(f"- Contains HTML or HTML-rendered usage: {'yes' if html else 'no'}")
        out.append(f"- Nearby buttons: {format_list(nearby_buttons)}")
        out.append(f"- Nearby strings: {format_list(nearby[:12])}")
        out.append("- Usage:")
        if locations:
            for loc in locations[:8]:
                out.append(f"  - `{loc['path']}:{loc['line']}` ({loc['role']})")
            if len(locations) > 8:
                out.append(f"  - plus {len(locations) - 8} more usage(s)")
        else:
            out.append("  - not found in scanned source; verify whether this is runtime-only or stale")
        out.append("- Checklist:")
        out.append("  - [ ] Context documented")
        out.append("  - [ ] Translated into all target locales")
        out.append("  - [ ] Placeholder/HTML validation passed")
        out.append("")

    return "\n".join(out)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--output",
        type=Path,
        default=Path.home() / "Documents" / "Codex" / "tcno-acc-switcher-i18n-plan.md",
        help="Markdown plan path to write.",
    )
    args = parser.parse_args()

    strings = load_source_strings()
    locales = target_locales()
    usage = collect_usage(strings)
    markdown = build_markdown(strings, usage, locales)

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(markdown + "\n", encoding="utf-8", newline="\n")
    print(f"Wrote {args.output}")
    print(f"Source strings: {len(strings)}")
    print(f"Target locales: {len(locales)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
