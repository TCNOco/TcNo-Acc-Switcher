import { describe, expect, it, vi } from "vitest";
import { dismissModal, openPrompt } from "../../stores/modal";
import { saveCurrentAccount } from "./saveCurrent";

function steps() {
  return {
    prepare: vi.fn(async () => "Discord user"),
    prompt: vi.fn(async (_name: string): Promise<string | null> => " My account "),
    save: vi.fn(async (_name: string) => {}),
    cancelPreparation: vi.fn(async () => {}),
    isActive: () => true,
  };
}

describe("save current dialog", () => {
  it("prefills the prepared name, saves, then releases preparation", async () => {
    const flow = steps();
    expect(await saveCurrentAccount(flow)).toBe(true);
    expect(flow.prompt).toHaveBeenCalledWith("Discord user");
    expect(flow.save).toHaveBeenCalledWith("My account");
    expect(flow.save.mock.invocationCallOrder[0]).toBeLessThan(flow.cancelPreparation.mock.invocationCallOrder[0]);
    expect(flow.cancelPreparation).toHaveBeenCalledOnce();
  });

  it.each([undefined, null, "", "   "])("does not save when the dialog closes with %s", async (result) => {
    const flow = steps();
    flow.prompt.mockImplementation(async () => {
      const pending = openPrompt({ title: "Save current session" });
      dismissModal(result);
      return pending;
    });
    expect(await saveCurrentAccount(flow)).toBe(false);
    expect(flow.save).not.toHaveBeenCalled();
    expect(flow.cancelPreparation).toHaveBeenCalledOnce();
  });

  it("does not cancel another dialog when preparation is rejected", async () => {
    const flow = steps();
    flow.prepare.mockRejectedValue(new Error("a save dialog is already being prepared"));
    await expect(saveCurrentAccount(flow)).rejects.toThrow("already being prepared");
    expect(flow.prompt).not.toHaveBeenCalled();
    expect(flow.save).not.toHaveBeenCalled();
    expect(flow.cancelPreparation).not.toHaveBeenCalled();
  });

  it.each(["prompt", "save"] as const)("releases preparation when %s fails", async (step) => {
    const flow = steps();
    const error = new Error(`${step} failed`);
    flow[step].mockRejectedValue(error);
    await expect(saveCurrentAccount(flow)).rejects.toThrow(error);
    expect(flow.cancelPreparation).toHaveBeenCalledOnce();
  });

  it("cancels if the page changes during preparation", async () => {
    const flow = steps();
    flow.isActive = () => false;
    expect(await saveCurrentAccount(flow)).toBe(false);
    expect(flow.prompt).not.toHaveBeenCalled();
    expect(flow.save).not.toHaveBeenCalled();
    expect(flow.cancelPreparation).toHaveBeenCalledOnce();
  });

  it("cancels if the page changes while the prompt is open", async () => {
    const flow = steps();
    let active = true;
    flow.isActive = () => active;
    flow.prompt.mockImplementation(async () => {
      active = false;
      return "My account";
    });
    expect(await saveCurrentAccount(flow)).toBe(false);
    expect(flow.save).not.toHaveBeenCalled();
    expect(flow.cancelPreparation).toHaveBeenCalledOnce();
  });
});
