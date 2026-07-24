// Effective-alignment resolution for a role in a given phase.
//
// `defaultAlignmentForTeam` is the single source of truth for the team ->
// alignment default (replicated from NightOrder.svelte's former local copy,
// which is to be deleted and import from here during integration).

import {
  Team,
  TravellerAlignment,
} from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";

/**
 * Default alignment implied by a character's team, before any override.
 * Travellers, Fabled and Loric have no default (their alignment is set
 * explicitly by the storyteller).
 */
export function defaultAlignmentForTeam(
  team: Team,
): "good" | "evil" | undefined {
  if (team === Team.TOWNSFOLK || team === Team.OUTSIDER) return "good";
  if (team === Team.MINION || team === Team.DEMON) return "evil";
  return undefined; // Travellers, Fabled, Loric
}

/**
 * Resolve a role's alignment for the current phase.
 *
 * Precedence: an explicit phase override wins, then a traveller alignment
 * fallback, then the team default.
 */
export function effectiveAlignment(
  roleId: string,
  team: Team,
  phaseAlignments: ReadonlyMap<string, string> | undefined,
  travellerAlignments: Record<string, TravellerAlignment> | undefined,
): "good" | "evil" | undefined {
  const override = phaseAlignments?.get(roleId);
  if (override === "good" || override === "evil") return override;

  const traveller = travellerAlignments?.[roleId];
  if (traveller === TravellerAlignment.GOOD) return "good";
  if (traveller === TravellerAlignment.EVIL) return "evil";

  return defaultAlignmentForTeam(team);
}
