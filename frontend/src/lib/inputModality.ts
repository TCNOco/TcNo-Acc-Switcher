export type InputModality = "keyboard" | "pointer" | "controller";

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

let currentModality: InputModality = "pointer";

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
}

export function getInputModality(): InputModality {
  return currentModality;
}

export function markControllerInput(): void {
  setInputModality("controller");
}

export function installInputModalityTracking(): () => void {
  if (typeof window === "undefined" || typeof document === "undefined") {
    return () => {};
  }

  setInputModality(currentModality);

  const onPointerInput = (): void => setInputModality("pointer");
  const onKeydown = (event: KeyboardEvent): void => {
    if (event.metaKey || event.ctrlKey || event.altKey) return;
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
