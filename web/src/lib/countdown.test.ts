import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { createCountdown } from "./countdown";

describe("createCountdown", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("reports the starting value before the first tick", () => {
    const ticks: number[] = [];
    const countdown = createCountdown({
      seconds: 3,
      onExpire: () => {},
      onTick: (value) => ticks.push(value),
    });

    countdown.start();

    expect(countdown.remaining).toBe(3);
    expect(ticks).toEqual([3]);
  });

  it("ticks down once per second", () => {
    const ticks: number[] = [];
    const countdown = createCountdown({
      seconds: 3,
      onExpire: () => {},
      onTick: (value) => ticks.push(value),
    });

    countdown.start();
    vi.advanceTimersByTime(2_000);

    expect(countdown.remaining).toBe(1);
    expect(ticks).toEqual([3, 2, 1]);
  });

  it("expires exactly once and stops ticking", () => {
    const onExpire = vi.fn();
    const ticks: number[] = [];
    const countdown = createCountdown({
      seconds: 2,
      onExpire,
      onTick: (value) => ticks.push(value),
    });

    countdown.start();
    vi.advanceTimersByTime(10_000);

    expect(onExpire).toHaveBeenCalledTimes(1);
    expect(countdown.remaining).toBe(0);
    expect(ticks).toEqual([2, 1, 0]);
  });

  it("does not expire after stop()", () => {
    const onExpire = vi.fn();
    const countdown = createCountdown({ seconds: 5, onExpire });

    countdown.start();
    vi.advanceTimersByTime(2_000);
    countdown.stop();
    vi.advanceTimersByTime(60_000);

    expect(onExpire).not.toHaveBeenCalled();
    expect(countdown.remaining).toBe(0);
  });

  it("reports zero on stop()", () => {
    const ticks: number[] = [];
    const countdown = createCountdown({
      seconds: 5,
      onExpire: () => {},
      onTick: (value) => ticks.push(value),
    });

    countdown.start();
    vi.advanceTimersByTime(1_000);
    countdown.stop();

    expect(ticks).toEqual([5, 4, 0]);
  });

  it("restarts from the top without leaving the old timer running", () => {
    const onExpire = vi.fn();
    const countdown = createCountdown({ seconds: 3, onExpire });

    countdown.start();
    vi.advanceTimersByTime(2_000);
    expect(countdown.remaining).toBe(1);

    countdown.start();
    expect(countdown.remaining).toBe(3);

    // The first run would have expired 1s in; only the restarted one counts.
    vi.advanceTimersByTime(2_000);
    expect(onExpire).not.toHaveBeenCalled();

    vi.advanceTimersByTime(1_000);
    expect(onExpire).toHaveBeenCalledTimes(1);
  });

  it("can be started again after expiring", () => {
    const onExpire = vi.fn();
    const countdown = createCountdown({ seconds: 1, onExpire });

    countdown.start();
    vi.advanceTimersByTime(1_000);
    expect(onExpire).toHaveBeenCalledTimes(1);

    countdown.start();
    vi.advanceTimersByTime(1_000);
    expect(onExpire).toHaveBeenCalledTimes(2);
  });

  it("expires immediately for a zero-second countdown", () => {
    const onExpire = vi.fn();
    const countdown = createCountdown({ seconds: 0, onExpire });

    countdown.start();

    expect(onExpire).toHaveBeenCalledTimes(1);
    expect(countdown.remaining).toBe(0);
    vi.advanceTimersByTime(10_000);
    expect(onExpire).toHaveBeenCalledTimes(1);
  });
});

// A hidden tab gets its timers throttled to a crawl or stopped outright, so the
// countdown must be a function of the wall clock and not of how many intervals
// actually fired. The token on screen belongs to one player only.
describe("createCountdown wall clock", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  /** Moves the clock without letting a single interval callback run. */
  function skipTimeWithoutTicks(ms: number) {
    vi.setSystemTime(Date.now() + ms);
  }

  function becomeVisible(state: DocumentVisibilityState = "visible") {
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => state,
    });
    document.dispatchEvent(new Event("visibilitychange"));
  }

  it("catches up in one tick after a throttled stretch", () => {
    const ticks: number[] = [];
    const countdown = createCountdown({
      seconds: 10,
      onExpire: () => {},
      onTick: (value) => ticks.push(value),
    });

    countdown.start();
    skipTimeWithoutTicks(6_000);
    vi.advanceTimersByTime(1_000);

    // Not 9: six seconds of wall clock went by while the tab was throttled.
    expect(countdown.remaining).toBe(3);
    expect(ticks).toEqual([10, 3]);
  });

  it("expires on the next tick when the deadline passed while throttled", () => {
    const onExpire = vi.fn();
    const countdown = createCountdown({ seconds: 5, onExpire });

    countdown.start();
    skipTimeWithoutTicks(30_000);
    vi.advanceTimersByTime(1_000);

    expect(onExpire).toHaveBeenCalledTimes(1);
    expect(countdown.remaining).toBe(0);
  });

  it("expires immediately when the tab comes back past the deadline", () => {
    const onExpire = vi.fn();
    const countdown = createCountdown({ seconds: 5, onExpire });

    countdown.start();
    skipTimeWithoutTicks(30_000);
    becomeVisible();

    // Without waiting for an interval: the token is already overdue.
    expect(onExpire).toHaveBeenCalledTimes(1);
    expect(countdown.remaining).toBe(0);
  });

  it("re-syncs without expiring when the tab comes back in time", () => {
    const onExpire = vi.fn();
    const ticks: number[] = [];
    const countdown = createCountdown({
      seconds: 10,
      onExpire,
      onTick: (value) => ticks.push(value),
    });

    countdown.start();
    skipTimeWithoutTicks(4_000);
    becomeVisible();

    expect(onExpire).not.toHaveBeenCalled();
    expect(countdown.remaining).toBe(6);
    expect(ticks).toEqual([10, 6]);
  });

  it("ignores a visibilitychange while hidden, and after the run ended", () => {
    const onExpire = vi.fn();
    const ticks: number[] = [];
    const countdown = createCountdown({
      seconds: 5,
      onExpire,
      onTick: (value) => ticks.push(value),
    });

    countdown.start();
    skipTimeWithoutTicks(30_000);
    becomeVisible("hidden");
    expect(onExpire).not.toHaveBeenCalled();

    becomeVisible();
    expect(onExpire).toHaveBeenCalledTimes(1);

    // The run is over; a later visibilitychange must not fire onExpire again.
    ticks.length = 0;
    becomeVisible();
    expect(onExpire).toHaveBeenCalledTimes(1);
    expect(ticks).toEqual([]);
  });

  it("stops listening once stopped", () => {
    const onExpire = vi.fn();
    const countdown = createCountdown({ seconds: 5, onExpire });

    countdown.start();
    countdown.stop();
    skipTimeWithoutTicks(30_000);
    becomeVisible();

    expect(onExpire).not.toHaveBeenCalled();
  });
});
