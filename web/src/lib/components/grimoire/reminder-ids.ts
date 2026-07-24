// Stable reminder-token ids, mirroring the Go scheme in
// internal/web/api_bagsub.go (stableReminderIDs / canonicalizeReminderKey).
//
// A reminder token's stable id is `reminder-${characterId}-${n}`, where n is
// the zero-based occurrence index of that characterId within the ordered token
// list. The token order (per character in selected-roles -> travellers ->
// extras, each character's `reminders` first then `remindersGlobal`) is
// produced server-side by buildReminderTokens; the client consumes it via
// `game.reminderTokens`. Because the id is derived from the character rather
// than the token's position, it is stable across changes to selected_roles.

const LEGACY_REMINDER_KEY_RE = /^reminder-(\d+)$/;

/**
 * Returns a stable id for each reminder token, occurrence-counted per
 * characterId in list order. Matches Go's stableReminderIDs exactly.
 */
export function stableReminderIds(
  tokens: ReadonlyArray<{ characterId: string }>,
): string[] {
  const counts = new Map<string, number>();
  const ids: string[] = [];
  for (const t of tokens) {
    const n = counts.get(t.characterId) ?? 0;
    counts.set(t.characterId, n + 1);
    ids.push(`reminder-${t.characterId}-${n}`);
  }
  return ids;
}

/**
 * Canonicalizes reminder-keyed state to stable ids.
 *
 * Input accepts either a plain record (as held in the proto maps, e.g.
 * `game.grimoireReminderAttachments`) or a Map (as held in local component
 * state). It always returns a Map so both call sites can use it uniformly.
 *
 * Key handling:
 * - Legacy positional keys `reminder-<n>` are remapped to `stableIds[n]`.
 *   Out-of-range indices reference tokens that no longer exist and are dropped.
 * - Stable keys `reminder-<charId>-<n>` and synthesized `bagsub-reminder-*`
 *   keys (and any other non-legacy key) pass through untouched.
 * - Collision: if a legacy key canonicalizes onto a stable id that is already
 *   present (a genuine stable key, or an earlier legacy key), the existing
 *   entry is kept (keep-existing-stable / keep-first).
 */
export function canonicalizeReminderKeys<T>(
  entries: ReadonlyMap<string, T> | Record<string, T>,
  tokens: ReadonlyArray<{ characterId: string }>,
): Map<string, T> {
  const stableIds = stableReminderIds(tokens);
  const result = new Map<string, T>();
  const legacy: Array<{ index: number; value: T }> = [];

  // Pass 1: pass-through keys (stable, bagsub, anything non-legacy) are
  // authoritative and added first so legacy keys cannot clobber them.
  for (const [key, value] of iterateEntries(entries)) {
    const m = LEGACY_REMINDER_KEY_RE.exec(key);
    if (m) {
      legacy.push({ index: Number(m[1]), value });
    } else {
      result.set(key, value);
    }
  }

  // Pass 2: legacy keys fill in only where their stable id is not taken.
  for (const { index, value } of legacy) {
    if (index < 0 || index >= stableIds.length) continue; // out of range: drop
    const id = stableIds[index];
    if (result.has(id)) continue; // keep existing stable / keep first
    result.set(id, value);
  }

  return result;
}

function iterateEntries<T>(
  entries: ReadonlyMap<string, T> | Record<string, T>,
): Iterable<[string, T]> {
  if (entries instanceof Map) return entries.entries();
  return Object.entries(entries as Record<string, T>);
}
