import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { Code, ConnectError } from "@connectrpc/connect";
import {
  watchLoop,
  backoffDelay,
  isFatalStreamError,
  type WatchStatus,
} from "./stream-retry";

/**
 * A stream we can feed, end and fail from the test, and that ends when the
 * loop's AbortSignal fires — the same contract a real Connect stream has.
 */
function channel<T>() {
  const queued: T[] = [];
  let wake: (() => void) | null = null;
  let closed = false;
  let failure: unknown = null;

  function bump() {
    wake?.();
    wake = null;
  }

  return {
    push(value: T) {
      queued.push(value);
      bump();
    },
    close() {
      closed = true;
      bump();
    },
    fail(err: unknown) {
      failure = err;
      bump();
    },
    async *open(signal: AbortSignal): AsyncIterable<T> {
      while (true) {
        while (queued.length > 0) yield queued.shift() as T;
        if (failure !== null) throw failure;
        if (closed) return;
        if (signal.aborted) throw new ConnectError("canceled", Code.Canceled);
        await new Promise<void>((resolve) => {
          wake = resolve;
          signal.addEventListener("abort", () => resolve(), { once: true });
        });
        if (signal.aborted) throw new ConnectError("canceled", Code.Canceled);
      }
    },
  };
}

/** An iterable that ends immediately — a server closing the stream at once. */
async function* endsImmediately(): AsyncIterable<never> {}

/** Lets pending microtasks (the for-await handoffs) settle. */
async function flush() {
  await vi.advanceTimersByTimeAsync(0);
}

describe("backoffDelay", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("grows exponentially and caps at 15s", () => {
    vi.spyOn(Math, "random").mockReturnValue(0.5);
    expect([0, 1, 2, 3, 4, 5, 9].map(backoffDelay)).toEqual([
      1000, 2000, 4000, 8000, 15000, 15000, 15000,
    ]);
  });

  it("stays within +/-20% of the base delay", () => {
    for (const random of [0, 0.25, 0.5, 0.75, 1]) {
      vi.spyOn(Math, "random").mockReturnValue(random);
      expect(backoffDelay(0)).toBeGreaterThanOrEqual(800);
      expect(backoffDelay(0)).toBeLessThanOrEqual(1200);
      expect(backoffDelay(4)).toBeGreaterThanOrEqual(12000);
      expect(backoffDelay(4)).toBeLessThanOrEqual(18000);
    }
  });

  it("actually jitters", () => {
    vi.spyOn(Math, "random").mockReturnValue(0);
    expect(backoffDelay(0)).toBe(800);
    vi.spyOn(Math, "random").mockReturnValue(1);
    expect(backoffDelay(0)).toBe(1200);
  });
});

describe("isFatalStreamError", () => {
  it("treats unknown code / rejected credential as fatal", () => {
    expect(isFatalStreamError(new ConnectError("x", Code.NotFound))).toBe(true);
    expect(
      isFatalStreamError(new ConnectError("x", Code.Unauthenticated)),
    ).toBe(true);
    expect(
      isFatalStreamError(new ConnectError("x", Code.PermissionDenied)),
    ).toBe(true);
  });

  it("treats transport trouble as retryable", () => {
    expect(isFatalStreamError(new ConnectError("x", Code.Unavailable))).toBe(
      false,
    );
    expect(isFatalStreamError(new ConnectError("x", Code.Canceled))).toBe(
      false,
    );
    expect(isFatalStreamError(new Error("network"))).toBe(false);
  });
});

