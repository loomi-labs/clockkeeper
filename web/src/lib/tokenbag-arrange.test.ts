import { describe, it, expect } from "vitest";
import { mapSeatingToRoles, seatingNames } from "./tokenbag-arrange";

describe("seatingNames", () => {
  const players = [
    { id: "1", name: "Alice" },
    { id: "2", name: "Bob" },
    { id: "3", name: "Cara" },
  ];

  it("projects ids to names in seat order", () => {
    expect(seatingNames(["2", "3", "1"], players)).toEqual([
      "Bob",
      "Cara",
      "Alice",
    ]);
  });

  it("accepts the proto's bigint ids", () => {
    expect(seatingNames([1n, 2n], players)).toEqual(["Alice", "Bob"]);
  });

  it("drops ids with no matching registrant", () => {
    expect(seatingNames(["1", "99", "2"], players)).toEqual(["Alice", "Bob"]);
  });

  it("drops registrants with an empty name", () => {
    expect(
      seatingNames(["1", "2"], [{ id: "1", name: "" }, players[1]]),
    ).toEqual(["Bob"]);
  });

  it("returns an empty list for an empty ring", () => {
    expect(seatingNames([], players)).toEqual([]);
  });
});

describe("mapSeatingToRoles", () => {
  it("maps every name to its seat's role id, in order", () => {
    const grimoire = new Map([
      ["washerwoman", "Alice"],
      ["imp", "Bob"],
      ["chef", "Cara"],
    ]);
    expect(mapSeatingToRoles(["Bob", "Cara", "Alice"], grimoire)).toEqual({
      roleIds: ["imp", "chef", "washerwoman"],
    });
  });

  it("reports every unmatched name and maps nothing", () => {
    const grimoire = new Map([["washerwoman", "Alice"]]);
    expect(mapSeatingToRoles(["Alice", "Bob", "Cara"], grimoire)).toEqual({
      missing: ["Bob", "Cara"],
    });
  });

  it("matches case, whitespace runs and surrounding space like the backend", () => {
    const grimoire = new Map([
      ["washerwoman", "Ana B"],
      ["chef", "  Bob  "],
    ]);
    expect(mapSeatingToRoles(["  ana   b ", "BOB"], grimoire)).toEqual({
      roleIds: ["washerwoman", "chef"],
    });
  });

  it("ignores grimoire seats nobody in the ring is sitting in", () => {
    const grimoire = new Map([
      ["washerwoman", "Alice"],
      ["chef", "Dora"],
      ["imp", ""],
    ]);
    expect(mapSeatingToRoles(["Alice"], grimoire)).toEqual({
      roleIds: ["washerwoman"],
    });
  });

  it("resolves a duplicated grimoire name to the first seat holding it", () => {
    const grimoire = new Map([
      ["washerwoman", "Alice"],
      ["chef", "alice"],
    ]);
    expect(mapSeatingToRoles(["Alice"], grimoire)).toEqual({
      roleIds: ["washerwoman"],
    });
  });

  it("treats a whitespace-only registrant name as missing", () => {
    const grimoire = new Map([["imp", "   "]]);
    expect(mapSeatingToRoles(["  "], grimoire)).toEqual({ missing: ["  "] });
  });

  it("maps an empty ring to an empty seat list", () => {
    expect(mapSeatingToRoles([], new Map([["imp", "Bob"]]))).toEqual({
      roleIds: [],
    });
  });
});
