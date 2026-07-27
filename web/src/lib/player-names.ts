// Pure helpers for assigning preset / free-text player names to seats.
//
// Seats are keyed by role id -> name (the same shape as
// `game.grimoirePlayerNames` and the local `grimoireNames` map in the game
// page). These functions never mutate their inputs; they always return a NEW
// Map so callers can assign the result back to reactive state.
//
// The behavior here mirrors the logic that previously lived inline in
// `web/src/routes/games/[id]/+page.svelte` (`assignNameToPlayer` and
// `handleAssignPresets`); it is extracted so it can be unit-tested and reused.

/**
 * Assign `name` to `playerId`, returning a new map.
 *
 * - Empty / whitespace-only `name` unassigns the seat (deletes the key).
 * - If `name` is one of `presetNames` and is already assigned to a different
 *   seat, that other seat is unassigned first (duplicate-steal) so a preset
 *   name is only ever held by one seat at a time.
 * - Free-text names (not in `presetNames`) may be duplicated across seats.
 */
export function assignNameInMap(
  map: ReadonlyMap<string, string>,
  playerId: string,
  name: string,
  presetNames: readonly string[],
): Map<string, string> {
  const next = new Map(map);

  // Empty / whitespace name -> unassign this seat.
  if (name.trim() === "") {
    next.delete(playerId);
    return next;
  }

  // Preset names are unique across seats: steal from the first other holder.
  if (presetNames.includes(name)) {
    for (const [id, existingName] of next) {
      if (existingName === name && id !== playerId) {
        next.delete(id);
        break;
      }
    }
  }

  next.set(playerId, name);
  return next;
}

/**
 * Unassign a seat, returning a new map with `playerId` removed.
 */
export function unassignName(
  map: ReadonlyMap<string, string>,
  playerId: string,
): Map<string, string> {
  const next = new Map(map);
  next.delete(playerId);
  return next;
}

/**
 * Assign preset names to seats in order, returning a new map.
 *
 * Iterates in parallel over `playerIds` and `presetNames`, stopping at
 * whichever list is shorter. Existing entries for seats beyond the assigned
 * range are preserved.
 *
 * Each assigned name is also removed from any OTHER seat first, so a preset
 * name is only ever held by a single seat — otherwise a name previously
 * assigned by hand to a seat outside `playerIds` would linger as a duplicate.
 *
 * Seats in `locked` are pinned: they are skipped as assignment targets (keeping
 * whatever name they already hold), the names they hold are withheld from the
 * name pool, and the duplicate-steal never unassigns them.
 */
export function assignInOrder(
  map: ReadonlyMap<string, string>,
  playerIds: readonly string[],
  presetNames: readonly string[],
  locked: ReadonlySet<string> = new Set(),
): Map<string, string> {
  const next = new Map(map);
  // Locked seats drop out of the targets; their names drop out of the pool.
  const targets = playerIds.filter((id) => !locked.has(id));
  const lockedNames = new Set<string>();
  for (const id of locked) {
    const name = next.get(id);
    if (name !== undefined) lockedNames.add(name);
  }
  const pool = presetNames.filter((name) => !lockedNames.has(name));
  const count = Math.min(targets.length, pool.length);
  for (let i = 0; i < count; i++) {
    const playerId = targets[i];
    const name = pool[i];
    // Drop this name from any other unlocked seat before claiming it here.
    for (const [id, existingName] of next) {
      if (existingName === name && id !== playerId && !locked.has(id)) {
        next.delete(id);
      }
    }
    next.set(playerId, name);
  }
  return next;
}

/**
 * Rename every seat assignment whose value equals `oldName` to `newName`,
 * returning a new map.
 *
 * - `newName` is trimmed. A whitespace-only / empty `newName` unassigns the
 *   matching seats instead (deletes their keys).
 * - Seats not currently holding `oldName` are left untouched, so this is a
 *   no-op when `oldName` is absent.
 */
export function renameAssignedName(
  map: ReadonlyMap<string, string>,
  oldName: string,
  newName: string,
): Map<string, string> {
  const next = new Map(map);
  const trimmed = newName.trim();
  for (const [id, existingName] of next) {
    if (existingName !== oldName) continue;
    if (trimmed === "") {
      next.delete(id);
    } else {
      next.set(id, trimmed);
    }
  }
  return next;
}

/**
 * Returns a new array with the elements of `arr` in a uniformly random order
 * (Fisher-Yates). The input is never mutated. Used to randomize which preset
 * name lands on which seat before delegating to {@link assignInOrder}.
 *
 * `Math.random` is intentional here — this is UI convenience shuffling, not a
 * security- or reproducibility-sensitive path.
 */
export function shuffled<T>(arr: readonly T[]): T[] {
  const result = [...arr];
  for (let i = result.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [result[i], result[j]] = [result[j], result[i]];
  }
  return result;
}
