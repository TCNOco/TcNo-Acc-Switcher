export type FocusSubscriber = (handler: () => void) => () => void;

export function createLatestRequestGuard(): {
  begin: () => number;
  isCurrent: (request: number) => boolean;
  invalidate: () => void;
} {
  let current = 0;
  return {
    begin: () => ++current,
    isCurrent: (request) => request === current,
    invalidate: () => { current++; },
  };
}

export function registerWindowFocusAccountRefresh(
  subscribe: FocusSubscriber,
  refresh: () => Promise<void>,
): () => void {
  let running = false;
  let pending = false;
  let stopped = false;

  async function run(): Promise<void> {
    if (running) {
      pending = true;
      return;
    }

    running = true;
    try {
      do {
        pending = false;
        await refresh();
      } while (pending && !stopped);
    } finally {
      running = false;
    }
  }

  const unsubscribe = subscribe(() => { void run(); });
  return () => {
    stopped = true;
    pending = false;
    unsubscribe();
  };
}
