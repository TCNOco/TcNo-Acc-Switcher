import { get } from "svelte/store";
import { describe, expect, it } from "vitest";
import { activeModal, cancelActiveModal, dismissModal } from "../stores/modal";
import { modalResult } from "./modalResult";

// The component is never rendered here; only the wiring is under test.
const body = {} as never;

describe("modalResult", () => {
  it("resolves with what the body reported", async () => {
    const pending = modalResult<{ ok: boolean }>("Unlock", body, { intro: "hi" });

    const modal = get(activeModal) as unknown as {
      bodyProps: { onDone: (v: unknown) => void; intro: string };
    };
    expect(modal.bodyProps.intro).toBe("hi");
    modal.bodyProps.onDone({ ok: true });
    dismissModal();

    await expect(pending).resolves.toEqual({ ok: true });
  });

  // Escape and the backdrop close the modal without the body reporting
  // anything. Awaiting onDone alone left the caller hanging there, which reads
  // as a button that does nothing rather than as a cancelled dialog.
  it("resolves null when the modal is dismissed without a result", async () => {
    const pending = modalResult<string>("Unlock", body, {});

    dismissModal();

    await expect(pending).resolves.toBeNull();
  });

  it("resolves null when the modal is cancelled", async () => {
    const pending = modalResult<string>("Unlock", body, {});

    cancelActiveModal();

    await expect(pending).resolves.toBeNull();
  });
});
