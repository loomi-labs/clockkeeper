import { describe, it, expect } from "vitest";
import {
  seatingOrder,
  aliveNeighbours,
  countEvilAliveNeighbours,
  countAdjacentEvilPairs,
  type Point,
  type Alignment,
} from "./seating";

function posMap(entries: Record<string, Point>): Map<string, Point> {
  return new Map(Object.entries(entries));
}

function alignFrom(m: Record<string, Alignment>) {
  return (id: string): Alignment => m[id];
}

describe("seatingOrder", () => {
  it("orders a perfect circle by angle around the centroid", () => {
    // Cardinal points around origin.
    const positions = posMap({
      a: { x: 100, y: 0 }, // east, angle 0
      b: { x: 0, y: 100 }, // south, angle +pi/2
      c: { x: -100, y: 0 }, // west, angle +pi
      d: { x: 0, y: -100 }, // north, angle -pi/2
    });
    // Ascending atan2: north(-pi/2), east(0), south(pi/2), west(pi).
    expect(seatingOrder(positions, ["a", "b", "c", "d"])).toEqual([
      "d",
      "a",
      "b",
      "c",
    ]);
  });

  it("uses the centroid, not the origin (offset circle)", () => {
    const positions = posMap({
      p1: { x: 150, y: 50 }, // east of centroid (50,50)
      p2: { x: 50, y: 150 }, // south
      p3: { x: -50, y: 50 }, // west
      p4: { x: 50, y: -50 }, // north
    });
    expect(seatingOrder(positions, ["p1", "p2", "p3", "p4"])).toEqual([
      "p4",
      "p1",
      "p2",
      "p3",
    ]);
  });

  it("breaks angle ties by distance from centroid", () => {
    // a and b are both due-north of the centroid (same angle), a is closer.
    const positions = posMap({
      a: { x: 0, y: -10 },
      b: { x: 0, y: -20 },
      c: { x: 30, y: 0 },
      d: { x: -30, y: 0 },
    });
    expect(seatingOrder(positions, ["a", "b", "c", "d"])).toEqual([
      "a",
      "b",
      "c",
      "d",
    ]);
  });

  it("breaks full ties (same position) by id", () => {
    const positions = posMap({
      z: { x: 0, y: -10 },
      a: { x: 0, y: -10 },
      c: { x: 30, y: 5 },
    });
    const order = seatingOrder(positions, ["z", "a", "c"]);
    // z and a share angle+distance -> id order a before z.
    expect(order.indexOf("a")).toBeLessThan(order.indexOf("z"));
  });

  it("falls back to id order when positions are stacked (degenerate)", () => {
    const positions = posMap({
      a: { x: 5, y: 5 },
      b: { x: 5, y: 5 },
      c: { x: 5, y: 5 },
    });
    expect(seatingOrder(positions, ["a", "b", "c"])).toEqual(["a", "b", "c"]);
  });

  it("falls back to id order when any position is missing", () => {
    const positions = posMap({
      a: { x: 100, y: 0 },
      c: { x: -100, y: 0 },
    });
    expect(seatingOrder(positions, ["a", "b", "c"])).toEqual(["a", "b", "c"]);
  });

  it("returns an empty order for no players", () => {
    expect(seatingOrder(new Map(), [])).toEqual([]);
  });
});

describe("aliveNeighbours", () => {
  const order = ["a", "b", "c", "d", "e"];

  it("returns immediate neighbours when all alive", () => {
    expect(aliveNeighbours(order, "a", new Set())).toEqual({
      cw: "b",
      ccw: "e",
    });
    expect(aliveNeighbours(order, "c", new Set())).toEqual({
      cw: "d",
      ccw: "b",
    });
  });

  it("skips dead players in each direction", () => {
    expect(aliveNeighbours(order, "a", new Set(["b", "e"]))).toEqual({
      cw: "c",
      ccw: "d",
    });
  });

  it("returns undefined when no alive neighbour exists", () => {
    expect(aliveNeighbours(order, "a", new Set(["b", "c", "d", "e"]))).toEqual({
      cw: undefined,
      ccw: undefined,
    });
  });

  it("returns both undefined when the player is absent", () => {
    expect(aliveNeighbours(order, "zzz", new Set())).toEqual({
      cw: undefined,
      ccw: undefined,
    });
  });

  it("returns the same single player for both directions with two alive", () => {
    expect(aliveNeighbours(["a", "b", "c"], "a", new Set(["c"]))).toEqual({
      cw: "b",
      ccw: "b",
    });
  });
});

