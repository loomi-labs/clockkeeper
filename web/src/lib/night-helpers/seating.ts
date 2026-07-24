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
 * Count how many of a player's alive neighbours are evil (Empath).
 *
 * `unknown` is true when at least one alive neighbour has an undefined
 * alignment (e.g. a traveller whose alignment the ST has not set), meaning the
 * count cannot be trusted. When only two players are alive the single alive
 * neighbour is counted once, not twice.
 */
export function countEvilAliveNeighbours(
  order: readonly string[],
  playerId: string,
  dead: ReadonlySet<string>,
  alignmentOf: (id: string) => Alignment,
): { count: number; unknown: boolean } {
  const { cw, ccw } = aliveNeighbours(order, playerId, dead);
  const neighbours = new Set<string>();
  if (cw) neighbours.add(cw);
  if (ccw) neighbours.add(ccw);

  let count = 0;
  let unknown = false;
  for (const id of neighbours) {
    const a = alignmentOf(id);
    if (a === "evil") count++;
    else if (a === undefined) unknown = true;
  }
  return { count, unknown };
}

/**
 * Count pairs of adjacent evil players around the alive circle (Chef).
 *
 * Adjacency wraps around, and three evil players in a row form two pairs. Dead
 * players are removed before adjacency is considered. `unknown` is true when
 * any alive player has an undefined alignment (the true count may differ).
 */
export function countAdjacentEvilPairs(
  order: readonly string[],
  dead: ReadonlySet<string>,
  alignmentOf: (id: string) => Alignment,
): { pairs: number; unknown: boolean } {
  const alive = order.filter((id) => !dead.has(id));
  const m = alive.length;
  const unknown = alive.some((id) => alignmentOf(id) === undefined);
  const isEvil = (id: string) => alignmentOf(id) === "evil";

  if (m < 2) return { pairs: 0, unknown };
  // Two alive players are adjacent exactly once (avoid double-counting the wrap).
  if (m === 2) {
    return { pairs: isEvil(alive[0]) && isEvil(alive[1]) ? 1 : 0, unknown };
  }

  let pairs = 0;
  for (let i = 0; i < m; i++) {
    if (isEvil(alive[i]) && isEvil(alive[(i + 1) % m])) pairs++;
  }
  return { pairs, unknown };
}
