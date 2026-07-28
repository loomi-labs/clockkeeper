import { describe, it, expect } from "vitest";
import { centroidOf, rotatePositions } from "./layout";

type Point = { x: number; y: number };

/** Positions compare to ~9 decimals: cos(pi/2) is 6.1e-17, not 0. */
function expectPoints(
  actual: ReadonlyMap<string, Point>,
  expected: Record<string, Point>,
) {
  expect([...actual.keys()].sort()).toEqual(Object.keys(expected).sort());
  for (const [id, want] of Object.entries(expected)) {
    const got = actual.get(id);
    expect(got, `missing ${id}`).toBeDefined();
    expect(got!.x, `${id}.x`).toBeCloseTo(want.x, 9);
    expect(got!.y, `${id}.y`).toBeCloseTo(want.y, 9);
  }
}

describe("centroidOf", () => {
  it("averages the points", () => {
    expect(
      centroidOf([
        { x: 0, y: 0 },
        { x: 10, y: 20 },
      ]),
    ).toEqual({ x: 5, y: 10 });
  });

  it("returns null for no points", () => {
    expect(centroidOf([])).toBeNull();
  });
});

describe("rotatePositions", () => {
  // A square whose corners are one quarter-turn apart: rotating by 90 degrees
  // has to land every seat exactly on the next seat's spot.
  const square: Record<string, Point> = {
    n: { x: 0, y: -100 },
    e: { x: 100, y: 0 },
    s: { x: 0, y: 100 },
    w: { x: -100, y: 0 },
  };

  it("maps a 4-seat square onto itself when rotated a quarter turn", () => {
    // Canvas y grows downwards, so +90 degrees goes north -> east -> south -> west.
    expectPoints(rotatePositions(square, Math.PI / 2), {
      n: square.e,
      e: square.s,
      s: square.w,
      w: square.n,
    });
  });

  it("rotates the other way for a negative angle", () => {
    expectPoints(rotatePositions(square, -Math.PI / 2), {
      n: square.w,
      e: square.n,
      s: square.e,
      w: square.s,
    });
  });

  it("keeps the centroid fixed, wherever the circle sits", () => {
    // Same square translated to (400, 250) — a circle dragged off-centre.
    const offset = new Map<string, Point>(
      Object.entries(square).map(([id, p]) => [
        id,
        { x: p.x + 400, y: p.y + 250 },
      ]),
    );
    const rotated = rotatePositions(offset, Math.PI / 3);
    const before = centroidOf(offset.values())!;
    const after = centroidOf(rotated.values())!;
    expect(after.x).toBeCloseTo(before.x, 9);
    expect(after.y).toBeCloseTo(before.y, 9);
    // Each seat keeps its distance from that centroid.
    for (const [id, p] of rotated) {
      const from = offset.get(id)!;
      expect(Math.hypot(p.x - after.x, p.y - after.y)).toBeCloseTo(
        Math.hypot(from.x - before.x, from.y - before.y),
        9,
      );
    }
  });

  it("accepts an explicit centre", () => {
    // Rotating a single point about the origin, not about itself.
    expectPoints(
      rotatePositions({ a: { x: 10, y: 0 } }, Math.PI, { x: 0, y: 0 }),
      { a: { x: -10, y: 0 } },
    );
  });

  it("is a no-op for a single seat (it is its own centroid)", () => {
    expectPoints(rotatePositions({ only: { x: 42, y: -7 } }, Math.PI / 4), {
      only: { x: 42, y: -7 },
    });
  });

  it("returns an empty map for no seats", () => {
    expect(rotatePositions(new Map(), Math.PI).size).toBe(0);
    expect(rotatePositions({}, Math.PI).size).toBe(0);
  });

  it("does not mutate the input map", () => {
    const input = new Map<string, Point>([["a", { x: 100, y: 0 }]]);
    rotatePositions(input, Math.PI / 2, { x: 0, y: 0 });
    expect(input.get("a")).toEqual({ x: 100, y: 0 });
  });
});
