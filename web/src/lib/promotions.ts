// Role-promotion overlay — the pure mapping behind "acts as" seat overrides
// (Baron → Imp on a star pass, Scarlet Woman → Imp on Demon death, …).
//
// A game carries `rolePromotions: {roleId, actsAsRoleId}[]`. This module turns
// those pairs into per-original-role display info the page uses to override a
// promoted seat's team/character/edition/name (so a promoted Baron reads as
// "Imp (ex Baron)" and classifies as the Demon everywhere downstream).

import type { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";

/** A stored promotion: the original role now acts as another role. */
export interface PromotionInput {
  roleId: string;
  actsAsRoleId: string;
}

/** Resolved display info for a promoted seat, keyed later by the original role. */
export interface PromotionDisplay {
  actsAsId: string;
  actsAsName: string;
  actsAsEdition: string;
  actsAsTeam: Team;
  /** e.g. "Imp (ex Baron)" — the acts-as name plus the original role name. */
  label: string;
}

/**
 * Build a lookup from original role id -> how that seat now displays.
 *
 * The acts-as character (team/name/edition) is resolved from `charLookup`; the
 * original name (for the "(ex …)" suffix) is resolved the same way, falling back
 * to the raw role id when unknown. Entries whose acts-as character is missing
 * from the lookup are skipped defensively — a promotion we cannot display is
 * dropped rather than rendered half-built.
 */
export function buildPromotionsByRole(
  promos: readonly PromotionInput[],
  charLookup: ReadonlyMap<
    string,
    { id: string; name: string; edition: string; team: Team }
  >,
): Map<string, PromotionDisplay> {
  const byRole = new Map<string, PromotionDisplay>();
  for (const { roleId, actsAsRoleId } of promos) {
    const actsAs = charLookup.get(actsAsRoleId);
    if (!actsAs) continue; // Cannot display an unknown acts-as character.
    const originalName = charLookup.get(roleId)?.name ?? roleId;
    byRole.set(roleId, {
      actsAsId: actsAs.id,
      actsAsName: actsAs.name,
      actsAsEdition: actsAs.edition,
      actsAsTeam: actsAs.team,
      label: `${actsAs.name} (ex ${originalName})`,
    });
  }
  return byRole;
}
