// Who owns the player-name list on the game page: the Token Bag, or the
// Storyteller's saved presets.
//
// The Token Bag panel only exists in the setup tab of a game still in setup, so
// what it last reported can outlive the situation it was reported for — the game
// starting, or the Storyteller navigating to another game. Deciding from the
// report ALONE would leave those stale names driving the assignment UI, which is
// why the decision is a function of the report plus the state of the page that is
// on screen right now.

import { NO_ID, normalizeName, type BagPlayer } from "./tokenbag";

/** What the Token Bag panel reports up to the page. */
export type BagRegistrants = {
  /** Registered names, in join order. */
  names: readonly string[];
  /**
   * The bag still accepts registrations (phase OPEN or CLOSED). A name the page
   * writes onto a seat while it does has to be registered as well, or the
   * reveal's by-name match cannot find it — see `registerBagName` in
   * `routes/games/[id]/+page.svelte`.
   */
  editable: boolean;
  /** The game this report is about, so a stale one cannot leak into another. */
  gameId: string;
};

/**
 * The seat names that are actually taken: the grimoire names of the roles IN
 * PLAY, and nothing else.
 *
 * `grimoirePlayerNames` is never pruned when the Storyteller swaps a character
 * out of the script, so it keeps names on seats that no longer exist. Counting
 * those as "assigned" would badge a registrant as dealt in — and enable Reveal —
 * for a role nobody is holding, while the server (which restricts the same map to
 * the roles in play) refuses the reveal.
 *
 * MIRRORS the `inPlay` filter in `RevealTokenBag`
 * (`internal/web/api_tokenbag.go`); the server is the authority.
 */
export function assignedSeatNames(
  inPlayRoleIds: readonly string[],
  grimoireNames: ReadonlyMap<string, string>,
): Set<string> {
  const names = new Set<string>();
  for (const roleId of inPlayRoleIds) {
    const name = grimoireNames.get(roleId);
    if (name !== undefined && name !== "") names.add(name);
  }
  return names;
}

/** What a grimoire seat inherits from the registrant sitting in it. */
export type SeatRegistration = {
  /** They were added on the shared tablet rather than joining from a phone. */
  viaShared: boolean;
  /** Their left neighbor pick, resolved to a name. Absent while unclaimed. */
  leftName?: string;
  /** Their right neighbor pick, resolved to a name. Absent while unclaimed. */
  rightName?: string;
};

/** A grimoire seat, as the assignment UI holds it: a role id and its name. */
export type NamedSeat = {
  id: string;
  name?: string | undefined;
};

/**
 * Ties each seat to the registrant whose name it holds, so a seat row can show
 * where that player joined from and who they said they are sitting between.
 *
 * Matching is by normalized name — the same by-name link `RevealTokenBag`
 * depends on, so what a row shows and what the server will match cannot
 * disagree. Seats with no name, or a name nobody registered, are absent from the
 * result.
 *
 * Duplicates are safe both ways: two seats carrying the same name both resolve
 * to that registrant, and if two registrants normalize to the same name (the
 * server forbids it, but nothing here depends on that) the first one wins.
 */
export function matchRegistrantsToSeats(
  registrants: readonly BagPlayer[],
  seats: readonly NamedSeat[],
): Map<string, SeatRegistration> {
  const byName = new Map<string, BagPlayer>();
  const nameById = new Map<string, string>();
  for (const registrant of registrants) {
    const key = normalizeName(registrant.name);
    if (key !== "" && !byName.has(key)) byName.set(key, registrant);
    nameById.set(registrant.id, registrant.name);
  }

  const neighbor = (id: string): string | undefined =>
    id === NO_ID ? undefined : nameById.get(id);

  const seatMeta = new Map<string, SeatRegistration>();
  for (const seat of seats) {
    if (seat.name === undefined || seat.name === "") continue;
    const registrant = byName.get(normalizeName(seat.name));
    if (!registrant) continue;
    seatMeta.set(seat.id, {
      viaShared: registrant.viaSharedDevice,
      leftName: neighbor(registrant.leftId),
      rightName: neighbor(registrant.rightId),
    });
  }
  return seatMeta;
}

export type NameSourceInput = {
  /** The panel's latest report, or null if it never reported (or was cleared). */
  registrants: BagRegistrants | null;
  /** The game on screen. `""` while none is loaded. */
  gameId: string;
  /** Only a game still in setup has a Token Bag panel to drive its names. */
  isSetup: boolean;
  presetNames: readonly string[];
};

export type NameSource = {
  /** True when the Token Bag owns the name list (presets step aside). */
  bagActive: boolean;
  /** The names the assignment UI should offer. */
  names: string[];
};

/**
 * Resolves the name list for the page as it stands.
 *
 * Falls back to the presets — i.e. exactly the behavior from before the Token Bag
 * existed — whenever the report cannot apply: no report, a report for a different
 * game, a game that has left setup, or an empty bag.
 */
export function deriveNameSource(input: NameSourceInput): NameSource {
  const { registrants, gameId, isSetup, presetNames } = input;

  const presets = (): NameSource => ({
    bagActive: false,
    names: [...presetNames],
  });

  if (!isSetup || gameId === "") return presets();
  if (registrants === null || registrants.gameId !== gameId) return presets();
  if (registrants.names.length === 0) return presets();

  return { bagActive: true, names: [...registrants.names] };
}
