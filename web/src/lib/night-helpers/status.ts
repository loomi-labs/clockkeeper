// Poisoned / drunk derivation from reminder tokens attached to players.
//
// SINGLE SOURCE OF TRUTH ESCAPE HATCH (docs/development-guidelines.md):
// Normally computed state lives on the backend. Poisoned/drunk status is the
// exception: it is derived here on the frontend because it is read purely from
// reminder-token *attachments*, which are client-owned, opaque UI state
// (the storyteller drags tokens onto seats in the grimoire). There is no
// server-side notion of "this player is poisoned" — only the tokens the ST
// placed. Deriving it here keeps the badges in lock-step with the local
// grimoire (no debounce/round-trip lag) and avoids inventing a status field
// the game rules deliberately leave to the storyteller.

/** Minimal shape of a reminder token needed to derive status. */
export interface StatusReminderInput {
  text: string;
  attachedTo?: string;
  characterName?: string;
}

/** Minimal shape of a bag substitution needed to alias derived status. */
export interface BagSubInput {
  characterId: string;
  causedById: string;
}

/** Derived status for a single player (keyed by role/player id). */
export interface PlayerStatus {
  poisoned: boolean;
  drunk: boolean;
  /** Names of the characters whose tokens caused the status. */
  sources: string[];
}

function normalize(text: string): string {
  return text.trim().toLowerCase();
}

/** True when a reminder token marks its player as poisoned. */
export function isPoisonReminderText(text: string): boolean {
  return normalize(text) === "poisoned";
}

/**
 * True when a reminder token marks its player as drunk.
 *
 * Matches the "Drunk" family of tokens: `"Drunk"`, `"Drunk 1/2/3"` and the
 * Drunk role's own `"Is The Drunk"`. Bag substitutions synthesize the text
 * `` `Is the ${causedByName}` `` (e.g. "Is the Drunk") — matched here
 * case-insensitively. The global Fabled token `"Everyone Is Drunk"` is
 * explicitly NOT a per-player drunk marker and must not match.
 */
export function isDrunkReminderText(text: string): boolean {
  const t = normalize(text);
  if (t === "everyone is drunk") return false;
  return t === "drunk" || /^drunk \d+$/.test(t) || t === "is the drunk";
}

/**
 * Derive poisoned/drunk status per player from attached reminder tokens.
 *
 * Only reminders with `attachedTo` set contribute (loose/unattached tokens are
 * ignored). `sources` accumulates the causing character names.
 *
 * After building the base map, bag substitutions are aliased: a player who
 * drew a substitute character (e.g. the Drunk playing as the Empath) has their
 * status token attached under the underlying character id (`causedById`, the
 * Drunk), but their night-sheet row is keyed by the shown character id
 * (`characterId`, the Empath). Aliasing `characterId -> causedById` lets the
 * substituted row resolve to the same status object.
 */
export function derivePlayerStatuses(
  reminders: readonly StatusReminderInput[],
  bagSubstitutions: readonly BagSubInput[],
): Map<string, PlayerStatus> {
  const map = new Map<string, PlayerStatus>();

  for (const r of reminders) {
    if (!r.attachedTo) continue;
    const poisoned = isPoisonReminderText(r.text);
    const drunk = isDrunkReminderText(r.text);
    if (!poisoned && !drunk) continue;

    let entry = map.get(r.attachedTo);
    if (!entry) {
      entry = { poisoned: false, drunk: false, sources: [] };
      map.set(r.attachedTo, entry);
    }
    if (poisoned) entry.poisoned = true;
    if (drunk) entry.drunk = true;

    const src = r.characterName?.trim();
    if (src && !entry.sources.includes(src)) entry.sources.push(src);
  }

  // Alias bag-sub rows onto their underlying character's derived status.
  for (const bs of bagSubstitutions) {
    if (!bs.characterId || !bs.causedById) continue;
    const underlying = map.get(bs.causedById);
    if (underlying) map.set(bs.characterId, underlying);
  }

  return map;
}
