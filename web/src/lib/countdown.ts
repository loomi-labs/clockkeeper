// A visible, restartable countdown.
//
// The shared device shows a revealed character for a fixed number of seconds and
// then hides it, whether or not the player taps "Done". That timer is the only
// thing standing between one player's token and the next person to pick the
// tablet up, so it lives here as a plain function of setInterval: no Svelte, no
// component, testable with fake timers.

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

  function clear() {
    if (timer !== null) {
      clearInterval(timer);
      timer = null;
    }
  }

  function set(next: number) {
    remaining = next;
    opts.onTick?.(next);
  }

  function start() {
    // A second start() is a restart, not a second timer.
    clear();
    set(Math.max(0, Math.ceil(opts.seconds)));
    if (remaining === 0) {
      // A zero-second countdown is already over; expire rather than arm a timer
      // that would first tick to -1.
      opts.onExpire();
      return;
    }
    timer = setInterval(() => {
      set(remaining - 1);
      if (remaining <= 0) {
        // Disarm before notifying: onExpire may start a new run.
        clear();
        opts.onExpire();
      }
    }, 1_000);
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
