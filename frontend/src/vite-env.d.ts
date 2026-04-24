/// <reference types="svelte" />
/// <reference types="vite/client" />

interface WindowToastOpts {
  type: string
  title?: string
  message: string
  renderTo?: string
  duration?: number
}

interface Window {
  /** Global toast API (mirrors legacy host bridge); set while Toast.svelte is mounted. */
  notification?: {
    new: (opts: WindowToastOpts) => void
  }
}

/** Typings for Wails shortcuts bindings (see `wails-shortcuts-service` path in tsconfig / Vite alias). */
declare module "wails-shortcuts-service" {
  import type { CancellablePromise } from "@wailsio/runtime"
  import type { ShortcutDTO } from "../bindings/TcNo-Acc-Switcher/internal/shortcuts/models.js"

  export function CreateAccountShortcut(
    platformKey: string,
    uniqueID: string,
    displayName: string,
    stateSuffix: string,
    stateTitle: string,
    accountLogin: string,
  ): CancellablePromise<string>

  export function CreateGameAccountShortcut(
    platformKey: string,
    uniqueID: string,
    accountDisplayName: string,
    accountLogin: string,
    gameFileName: string,
  ): CancellablePromise<string>

  export function CreatePlatformShortcut(platformKey: string): CancellablePromise<string>
  export function DeletePlatformShortcut(platformKey: string): CancellablePromise<void>
  export function HideShortcut(platformKey: string, fileName: string): CancellablePromise<void>
  export function ListShortcuts(platformKey: string): CancellablePromise<ShortcutDTO[]>
  export function OpenShortcutFolder(platformKey: string): CancellablePromise<void>
  export function PlatformShortcutExists(platformKey: string): CancellablePromise<boolean>
  export function ReportSVGRenderResult(
    id: string,
    pngBase64: string,
    errMsg: string,
  ): CancellablePromise<void>
  export function ResolveAccountShortcutStem(
    platformKey: string,
    uniqueID: string,
    displayName: string,
    accountLogin: string,
  ): CancellablePromise<string>
  export function RunShortcut(
    platformKey: string,
    fileName: string,
    admin: boolean,
    selectedUniqueID: string,
  ): CancellablePromise<void>
  export function SaveShortcutOrder(
    platformKey: string,
    pinned: string[],
    dropdown: string[],
  ): CancellablePromise<void>
  export function ScanShortcuts(platformKey: string): CancellablePromise<void>
}
