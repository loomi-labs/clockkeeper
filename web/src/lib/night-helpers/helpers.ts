// Per-character night-helper computations (Empath, Chef, Fortune Teller,
// Undertaker). Pure logic only — UI lives in the night-helper components.

import type { Phase } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
import { DeathCause, Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
import { countAdjacentEvilPairs, countEvilAliveNeighbours } from "./seating";

/** Night-scoped view of a player, everything the helpers need. */
export interface HelperPlayer {
  id: string;
  name: string;
  characterId: string;
  characterName: string;
  team: Team;
  edition: string;
  isDead: boolean;
  alignment: "good" | "evil" | undefined;
}

function deadSet(players: ReadonlyMap<string, HelperPlayer>): Set<string> {
  const s = new Set<string>();
  for (const [id, p] of players) if (p.isDead) s.add(id);
  return s;
}

/**
 * Empath: number of the Empath's alive neighbours that are evil.
 * Returns `undefined` when the Empath is missing or dead (no reading).
 */
export function computeEmpath(
  order: readonly string[],
  players: ReadonlyMap<string, HelperPlayer>,
  empathPlayerId: string,
): { count: number; unknown: boolean } | undefined {
  const empath = players.get(empathPlayerId);
  if (!empath || empath.isDead) return undefined;
  return countEvilAliveNeighbours(
    order,
    empathPlayerId,
    deadSet(players),
    (id) => players.get(id)?.alignment,
  );
}

/**
 * Chef: number of pairs of evil players sitting adjacent around the circle.
 */
export function computeChef(
  order: readonly string[],
  players: ReadonlyMap<string, HelperPlayer>,
): { pairs: number; unknown: boolean } {
  return countAdjacentEvilPairs(
    order,
    deadSet(players),
    (id) => players.get(id)?.alignment,
  );
}

/**
 * Fortune Teller: does either of the two picked players register as the Demon?
 *
 * Returns `undefined` until two players are picked. The answer is "yes" when a
 * pick's base team is Demon, or when a pick is the Fortune Teller's Red Herring.
 * `viaRedHerring` flags a "yes" caused *solely* by the red herring (a false
 * positive with no actual Demon among the picks).
 */
export function computeFortuneTeller(
  picks: readonly string[],
  players: ReadonlyMap<string, HelperPlayer>,
  redHerringPlayerId: string | undefined,
): { answer: "yes" | "no"; viaRedHerring: boolean } | undefined {
  if (picks.length < 2) return undefined;

  const demonPicked = picks.some((id) => players.get(id)?.team === Team.DEMON);
  const redHerringPicked =
    redHerringPlayerId !== undefined && picks.includes(redHerringPlayerId);

  const yes = demonPicked || redHerringPicked;
  return {
    answer: yes ? "yes" : "no",
    viaRedHerring: yes && redHerringPicked && !demonPicked,
  };
}

/**
 * Undertaker: which role was executed on the previous day.
 *
 * Exact match: a day-phase death recorded with cause EXECUTION. Legacy rows
 * (recorded before `Death.cause` existed, cause UNSPECIFIED) fall back to a
 * heuristic — a role that appears in the day's deaths but NOT in the same
 * round's night deaths (night deaths are propagated forward into the day, so
 * subtracting them isolates the day's own executed player). The heuristic
 * result is flagged so the UI can caveat it.
 */
export function findExecutedToday(
  prevRound: { night?: Phase; day?: Phase } | undefined,
): { roleId: string; heuristic: boolean } | undefined {
  const day = prevRound?.day;
  if (!day) return undefined;

  const dayDeaths = day.deaths ?? [];

  const exact = dayDeaths.find((d) => d.cause === DeathCause.EXECUTION);
  if (exact) return { roleId: exact.roleId, heuristic: false };

  const nightRoleIds = new Set(
    (prevRound?.night?.deaths ?? []).map((d) => d.roleId),
  );
  const legacy = dayDeaths.find(
    (d) => d.cause === DeathCause.UNSPECIFIED && !nightRoleIds.has(d.roleId),
  );
  if (legacy) return { roleId: legacy.roleId, heuristic: true };

  return undefined;
}
