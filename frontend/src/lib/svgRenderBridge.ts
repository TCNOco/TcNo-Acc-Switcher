import { Events } from "@wailsio/runtime";
import * as Shortcuts from "../../bindings/TcNo-Acc-Switcher/internal/shortcuts/service.js";

type RenderSVGRequest = {
  id: string;
  svg: string;
  size: number;
};

function arrayBufferToBase64(buffer: ArrayBuffer): string {
  let binary = "";
  const bytes = new Uint8Array(buffer);
  const chunk = 0x8000;
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
  }
  return btoa(binary);
}

/**
 * Listens for Go-side SVG raster fallback (oksvg failed) and renders via canvas, then reports PNG back.
 */
export function registerSvgRenderBridge(): () => void {
  return Events.On("shortcut:render-svg-request", async (ev) => {
    const d = ev.data as RenderSVGRequest;
    if (!d?.id || !d.svg || !d.size) {
      return;
    }
    try {
      const canvas = document.createElement("canvas");
      canvas.width = d.size;
      canvas.height = d.size;
      const ctx = canvas.getContext("2d");
      if (!ctx) {
        await Shortcuts.ReportSVGRenderResult(d.id, "", "no 2d context");
        return;
      }
      const img = new Image();
      const svg64 =
        "data:image/svg+xml;charset=utf-8," + encodeURIComponent(d.svg);
      await new Promise<void>((resolve, reject) => {
        img.onload = () => resolve();
        img.onerror = () => reject(new Error("svg image load failed"));
        img.src = svg64;
      });
      ctx.clearRect(0, 0, d.size, d.size);
      ctx.drawImage(img, 0, 0, d.size, d.size);
      const blob = await new Promise<Blob | null>((r) =>
        canvas.toBlob((b) => r(b), "image/png"),
      );
      if (!blob) {
        await Shortcuts.ReportSVGRenderResult(d.id, "", "toBlob failed");
        return;
      }
      const buf = await blob.arrayBuffer();
      const b64 = arrayBufferToBase64(buf);
      await Shortcuts.ReportSVGRenderResult(d.id, b64, "");
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      await Shortcuts.ReportSVGRenderResult(d.id, "", msg);
    }
  });
}
