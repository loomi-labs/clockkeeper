// Pure validation + hint logic for reassigning a bag substitution (today only
// the Drunk) by dragging its synthesized `bagsub-reminder-*` token onto another
// seat. The backend (ReassignBagSubstitution) enforces the same rules; this is
// the client-side gate that decides whether a drop confirms, plain re-attaches,
// or bounces back with a hint.

import { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";

export type BagSubVerdict = "ok" | "self" | "not-in-play" | "wrong-team";

/**
 * Decides whether the bag-sub token may be reassigned onto `targetId`.
 *
 * - "self": dropped back on the seat that already causes the substitution.
 * - "not-in-play": target role isn't among the selected roles (e.g. a
 *   traveller seat) so it cannot receive the substitution.
 * - "wrong-team": target's real team differs from the required team (the
 *   substitution's shown character must remain plausible, e.g. the Drunk can
 *   only appear on a Townsfolk seat).
 * - "ok": a valid reassignment target.
 */
export function bagSubDropTarget(
  targetId: string,
  causedById: string,
  selectedRoleIds: ReadonlySet<string>,
  teamById: (id: string) => Team | undefined,
  requiredTeam: Team,
): BagSubVerdict {
  if (targetId === causedById) return "self";
  if (!selectedRoleIds.has(targetId)) return "not-in-play";
  if (teamById(targetId) !== requiredTeam) return "wrong-team";
  return "ok";
}

/**
 * A short, human-readable explanation for a non-actionable drop. Returns the
 * empty string for verdicts that need no hint ("ok" confirms, "self" is a
 * harmless plain re-attach).
 */
export function bagSubDropHint(
  verdict: BagSubVerdict,
  causedByName: string,
  requiredTeamLabel: string,
): string {
  switch (verdict) {
    case "wrong-team":
      return `The ${causedByName} must think they're a ${requiredTeamLabel} — drop the token on a ${requiredTeamLabel} seat.`;
    case "not-in-play":
      return `That seat's role isn't in play — the ${causedByName} can only move to a seat holding a selected role.`;
    case "ok":
    case "self":
    default:
      return "";
  }
}
