// Pure token bag logic: proto -> plain state, URLs, and the view-state decision
// table shared by the player page, the shared device page and the Storyteller
// panel. No Svelte imports here on purpose — everything below is a function of
// its arguments and unit-testable without a component.

import { TokenBagPhase } from "./gen/clockkeeper/v1/clockkeeper_pb";
import type {
  Character,
  TokenBag,
  TokenBagPlayer,
  WatchTokenBagResponse,
} from "./gen/clockkeeper/v1/clockkeeper_pb";
import type { WatchStatus } from "./stream-retry";

/** Proto sentinel for "no registration" / "neighbor not claimed". */
export const NO_ID = "0";

/**
 * A registrant, with the proto's int64 ids flattened to strings.
 *
 * bigint is normalized away exactly once (here) so nothing downstream has to mix
 * bigint and string ids, compare across the two, or worry about a bigint leaking
 * into JSON (`JSON.stringify` throws on it).
 */
export type BagPlayer = {
  id: string;
  name: string;
  viaSharedDevice: boolean;
  /** `"0"` when unclaimed. */
  leftId: string;
  /** `"0"` when unclaimed. */
  rightId: string;
};

/** The public watch stream's payload as plain state. */
export type BagSnapshot = {
  phase: TokenBagPhase;
  gameName: string;
  players: BagPlayer[];
  /** `"0"` when this device has no (or a rejected/removed) registration. */
  selfId: string;
  selfToken: Character | null;
};

/** The Storyteller's own view — adds the codes, has no self/game name. */
export type OwnerBag = {
  phase: TokenBagPhase;
  joinCode: string;
  sharedCode: string;
  players: BagPlayer[];
};

export function toBagPlayer(player: TokenBagPlayer): BagPlayer {
  return {
    id: String(player.registrationId),
    name: player.name,
    viaSharedDevice: player.viaSharedDevice,
    leftId: String(player.leftNeighborId),
    rightId: String(player.rightNeighborId),
  };
}

export function toBagPlayers(
  players: readonly TokenBagPlayer[] | undefined,
): BagPlayer[] {
  return (players ?? []).map(toBagPlayer);
}

export function emptySnapshot(): BagSnapshot {
  return {
    phase: TokenBagPhase.UNSPECIFIED,
    gameName: "",
    players: [],
    selfId: NO_ID,
    selfToken: null,
  };
}

/**
 * Every stream message is a full snapshot, so this replaces state wholesale —
 * there is no merge to get wrong.
 */
export function applySnapshot(
  resp: WatchTokenBagResponse | undefined,
): BagSnapshot {
  if (!resp) return emptySnapshot();
  return {
    phase: resp.phase,
    gameName: resp.gameName,
    players: toBagPlayers(resp.players),
    selfId: String(resp.selfRegistrationId),
    selfToken: resp.selfToken ?? null,
  };
}

export function applyOwnerBag(bag: TokenBag | undefined): OwnerBag {
  return {
    phase: bag?.phase ?? TokenBagPhase.UNSPECIFIED,
    joinCode: bag?.joinCode ?? "",
    sharedCode: bag?.sharedCode ?? "",
    players: toBagPlayers(bag?.players),
  };
}

// ---------------------------------------------------------------------------
// URLs (what the QR codes encode)
// ---------------------------------------------------------------------------

function bagUrl(origin: string, segment: string, code: string): string {
  return `${origin.replace(/\/+$/, "")}/${segment}/${encodeURIComponent(code)}`;
}

export function joinUrl(origin: string, joinCode: string): string {
  return bagUrl(origin, "join", joinCode);
}

export function deviceUrl(origin: string, sharedCode: string): string {
  return bagUrl(origin, "device", sharedCode);
}

// ---------------------------------------------------------------------------
// Player view state
// ---------------------------------------------------------------------------

/**
 * What a player's device should show.
 *
 * `waiting_reveal` is a page-side refinement: {@link derivePlayerView} returns
 * `neighbor_pick` for the whole closed phase and hands over `hasNeighbors`, so
 * the page can decide between "pick your neighbors" and "you're done, waiting"
 * (and let the player go back to editing).
 */
export type PlayerView =
  | { kind: "loading" }
  | { kind: "enter_name" }
  | { kind: "waiting_open" }
  | { kind: "neighbor_pick"; hasNeighbors: boolean }
  | { kind: "waiting_reveal" }
  | { kind: "revealed_shown" }
  | { kind: "revealed_hidden" }
  | { kind: "removed" }
  | { kind: "gone" };

