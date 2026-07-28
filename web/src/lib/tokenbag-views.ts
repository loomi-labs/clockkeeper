// Page-side view refinements on top of `tokenbag.ts`.
//
// `derivePlayerView` answers "what does the server say about this device". These
// two functions answer "what should the page show", which also depends on what
// the player did on this device in this session (submitted their neighbors,
// skipped the picker). Kept out of the components so the transitions can be
// tested without a DOM.

import { TokenBagPhase } from "./gen/clockkeeper/v1/clockkeeper_pb";
import type { PlayerView } from "./tokenbag";
import type { WatchStatus } from "./stream-retry";

/** What happened on this device, on top of what the server reported. */
export type PlayerPageFlags = {
  /** Submitted or skipped — the picker has had its turn. */
  settled: boolean;
  /** Reopened the picker to change an earlier answer. */
  editing: boolean;
  /**
   * The watch stream ended for good: an unknown code, a rejected credential, or
   * any other fatal error. Whatever the last snapshot said is now frozen and
   * unverifiable, so the page must stop pretending it is live.
   */
  streamDead: boolean;
};

/**
 * Collapses the closed phase into "pick your neighbors" or "waiting for the
 * reveal", and turns a dead stream into `gone`.
 *
 * `derivePlayerView` only maps a stopped stream to `gone` before the first
 * snapshot — after one, it has a phase to report and no way to know the stream
 * behind it died. That case (Storyteller deletes or resets the game mid-session)
 * would otherwise leave the player staring at a stale "waiting" screen forever,
 * so it is folded in here.
 *
 * `derivePlayerView` also deliberately reports `neighbor_pick` for the whole
 * closed phase; the picker is optional, so it must not block the waiting screen
 * once the player is done with it. Editing wins over both settled and
 * already-saved neighbors, which is what makes the answer changeable until the
 * reveal.
 */
export function refinePlayerView(
  view: PlayerView,
  flags: PlayerPageFlags,
): PlayerView {
  if (flags.streamDead) return { kind: "gone" };
  if (view.kind !== "neighbor_pick") return view;
  if (flags.editing) return view;
  if (flags.settled || view.hasNeighbors) return { kind: "waiting_reveal" };
  return view;
}

/** What the shared device should show. */
export type DeviceView =
  | "loading"
  | "add_names"
  | "closed"
  | "reveal_list"
  | "gone";

export function deriveDeviceView(
  phase: TokenBagPhase,
  streamStatus: WatchStatus,
): DeviceView {
  // A stopped stream is terminal (the loop only ends on a fatal error or on
  // teardown), so any phase it left behind is stale. Showing a tappable name
  // grid for a game that no longer exists is worse than saying so.
  if (streamStatus === "stopped") return "gone";
  // No snapshot yet.
  if (phase === TokenBagPhase.UNSPECIFIED) return "loading";
  switch (phase) {
    case TokenBagPhase.OPEN:
      return "add_names";
    case TokenBagPhase.CLOSED:
      return "closed";
    case TokenBagPhase.REVEALED:
      return "reveal_list";
    default:
      // INACTIVE: the bag was reset, so this device has nothing to show.
      return "gone";
  }
}
