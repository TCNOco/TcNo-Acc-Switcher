export type InputModality = "keyboard" | "pointer" | "controller";
export type ControllerGlyphScheme = "xbox" | "playstation" | "nintendo" | "generic";

const MODALITY_CLASSES: Record<InputModality, string> = {
  keyboard: "input-modality-keyboard",
  pointer: "input-modality-pointer",
  controller: "input-modality-controller",
};

const KEYBOARD_FOCUS_KEYS = new Set([
  "Tab",
  "ArrowUp",
  "ArrowDown",
  "ArrowLeft",
  "ArrowRight",
  "Home",
  "End",
  "PageUp",
  "PageDown",
  "Enter",
  " ",
  "Escape",
]);
const SYNTHETIC_CONTROLLER_KEY_EVENT = "__tcnoControllerKeyEvent";

let currentModality: InputModality = "pointer";
let currentControllerGlyphScheme: ControllerGlyphScheme = "xbox";

function rootElement(): HTMLElement | null {
  return document.documentElement ?? document.body ?? null;
}

export function setInputModality(modality: InputModality): void {
  currentModality = modality;
  const root = rootElement();
  if (!root) return;
  root.classList.remove(...Object.values(MODALITY_CLASSES));
  root.classList.add(MODALITY_CLASSES[modality]);
  root.dataset.inputModality = modality;
  if (!root.dataset.controllerScheme) {
    root.dataset.controllerScheme = currentControllerGlyphScheme;
  }
}

export function getInputModality(): InputModality {
  return currentModality;
}

export function getControllerGlyphScheme(): ControllerGlyphScheme {
  return currentControllerGlyphScheme;
}

export function setControllerGlyphScheme(scheme: ControllerGlyphScheme): void {
  currentControllerGlyphScheme = scheme;
  const root = rootElement();
  if (!root) return;
  root.dataset.controllerScheme = scheme;
}

export function inferControllerGlyphScheme(id: string | null | undefined): ControllerGlyphScheme {
  const normalized = (id ?? "").toLowerCase();
  if (normalized.includes("playstation") || normalized.includes("dualsense") || normalized.includes("dualshock") || normalized.includes("sony")) {
    return "playstation";
  }
  if (normalized.includes("nintendo") || normalized.includes("switch") || normalized.includes("joy-con") || normalized.includes("joycon")) {
    return "nintendo";
  }
  if (normalized.includes("xbox") || normalized.includes("xinput") || normalized.includes("microsoft")) {
    return "xbox";
  }
  return "generic";
}

export function markControllerInput(scheme?: ControllerGlyphScheme): void {
  if (scheme) {
    setControllerGlyphScheme(scheme);
  }
  setInputModality("controller");
}

export function markSyntheticControllerKeyEvent(event: KeyboardEvent): void {
  Object.defineProperty(event, SYNTHETIC_CONTROLLER_KEY_EVENT, {
    value: true,
    configurable: true,
  });
}

export function isSyntheticControllerKeyEvent(event: KeyboardEvent): boolean {
  return (event as KeyboardEvent & Record<string, unknown>)[SYNTHETIC_CONTROLLER_KEY_EVENT] === true;
}

export function installInputModalityTracking(): () => void {
  if (typeof window === "undefined" || typeof document === "undefined") {
    return () => {};
  }

  setInputModality(currentModality);

  const onPointerInput = (): void => setInputModality("pointer");
  const onKeydown = (event: KeyboardEvent): void => {
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    if (isSyntheticControllerKeyEvent(event)) return;
    if (KEYBOARD_FOCUS_KEYS.has(event.key)) {
      setInputModality("keyboard");
    }
  };

  window.addEventListener("keydown", onKeydown, true);
  window.addEventListener("mousedown", onPointerInput, true);
  window.addEventListener("pointerdown", onPointerInput, true);
  window.addEventListener("touchstart", onPointerInput, true);

  return () => {
    window.removeEventListener("keydown", onKeydown, true);
    window.removeEventListener("mousedown", onPointerInput, true);
    window.removeEventListener("pointerdown", onPointerInput, true);
    window.removeEventListener("touchstart", onPointerInput, true);
  };
}
