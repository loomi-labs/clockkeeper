// Per-character night-helper computations (Empath, Chef, Fortune Teller,
// Undertaker). Pure logic only — UI lives in the night-helper components.

import type { Phase } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
import { DeathCause, Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
import type { PromotionDisplay } from "~/lib/promotions";
import { countEvilAliveNeighbours, type Registration } from "./seating";

/** Everything needed to name and render a character token. */
export interface CharacterRef {
  id: string;
  name: string;
  edition: string;
  team: Team;
}

/** Night-scoped view of a player, everything the helpers need. */
export interface HelperPlayer {
  /**
   * Seat id, which in this data model is the REAL character id of the seat
   * (the role in play). For a bag-substituted seat this stays the underlying
   * role (e.g. `drunk`) while `characterId` becomes the shown token.
   */
  id: string;
  name: string;
  /**
   * The DISPLAYED character id (bag-sub aware). Differs from `id` only for a
   * substituted seat. Registration classification uses `id` (the real
   * character), never this, so an ability-hiding token cannot mask a Recluse/Spy.
   */
  characterId: string;
  characterName: string;
  team: Team;
  edition: string;
  isDead: boolean;
  alignment: "good" | "evil" | undefined;
  /**
   * The seat's TRUE current character — grimoire truth, never the bag-sub
   * facade. Use this for info a character *learns* about a seat (Undertaker,
   * Ravenkeeper): the Drunk is learned as the Drunk, not as the Townsfolk they
   * believe they are. Built with {@link resolveTrueCharacter}.
   */
  trueCharacter: CharacterRef;
}

/**
 * Resolve a seat's TRUE current character: the promotion's acts-as character
 * when the seat has been promoted (a star-passed Baron really IS the Imp now),
 * otherwise the seat's OWN role.
 *
 * `bagSub` is accepted and deliberately IGNORED — that is the whole point of
 * this function. A bag substitution is only a facade the player believes in (the
 * Drunk holding a Townsfolk token), so it must never leak into grimoire-truth
 * info: the Undertaker sees the executed player's real character and the
 * Ravenkeeper learns the real character. For players'-perspective displays
 * (grimoire tokens, first-night info) use the displayed character instead.
 */
export function resolveTrueCharacter(seat: {
  own: CharacterRef;
  promotion?: PromotionDisplay | undefined;
  bagSub?: { characterId?: string; characterName?: string } | undefined;
}): CharacterRef {
  const { own, promotion } = seat;
  if (promotion) {
    return {
      id: promotion.actsAsId,
      name: promotion.actsAsName,
      edition: promotion.actsAsEdition,
      team: promotion.actsAsTeam,
    };
  }
  return {
    id: own.id,
    name: own.name,
    edition: own.edition,
    team: own.team,
  };
}

/** Real character ids whose registration is ambiguous. */
export const RECLUSE_ID = "recluse";
export const SPY_ID = "spy";

/**
 * Classify how a seat registers to a Chef/Empath check.
 *
 * A Recluse (good outsider, "might register as evil") or Spy (evil minion,
 * "might register as good") is `ambiguous` regardless of its current effective
 * alignment — an alignment-flipped Recluse is still a Recluse. Everyone else is
 * `evil`/`good` by effective alignment, or `unknown` when alignment is unset.
 * Uses the REAL character id (`p.id`), never the displayed `characterId`.
 */
export function classifyRegistration(
  p: HelperPlayer | undefined,
): Registration {
  if (!p) return "unknown";
  if (p.id === RECLUSE_ID || p.id === SPY_ID) return "ambiguous";
  if (p.alignment === "evil") return "evil";
  if (p.alignment === "good") return "good";
  return "unknown";
}

function deadSet(players: ReadonlyMap<string, HelperPlayer>): Set<string> {
  const s = new Set<string>();
  for (const [id, p] of players) if (p.isDead) s.add(id);
  return s;
}

/**
 * Which ambiguous characters (Recluse/Spy) actually widen the Chef range — i.e.
 * are a member of at least one adjacent maybe-evil pair around the alive circle.
 * An ambiguous player with no evil-or-ambiguous alive neighbour does not affect
 * the count, so it is not reported.
 */
export function chefRangeCauses(
  order: readonly string[],
  players: ReadonlyMap<string, HelperPlayer>,
): { recluse: boolean; spy: boolean } {
  const dead = deadSet(players);
  const alive = order.filter((id) => !dead.has(id));
  const causes = { recluse: false, spy: false };
  const m = alive.length;
  if (m < 2) return causes;

  const reg = (id: string) => classifyRegistration(players.get(id));
  const mayEvil = (id: string) => {
    const r = reg(id);
    return r === "evil" || r === "ambiguous";
  };
  const mark = (id: string) => {
    if (reg(id) !== "ambiguous") return;
    const real = players.get(id)?.id;
    if (real === RECLUSE_ID) causes.recluse = true;
    else if (real === SPY_ID) causes.spy = true;
  };

  // Two alive players are adjacent exactly once (no wrap double-count).
  const pairs: Array<[string, string]> =
    m === 2
      ? [[alive[0], alive[1]]]
      : alive.map((id, i) => [id, alive[(i + 1) % m]]);
  for (const [a, b] of pairs) {
    if (mayEvil(a) && mayEvil(b)) {
      mark(a);
      mark(b);
    }
  }
  return causes;
}

/**
 * Empath: how many of the Empath's alive neighbours register as evil.
 *
 * Returns a `{min, max}` range (exact when `min === max`): `min` counts
 * definite-evil neighbours, `max` additionally counts `ambiguous` ones (a
 * neighbouring Recluse/Spy). Returns `undefined` when the Empath is missing or
 * dead (no reading). The Empath's own character is irrelevant.
 */
export function computeEmpath(
  order: readonly string[],
  players: ReadonlyMap<string, HelperPlayer>,
  empathPlayerId: string,
): { min: number; max: number; unknown: boolean } | undefined {
  const empath = players.get(empathPlayerId);
  if (!empath || empath.isDead) return undefined;
  return countEvilAliveNeighbours(
    order,
    empathPlayerId,
    deadSet(players),
    (id) => classifyRegistration(players.get(id)),
  );
}

/** An adjacent, maybe-evil pair around the alive circle (see {@link chefPairs}). */
export interface ChefPair {
  a: HelperPlayer;
  b: HelperPlayer;
  /**
   * `true` when the pair only counts if a Recluse/Spy registers accordingly —
   * i.e. at least one member is `ambiguous` (and the other is evil-or-ambiguous,
   * which every pair here satisfies). `false` when both members are definite
   * evils.
   */
  ambiguous: boolean;
}

/**
 * The alive-adjacent pairs a Chef counts: every pair of neighbours (dead
 * skipped, wrap-around) where both members register evil-or-`ambiguous`.
 *
 * This is the single source of truth for {@link computeChef} — `min` is the
 * count of non-`ambiguous` pairs, `max` is the total — so the reported count and
 * this list can never diverge. Two alive players are adjacent exactly once (the
 * wrap is not double-counted).
 */
export function chefPairs(
  order: readonly string[],
  players: ReadonlyMap<string, HelperPlayer>,
): ChefPair[] {
  const dead = deadSet(players);
  const alive = order.filter((id) => !dead.has(id));
  const m = alive.length;
  if (m < 2) return [];

  const reg = (id: string) => classifyRegistration(players.get(id));
  const isEvil = (id: string) => reg(id) === "evil";
  const mayEvil = (id: string) => {
    const r = reg(id);
    return r === "evil" || r === "ambiguous";
  };

  // Two alive players are adjacent exactly once (no wrap double-count).
  const indices: Array<[string, string]> =
    m === 2
      ? [[alive[0], alive[1]]]
      : alive.map((id, i) => [id, alive[(i + 1) % m]]);

  const pairs: ChefPair[] = [];
  for (const [aId, bId] of indices) {
    if (!mayEvil(aId) || !mayEvil(bId)) continue;
    const a = players.get(aId);
    const b = players.get(bId);
    if (!a || !b) continue;
    pairs.push({ a, b, ambiguous: !(isEvil(aId) && isEvil(bId)) });
  }
  return pairs;
}

/**
 * Chef: how many pairs of evil-registering players sit adjacent around the
 * circle. Returns a `{min, max}` range (exact when `min === max`): `min` counts
 * pairs of definite evils, `max` counts pairs where both members register
 * evil-or-`ambiguous` (e.g. evil/Recluse/evil ⇒ {min:0, max:2}). Both bounds are
 * derived from {@link chefPairs} so the count and the listed pairs stay in sync.
 */
export function computeChef(
  order: readonly string[],
  players: ReadonlyMap<string, HelperPlayer>,
): { min: number; max: number; unknown: boolean } {
  const dead = deadSet(players);
  const pairs = chefPairs(order, players);
  const unknown = order.some(
    (id) =>
      !dead.has(id) && classifyRegistration(players.get(id)) === "unknown",
  );
  return {
    min: pairs.filter((p) => !p.ambiguous).length,
    max: pairs.length,
    unknown,
  };
}

/**
 * The seats that register as the Demon to a Fortune Teller check — those whose
 * (night-scoped) team is Demon. This is the single source of truth for the
 * demon side of {@link computeFortuneTeller}, so the "which picks give YES"
 * detection and the demon display in the helper cannot diverge. Team is taken
 * as-is (never the displayed `characterId`), exactly as the Fortune Teller
 * computation treats it. Returns an empty array when no seat is a Demon.
 */
export function findDemonPlayers(
  players: ReadonlyMap<string, HelperPlayer>,
): HelperPlayer[] {
  return [...players.values()].filter((p) => p.team === Team.DEMON);
}

/**
 * Fortune Teller: does either of the two picked players register as the Demon?
 *
 * Returns `undefined` until two players are picked. The answer is "yes" when a
 * pick's base team is Demon, or when a pick is the Fortune Teller's Red Herring.
 * `viaRedHerring` flags a "yes" caused *solely* by the red herring (a false
 * positive with no actual Demon among the picks). `recluseMayYes` flags a "no"
 * that a picked Recluse could turn into a "yes" (the Recluse may register as the
 * Demon) — only ever true on a "no" answer.
 */
export function computeFortuneTeller(
  picks: readonly string[],
  players: ReadonlyMap<string, HelperPlayer>,
  redHerringPlayerId: string | undefined,
):
  | {
      answer: "yes" | "no";
      viaRedHerring: boolean;
      recluseMayYes: boolean;
    }
  | undefined {
  if (picks.length < 2) return undefined;

  const demonIds = new Set(findDemonPlayers(players).map((p) => p.id));
  const demonPicked = picks.some((id) => demonIds.has(id));
  const redHerringPicked =
    redHerringPlayerId !== undefined && picks.includes(redHerringPlayerId);

  const yes = demonPicked || redHerringPicked;
  const recluseMayYes =
    !yes && picks.some((id) => players.get(id)?.id === RECLUSE_ID);
  return {
    answer: yes ? "yes" : "no",
    viaRedHerring: yes && redHerringPicked && !demonPicked,
    recluseMayYes,
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

/**
 * Ravenkeeper: which role ids died for the FIRST time in tonight's phase.
 *
 * Deaths propagate forward (a night death reappears in later phases' `deaths`),
 * so a role that is in the current night's deaths but was NOT in the previous
 * round's day deaths is one that first died this night — i.e. died tonight.
 * On the first night `prevDayDeaths` is empty, so every night death qualifies.
 */
export function newDeathsTonight(
  nightDeaths: readonly { roleId: string }[],
  prevDayDeaths: readonly { roleId: string }[],
): Set<string> {
  const carried = new Set(prevDayDeaths.map((d) => d.roleId));
  const fresh = new Set<string>();
  for (const d of nightDeaths) if (!carried.has(d.roleId)) fresh.add(d.roleId);
  return fresh;
}

/**
 * Scarlet Woman: whether the "you are now the Demon" promotion alert should
 * show.
 *
 * Fires when every demon-team seat is dead (and there IS at least one), an alive
 * scarletwoman minion seat exists that has not already been promoted (a promoted
 * seat reads as team Demon, so an alive Demon-team Scarlet Woman fails the
 * "all demons dead" gate), and at least 4 non-Traveller seats are still alive.
 * The count is taken AFTER the Demon's death, so >= 4 alive ⇔ 5+ were alive when
 * the Demon died.
 */
export function scarletWomanPromotionAlert(
  players: ReadonlyMap<string, HelperPlayer>,
): boolean {
  const all = [...players.values()];

  const demons = all.filter((p) => p.team === Team.DEMON);
  if (demons.length === 0) return false;
  if (demons.some((p) => !p.isDead)) return false;

  const hasAliveScarletWoman = all.some(
    (p) => p.id === "scarletwoman" && p.team === Team.MINION && !p.isDead,
  );
  if (!hasAliveScarletWoman) return false;

  const aliveNonTravellers = all.filter(
    (p) => !p.isDead && p.team !== Team.TRAVELLER,
  ).length;
  return aliveNonTravellers >= 4;
}
