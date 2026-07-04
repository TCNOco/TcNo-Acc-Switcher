#!/usr/bin/env python3
"""Build OPEN_SOURCE_LICENSES.txt from pinned upstream source packages.

The collector is intentionally evidence-first:
- Go source comes from exact module versions via the Go module cache/download.
- npm source comes from exact package versions via npm pack, with a pnpm
  node_modules fallback for offline runs.
- License/notice texts are grouped only by raw byte SHA-256.
"""

from __future__ import annotations

import argparse
import datetime as dt
import glob
import hashlib
import json
import os
import re
import shutil
import stat
import subprocess
import sys
import tarfile
import textwrap
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


LICENSE_FILE_PREFIXES = (
    "LICENSE",
    "LICENCE",
    "COPYING",
    "NOTICE",
    "PATENTS",
)

NPM_CACHE_METADATA_FILE = ".license-source.json"

QsortBSD = b"""/*-
 * Copyright 2023 konoui
 * Copyright (c) 1991, 1993
 *	The Regents of the University of California.  All rights reserved.
 *
 * This code is derived from software contributed to Berkeley by
 * Ronnie Kon at Mindcraft Inc., Kevin Lew and Elmer Yglesias.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 * 4. Neither the name of the University nor the names of its contributors
 *    may be used to endorse or promote products derived from this software
 *    without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE REGENTS AND CONTRIBUTORS ``AS IS'' AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 * IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED.  IN NO EVENT SHALL THE REGENTS OR CONTRIBUTORS BE LIABLE
 * FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 * DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
 * OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
 * LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
 * OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
 * SUCH DAMAGE.
 */
"""

GO_LOCALEREADER_MIT = b"""The MIT License (MIT)

Copyright (c) 2022 Yasuhiro Matsumoto

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
"""

MANUAL_LICENSE_OVERRIDES = (
    {
        "ecosystem": "Go module",
        "name": "github.com/konoui/go-qsort",
        "version": "v0.1.0",
        "text": QsortBSD,
        "source": "https://raw.githubusercontent.com/konoui/go-qsort/main/qsort.go",
        "note": (
            "Manual override: current upstream qsort.go file header. "
            "The cached exact module source for v0.1.0 has no root license file and no file header."
        ),
    },
    {
        "ecosystem": "Go module",
        "name": "github.com/mattn/go-localereader",
        "version": "v0.0.1",
        "text": GO_LOCALEREADER_MIT,
        "source": "https://raw.githubusercontent.com/mattn/go-localereader/2491eb6c1c75720122ef321ed7acc3a8d9de95b1/LICENSE",
        "note": (
            "Manual override: upstream LICENSE file from commit "
            "2491eb6c1c75720122ef321ed7acc3a8d9de95b1. "
            "The cached exact module source for v0.0.1 has no root license file."
        ),
    },
)

CANONICAL_MIT_TEXT = b"""MIT License

Copyright <YEAR> <COPYRIGHT HOLDER>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
"""

CANONICAL_MIT_TEXT_SOURCE = "Open Source Initiative MIT License text (SPDX MIT): https://opensource.org/license/mit"

TITLE_PATTERNS = (
    re.compile(r"^(the\s+)?mit license(?:\s*\(.+\))?$", re.IGNORECASE),
    re.compile(r"^apache license$", re.IGNORECASE),
    re.compile(r"^mozilla public license(?:\s+version\s+.+)?$", re.IGNORECASE),
    re.compile(r"^gnu (lesser|library|affero )?general public license$", re.IGNORECASE),
    re.compile(r"^bsd(?: zero clause| [0-9]-clause)? license$", re.IGNORECASE),
    re.compile(r"^the isc license$", re.IGNORECASE),
    re.compile(r"^isc license$", re.IGNORECASE),
    re.compile(r"^sil open font license$", re.IGNORECASE),
    re.compile(r"^boost software license(?:\s+.+)?$", re.IGNORECASE),
    re.compile(r"^the unlicense$", re.IGNORECASE),
    re.compile(r"^cc0\s+.+$", re.IGNORECASE),
    re.compile(r"^blue oak model license(?:\s+.+)?$", re.IGNORECASE),
    re.compile(r"^zlib license$", re.IGNORECASE),
    re.compile(r"^creative commons .+$", re.IGNORECASE),
    re.compile(r"^additional ip rights grant(?:\s+\(.+\))?$", re.IGNORECASE),
)

BUNDLED_ASSET_LICENSES = (
    {
        "name": "Montserrat font",
        "version": "Variable font file bundled in frontend/public",
        "license_file": "frontend/public/Montserrat-OFL.txt",
        "evidence": "bundled asset license file",
    },
)


@dataclass(frozen=True)
class Component:
    ecosystem: str
    name: str
    version: str
    relationship: str
    source_dir: Path | None
    evidence: str
    source_cache: Path | None = None
    source_url: str | None = None
    missing_reason: str | None = None
    notice_note: str | None = None

    @property
    def label(self) -> str:
        if self.version:
            return f"{self.ecosystem}: {self.name} {self.version}"
        return f"{self.ecosystem}: {self.name}"


@dataclass
class LicenseUse:
    component: Component
    license_path: Path


