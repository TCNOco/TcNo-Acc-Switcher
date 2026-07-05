import { describe, expect, it } from "vitest";

import { applyAnimationClass } from "./animationClass";

function createClassList() {
  const classes = new Set<string>();

  return {
    add(name: string) {
      classes.add(name);
    },
    remove(name: string) {
      classes.delete(name);
    },
    contains(name: string): boolean {
      return classes.has(name);
    },
    toggle(name: string, force?: boolean): boolean {
      if (force === true) {
        classes.add(name);
        return true;
      }
      if (force === false) {
        classes.delete(name);
        return false;
      }
      if (classes.has(name)) {
        classes.delete(name);
        return false;
      }
      classes.add(name);
      return true;
    },
  };
}

function createTarget(): Pick<Document, "documentElement" | "body"> {
  return {
    documentElement: { classList: createClassList() },
    body: { classList: createClassList() },
  } as unknown as Pick<Document, "documentElement" | "body">;
}

function expectAnimationClass(target: Pick<Document, "documentElement" | "body">, present: boolean): void {
  expect(target.documentElement.classList.contains("animations-disabled")).toBe(present);
  expect(target.body.classList.contains("animations-disabled")).toBe(present);
}

describe("applyAnimationClass", () => {
  it("removes the disabled class from html and body when animations are enabled", () => {
    const target = createTarget();
    target.documentElement.classList.add("animations-disabled");
    target.body.classList.add("animations-disabled");

    applyAnimationClass(true, target);

    expectAnimationClass(target, false);
  });

  it("applies the disabled class to html and body when animations are disabled", () => {
    const target = createTarget();

    applyAnimationClass(false, target);

    expectAnimationClass(target, true);
  });

  it("cleans up the disabled class from html and body", () => {
    const target = createTarget();
    const cleanup = applyAnimationClass(false, target);

    cleanup();

    expectAnimationClass(target, false);
  });
});
