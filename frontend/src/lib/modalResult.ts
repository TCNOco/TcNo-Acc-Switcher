import type { Component } from "svelte";
import { openAlertNoButton } from "../stores/modal";

/**
 * Opens a modal whose body reports its own result through an `onDone` prop, and
 * settles whichever way the modal actually closes.
 *
 * Escape and the backdrop dismiss a modal without the body ever calling
 * `onDone`, so the dialog's own promise is raced against it. Awaiting `onDone`
 * alone leaves the caller hanging on those paths, which looks like a dead
 * button rather than a cancelled dialog.
 *
 * The body is responsible for calling `dismissModal()` itself, synchronously,
 * in the same handler that reports the result. Resolving first and dismissing
 * from the awaiting code lets Svelte flush the still-mounted modal after the
 * store entry has gone.
 */
export function modalResult<T>(
  title: string,
  bodyComponent: Component<any>,
  bodyProps: Record<string, unknown>,
): Promise<T | null> {
  let settle: (value: T | null) => void = () => {};
  const result = new Promise<T | null>((resolve) => {
    settle = resolve;
  });
  void openAlertNoButton({
    title,
    bodyComponent,
    bodyProps: { ...bodyProps, onDone: (value: T | null) => settle(value ?? null) },
  }).then(() => settle(null));
  return result;
}
