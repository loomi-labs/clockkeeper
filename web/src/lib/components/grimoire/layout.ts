export function circleLayout(
  count: number,
  centerX: number,
  centerY: number,
  radius: number,
): { x: number; y: number }[] {
  return Array.from({ length: count }, (_, i) => {
    const angle = (2 * Math.PI * i) / count - Math.PI / 2;
    return {
      x: centerX + radius * Math.cos(angle),
      y: centerY + radius * Math.sin(angle),
    };
  });
}

// Reminder token attachment constants
export const ORBIT_RADIUS = 76;
export const ATTACH_THRESHOLD = 80;
export const DETACH_THRESHOLD = 100;

export function orbitPosition(
  playerX: number,
  playerY: number,
  angle: number,
): { x: number; y: number } {
  return {
    x: playerX + ORBIT_RADIUS * Math.cos(angle),
    y: playerY + ORBIT_RADIUS * Math.sin(angle),
  };
}

export function distributeAngles(count: number): number[] {
  return Array.from(
    { length: count },
    (_, i) => (2 * Math.PI * i) / count - Math.PI / 2,
  );
}

export function angleFromPosition(
  px: number,
  py: number,
  cx: number,
  cy: number,
): number {
  return Math.atan2(py - cy, px - cx);
}

export function distanceBetween(
  x1: number,
  y1: number,
  x2: number,
  y2: number,
): number {
  return Math.hypot(x2 - x1, y2 - y1);
}

/**
 * The average of the given points, or `null` for an empty input.
 *
 * Same convention as `seatingOrder` in `~/lib/night-helpers/seating`: the middle
 * of the table is wherever the seats say it is, never a hard-coded origin, so a
 * circle the Storyteller dragged off-centre still rotates about its own middle.
 */
export function centroidOf(
  points: Iterable<{ x: number; y: number }>,
): { x: number; y: number } | null {
  let sumX = 0;
  let sumY = 0;
  let n = 0;
  for (const p of points) {
    sumX += p.x;
    sumY += p.y;
    n++;
  }
  if (n === 0) return null;
  return { x: sumX / n, y: sumY / n };
}

/**
 * Rotates every given position by `angleRadians` about `center` (the centroid of
 * the positions themselves when no centre is passed).
 *
 * Canvas y grows downwards, so a positive angle turns clockwise on screen —
 * matching `circleLayout`, which walks seat 0, 1, 2 … clockwise.
 *
 * Returns a new map and never mutates the input; an empty input and a single
 * point both come back unchanged (a lone point IS its own centroid).
 */
export function rotatePositions(
  positions:
    | ReadonlyMap<string, { x: number; y: number }>
    | Readonly<Record<string, { x: number; y: number }>>,
  angleRadians: number,
  center?: { x: number; y: number },
): Map<string, { x: number; y: number }> {
  const entries =
    positions instanceof Map
      ? [...(positions as ReadonlyMap<string, { x: number; y: number }>)]
      : Object.entries(
          positions as Readonly<Record<string, { x: number; y: number }>>,
        );

  const origin = center ?? centroidOf(entries.map(([, p]) => p));
  if (!origin) return new Map();

  const cos = Math.cos(angleRadians);
  const sin = Math.sin(angleRadians);
  const rotated = new Map<string, { x: number; y: number }>();
  for (const [id, p] of entries) {
    const dx = p.x - origin.x;
    const dy = p.y - origin.y;
    rotated.set(id, {
      x: origin.x + dx * cos - dy * sin,
      y: origin.y + dx * sin + dy * cos,
    });
  }
  return rotated;
}
