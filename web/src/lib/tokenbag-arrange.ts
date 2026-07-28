// Mapping between the token bag's seating answer and the grimoire.
//
// The server resolves neighbor picks into a clockwise ring of registration ids.
// Turning that into grimoire seats needs two hops — ids to registrant names, and
// names to the role ids the Storyteller assigned them to — and both are pure, so
// they live here rather than in the panel or the 3000-line game page.

import { normalizeName } from "./tokenbag";

/** The subset of a registrant this module needs. */
export type SeatedPlayer = {
  id: string;
  name: string;
};

/**
 * Projects the server's clockwise `ordered_registration_ids` onto registrant
 * names, in the same order.
 *
 * Ids cross the wire as int64, so `bigint` is accepted and normalized the same
 * way `toBagPlayer` normalizes them. Ids with no matching registrant are
 * dropped: a snapshot that arrived after the seating call could have lost a
 * registrant, and a ring with a hole in it is still worth arranging.
 */
export function seatingNames(
  orderedRegistrationIds: readonly (bigint | string)[],
  players: readonly SeatedPlayer[],
): string[] {
  const byId = new Map(players.map((player) => [player.id, player.name]));
  const names: string[] = [];
  for (const id of orderedRegistrationIds) {
    const name = byId.get(String(id));
    if (name !== undefined && name !== "") names.push(name);
  }
  return names;
}

/**
 * Resolves seat-ordered registrant names to grimoire role ids.
 *
 * `grimoireNames` is the page's role-id -> player-name map (the same shape as
 * `game.grimoirePlayerNames`). Matching is normalized, so a registrant who typed
 * "ana  b" is found in a seat the Storyteller labelled "Ana B".
 *
 * All-or-nothing: any name without a seat comes back as `{missing}` and the
 * caller must not arrange anything, because a partial ring would move some
 * players and silently leave others where they were — worse than not arranging.
 *
 * Duplicate grimoire names (possible for free-text seat names, not for bag
 * registrants, whose uniqueness the server enforces) resolve to the first seat
 * in map order.
 */
export function mapSeatingToRoles(
  orderedNames: readonly string[],
  grimoireNames: ReadonlyMap<string, string>,
): { roleIds: string[] } | { missing: string[] } {
  const roleByName = new Map<string, string>();
  for (const [roleId, name] of grimoireNames) {
    const key = normalizeName(name);
    if (key === "") continue;
    if (!roleByName.has(key)) roleByName.set(key, roleId);
  }

  const roleIds: string[] = [];
  const missing: string[] = [];
  for (const name of orderedNames) {
    const roleId = roleByName.get(normalizeName(name));
    if (roleId === undefined) missing.push(name);
    else roleIds.push(roleId);
  }

  return missing.length > 0 ? { missing } : { roleIds };
}

/**
 * The full list of seats to lay out on the circle: the ring the players picked,
 * followed by every other in-play seat the ring did not cover.
 *
 * Without the tail, `circleLayout` would only space the seats it was given and
 * the rest would keep whatever position they had — usually the same default
 * spot, so several tokens end up stacked on top of each other. Appending them
 * gives every seat its own place on the circle; the picked ring still comes
 * first, so the players' own order is what the arrangement follows.
 *
 * Duplicates are dropped (two grimoire seats can carry the same name, which
 * `mapSeatingToRoles` resolves to one role id) so no seat silently loses its
 * position to a later one.
 */
export function seatOrderForLayout(
  mappedRoleIds: readonly string[],
  inPlayRoleIds: readonly string[],
): string[] {
  const seen = new Set<string>();
  const order: string[] = [];
  for (const roleId of [...mappedRoleIds, ...inPlayRoleIds]) {
    if (roleId === "" || seen.has(roleId)) continue;
    seen.add(roleId);
    order.push(roleId);
  }
  return order;
}
