import { derived, get, writable, type Readable, type Writable } from "svelte/store";
import { pushToast } from "../stores/toast";
import { formatWailsError } from "./formatWailsError";
import { t } from "../stores/i18n";

export interface ToggleController extends Readable<{ value: boolean; loading: boolean }> {
  value: Writable<boolean>;
  loading: Writable<boolean>;
  init: () => Promise<void>;
  toggle: () => Promise<void>;
}

/**
 * Creates a reactive toggle for a boolean application setting.
 *
 * @param getter  Function that fetches the current state from the backend.
 * @param setter  Function that persists the new state to the backend.
 * @param toastLabel  The translated label to show in success toasts.
 * @param guard   Optional guard function — if it returns false, the toggle is blocked.
 */
export function createToggle(
  getter: () => Promise<boolean>,
  setter: (value: boolean) => Promise<void>,
  toastLabel: string,
  guard?: () => boolean,
): ToggleController {
  const value = writable(false);
  const loading = writable(false);

  const store = derived([value, loading], ([$value, $loading]) => ({
    value: $value,
    loading: $loading,
  }));

  const toggle = async () => {
    if (get(loading) || (guard && !guard())) return;
    const next = !get(value);
    loading.set(true);
    try {
      await setter(next);
      value.set(next);
      pushToast({
        type: "success",
        message: get(t)("Toast_SavedItem", { item: toastLabel }),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatWailsError(e),
        duration: 8000,
      });
    } finally {
      loading.set(false);
    }
  };

  return {
    subscribe: store.subscribe,
    value,
    loading,
    init: async () => {
      try {
        value.set(await getter());
      } catch {
        value.set(false);
      }
    },
    toggle,
  };
}
