const ANIMATIONS_DISABLED_CLASS = "animations-disabled";

type AnimationClassTarget = Pick<Document, "documentElement" | "body">;

export function applyAnimationClass(enabled: boolean, target: AnimationClassTarget = document): () => void {
  target.documentElement.classList.toggle(ANIMATIONS_DISABLED_CLASS, !enabled);
  target.body.classList.toggle(ANIMATIONS_DISABLED_CLASS, !enabled);

  return () => {
    target.documentElement.classList.remove(ANIMATIONS_DISABLED_CLASS);
    target.body.classList.remove(ANIMATIONS_DISABLED_CLASS);
  };
}