describe("countEvilAliveNeighbours", () => {
  const order = ["ft", "left", "right", "x", "y"];

  it("counts 0 / 1 / 2 evil neighbours", () => {
    expect(
      countEvilAliveNeighbours(order, "ft", new Set(), () => "good"),
    ).toEqual({ count: 0, unknown: false });

    expect(
      countEvilAliveNeighbours(
        order,
        "ft",
        new Set(),
        alignFrom({ left: "good", y: "evil" }),
      ),
    ).toEqual({ count: 1, unknown: false });

    expect(
      countEvilAliveNeighbours(
        order,
        "ft",
        new Set(),
        alignFrom({ left: "evil", y: "evil" }),
      ),
    ).toEqual({ count: 2, unknown: false });
  });

  it("skips dead neighbours to the next alive one", () => {
    // left is dead -> clockwise neighbour becomes 'right'.
    expect(
      countEvilAliveNeighbours(
        order,
        "ft",
        new Set(["left"]),
        alignFrom({ right: "evil", y: "good" }),
      ),
    ).toEqual({ count: 1, unknown: false });
  });

  it("flags unknown when a neighbour has undefined alignment", () => {
    expect(
      countEvilAliveNeighbours(
        order,
        "ft",
        new Set(),
        alignFrom({ left: undefined, y: "evil" }),
      ),
    ).toEqual({ count: 1, unknown: true });
  });

  it("counts a shared two-alive neighbour once", () => {
    // Only ft and x alive; x is both cw and ccw neighbour.
    expect(
      countEvilAliveNeighbours(
        ["ft", "x", "y"],
        "ft",
        new Set(["y"]),
        alignFrom({ x: "evil" }),
      ),
    ).toEqual({ count: 1, unknown: false });
  });
});

describe("countAdjacentEvilPairs", () => {
  it("counts adjacent evil pairs with wrap-around", () => {
    // Evil at index 0 and 3 (a,d) — adjacent via the wrap a<->d.
    const order = ["a", "b", "c", "d"];
    expect(
      countAdjacentEvilPairs(
        order,
        new Set(),
        alignFrom({ a: "evil", b: "good", c: "good", d: "evil" }),
      ),
    ).toEqual({ pairs: 1, unknown: false });
  });

  it("counts three-in-a-row as two pairs", () => {
    const order = ["a", "b", "c", "d", "e"];
    expect(
      countAdjacentEvilPairs(
        order,
        new Set(),
        alignFrom({ a: "evil", b: "evil", c: "evil", d: "good", e: "good" }),
      ),
    ).toEqual({ pairs: 2, unknown: false });
  });

  it("counts all-evil circle as one pair per seat", () => {
    const order = ["a", "b", "c", "d"];
    expect(countAdjacentEvilPairs(order, new Set(), () => "evil")).toEqual({
      pairs: 4,
      unknown: false,
    });
  });

  it("removes dead players before considering adjacency", () => {
    // a and c are evil but separated by b; killing b makes them adjacent.
    const order = ["a", "b", "c", "d"];
    expect(
      countAdjacentEvilPairs(
        order,
        new Set(["b"]),
        alignFrom({ a: "evil", c: "evil", d: "good" }),
      ),
    ).toEqual({ pairs: 1, unknown: false });
  });

  it("counts two alive evil players as a single pair", () => {
    const order = ["a", "b", "c"];
    expect(
      countAdjacentEvilPairs(
        order,
        new Set(["c"]),
        alignFrom({ a: "evil", b: "evil" }),
      ),
    ).toEqual({ pairs: 1, unknown: false });
  });

  it("flags unknown when an alive player has undefined alignment", () => {
    const order = ["a", "b", "c"];
    expect(
      countAdjacentEvilPairs(
        order,
        new Set(),
        alignFrom({ a: "evil", b: "evil", c: undefined }),
      ),
    ).toEqual({ pairs: 1, unknown: true });
  });
});
