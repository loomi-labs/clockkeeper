import { describe, it, expect } from "vitest";
import {
  assignNameInMap,
  unassignName,
  assignInOrder,
  renameAssignedName,
  shuffled,
} from "./player-names";

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

describe("assignInOrder with locks", () => {
  it("keeps a locked seat's name through a full reassign", () => {
    const map = new Map([
      ["s1", "Carol"],
      ["s2", "Alice"],
    ]);
    const result = assignInOrder(
      map,
      ["s1", "s2", "s3"],
      PRESETS,
      new Set(["s1"]),
    );
    expect(result.get("s1")).toBe("Carol");
  });

  it("never hands a locked seat's name to another seat", () => {
    const map = new Map([["s2", "Bob"]]);
    const result = assignInOrder(
      map,
      ["s1", "s2", "s3"],
      PRESETS,
      new Set(["s2"]),
    );
    expect(result.get("s2")).toBe("Bob");
    expect([...result.values()].filter((n) => n === "Bob")).toHaveLength(1);
  });

  it("gives the unlocked seats the remaining names in order", () => {
    const map = new Map([["s2", "Bob"]]);
    const result = assignInOrder(
      map,
      ["s1", "s2", "s3"],
      PRESETS,
      new Set(["s2"]),
    );
    // 'Bob' is withheld, so s1/s3 get Alice/Carol — s2 is skipped entirely.
    expect(result.get("s1")).toBe("Alice");
    expect(result.get("s2")).toBe("Bob");
    expect(result.get("s3")).toBe("Carol");
  });

  it("shifts remaining names up when a locked seat holds a free-text name", () => {
    const map = new Map([["s2", "Guest"]]);
    const result = assignInOrder(
      map,
      ["s1", "s2", "s3"],
      PRESETS,
      new Set(["s2"]),
    );
    expect(result.get("s1")).toBe("Alice");
    expect(result.get("s2")).toBe("Guest");
    expect(result.get("s3")).toBe("Bob");
  });

  it("does not remove a locked seat's entry via the duplicate-steal", () => {
    // s9 is locked outside playerIds and duplicates a name held by a seat that
    // IS being reassigned. The steal that dedupes preset names must never evict
    // s9 (the pool filter also withholds 'Alice', so nothing reclaims it).
    const map = new Map([
      ["s1", "Alice"],
      ["s9", "Alice"],
    ]);
    const result = assignInOrder(
      map,
      ["s1", "s2", "s3"],
      PRESETS,
      new Set(["s9"]),
    );
    expect(result.get("s9")).toBe("Alice");
    expect(result.get("s1")).toBe("Bob");
    expect(result.get("s2")).toBe("Carol");
    expect([...result.values()].filter((n) => n === "Alice")).toHaveLength(1);
  });

  it("treats a locked seat with no name as a plain skip", () => {
    const result = assignInOrder(
      new Map(),
      ["s1", "s2", "s3"],
      PRESETS,
      new Set(["s2"]),
    );
    expect(result.has("s2")).toBe(false);
    expect(result.get("s1")).toBe("Alice");
    expect(result.get("s3")).toBe("Bob");
    expect(result.size).toBe(2);
  });

  it("behaves exactly like the unlocked version for an empty lock set", () => {
    const map = new Map([["s9", "Bob"]]);
    const withEmpty = assignInOrder(
      map,
      ["s1", "s2", "s3"],
      PRESETS,
      new Set(),
    );
    const omitted = assignInOrder(map, ["s1", "s2", "s3"], PRESETS);
    expect([...withEmpty.entries()]).toEqual([...omitted.entries()]);
    expect(withEmpty.get("s1")).toBe("Alice");
    expect(withEmpty.get("s2")).toBe("Bob");
    expect(withEmpty.get("s3")).toBe("Carol");
    expect(withEmpty.has("s9")).toBe(false);
  });

  it("does not mutate the input map (immutability)", () => {
    const map = new Map([["s1", "Carol"]]);
    const before = new Map(map);
    assignInOrder(map, ["s1", "s2"], PRESETS, new Set(["s1"]));
    expect([...map.entries()]).toEqual([...before.entries()]);
  });
});

describe("renameAssignedName", () => {
  it("renames a single seat holding the old name", () => {
    const map = new Map([
      ["s1", "Alice"],
      ["s2", "Bob"],
    ]);
    const result = renameAssignedName(map, "Alice", "Alicia");
    expect(result.get("s1")).toBe("Alicia");
    expect(result.get("s2")).toBe("Bob");
  });

  it("renames every seat holding the old name (free-text duplicates)", () => {
    const map = new Map([
      ["s1", "Guest"],
      ["s2", "Guest"],
      ["s3", "Bob"],
    ]);
    const result = renameAssignedName(map, "Guest", "Visitor");
    expect(result.get("s1")).toBe("Visitor");
    expect(result.get("s2")).toBe("Visitor");
    expect(result.get("s3")).toBe("Bob");
  });

  it("trims the new name", () => {
    const map = new Map([["s1", "Alice"]]);
    const result = renameAssignedName(map, "Alice", "  Alicia  ");
    expect(result.get("s1")).toBe("Alicia");
  });

  it("is a no-op when the old name is absent", () => {
    const map = new Map([
      ["s1", "Alice"],
      ["s2", "Bob"],
    ]);
    const result = renameAssignedName(map, "Carol", "Carla");
    expect([...result.entries()]).toEqual([...map.entries()]);
  });

  it("unassigns matching seats when the new name is empty", () => {
    const map = new Map([
      ["s1", "Guest"],
      ["s2", "Guest"],
      ["s3", "Bob"],
    ]);
    const result = renameAssignedName(map, "Guest", "");
    expect(result.has("s1")).toBe(false);
    expect(result.has("s2")).toBe(false);
    expect(result.get("s3")).toBe("Bob");
  });

  it("unassigns matching seats when the new name is whitespace-only", () => {
    const map = new Map([["s1", "Alice"]]);
    const result = renameAssignedName(map, "Alice", "   ");
    expect(result.has("s1")).toBe(false);
  });

  it("does not mutate the input map (immutability)", () => {
    const map = new Map([["s1", "Alice"]]);
    const before = new Map(map);
    renameAssignedName(map, "Alice", "Alicia");
    expect([...map.entries()]).toEqual([...before.entries()]);
  });
});

describe("shuffled", () => {
  it("returns a permutation of the input (same multiset of elements)", () => {
    const input = [1, 2, 3, 4, 5, 6, 7, 8];
    const result = shuffled(input);
    expect(result).toHaveLength(input.length);
    expect([...result].sort((a, b) => a - b)).toEqual(input);
  });

  it("does not mutate the input array (immutability)", () => {
    const input = ["a", "b", "c", "d"];
    const before = [...input];
    shuffled(input);
    expect(input).toEqual(before);
  });

  it("returns a new array instance", () => {
    const input = [1, 2, 3];
    expect(shuffled(input)).not.toBe(input);
  });

  it("handles empty and single-element arrays", () => {
    expect(shuffled([])).toEqual([]);
    expect(shuffled([42])).toEqual([42]);
  });
});
