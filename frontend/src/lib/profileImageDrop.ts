/** Extensions supported by profile image cache (see internal/profileimage). */
const PROFILE_IMAGE_EXT_RE = /\.(jpe?g|png|webp|gif|webm|mp4)$/i;

/** Drag classification: image/video → row cues; shortcut → fullscreen overlay; incompatible → error overlay. */
export type DragFileCategory = "image" | "shortcut" | "incompatible";

function isMediaType(ty: string): boolean {
  return ty.startsWith("image/") || ty === "video/webm" || ty === "video/mp4";
}

/**
 * Classifies a drag's payload during dragenter/dragover.
 * - "image"       — recognised image/video MIME type → per-account row drop cues
 * - "shortcut"    — empty MIME type (Windows .lnk/.url) or no type info → fullscreen shortcut overlay
 * - "incompatible"— non-empty, non-image MIME type → error overlay
 */
export function getDragFileCategory(dt: DataTransfer | null): DragFileCategory {
  if (!dt?.types || typeof dt.types.length !== "number") return "shortcut";
  const typesArr = Array.from(dt.types as unknown as Iterable<string>);
  if (!typesArr.includes("Files")) return "shortcut";

  let sawFileItem = false;
  try {
    const items = dt.items;
    if (items?.length) {
      for (let i = 0; i < items.length; i++) {
        const it = items[i];
        if (!it || it.kind !== "file") continue;
        sawFileItem = true;
        const ty = (it.type ?? "").trim().toLowerCase();
        if (isMediaType(ty)) return "image";
        if (ty !== "") return "incompatible"; // known non-image MIME type
      }
    }
  } catch {
    /* stale DataTransfer during drag */
  }
  // Items had files but all had empty types → Windows shortcuts (.lnk / .url)
  if (sawFileItem) return "shortcut";

  try {
    const files = dt.files;
    if (files?.length) {
      for (let i = 0; i < files.length; i++) {
        const f = files[i];
        const ty = ((f?.type ?? "") as string).trim().toLowerCase();
        if (isMediaType(ty)) return "image";
        if (PROFILE_IMAGE_EXT_RE.test((f?.name ?? "") as string)) return "image";
        if (ty !== "") return "incompatible";
      }
      return "shortcut";
    }
  } catch {
    /* empty during drag until drop */
  }
  return "shortcut"; // fallback — no type info, assume shortcut
}

/** True during drag when we prefer per-account avatar drop UI instead of fullscreen shortcut overlay. */
export function shouldUseAccountProfileRowDropCue(dt: DataTransfer | null): boolean {
  return getDragFileCategory(dt) === "image";
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
