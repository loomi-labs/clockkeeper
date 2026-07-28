// A visible, restartable countdown.
//
// The shared device shows a revealed character for a fixed number of seconds and
// then hides it, whether or not the player taps "Done". That timer is the only
// thing standing between one player's token and the next person to pick the
// tablet up, so it lives here as a plain function of setInterval: no Svelte, no
// component, testable with fake timers.
//
// It counts against the WALL CLOCK, not against how many intervals fired. A
// backgrounded or throttled tab (every mobile browser throttles timers in a
// hidden tab, some stop them altogether) would otherwise leave a token on screen
// long past its time — so the remaining value is always derived from a deadline,
// and coming back into view re-syncs immediately.

export type CountdownOptions = {
  /** Starting value in seconds. Restarts always go back to this. */
  seconds: number;
  /** Called once per run, when the countdown reaches zero on its own. */
  onExpire: () => void;
  /** Called with the new value on start, on every tick and on stop(). */
  onTick?: (remaining: number) => void;
};

export type Countdown = {
  /** (Re)starts from `seconds`, discarding any run in flight. */
  start: () => void;
  /** Ends the run without expiring it. Idempotent. */
  stop: () => void;
  readonly remaining: number;
};

export function createCountdown(opts: CountdownOptions): Countdown {
  let timer: ReturnType<typeof setInterval> | null = null;
  let remaining = 0;
  /** Wall-clock ms at which this run is over. Only valid while timer !== null. */
  let deadline = 0;

  // Server-side rendering has no document to listen on.
  const doc = typeof document === "undefined" ? null : document;

  function clear() {
    if (timer !== null) {
      clearInterval(timer);
      timer = null;
    }
    doc?.removeEventListener("visibilitychange", onVisibility);
  }

  function set(next: number) {
    remaining = next;
    opts.onTick?.(next);
  }

  /**
   * Publishes the value the wall clock implies, and expires the run if the
   * deadline has passed — however much or little of it the interval saw.
   */
  function sync() {
    const left = Math.max(0, Math.ceil((deadline - Date.now()) / 1_000));
    set(left);
    if (left === 0) {
      // Disarm before notifying: onExpire may start a new run.
      clear();
      opts.onExpire();
    }
  }

  function onVisibility() {
    // A tab that was hidden may have missed most of its ticks; catch up now.
    if (timer === null) return;
    if (doc?.visibilityState !== "visible") return;
    sync();
  }

  function start() {
    // A second start() is a restart, not a second timer.
    clear();
    const seconds = Math.max(0, Math.ceil(opts.seconds));
    deadline = Date.now() + seconds * 1_000;
    set(seconds);
    if (remaining === 0) {
      // A zero-second countdown is already over; expire rather than arm a timer
      // that would first tick to -1.
      opts.onExpire();
      return;
    }
    timer = setInterval(sync, 1_000);
    doc?.addEventListener("visibilitychange", onVisibility);
  }

  function stop() {
    clear();
    set(0);
  }

  return {
    start,
    stop,
    get remaining() {
      return remaining;
    },
  };
}
