// Who owns the player-name list on the game page: the Token Bag, or the
// Storyteller's saved presets.
//
// The Token Bag panel only exists in the setup tab of a game still in setup, so
// what it last reported can outlive the situation it was reported for — the game
// starting, or the Storyteller navigating to another game. Deciding from the
// report ALONE would leave those stale names driving the assignment UI, which is
// why the decision is a function of the report plus the state of the page that is
// on screen right now.

import { normalizeName } from "./tokenbag";

/** What the Token Bag panel reports up to the page. */
export type BagRegistrants = {
  /** Registered names, in join order. */
  names: readonly string[];
  /**
   * Before the reveal, the server matches registrations to roles BY NAME, so a
   * seat rename would silently break the match. Once revealed it has persisted
   * `assigned_role_id` per registration, and renaming is harmless again.
   */
  prereveal: boolean;
  /** The game this report is about, so a stale one cannot leak into another. */
  gameId: string;
};

export type NameSourceInput = {
  /** The panel's latest report, or null if it never reported (or was cleared). */
  registrants: BagRegistrants | null;
  /** The game on screen. `""` while none is loaded. */
  gameId: string;
  /** Only a game still in setup has a Token Bag panel to drive its names. */
  isSetup: boolean;
  presetNames: readonly string[];
  /** Grimoire seats: role id -> player name. */
  grimoireNames: ReadonlyMap<string, string>;
};

export type NameSource = {
  /** True when the Token Bag owns the name list (presets step aside). */
  bagActive: boolean;
  /** The names the assignment UI should offer. */
  names: string[];
  /** Seats whose name must not be edited here. Empty unless the bag owns them. */
  renameLockedIds: Set<string>;
};

/**
 * Resolves the name list and the rename lock for the page as it stands.
 *
 * Falls back to the presets — i.e. exactly the behavior from before the Token Bag
 * existed — whenever the report cannot apply: no report, a report for a different
 * game, a game that has left setup, or an empty bag.
 */
export function deriveNameSource(input: NameSourceInput): NameSource {
  const { registrants, gameId, isSetup, presetNames, grimoireNames } = input;

  const presets = (): NameSource => ({
    bagActive: false,
    names: [...presetNames],
    renameLockedIds: new Set<string>(),
  });

  if (!isSetup || gameId === "") return presets();
  if (registrants === null || registrants.gameId !== gameId) return presets();
  if (registrants.names.length === 0) return presets();

  const names = [...registrants.names];
  const renameLockedIds = new Set<string>();
  if (registrants.prereveal) {
    const registered = new Set(names.map(normalizeName));
    for (const [roleId, name] of grimoireNames) {
      if (registered.has(normalizeName(name))) renameLockedIds.add(roleId);
    }
  }
  return { bagActive: true, names, renameLockedIds };
}
