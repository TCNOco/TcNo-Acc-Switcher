/** Extensions supported by profile image cache (see internal/profileimage). */
const PROFILE_IMAGE_EXT_RE = /\.(jpe?g|png|webp|gif|webm|mp4)$/i;

const PROFILE_VIDEO_EXT_RE = /\.(webm|mp4)$/i;

/** True when a cached profile public URL points at an animated avatar (webm/mp4). */
export function isProfileVideoUrl(url: string | null | undefined): boolean {
  const u = (url ?? "").trim().split(/[?#]/)[0] ?? "";
  return PROFILE_VIDEO_EXT_RE.test(u);
}

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
function classifyItemType(type: string): DragFileCategory | null {
  const ty = type.trim().toLowerCase();
  if (!ty) return null;
  if (isMediaType(ty)) return "image";
  return "incompatible";
}

function classifyFileByExt(name: string): DragFileCategory | null {
  return PROFILE_IMAGE_EXT_RE.test(name.trim()) ? "image" : null;
}

function classifyItems(items: DataTransferItemList): DragFileCategory | null {
  let sawFileItem = false;
  const len = items.length;
  for (let i = 0; i < len; i++) {
    const it = items[i];
    if (!it || it.kind !== "file") continue;
    sawFileItem = true;
    const result = classifyItemType(it.type ?? "");
    if (result) return result;
  }
  return sawFileItem ? "shortcut" : null;
}

function classifyFiles(files: FileList): DragFileCategory | null {
  const len = files.length;
  for (let i = 0; i < len; i++) {
    const f = files[i];
    const result = classifyItemType((f?.type ?? "") as string);
    if (result) return result;
    if (classifyFileByExt((f?.name ?? "") as string)) return "image";
  }
  return "shortcut";
}

export function getDragFileCategory(dt: DataTransfer | null): DragFileCategory {
  if (!dt?.types || typeof dt.types.length !== "number") return "shortcut";
  const typesArr = Array.from(dt.types as unknown as Iterable<string>);
  if (!typesArr.includes("Files")) return "shortcut";

  try {
    if (dt.items?.length) {
      const result = classifyItems(dt.items);
      if (result) return result;
    }
  } catch {}

  try {
    if (dt.files?.length) {
      const result = classifyFiles(dt.files);
      if (result) return result;
    }
  } catch {}

  return "shortcut";
}

export function shouldUseAccountProfileRowDropCue(dt: DataTransfer | null): boolean {
  return getDragFileCategory(dt) === "image";
}

function isProfileImagePath(fsPath: string): boolean {
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
