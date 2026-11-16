interface SaveCurrentSteps {
  prepare: () => Promise<string>;
  prompt: (suggestedName: string) => Promise<string | null>;
  save: (displayName: string) => Promise<void>;
  cancelPreparation: () => Promise<void>;
  isActive: () => boolean;
}

export async function saveCurrentAccount(steps: SaveCurrentSteps): Promise<boolean> {
  // A failed preparation restores its own launcher. Cancelling it here could
  // release a different dialog's preparation if the backend rejected overlap.
  const suggestedName = await steps.prepare();
  try {
    if (!steps.isActive()) return false;
    const displayName = await steps.prompt(suggestedName);
    if (!steps.isActive() || typeof displayName !== "string" || !displayName.trim()) return false;
    await steps.save(displayName.trim());
    return true;
  } finally {
    await steps.cancelPreparation();
  }
}
