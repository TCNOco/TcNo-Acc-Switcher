export type SteamGuardDropTarget = "none" | "qr";

export interface SteamGuardDropAdapter {
  importMaFiles(paths: string[]): Promise<void>;
  decodeQrScreenshot(path: string): Promise<void>;
  reportError?(error: unknown): void;
}
let adapter: SteamGuardDropAdapter | null = null;
let target: SteamGuardDropTarget = "none";
let qrHandler: ((path: string) => Promise<void>) | null = null;

const maFilePattern = /\.mafile$/i;
const qrImagePattern = /\.(?:png|jpe?g)$/i;
const windowsAbsolutePattern = /^(?:[a-z]:[\\/]|\\\\)/i;

function isAbsolutePath(path: string): boolean {
  return windowsAbsolutePattern.test(path) || path.startsWith("/");
}

function safePaths(paths: string[]): string[] {
  return paths.filter((path) => path.length > 0 && isAbsolutePath(path));
}

async function consume(operation: () => Promise<void>): Promise<boolean> {
  try {
    await operation();
  } catch (error) {
    adapter?.reportError?.(error);
  }
  return true;
}

export function configureSteamGuardDropAdapter(next: SteamGuardDropAdapter | null): void {
  adapter = next;
  if (!next) {
    target = "none";
    qrHandler = null;
  }
}

export function setSteamGuardDropTarget(
  next: SteamGuardDropTarget,
  handler: ((path: string) => Promise<void>) | null = null,
): void {
  target = next;
  qrHandler = next === "qr" ? handler : null;
}

export async function handleSteamGuardDrop(paths: string[]): Promise<boolean> {
  if (!adapter) return false;

  const absolutePaths = safePaths(paths);
  const maFiles = absolutePaths.filter((path) => maFilePattern.test(path));
  if (maFiles.length > 0) {
    return consume(() => adapter!.importMaFiles(maFiles));
  }

  if (target !== "qr") return false;
  const images = absolutePaths.filter((path) => qrImagePattern.test(path));
  if (images.length === 0) return false;
  if (images.length !== 1) {
    return consume(async () => {
      throw new Error("Drop exactly one QR screenshot");
    });
  }
  return consume(() => qrHandler ? qrHandler(images[0]) : adapter!.decodeQrScreenshot(images[0]));
}
