/** Extensions supported by profile image cache (see internal/profileimage). */
const PROFILE_IMAGE_EXT_RE = /\.(jpe?g|png|webp|gif|webm|mp4)$/i;

/** True during drag when we prefer per-account avatar drop UI instead of fullscreen shortcut overlay */
export function shouldUseAccountProfileRowDropCue(dt: DataTransfer | null): boolean {
  if (!dt?.types || typeof dt.types.length !== "number") {
    return false;
  }
  const typesArr = Array.from(dt.types as unknown as Iterable<string>);
  if (!typesArr.includes("Files")) {
    return false;
  }
  try {
    const items = dt.items;
    if (items?.length) {
      for (let i = 0; i < items.length; i++) {
        const it = items[i];
        if (!it || it.kind !== "file") {
          continue;
        }
        const ty = (it.type ?? "").trim().toLowerCase();
        if (ty.startsWith("image/") || ty === "video/webm" || ty === "video/mp4") {
          return true;
        }
      }
    }
  } catch {
    /* stale DataTransfer during drag */
  }
  try {
    const files = dt.files;
    if (files?.length) {
      for (let i = 0; i < files.length; i++) {
        const f = files[i];
        const ty = ((f?.type ?? "") as string).trim().toLowerCase();
        if (ty.startsWith("image/") || ty === "video/webm" || ty === "video/mp4") {
          return true;
        }
        if (PROFILE_IMAGE_EXT_RE.test((f?.name ?? "") as string)) {
          return true;
        }
      }
      /* Known non-image filenames (.lnk, etc.) → keep fullscreen shortcut cue */
      return false;
    }
  } catch {
    /* empty during drag until drop */
  }
  /* Explorer often exposes only "Files" until drop */
  return true;
}

export function isProfileImagePath(fsPath: string): boolean {
  return PROFILE_IMAGE_EXT_RE.test(fsPath.trim());
}

export function firstProfileImagePath(paths: string[]): string | undefined {
  for (const p of paths) {
    const t = p.trim();
    if (t && PROFILE_IMAGE_EXT_RE.test(t)) {
      return t;
    }
  }
  return undefined;
}

/** True when every path looks like a profile media file (for silencing shortcut import toasts). */
export function pathsAreOnlyProfileMedia(paths: string[]): boolean {
  if (paths.length === 0) {
    return false;
  }
  return paths.every((p) => PROFILE_IMAGE_EXT_RE.test(p.trim()));
}