export type PlayerViewInput = {
  phase: TokenBagPhase;
  players: readonly BagPlayer[];
  /** This device's registration id, `"0"` / `""` when it has none. */
  selfId: string;
  hasCredential: boolean;
  dismissed: boolean;
  streamStatus: WatchStatus;
};

/** True once the player has claimed both sides of their seat. */
export function hasBothNeighbors(
  players: readonly BagPlayer[],
  selfId: string,
): boolean {
  const self = players.find((player) => player.id === selfId);
  if (!self) return false;
  return self.leftId !== NO_ID && self.rightId !== NO_ID;
}

export function derivePlayerView(input: PlayerViewInput): PlayerView {
  const { phase, players, selfId, hasCredential, dismissed } = input;

  // No snapshot yet. A stopped stream at this point means the loop hit a fatal
  // error (unknown code) rather than that it is still on its way.
  if (phase === TokenBagPhase.UNSPECIFIED) {
    return input.streamStatus === "stopped"
      ? { kind: "gone" }
      : { kind: "loading" };
  }

  // The Storyteller reset the bag (or never opened it): nobody's registration
  // survives, so a stored credential means nothing here.
  if (phase === TokenBagPhase.INACTIVE) return { kind: "gone" };

  if (!hasCredential) {
    // Registration is the only way in, and only while it is open.
    return phase === TokenBagPhase.OPEN
      ? { kind: "enter_name" }
      : { kind: "gone" };
  }

  // We hold a credential the server does not recognize any more — kicked from
  // the list, or the secret belongs to a previous bag.
  if (selfId === "" || selfId === NO_ID) return { kind: "removed" };

  switch (phase) {
    case TokenBagPhase.OPEN:
      return { kind: "waiting_open" };
    case TokenBagPhase.CLOSED:
      return {
        kind: "neighbor_pick",
        hasNeighbors: hasBothNeighbors(players, selfId),
      };
    case TokenBagPhase.REVEALED:
      return dismissed
        ? { kind: "revealed_hidden" }
        : { kind: "revealed_shown" };
    default:
      return { kind: "gone" };
  }
}

/**
 * Who a player may name as a neighbor: everyone but themselves. Picking the same
 * player on both sides is allowed (a two-player bag has no other option) — the
 * server resolves the seating.
 */
export function neighborOptions(
  players: readonly BagPlayer[],
  selfId: string,
): BagPlayer[] {
  return players.filter((player) => player.id !== selfId);
}

// ---------------------------------------------------------------------------
// Name matching
// ---------------------------------------------------------------------------

// Go's unicode.IsSpace, spelled out: JavaScript's own `\s` is not the same set
// (it omits U+0085 and adds U+FEFF, which Go classifies as a format character
// and this function therefore drops instead of turning into a space).
const GO_SPACE =
  /[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]/g;

// Go's unicode.IsControl (category Cc) plus unicode.Cf: zero-width spaces and
// joiners, soft hyphens, bidi overrides such as U+202E, the BOM. Dropped
// outright, so two names that read identically at the table cannot both
// register.
const GO_DROPPED = /[\p{Cc}\p{Cf}]/gu;

/**
 * Case-folds and collapses a name the way the backend does, so a registrant can
 * be matched against a grimoire seat name typed by the Storyteller.
 *
 * DUPLICATED LOGIC — the authority is `normalizeName` in
 * `internal/web/tokenbag_helpers.go` (drop control AND format characters,
 * collapse whitespace runs to single spaces, trim, lowercase). It has to exist
 * on both sides: the server owns uniqueness, but this comparison is a pure client-side
 * UI affordance (which registrants are still unassigned) with no RPC to ask.
 * Keep the two in sync.
 */
export function normalizeName(raw: string): string {
  return (
    raw
      // Whitespace first: Go classifies tab/newline as space, not as control.
      .replace(GO_SPACE, " ")
      .replace(GO_DROPPED, "")
      .replace(/ +/g, " ")
      .trim()
      .toLowerCase()
  );
}

/**
 * Registrant names that no grimoire seat holds yet, in registration order.
 * Comparison is normalized, so "Ana  B" and "ana b" are the same person.
 */
export function unassignedRegistrants(
  registrantNames: readonly string[],
  assignedNames: ReadonlySet<string>,
): string[] {
  const taken = new Set<string>();
  for (const name of assignedNames) taken.add(normalizeName(name));
  return registrantNames.filter((name) => !taken.has(normalizeName(name)));
}