@dataclass
class LicenseSection:
    sha256: str
    size: int
    text: bytes
    section_note: str | None = None
    uses: list[LicenseUse] = field(default_factory=list)


def repo_root_from_script() -> Path:
    return Path(__file__).resolve().parents[1]


def run(
    args: list[str],
    cwd: Path,
    env: dict[str, str] | None = None,
    allow_failure: bool = False,
) -> subprocess.CompletedProcess[str]:
    merged_env = os.environ.copy()
    if env:
        merged_env.update(env)
    proc = subprocess.run(
        args,
        cwd=str(cwd),
        env=merged_env,
        text=True,
        encoding="utf-8",
        errors="replace",
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if proc.returncode != 0 and not allow_failure:
        command = " ".join(args)
        raise RuntimeError(
            f"Command failed ({proc.returncode}): {command}\n"
            f"stdout:\n{proc.stdout}\n"
            f"stderr:\n{proc.stderr}"
        )
    return proc


def run_bytes(
    args: list[str],
    cwd: Path,
    allow_failure: bool = False,
) -> subprocess.CompletedProcess[bytes]:
    proc = subprocess.run(
        args,
        cwd=str(cwd),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if proc.returncode != 0 and not allow_failure:
        command = " ".join(args)
        stderr = proc.stderr.decode("utf-8", errors="replace")
        stdout = proc.stdout.decode("utf-8", errors="replace")
        raise RuntimeError(
            f"Command failed ({proc.returncode}): {command}\n"
            f"stdout:\n{stdout}\n"
            f"stderr:\n{stderr}"
        )
    return proc


def command_path(*names: str) -> str:
    for name in names:
        found = shutil.which(name)
        if found:
            return found
    return names[0]


def parse_concatenated_json_objects(text: str) -> list[dict[str, Any]]:
    decoder = json.JSONDecoder()
    offset = 0
    objects: list[dict[str, Any]] = []
    while offset < len(text):
        while offset < len(text) and text[offset].isspace():
            offset += 1
        if offset >= len(text):
            break
        obj, offset = decoder.raw_decode(text, offset)
        if isinstance(obj, dict):
            objects.append(obj)
    return objects


def safe_id(ecosystem: str, name: str, version: str) -> str:
    key = f"{ecosystem}:{name}@{version}".encode("utf-8")
    digest = hashlib.sha256(key).hexdigest()[:16]
    readable = re.sub(r"[^A-Za-z0-9._-]+", "_", f"{name}@{version}").strip("_")
    readable = readable[:90] or "component"
    return f"{digest}-{readable}"


def short_cache_id(*parts: str) -> str:
    key = "\0".join(parts).encode("utf-8")
    return hashlib.sha256(key).hexdigest()[:24]


def repo_relative(path: Path, repo_root: Path) -> str:
    try:
        return path.resolve().relative_to(repo_root.resolve()).as_posix()
    except ValueError:
        return str(path)


def resolve_repo_path(repo_root: Path, value: str) -> Path:
    path = Path(value)
    if path.is_absolute():
        return path
    return repo_root / path


def ensure_inside(path: Path, parent: Path) -> None:
    resolved = path.resolve()
    parent_resolved = parent.resolve()
    if resolved == parent_resolved:
        raise ValueError(f"Refusing to operate on cache root: {path}")
    if parent_resolved not in resolved.parents:
        raise ValueError(f"Path escapes cache root: {path}")


def reset_dir(path: Path, cache_root: Path) -> None:
    ensure_inside(path, cache_root)
    if path.exists():
        remove_tree(path)
    path.mkdir(parents=True, exist_ok=True)


def copy_source_tree(src: Path, dest: Path, cache_root: Path, refresh: bool) -> None:
    ensure_inside(dest, cache_root)
    if dest.exists() and not refresh:
        return
    if dest.exists():
        remove_tree(dest)

    def ignore(_: str, names: list[str]) -> set[str]:
        return {name for name in names if name in {".git", "node_modules"}}

    shutil.copytree(src, dest, ignore=ignore)


def remove_tree(path: Path) -> None:
    def make_writable(function: Any, target: str, _: Any) -> None:
        os.chmod(target, stat.S_IWRITE)
        function(target)

    shutil.rmtree(path, onexc=make_writable)


def is_license_filename(path: Path) -> bool:
    name = path.name.upper()
    if name in {"UNLICENSE", "UNLICENCE"}:
        return True
    return any(name.startswith(prefix) for prefix in LICENSE_FILE_PREFIXES)


def root_license_files(source_dir: Path) -> list[Path]:
    if not source_dir.exists() or not source_dir.is_dir():
        return []
    files = [path for path in source_dir.iterdir() if path.is_file() and is_license_filename(path)]
    return sorted(files, key=lambda path: path.name.casefold())


def git_remote_url(repo_root: Path) -> str | None:
    proc = run(["git", "config", "--get", "remote.origin.url"], repo_root, allow_failure=True)
    value = proc.stdout.strip()
    return value or None


def go_modules(repo_root: Path) -> list[dict[str, Any]]:
    proc = run(["go", "list", "-m", "-json", "all"], repo_root)
    modules = parse_concatenated_json_objects(proc.stdout)
    return [module for module in modules if not module.get("Main")]


def go_module_cache_dir(repo_root: Path, module: dict[str, Any], no_fetch: bool) -> tuple[Path | None, str | None]:
    path = module["Path"]
    version = module.get("Version", "")
    replacement = module.get("Replace") or {}
    if replacement.get("Dir"):
        return (repo_root / replacement["Dir"]).resolve(), "local replace directory"
    if replacement.get("Path") and replacement.get("Version"):
        path = replacement["Path"]
        version = replacement["Version"]
    if module.get("Dir"):
        return Path(module["Dir"]), "go list module directory"
    if no_fetch:
        return None, "not downloaded locally"

    target = f"{path}@{version}" if version else path
    proc = run(["go", "mod", "download", "-json", target], repo_root)
    info = json.loads(proc.stdout)
    if info.get("Error"):
        return None, info["Error"]
    if info.get("Dir"):
        return Path(info["Dir"]), "go mod download"
    return None, "go mod download did not return a source directory"


def collect_go_components(repo_root: Path, cache_root: Path, refresh: bool, no_fetch: bool) -> list[Component]:
    components: list[Component] = []
    go_cache = cache_root / "go"
    go_cache.mkdir(parents=True, exist_ok=True)
    for module in go_modules(repo_root):
        original_name = module["Path"]
        original_version = module.get("Version", "")
        replacement = module.get("Replace") or {}
        name = original_name
        version = original_version
        notice_note = None
        if replacement.get("Path") and replacement.get("Version") and not replacement.get("Dir"):
            name = replacement["Path"]
            version = replacement["Version"]
            notice_note = f"replaces {original_version} via go.mod."
        relationship = "transitive" if module.get("Indirect") else "direct"
        cache_dir = go_cache / safe_id("go", name, version)
        missing_reason = None
        source_dir: Path | None = None
        evidence = "cached Go module source"
        if cache_dir.exists() and not refresh:
            source_dir = cache_dir
        else:
            src, evidence = go_module_cache_dir(repo_root, module, no_fetch)
            if src and src.exists():
                copy_source_tree(src, cache_dir, cache_root, refresh)
                source_dir = cache_dir
            else:
                missing_reason = evidence or "source directory not found"
        components.append(
            Component(
                ecosystem="Go module",
                name=name,
                version=version,
                relationship=relationship,
                source_dir=source_dir,
                source_cache=cache_dir if source_dir else None,
                evidence=evidence or "go list -m -json all",
                missing_reason=missing_reason,
                notice_note=notice_note,
            )
        )
    return components


def cached_npm_package_source(cache_dir: Path) -> Path | None:
    package_dir = cache_dir / "package"
    if package_dir.exists():
        return package_dir
    if (cache_dir / "package.json").exists():
        return cache_dir
    children = [child for child in cache_dir.iterdir() if child.is_dir()] if cache_dir.exists() else []
    if len(children) == 1 and (children[0] / "package.json").exists():
        return children[0]
    return None


def read_npm_cache_metadata(cache_dir: Path) -> dict[str, Any]:
    path = cache_dir / NPM_CACHE_METADATA_FILE
    if not path.exists():
        return {}
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return {}


def write_npm_cache_metadata(cache_dir: Path, metadata: dict[str, Any]) -> None:
    cache_dir.mkdir(parents=True, exist_ok=True)
    (cache_dir / NPM_CACHE_METADATA_FILE).write_text(json.dumps(metadata, indent=2), encoding="utf-8")


def parse_package_json(frontend_dir: Path) -> tuple[set[str], set[str]]:
    data = json.loads((frontend_dir / "package.json").read_text(encoding="utf-8"))
    direct = set(data.get("dependencies", {}).keys())
    dev = set(data.get("devDependencies", {}).keys())
    return direct, dev


def parse_pnpm_lock_packages(lock_path: Path) -> list[tuple[str, str]]:
    packages: list[tuple[str, str]] = []
    in_packages = False
    for raw_line in lock_path.read_text(encoding="utf-8").splitlines():
        if raw_line == "packages:":
            in_packages = True
            continue
        if in_packages and raw_line and not raw_line.startswith(" "):
            break
        if not in_packages:
            continue
        match = re.match(r"^  (?! )['\"]?(.+?)['\"]?:\s*$", raw_line)
        if not match:
            continue
        key = match.group(1)
        parsed = parse_pnpm_package_key(key)
        if parsed:
            packages.append(parsed)
    return sorted(set(packages), key=lambda item: (item[0].casefold(), item[1]))


def parse_pnpm_package_key(key: str) -> tuple[str, str] | None:
    key = key.split("(", 1)[0]
    if key.startswith("/"):
        key = key[1:]
    if key.startswith("@"):
        at_index = key.rfind("@")
        if at_index <= 0:
            return None
        return key[:at_index], key[at_index + 1 :]
    if "@" not in key:
        return None
    name, version = key.rsplit("@", 1)
    if not name or not version:
        return None
    return name, version


def npm_pack(
    repo_root: Path,
    cache_root: Path,
    package_name: str,
    version: str,
    package_cache: Path,
) -> Path:
    tarballs = cache_root / "npm-tarballs"
    tarballs.mkdir(parents=True, exist_ok=True)
    spec = f"{package_name}@{version}"
    npm = command_path("npm.cmd", "npm")
    proc = run([npm, "pack", spec, "--pack-destination", str(tarballs), "--json"], repo_root)
    tarball_path: Path | None = None
    try:
        packed = json.loads(proc.stdout)
        if packed and packed[0].get("filename"):
            filename = packed[0]["filename"]
            tarball_path = Path(filename)
            if not tarball_path.is_absolute():
                tarball_path = tarballs / tarball_path
    except json.JSONDecodeError:
        matches = sorted(tarballs.glob("*.tgz"), key=lambda item: item.stat().st_mtime, reverse=True)
        if matches:
            tarball_path = matches[0]
    if not tarball_path or not tarball_path.exists():
        raise RuntimeError(f"npm pack did not produce a tarball for {spec}")

    reset_dir(package_cache, cache_root)
    with tarfile.open(tarball_path, "r:gz") as archive:
        safe_extract_tar(archive, package_cache)
    package_dir = package_cache / "package"
    if package_dir.exists():
        return package_dir
    children = [child for child in package_cache.iterdir() if child.is_dir()]
    if len(children) == 1:
        return children[0]
    return package_cache


def safe_extract_tar(archive: tarfile.TarFile, target: Path) -> None:
    target_resolved = target.resolve()
    for member in archive.getmembers():
        member_path = (target / member.name).resolve()
        if member_path != target_resolved and target_resolved not in member_path.parents:
            raise RuntimeError(f"Refusing unsafe tar path: {member.name}")
        if member.issym() or member.islnk():
            continue
        archive.extract(member, target)


def find_installed_pnpm_package(frontend_dir: Path, package_name: str, version: str) -> Path | None:
    pnpm_root = frontend_dir / "node_modules" / ".pnpm"
    if not pnpm_root.exists():
        return None
    pnpm_key = package_name.replace("/", "+")
    pattern = str(pnpm_root / f"{pnpm_key}@{version}*" / "node_modules")
    for node_modules in glob.glob(pattern):
        candidate = Path(node_modules)
        for part in package_name.split("/"):
            candidate = candidate / part
        if candidate.exists():
            return candidate
    return None


def read_package_json(source_dir: Path | None) -> dict[str, Any] | None:
    if not source_dir:
        return None
    package_json = source_dir / "package.json"
    if not package_json.exists():
        return None
    try:
        return json.loads(package_json.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return None


def normalize_git_url(repository: Any) -> str | None:
    if isinstance(repository, dict):
        url = repository.get("url")
    else:
        url = repository
    if not isinstance(url, str) or not url:
        return None
    url = url.strip()
    if url.startswith("git+"):
        url = url[4:]
    if url.startswith("git://github.com/"):
        url = "https://github.com/" + url[len("git://github.com/") :]
    if url.startswith("github:"):
        url = "https://github.com/" + url[len("github:") :]
    if url.startswith("http://github.com/"):
        url = "https://github.com/" + url[len("http://github.com/") :]
    return url


def npm_view_git_head(repo_root: Path, package_name: str, version: str, no_fetch: bool) -> str | None:
    if no_fetch:
        return None
    npm = command_path("npm.cmd", "npm")
    proc = run([npm, "view", f"{package_name}@{version}", "gitHead", "--json"], repo_root, allow_failure=True)
    if proc.returncode != 0:
        return None
    value = proc.stdout.strip()
    if not value or value == "undefined":
        return None
    try:
        parsed = json.loads(value)
        if isinstance(parsed, str) and parsed:
            return parsed
    except json.JSONDecodeError:
        pass
    return value.strip('"') or None


def checkout_candidates(package_name: str, version: str, git_head: str | None) -> list[str]:
    unscoped = package_name.rsplit("/", 1)[-1]
    candidates = [
        git_head or "",
        f"v{version}",
        version,
        f"{package_name}@{version}",
        f"{unscoped}@{version}",
    ]
    result: list[str] = []
    for candidate in candidates:
        if candidate and candidate not in result:
            result.append(candidate)
    return result


def git_root_license_names(repo_cache: Path, git: str, candidate: str) -> list[str]:
    proc = run([git, "-C", str(repo_cache), "ls-tree", "--name-only", candidate], repo_cache, allow_failure=True)
    if proc.returncode != 0:
        return []
    names: list[str] = []
    for line in proc.stdout.splitlines():
        path = Path(line.strip())
        if path.name and len(path.parts) == 1 and is_license_filename(path):
            names.append(path.name)
    return sorted(set(names), key=str.casefold)


def materialize_git_license_files(repo_cache: Path, git: str, candidate: str) -> bool:
    license_names = git_root_license_names(repo_cache, git, candidate)
    if not license_names:
        return False
    for existing in root_license_files(repo_cache):
        existing.unlink()
    for name in license_names:
        proc = run_bytes([git, "-C", str(repo_cache), "show", f"{candidate}:{name}"], repo_cache, allow_failure=True)
        if proc.returncode != 0:
            continue
        target = (repo_cache / name).resolve()
        if repo_cache.resolve() not in target.parents:
            raise RuntimeError(f"Refusing unsafe git path: {name}")
        target.write_bytes(proc.stdout)
    return bool(root_license_files(repo_cache))


def clone_repository_license_source(
    repo_root: Path,
    cache_root: Path,
    package_name: str,
    version: str,
    package_data: dict[str, Any],
    refresh: bool,
    no_fetch: bool,
) -> tuple[Path | None, str | None]:
    url = normalize_git_url(package_data.get("repository"))
    if not url:
        return None, "package.json has no repository URL"

    git_head = npm_view_git_head(repo_root, package_name, version, no_fetch)
    checkout_key = git_head or version
    repo_cache = cache_root / "npm-repos" / short_cache_id("npm-repo", url, checkout_key)
    if repo_cache.exists() and not refresh:
        if root_license_files(repo_cache):
            return repo_cache, f"repository license fallback from package.json: {url} at {checkout_key}"
        if no_fetch:
            return None, f"repository fallback cached without root license/notice file: {url} at {checkout_key}"
    if no_fetch:
        return None, "repository fallback not cached"
    if repo_cache.exists():
        remove_tree(repo_cache)
    repo_cache.parent.mkdir(parents=True, exist_ok=True)

    git = command_path("git.exe", "git")
    clone = run([git, "clone", "--no-checkout", "--filter=blob:none", url, str(repo_cache)], repo_root, allow_failure=True)
    if clone.returncode != 0:
        return None, f"repository clone failed: {clone.stderr.strip() or clone.stdout.strip()}"

    for candidate in checkout_candidates(package_name, version, git_head):
        if materialize_git_license_files(repo_cache, git, candidate):
            return repo_cache, f"repository license fallback from package.json: {url} at {candidate}"

    return None, "repository clone succeeded, but no exact gitHead/version tag contains a root license/notice file"


def collect_npm_components(repo_root: Path, cache_root: Path, refresh: bool, no_fetch: bool) -> list[Component]:
    frontend_dir = repo_root / "frontend"
    lock_path = frontend_dir / "pnpm-lock.yaml"
    if not lock_path.exists():
        return []

    direct, dev = parse_package_json(frontend_dir)
    packages = parse_pnpm_lock_packages(lock_path)
    npm_cache = cache_root / "npm"
    npm_cache.mkdir(parents=True, exist_ok=True)
    components: list[Component] = []
    for name, version in packages:
        relationship = "direct" if name in direct else "dev" if name in dev else "transitive"
        cache_dir = npm_cache / safe_id("npm", name, version)
        package_cache_dir = cache_dir
        source_dir: Path | None = None
        evidence = "frontend/pnpm-lock.yaml"
        missing_reason: str | None = None

        cached_package = cached_npm_package_source(cache_dir)
        if cached_package and not refresh:
            source_dir = cached_package
            evidence = "cached npm package source"
        elif not no_fetch:
            try:
                source_dir = npm_pack(repo_root, cache_root, name, version, cache_dir)
                evidence = "npm pack exact package version"
            except Exception as exc:  # keep going and report the unresolved package
                missing_reason = f"npm pack failed: {exc}"
        else:
            installed = find_installed_pnpm_package(frontend_dir, name, version)
            if installed:
                source_dir = installed
                evidence = "installed pnpm package source"
            else:
                missing_reason = "not present in cache or frontend/node_modules/.pnpm"

        if source_dir and not root_license_files(source_dir):
            metadata = read_npm_cache_metadata(package_cache_dir)
            if no_fetch and not refresh and metadata.get("repository_fallback_missing_reason"):
                missing_reason = metadata["repository_fallback_missing_reason"]
            cached_fallback = metadata.get("repository_fallback_source_dir")
            if isinstance(cached_fallback, str) and not refresh:
                cached_fallback_dir = resolve_repo_path(repo_root, cached_fallback)
                if root_license_files(cached_fallback_dir):
                    source_dir = cached_fallback_dir
                    cache_dir = cached_fallback_dir
                    evidence = metadata.get("repository_fallback_evidence") or "cached repository license fallback"
                    missing_reason = None

            package_data = read_package_json(source_dir)
            if package_data and not root_license_files(source_dir) and not (no_fetch and missing_reason):
                fallback_dir, fallback_evidence = clone_repository_license_source(
                    repo_root,
                    cache_root,
                    name,
                    version,
                    package_data,
                    refresh,
                    no_fetch,
                )
                if fallback_dir and root_license_files(fallback_dir):
                    source_dir = fallback_dir
                    cache_dir = fallback_dir
                    evidence = fallback_evidence or evidence
                    missing_reason = None
                    write_npm_cache_metadata(
                        package_cache_dir,
                        {
                            "name": name,
                            "version": version,
                            "repository_fallback_source_dir": repo_relative(fallback_dir, repo_root),
                            "repository_fallback_evidence": evidence,
                        },
                    )
                elif fallback_dir:
                    missing_reason = f"repository checkout has no root license/notice file: {fallback_evidence}"
                    write_npm_cache_metadata(
                        package_cache_dir,
                        {
                            "name": name,
                            "version": version,
                            "repository_fallback_missing_reason": missing_reason,
                        },
                    )
                elif fallback_evidence:
                    missing_reason = fallback_evidence
                    if not no_fetch:
                        write_npm_cache_metadata(
                            package_cache_dir,
                            {
                                "name": name,
                                "version": version,
                                "repository_fallback_missing_reason": missing_reason,
                            },
                        )

        components.append(
            Component(
                ecosystem="npm package",
                name=name,
                version=version,
                relationship=relationship,
                source_dir=source_dir,
                source_cache=cache_dir if cache_dir.exists() else None,
                evidence=evidence,
                missing_reason=missing_reason,
            )
        )
    return components


def collect_asset_components(repo_root: Path) -> list[Component]:
    components: list[Component] = []
    for asset in BUNDLED_ASSET_LICENSES:
        license_path = repo_root / asset["license_file"]
        if license_path.exists():
            source_dir = license_path.parent
            missing_reason = None
        else:
            source_dir = None
            missing_reason = f"missing configured asset license file: {asset['license_file']}"
        components.append(
            Component(
                ecosystem="bundled asset",
                name=asset["name"],
                version=asset["version"],
                relationship="bundled",
                source_dir=source_dir,
                evidence=asset["evidence"],
                missing_reason=missing_reason,
            )
        )
    return components


def component_license_files(component: Component, repo_root: Path) -> list[Path]:
    if component.ecosystem == "bundled asset":
        for asset in BUNDLED_ASSET_LICENSES:
            if asset["name"] == component.name:
                path = repo_root / asset["license_file"]
                return [path] if path.exists() else []
    if not component.source_dir:
        return []
    return root_license_files(component.source_dir)


def npm_mit_metadata_fallback(component: Component, repo_root: Path) -> tuple[bytes, Path, str] | None:
    if component.ecosystem != "npm package" or not component.source_dir:
        return None
    package_json = component.source_dir / "package.json"
    package_data = read_package_json(component.source_dir)
    if not package_data or package_data.get("license") != "MIT":
        return None
    note = (
        f'{relative_display(package_json, repo_root)} `license` field declares `MIT`.'
    )
    return CANONICAL_MIT_TEXT, package_json, note


def manual_license_override(
    component: Component,
    repo_root: Path,
    cache_root: Path,
) -> tuple[bytes, Path, str] | None:
    for override in MANUAL_LICENSE_OVERRIDES:
        if (
            component.ecosystem == override["ecosystem"]
            and component.name == override["name"]
            and component.version == override["version"]
        ):
            override_dir = cache_root / "manual-overrides"
            override_dir.mkdir(parents=True, exist_ok=True)
            source_file = override_dir / f"{safe_id(component.ecosystem, component.name, component.version)}.txt"
            text = override["text"]
            source_file.write_bytes(text)
            note = f"{override['note']} Source: {override['source']}"
            return text, source_file, note
    return None


def build_sections(
    repo_root: Path,
    components: list[Component],
    cache_root: Path,
) -> tuple[dict[str, LicenseSection], list[Component]]:
    sections: dict[str, LicenseSection] = {}
    missing: list[Component] = []
    for component in components:
        files = component_license_files(component, repo_root)
        if not files:
            manual_override = manual_license_override(component, repo_root, cache_root)
            if manual_override:
                text, evidence_path, note = manual_override
                sha = hashlib.sha256(text).hexdigest()
                section = sections.get(sha)
                if not section:
                    section = LicenseSection(sha256=sha, size=len(text), text=text)
                    sections[sha] = section
                section.uses.append(
                    LicenseUse(
                        component=Component(
                            ecosystem=component.ecosystem,
                            name=component.name,
                            version=component.version,
                            relationship=component.relationship,
                            source_dir=component.source_dir,
                            source_cache=component.source_cache,
                            evidence="manual license override",
                            source_url=component.source_url,
                            missing_reason=None,
                            notice_note=note,
                        ),
                        license_path=evidence_path,
                    )
                )
                continue
            fallback = npm_mit_metadata_fallback(component, repo_root)
            if fallback:
                text, evidence_path, note = fallback
                sha = hashlib.sha256(text).hexdigest()
                section = sections.get(sha)
                if not section:
                    section = LicenseSection(
                        sha256=sha,
                        size=len(text),
                        text=text,
                        section_note=f"License text source: {CANONICAL_MIT_TEXT_SOURCE}.",
                    )
                    sections[sha] = section
                elif section.section_note is None:
                    section.section_note = f"License text source: {CANONICAL_MIT_TEXT_SOURCE}."
                section.uses.append(
                    LicenseUse(
                        component=Component(
                            ecosystem=component.ecosystem,
                            name=component.name,
                            version=component.version,
                            relationship=component.relationship,
                            source_dir=component.source_dir,
                            source_cache=component.source_cache,
                            evidence="exact npm package package.json license field",
                            source_url=component.source_url,
                            missing_reason=None,
                            notice_note=note,
                        ),
                        license_path=evidence_path,
                    )
                )
                continue
            reason = component.missing_reason or "no root license/notice file found"
            missing.append(
                Component(
                    ecosystem=component.ecosystem,
                    name=component.name,
                    version=component.version,
                    relationship=component.relationship,
                    source_dir=component.source_dir,
                    source_cache=component.source_cache,
                    evidence=component.evidence,
                    source_url=component.source_url,
                    missing_reason=reason,
                    notice_note=component.notice_note,
                )
            )
            continue
        for license_file in files:
            text = license_file.read_bytes()
            sha = hashlib.sha256(text).hexdigest()
            section = sections.get(sha)
            if not section:
                section = LicenseSection(sha256=sha, size=len(text), text=text)
                sections[sha] = section
            section.uses.append(LicenseUse(component=component, license_path=license_file))

    text_dir = cache_root / "license-texts"
    text_dir.mkdir(parents=True, exist_ok=True)
    for section in sections.values():
        (text_dir / f"{section.sha256}.txt").write_bytes(section.text)

    return sections, missing


def relative_display(path: Path, repo_root: Path) -> str:
    return repo_relative(path, repo_root)


def stripped_line(line: bytes) -> str:
    return line.decode("utf-8", errors="replace").strip()


def is_title_underline(value: str) -> bool:
    return bool(value) and set(value) <= {"=", "-", "~"} and len(value) >= 3


def is_license_title(value: str) -> bool:
    if not value or value.endswith(":"):
        return False
    value = value.strip("#").strip()
    return any(pattern.match(value) for pattern in TITLE_PATTERNS)


def section_title_and_body(text: bytes) -> tuple[str | None, bytes]:
    lines = text.splitlines(keepends=True)
    first = next((index for index, line in enumerate(lines) if stripped_line(line)), None)
    if first is None:
        return None, text

    first_value = stripped_line(lines[first]).strip("#").strip()
    if not is_license_title(first_value):
        return None, text

    title_parts = [first_value]
    remove_until = first + 1
    while remove_until < len(lines) and not stripped_line(lines[remove_until]):
        remove_until += 1

    if remove_until < len(lines):
        next_value = stripped_line(lines[remove_until])
        if is_title_underline(next_value):
            remove_until += 1
        elif first_value.casefold() in {
            "apache license",
            "gnu general public license",
            "gnu lesser general public license",
            "gnu affero general public license",
            "sil open font license",
            "boost software license",
        } and next_value.casefold().startswith("version "):
            title_parts.append(next_value)
            remove_until += 1

    while remove_until < len(lines) and not stripped_line(lines[remove_until]):
        remove_until += 1

    return " ".join(title_parts), b"".join(lines[remove_until:])


def is_component_license_heading(value: str) -> bool:
    lowered = value.casefold()
    return lowered.endswith(" license:") or lowered.endswith(" licenses:")


def normalize_embedded_notice_titles(text: bytes) -> bytes:
    lines = text.splitlines(keepends=True)
    for index, line in enumerate(lines):
        heading = stripped_line(line)
        if not is_component_license_heading(heading):
            continue
        title_index = index + 1
        while title_index < len(lines) and not stripped_line(lines[title_index]):
            title_index += 1
        if title_index >= len(lines):
            continue
        title, remainder = section_title_and_body(b"".join(lines[title_index:]))
        if not title:
            continue
        prefix = b"".join(lines[:index])
        normalized = prefix
        normalized += line
        if not line.endswith(b"\n"):
            normalized += b"\n"
        normalized += f"{title}\n".encode("utf-8")
        normalized += b"\n"
        normalized += normalize_embedded_notice_titles(remainder)
        return normalized
    return text


def component_notice_name(component: Component) -> str:
    return component.name


def human_join(items: list[str]) -> str:
    unique = sorted(set(items), key=str.casefold)
    if not unique:
        return "Unknown component"
    if len(unique) == 1:
        return unique[0]
    if len(unique) == 2:
        return f"{unique[0]} and {unique[1]}"
    return f"{', '.join(unique[:-1])} and {unique[-1]}"


def write_wrapped_line(handle: Any, value: str, width: int = 100) -> None:
    wrapped = textwrap.wrap(value, width=width, break_long_words=False, break_on_hyphens=False)
    if not wrapped:
        handle.write(b"\n")
        return
    for line in wrapped:
        handle.write(f"{line}\n".encode("utf-8"))


def render_output(
    repo_root: Path,
    cache_root: Path,
    output_path: Path,
    components: list[Component],
    sections: dict[str, LicenseSection],
    missing: list[Component],
) -> None:
    generated_at = dt.datetime.now(dt.UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    by_section = sorted(sections.values(), key=lambda item: item.sha256)
    lines: list[str] = [
        "Open Source Licenses",
        "====================",
        "",
        f"Generated: {generated_at}",
        "Generator: tools/collect_open_source_licenses.py",
        f"Source cache: {relative_display(cache_root, repo_root)}",
        "",
        "Method",
        "------",
        "This file is generated from pinned upstream source packages.",
        "License and notice texts are not classified.",
        "Readable section titles come only from title lines already present in the source text.",
        "Sections are deduplicated only when the raw license file bytes have the same SHA-256 hash.",
        "If a single byte differs, the text is emitted as a separate section.",
        "If an exact npm package declares MIT in package.json but ships no license text, a separate metadata-derived MIT section is emitted.",
        "Manual overrides are emitted only when this script names the external source used for the text.",
        "If an npm package has no root license/notice file, the package repository is checked at gitHead or version tags.",
        "Repository fallbacks materialize only root license/notice files into the ignored cache.",
        "Go module version replacements from go.mod are treated as the effective source version.",
        "SHA-256 hashes, source files and cache paths are written to .cache/license-sources/license-memory.json.",
        "",
        "Inputs",
        "------",
        "- Go modules: go list -m -json all, then cached/downloaded module source",
        "- npm packages: frontend/pnpm-lock.yaml, then cached/npm-packed package source",
        "- Bundled assets: configured license files in this script",
        "",
        "Summary",
        "-------",
        f"- Components inventoried: {len(components)}",
        f"- Unique license/notice texts: {len(by_section)}",
        f"- Components still missing license/notice text: {len(missing)}",
        "",
    ]
    if missing:
        lines.extend(["Components Still Missing License/Notice Text", "---------------------------------------------"])
        for component in sorted(missing, key=lambda item: item.label.casefold()):
            lines.append(f"- {component.label} ({component.relationship}): {component.missing_reason}")
        lines.append("")

    header = "\n".join(lines).encode("utf-8")
    with output_path.open("wb") as handle:
        handle.write(header)
        handle.write(b"License/Notice Texts\n")
        handle.write(b"--------------------\n\n")
        for section in by_section:
            title, body = section_title_and_body(section.text)
            body = normalize_embedded_notice_titles(body)
            component_names = [component_notice_name(use.component) for use in section.uses]
            handle.write(b"--------------------------------------------------------------------------------\n\n")
            write_wrapped_line(handle, f"{human_join(component_names)} license:")
            if section.section_note:
                write_wrapped_line(handle, section.section_note)
            notice_notes = [
                (component_notice_name(use.component), use.component.notice_note)
                for use in section.uses
                if use.component.notice_note
            ]
            if notice_notes:
                handle.write(b"License source notes:\n")
                for component_name, notice_note in sorted(notice_notes, key=lambda item: item[0].casefold()):
                    write_wrapped_line(handle, f"- {component_name}: {notice_note}")
            if title:
                write_wrapped_line(handle, title)
            handle.write(b"\n")
            handle.write(body)
            if not body.endswith(b"\n"):
                handle.write(b"\n")
            handle.write(b"\n")


def write_memory(
    repo_root: Path,
    cache_root: Path,
    components: list[Component],
    sections: dict[str, LicenseSection],
    missing: list[Component],
) -> None:
    memory = {
        "generated_at": dt.datetime.now(dt.UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z"),
        "components": [
            {
                "ecosystem": component.ecosystem,
                "name": component.name,
                "version": component.version,
                "relationship": component.relationship,
                "source_dir": relative_display(component.source_dir, repo_root) if component.source_dir else None,
                "source_cache": relative_display(component.source_cache, repo_root) if component.source_cache else None,
                "evidence": component.evidence,
                "missing_reason": component.missing_reason,
                "notice_note": component.notice_note,
            }
            for component in components
        ],
        "sections": [
            {
                "sha256": section.sha256,
                "size": section.size,
                "text_file": f"license-texts/{section.sha256}.txt",
                "section_note": section.section_note,
                "uses": [
                    {
                        "component": use.component.label,
                        "relationship": use.component.relationship,
                        "license_file": relative_display(use.license_path, repo_root),
                        "evidence": use.component.evidence,
                        "notice_note": use.component.notice_note,
                    }
                    for use in section.uses
                ],
            }
            for section in sorted(sections.values(), key=lambda item: item.sha256)
        ],
        "missing": [
            {
                "component": component.label,
                "relationship": component.relationship,
                "reason": component.missing_reason,
            }
            for component in missing
        ],
    }
    (cache_root / "license-memory.json").write_text(json.dumps(memory, indent=2), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Collect exact upstream license texts for this repository.")
    parser.add_argument("--output", default="OPEN_SOURCE_LICENSES.txt", help="Output notice file path.")
    parser.add_argument("--cache-dir", default=".cache/license-sources", help="Ignored source cache directory.")
    parser.add_argument("--refresh", action="store_true", help="Refetch/rebuild cached package sources.")
    parser.add_argument(
        "--no-fetch",
        action="store_true",
        help="Use existing cache/module/node_modules sources only; do not call go mod download or npm pack.",
    )
    parser.add_argument("--skip-go", action="store_true", help="Skip Go module inventory.")
    parser.add_argument("--skip-npm", action="store_true", help="Skip frontend pnpm inventory.")
    parser.add_argument("--skip-assets", action="store_true", help="Skip configured bundled asset license files.")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    repo_root = repo_root_from_script()
    cache_root = (repo_root / args.cache_dir).resolve()
    cache_root.mkdir(parents=True, exist_ok=True)

    components: list[Component] = []
    if not args.skip_go:
        components.extend(collect_go_components(repo_root, cache_root, args.refresh, args.no_fetch))
    if not args.skip_npm:
        components.extend(collect_npm_components(repo_root, cache_root, args.refresh, args.no_fetch))
    if not args.skip_assets:
        components.extend(collect_asset_components(repo_root))

    sections, missing = build_sections(repo_root, components, cache_root)
    output_path = (repo_root / args.output).resolve()
    render_output(repo_root, cache_root, output_path, components, sections, missing)
    write_memory(repo_root, cache_root, components, sections, missing)

    print(f"Wrote {relative_display(output_path, repo_root)}")
    print(f"Inventoried {len(components)} component(s)")
    print(f"Emitted {len(sections)} unique license/notice text section(s)")
    if missing:
        print(f"Still missing license/notice text for {len(missing)} component(s)")
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
