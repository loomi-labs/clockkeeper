// Seating-order derivation and neighbour/adjacency counting for night helpers.
//
// The grimoire stores each seat at a free-form (x, y) position, so the seating
// order around the table is derived from the geometry: sort by angle around the
// centroid. When positions are missing or degenerate (all stacked on the
// centroid, e.g. a freshly created game before layout) we fall back to the
// caller-supplied id order.

import { angleFromPosition } from "~/lib/components/grimoire/layout";

export interface Point {
  x: number;
  y: number;
}

export type Alignment = "good" | "evil" | undefined;

/**
 * How a seat registers to an alive-neighbour / adjacent-pair check.
 *
 * - `evil` / `good`: a definite alignment (from effective alignment).
 * - `ambiguous`: a Recluse (may register evil) or Spy (may register good) —
 *   counted in the upper bound of a range but never the lower bound.
 * - `unknown`: alignment not set (e.g. a traveller the ST has not aligned).
 */
export type Registration = "good" | "evil" | "ambiguous" | "unknown";

/** Positions closer than this to the centroid count as "on" the centroid. */
const DEGENERATE_EPSILON = 1e-6;

function compareIds(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

/**
 * Return the players in clockwise seating order.
 *
 * Sorts `fallbackIds` by angle around the centroid of their positions,
 * tie-breaking by distance from the centroid then by id. If any id lacks a
 * position, or every position sits within `DEGENERATE_EPSILON` of the centroid
 * (degenerate/stacked layout), the `fallbackIds` order is returned unchanged.
 */
export function seatingOrder(
  positions: ReadonlyMap<string, Point>,
  fallbackIds: readonly string[],
): string[] {
  const ids = [...fallbackIds];
  const points = ids.map((id) => positions.get(id));
  if (points.some((p) => p === undefined)) return ids;

  const pts = points as Point[];
  const cx = pts.reduce((sum, p) => sum + p.x, 0) / pts.length;
  const cy = pts.reduce((sum, p) => sum + p.y, 0) / pts.length;

  const allDegenerate = pts.every(
    (p) => Math.hypot(p.x - cx, p.y - cy) < DEGENERATE_EPSILON,
  );
  if (allDegenerate) return ids;

  return ids
    .map((id, i) => ({
      id,
      angle: angleFromPosition(pts[i].x, pts[i].y, cx, cy),
      dist: Math.hypot(pts[i].x - cx, pts[i].y - cy),
    }))
    .sort(
      (a, b) => a.angle - b.angle || a.dist - b.dist || compareIds(a.id, b.id),
    )
    .map((e) => e.id);
}

/**
 * The nearest alive player in each direction around the circle, skipping the
 * dead. Returns `undefined` for a direction with no alive player (and both
 * `undefined` when `playerId` is not in `order`). With only two alive players
 * the single other alive player is returned as both `cw` and `ccw`.
 */
export function aliveNeighbours(
  order: readonly string[],
  playerId: string,
  dead: ReadonlySet<string>,
): { cw: string | undefined; ccw: string | undefined } {
  const n = order.length;
  const idx = order.indexOf(playerId);
  if (idx < 0) return { cw: undefined, ccw: undefined };

  const walk = (dir: 1 | -1): string | undefined => {
    for (let k = 1; k < n; k++) {
      const j = (((idx + dir * k) % n) + n) % n;
      const id = order[j];
      if (id === playerId) continue;
      if (!dead.has(id)) return id;
    }
    return undefined;
  };

  return { cw: walk(1), ccw: walk(-1) };
}

/**
 * Count how many of a player's alive neighbours register as evil (Empath).
 *
 * Returns a range: `min` counts neighbours that definitely register evil, while
 * `max` additionally counts `ambiguous` neighbours (a Recluse may register evil,
 * a Spy may register good — each check is independent). When `min === max` the
 * reading is exact.
 *
 * `unknown` is true when at least one alive neighbour has an `unknown`
 * registration (e.g. a traveller whose alignment the ST has not set), meaning
 * even the range cannot be trusted. When only two players are alive the single
 * alive neighbour is counted once, not twice.
 */
export function countEvilAliveNeighbours(
  order: readonly string[],
  playerId: string,
  dead: ReadonlySet<string>,
  registrationOf: (id: string) => Registration,
): { min: number; max: number; unknown: boolean } {
  const { cw, ccw } = aliveNeighbours(order, playerId, dead);
  const neighbours = new Set<string>();
  if (cw) neighbours.add(cw);
  if (ccw) neighbours.add(ccw);

  let min = 0;
  let max = 0;
  let unknown = false;
  for (const id of neighbours) {
    const r = registrationOf(id);
    if (r === "evil") {
      min++;
      max++;
    } else if (r === "ambiguous") {
      max++;
    } else if (r === "unknown") {
      unknown = true;
    }
  }
  return { min, max, unknown };
}

/**
 * Count pairs of adjacent evil-registering players around the alive circle (Chef).
 *
 * Adjacency wraps around, and three evil players in a row form two pairs. Dead
 * players are removed before adjacency is considered. Returns a range: `min`
 * counts pairs where both members definitely register evil, while `max` counts
 * pairs where both members register evil-or-`ambiguous`. `unknown` is true when
 * any alive player has an `unknown` registration (the true count may differ).
 */
export function countAdjacentEvilPairs(
  order: readonly string[],
  dead: ReadonlySet<string>,
  registrationOf: (id: string) => Registration,
): { min: number; max: number; unknown: boolean } {
  const alive = order.filter((id) => !dead.has(id));
  const m = alive.length;
  const unknown = alive.some((id) => registrationOf(id) === "unknown");
  const isEvil = (id: string) => registrationOf(id) === "evil";
  const mayEvil = (id: string) => {
    const r = registrationOf(id);
    return r === "evil" || r === "ambiguous";
  };

  if (m < 2) return { min: 0, max: 0, unknown };
  // Two alive players are adjacent exactly once (avoid double-counting the wrap).
  if (m === 2) {
    return {
      min: isEvil(alive[0]) && isEvil(alive[1]) ? 1 : 0,
      max: mayEvil(alive[0]) && mayEvil(alive[1]) ? 1 : 0,
      unknown,
    };
  }

  let min = 0;
  let max = 0;
  for (let i = 0; i < m; i++) {
    const a = alive[i];
    const b = alive[(i + 1) % m];
    if (isEvil(a) && isEvil(b)) min++;
    if (mayEvil(a) && mayEvil(b)) max++;
  }
  return { min, max, unknown };
}
