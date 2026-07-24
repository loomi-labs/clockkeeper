import { describe, it, expect } from "vitest";
import { assignNameInMap, unassignName, assignInOrder } from "./player-names";

const PRESETS = ["Alice", "Bob", "Carol"] as const;

describe("assignNameInMap", () => {
  it("assigns a name to a seat", () => {
    const result = assignNameInMap(new Map(), "s1", "Alice", PRESETS);
    expect(result.get("s1")).toBe("Alice");
  });

  it("steals a preset name from another seat (duplicate-steal)", () => {
    const map = new Map([["s1", "Alice"]]);
    const result = assignNameInMap(map, "s2", "Alice", PRESETS);
    expect(result.has("s1")).toBe(false);
    expect(result.get("s2")).toBe("Alice");
  });

  it("only steals from the first other holder of a preset name", () => {
    const map = new Map([
      ["s1", "Alice"],
      ["s2", "Alice"],
    ]);
    const result = assignNameInMap(map, "s3", "Alice", PRESETS);
    // First match (s1) stolen, s2 left as-is (matches original break behavior).
    expect(result.has("s1")).toBe(false);
    expect(result.get("s2")).toBe("Alice");
    expect(result.get("s3")).toBe("Alice");
  });

  it("does not steal when re-assigning the same name to the same seat", () => {
    const map = new Map([["s1", "Alice"]]);
    const result = assignNameInMap(map, "s1", "Alice", PRESETS);
    expect(result.get("s1")).toBe("Alice");
    expect(result.size).toBe(1);
  });

  it("allows free-text (non-preset) name duplicates across seats", () => {
    const map = new Map([["s1", "Guest"]]);
    const result = assignNameInMap(map, "s2", "Guest", PRESETS);
    expect(result.get("s1")).toBe("Guest");
    expect(result.get("s2")).toBe("Guest");
  });

  it("empty name unassigns the seat (deletes the key)", () => {
    const map = new Map([["s1", "Alice"]]);
    const result = assignNameInMap(map, "s1", "", PRESETS);
    expect(result.has("s1")).toBe(false);
  });

  it("whitespace-only name unassigns the seat", () => {
    const map = new Map([["s1", "Alice"]]);
    const result = assignNameInMap(map, "s1", "   ", PRESETS);
    expect(result.has("s1")).toBe(false);
  });

  it("does not mutate the input map (immutability)", () => {
    const map = new Map([["s1", "Alice"]]);
    const before = new Map(map);
    assignNameInMap(map, "s2", "Bob", PRESETS);
    expect([...map.entries()]).toEqual([...before.entries()]);
  });

  it("does not mutate the input map when stealing", () => {
    const map = new Map([["s1", "Alice"]]);
    const before = new Map(map);
    assignNameInMap(map, "s2", "Alice", PRESETS);
    expect([...map.entries()]).toEqual([...before.entries()]);
  });
});

describe("unassignName", () => {
  it("deletes the key for a seat", () => {
    const map = new Map([
      ["s1", "Alice"],
      ["s2", "Bob"],
    ]);
    const result = unassignName(map, "s1");
    expect(result.has("s1")).toBe(false);
    expect(result.get("s2")).toBe("Bob");
  });

  it("is a no-op for an unassigned seat", () => {
    const map = new Map([["s1", "Alice"]]);
    const result = unassignName(map, "s2");
    expect([...result.entries()]).toEqual([["s1", "Alice"]]);
  });

  it("does not mutate the input map (immutability)", () => {
    const map = new Map([["s1", "Alice"]]);
    const before = new Map(map);
    unassignName(map, "s1");
    expect([...map.entries()]).toEqual([...before.entries()]);
  });
});

describe("assignInOrder", () => {
  it("assigns presets to seats in order", () => {
    const result = assignInOrder(new Map(), ["s1", "s2", "s3"], PRESETS);
    expect(result.get("s1")).toBe("Alice");
    expect(result.get("s2")).toBe("Bob");
    expect(result.get("s3")).toBe("Carol");
  });

  it("truncates when there are more seats than names", () => {
    const result = assignInOrder(
      new Map(),
      ["s1", "s2", "s3", "s4"],
      ["Alice", "Bob"],
    );
    expect(result.get("s1")).toBe("Alice");
    expect(result.get("s2")).toBe("Bob");
    expect(result.has("s3")).toBe(false);
    expect(result.has("s4")).toBe(false);
    expect(result.size).toBe(2);
  });

  it("truncates when there are more names than seats", () => {
    const result = assignInOrder(new Map(), ["s1"], PRESETS);
    expect(result.get("s1")).toBe("Alice");
    expect(result.size).toBe(1);
  });

  it("preserves existing entries for seats beyond the assigned range", () => {
    const map = new Map([["s9", "Existing"]]);
    const result = assignInOrder(map, ["s1"], PRESETS);
    expect(result.get("s1")).toBe("Alice");
    expect(result.get("s9")).toBe("Existing");
  });

  it("removes a name from another seat when assigning it in order (dedupe)", () => {
    // 'Bob' was manually placed on seat s9 (outside playerIds). Assigning it in
    // order to s2 must not leave Bob lingering on s9 as a duplicate.
    const map = new Map([["s9", "Bob"]]);
    const result = assignInOrder(map, ["s1", "s2", "s3"], PRESETS);
    expect(result.get("s1")).toBe("Alice");
    expect(result.get("s2")).toBe("Bob");
    expect(result.get("s3")).toBe("Carol");
    expect(result.has("s9")).toBe(false);
  });

  it("does not mutate the input map (immutability)", () => {
    const map = new Map([["s9", "Existing"]]);
    const before = new Map(map);
    assignInOrder(map, ["s1", "s2"], PRESETS);
    expect([...map.entries()]).toEqual([...before.entries()]);
  });
});
