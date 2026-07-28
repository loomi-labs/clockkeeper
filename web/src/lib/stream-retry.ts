// Reconnecting consumer for a ConnectRPC server-streaming call.
//
// Phones sleep, wifi drops, proxies cut idle streams. The token bag stream always
// carries a FULL snapshot, so a reconnect needs no cursor or replay: reopening
// and taking the next message is enough to be correct again. What this loop adds
// is the retry policy (jittered exponential backoff), a status the UI can show,
// and the guarantee that nothing is delivered after stop().

import { Code, ConnectError } from "@connectrpc/connect";

export type WatchStatus = "connecting" | "live" | "reconnecting" | "stopped";

/** Backoff per consecutive failure; the last entry is the cap. */
const BACKOFF_MS = [1_000, 2_000, 4_000, 8_000, 15_000];

/** Spread reconnects so a whole table's phones do not retry in lockstep. */
const JITTER_RATIO = 0.2;

/**
 * Retrying cannot help these: the code is unknown (bag deleted or reset), or the
 * credential was rejected. Everything else — network blips, server restarts,
 * idle timeouts — is worth another attempt.
 */
export function isFatalStreamError(err: unknown): boolean {
  const code = ConnectError.from(err).code;
  return (
    code === Code.NotFound ||
    code === Code.Unauthenticated ||
    code === Code.PermissionDenied
  );
}

export function backoffDelay(failures: number): number {
  const base = BACKOFF_MS[Math.min(failures, BACKOFF_MS.length - 1)];
  const jitter = (Math.random() * 2 - 1) * JITTER_RATIO;
  return Math.round(base * (1 + jitter));
}

export type WatchLoopOptions<T> = {
  /** Opens a fresh stream. Called once per attempt; must honor `signal`. */
  open: (signal: AbortSignal) => AsyncIterable<T>;
  onMessage: (msg: T) => void;
  onStatus?: (status: WatchStatus) => void;
  /** Defaults to {@link isFatalStreamError}. */
  isFatal?: (err: unknown) => boolean;
};

export type WatchLoopHandle = {
  /** Idempotent. Aborts the attempt in flight and ends the loop for good. */
  stop: () => void;
};

export function watchLoop<T>(opts: WatchLoopOptions<T>): WatchLoopHandle {
  const isFatal = opts.isFatal ?? isFatalStreamError;

  let stopped = false;
  let controller: AbortController | null = null;
  let backoffTimer: ReturnType<typeof setTimeout> | null = null;
  let wakeFromBackoff: (() => void) | null = null;
  let failures = 0;

  /** Status updates stop dead once the loop is done — see `end()`. */
  function emit(status: WatchStatus) {
    if (stopped) return;
    opts.onStatus?.(status);
  }

  /** Terminal transition: the loop will never report anything again. */
  function end() {
    if (stopped) return;
    stopped = true;
    if (backoffTimer !== null) {
      clearTimeout(backoffTimer);
      backoffTimer = null;
    }
    wakeFromBackoff?.();
    wakeFromBackoff = null;
    controller?.abort();
    controller = null;
    opts.onStatus?.("stopped");
  }

  function backoff(ms: number): Promise<void> {
    return new Promise((resolve) => {
      wakeFromBackoff = resolve;
      backoffTimer = setTimeout(() => {
        backoffTimer = null;
        wakeFromBackoff = null;
        resolve();
      }, ms);
    });
  }

  async function run() {
    while (!stopped) {
      const attempt = new AbortController();
      controller = attempt;
      emit("connecting");
      let live = false;
      try {
        for await (const msg of opts.open(attempt.signal)) {
          // stop() may have landed while this message was in flight.
          if (stopped) return;
          if (!live) {
            live = true;
            // A stream that produced data is healthy: the next outage starts
            // from the shortest delay again.
            failures = 0;
            emit("live");
          }
          opts.onMessage(msg);
        }
      } catch (err) {
        // Our own abort surfaces here as a cancellation — stop() already
        // reported the terminal status, so it must stay silent.
        if (stopped) return;
        if (isFatal(err)) {
          end();
          return;
        }
      }
      if (stopped) return;
      // The server ended the stream, or it failed in a retryable way.
      emit("reconnecting");
      await backoff(backoffDelay(failures++));
      if (stopped) return;
    }
  }

  void run();

  return { stop: end };
}