describe("watchLoop", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("reopens with the backoff sequence, capped at 15s", async () => {
    vi.spyOn(Math, "random").mockReturnValue(0.5);
    const opens: number[] = [];
    const handle = watchLoop<number>({
      open: () => {
        opens.push(Date.now());
        return endsImmediately();
      },
      onMessage: () => {},
    });

    await flush();
    expect(opens.length).toBe(1);
    const expected = [1000, 2000, 4000, 8000, 15000, 15000];
    for (const delay of expected) {
      const before = opens.length;
      await vi.advanceTimersByTimeAsync(delay - 1);
      expect(opens.length).toBe(before);
      await vi.advanceTimersByTimeAsync(1);
      expect(opens.length).toBe(before + 1);
    }
    const gaps = opens.slice(1).map((at, i) => at - opens[i]);
    expect(gaps).toEqual(expected);
    handle.stop();
  });

  it("reports connecting -> live -> reconnecting", async () => {
    vi.spyOn(Math, "random").mockReturnValue(0.5);
    const stream = channel<string>();
    const statuses: WatchStatus[] = [];
    const messages: string[] = [];
    const handle = watchLoop<string>({
      open: (signal) => stream.open(signal),
      onMessage: (msg) => messages.push(msg),
      onStatus: (status) => statuses.push(status),
    });

    await flush();
    expect(statuses).toEqual(["connecting"]);
    stream.push("a");
    await flush();
    expect(messages).toEqual(["a"]);
    expect(statuses).toEqual(["connecting", "live"]);
    stream.push("b");
    await flush();
    expect(messages).toEqual(["a", "b"]);
    // No duplicate "live" for later messages of the same attempt.
    expect(statuses).toEqual(["connecting", "live"]);

    stream.close();
    await flush();
    expect(statuses).toEqual(["connecting", "live", "reconnecting"]);
    handle.stop();
    expect(statuses.at(-1)).toBe("stopped");
  });

  it("resets the backoff after a message arrives", async () => {
    vi.spyOn(Math, "random").mockReturnValue(0.5);
    const opens: number[] = [];
    // Each attempt delivers one message and then ends: the failure counter must
    // never climb, so every gap is the shortest delay.
    const handle = watchLoop<string>({
      open: () => {
        opens.push(Date.now());
        return (async function* () {
          yield "snapshot";
        })();
      },
      onMessage: () => {},
    });

    await flush();
    for (let i = 0; i < 4; i++) await vi.advanceTimersByTimeAsync(1000);
    const gaps = opens.slice(1).map((at, i) => at - opens[i]);
    expect(gaps).toEqual([1000, 1000, 1000, 1000]);
    handle.stop();
  });

  it("stops on a fatal code without retrying", async () => {
    const stream = channel<string>();
    const statuses: WatchStatus[] = [];
    let opened = 0;
    watchLoop<string>({
      open: (signal) => {
        opened++;
        return stream.open(signal);
      },
      onMessage: () => {},
      onStatus: (status) => statuses.push(status),
    });

    await flush();
    stream.fail(new ConnectError("gone", Code.NotFound));
    await flush();
    expect(statuses).toEqual(["connecting", "stopped"]);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(opened).toBe(1);
  });

  it("retries a non-fatal error", async () => {
    vi.spyOn(Math, "random").mockReturnValue(0.5);
    const statuses: WatchStatus[] = [];
    let opened = 0;
    const handle = watchLoop<string>({
      open: () => {
        opened++;
        return (async function* (): AsyncIterable<string> {
          throw new ConnectError("boom", Code.Unavailable);
        })();
      },
      onMessage: () => {},
      onStatus: (status) => statuses.push(status),
    });

    await flush();
    expect(statuses).toEqual(["connecting", "reconnecting"]);
    await vi.advanceTimersByTimeAsync(1000);
    expect(opened).toBe(2);
    handle.stop();
  });

  it("honors a custom isFatal", async () => {
    const statuses: WatchStatus[] = [];
    let opened = 0;
    watchLoop<string>({
      open: () => {
        opened++;
        return (async function* (): AsyncIterable<string> {
          throw new Error("nope");
        })();
      },
      onMessage: () => {},
      onStatus: (status) => statuses.push(status),
      isFatal: () => true,
    });

    await flush();
    expect(statuses).toEqual(["connecting", "stopped"]);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(opened).toBe(1);
  });

  it("stops mid-stream and delivers nothing afterwards", async () => {
    const stream = channel<string>();
    const statuses: WatchStatus[] = [];
    const messages: string[] = [];
    const handle = watchLoop<string>({
      open: (signal) => stream.open(signal),
      onMessage: (msg) => messages.push(msg),
      onStatus: (status) => statuses.push(status),
    });

    await flush();
    stream.push("a");
    await flush();
    handle.stop();
    stream.push("b");
    await vi.advanceTimersByTimeAsync(60_000);

    expect(messages).toEqual(["a"]);
    expect(statuses).toEqual(["connecting", "live", "stopped"]);
  });

  it("stops mid-backoff without reopening", async () => {
    vi.spyOn(Math, "random").mockReturnValue(0.5);
    const statuses: WatchStatus[] = [];
    let opened = 0;
    const handle = watchLoop<string>({
      open: () => {
        opened++;
        return endsImmediately();
      },
      onMessage: () => {},
      onStatus: (status) => statuses.push(status),
    });

    await flush();
    expect(statuses).toEqual(["connecting", "reconnecting"]);
    handle.stop();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(opened).toBe(1);
    expect(statuses).toEqual(["connecting", "reconnecting", "stopped"]);
  });

  it("is idempotent and reports stopped exactly once", async () => {
    const stream = channel<string>();
    const statuses: WatchStatus[] = [];
    const handle = watchLoop<string>({
      open: (signal) => stream.open(signal),
      onMessage: () => {},
      onStatus: (status) => statuses.push(status),
    });

    await flush();
    handle.stop();
    handle.stop();
    handle.stop();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(statuses.filter((s) => s === "stopped")).toEqual(["stopped"]);
  });
});
