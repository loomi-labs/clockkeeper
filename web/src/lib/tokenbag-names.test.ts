import { describe, it, expect } from "vitest";
import {
  assignedSeatNames,
  deriveNameSource,
  matchRegistrantsToSeats,
  type NameSourceInput,
} from "./tokenbag-names";
import { NO_ID, type BagPlayer } from "./tokenbag";

const PRESETS = ["Pia", "Quin"];

function input(overrides: Partial<NameSourceInput> = {}): NameSourceInput {
  return {
    registrants: {
      names: ["Alice", "Bob"],
      prereveal: true,
      gameId: "7",
    },
    gameId: "7",
    isSetup: true,
    presetNames: PRESETS,
    grimoireNames: new Map([
      ["washerwoman", "Alice"],
      ["chef", "Bob"],
      ["imp", "Zed"],
    ]),
    ...overrides,
  };
}

describe("deriveNameSource", () => {
  it("lets the bag own the names while the game is in setup", () => {
    const source = deriveNameSource(input());
    expect(source.bagActive).toBe(true);
    expect(source.names).toEqual(["Alice", "Bob"]);
    // Only seats holding a registrant's name; "Zed" is the Storyteller's own.
    expect([...source.renameLockedIds].sort()).toEqual(["chef", "washerwoman"]);
  });

  it("locks a seat whose name differs only by case and spacing", () => {
    const source = deriveNameSource(
      input({
        grimoireNames: new Map([["imp", "  ALICE "]]),
      }),
    );
    expect([...source.renameLockedIds]).toEqual(["imp"]);
  });

  // (a) Starting the game unmounts the panel, freezing its last report.
  it("falls back to presets once the game has left setup", () => {
    const source = deriveNameSource(input({ isSetup: false }));
    expect(source.bagActive).toBe(false);
    expect(source.names).toEqual(PRESETS);
    expect(source.renameLockedIds.size).toBe(0);
  });

  // (b) Navigating to another game must not inherit the first game's names.
  it("ignores a report belonging to a different game", () => {
    const source = deriveNameSource(input({ gameId: "8" }));
    expect(source.bagActive).toBe(false);
    expect(source.names).toEqual(PRESETS);
    expect(source.renameLockedIds.size).toBe(0);
  });

  it("falls back to presets while no game is loaded", () => {
    const source = deriveNameSource(input({ gameId: "" }));
    expect(source.bagActive).toBe(false);
    expect(source.names).toEqual(PRESETS);
  });

  it("falls back to presets when the panel never reported", () => {
    const source = deriveNameSource(input({ registrants: null }));
    expect(source.bagActive).toBe(false);
    expect(source.names).toEqual(PRESETS);
  });

  it("falls back to presets for an empty bag", () => {
    const source = deriveNameSource(
      input({ registrants: { names: [], prereveal: true, gameId: "7" } }),
    );
    expect(source.bagActive).toBe(false);
    expect(source.names).toEqual(PRESETS);
  });

  it("keeps owning the names after the reveal but stops locking renames", () => {
    const source = deriveNameSource(
      input({
        registrants: { names: ["Alice", "Bob"], prereveal: false, gameId: "7" },
      }),
    );
    expect(source.bagActive).toBe(true);
    expect(source.names).toEqual(["Alice", "Bob"]);
    // Post-reveal the server holds assigned_role_id, so a rename cannot desync.
    expect(source.renameLockedIds.size).toBe(0);
  });

  it("never hands back the caller's arrays or maps to mutate", () => {
    const presetNames = ["Pia"];
    const source = deriveNameSource(input({ isSetup: false, presetNames }));
    source.names.push("Rex");
    expect(presetNames).toEqual(["Pia"]);
  });
});

describe("matchRegistrantsToSeats", () => {
  function registrant(overrides: Partial<BagPlayer> = {}): BagPlayer {
    return {
      id: "1",
      name: "Ana",
      viaSharedDevice: false,
      leftId: NO_ID,
      rightId: NO_ID,
      ...overrides,
    };
  }

  const ana = registrant({ id: "1", name: "Ana", leftId: "3", rightId: "2" });
  const bob = registrant({ id: "2", name: "Bob", viaSharedDevice: true });
  const cleo = registrant({ id: "3", name: "Cleo" });
  const registrants = [ana, bob, cleo];

  it("ties a seat to the registrant whose name it holds", () => {
    const meta = matchRegistrantsToSeats(registrants, [
      { id: "washerwoman", name: "Ana" },
      { id: "chef", name: "Bob" },
    ]);
    expect(meta.get("washerwoman")).toEqual({
      viaShared: false,
      leftName: "Cleo",
      rightName: "Bob",
    });
    // Added on the tablet, and no neighbors picked yet.
    expect(meta.get("chef")).toEqual({
      viaShared: true,
      leftName: undefined,
      rightName: undefined,
    });
  });

  it("leaves out seats with no name and names nobody registered", () => {
    const meta = matchRegistrantsToSeats(registrants, [
      { id: "imp", name: "Zed" },
      { id: "chef", name: "" },
      { id: "monk", name: undefined },
    ]);
    expect(meta.size).toBe(0);
  });

  it("matches across case and spacing", () => {
    const meta = matchRegistrantsToSeats(registrants, [
      { id: "imp", name: "  aNa   " },
    ]);
    expect(meta.get("imp")?.leftName).toBe("Cleo");
  });

  it("matches every seat carrying the same name", () => {
    const meta = matchRegistrantsToSeats(registrants, [
      { id: "imp", name: "Cleo" },
      { id: "chef", name: "cleo" },
    ]);
    expect(meta.size).toBe(2);
    expect(meta.get("imp")).toEqual(meta.get("chef"));
  });

  it("keeps the first of two registrants with the same normalized name", () => {
    const meta = matchRegistrantsToSeats(
      [cleo, registrant({ id: "4", name: "CLEO", viaSharedDevice: true })],
      [{ id: "imp", name: "Cleo" }],
    );
    expect(meta.get("imp")?.viaShared).toBe(false);
  });

  it("ignores a neighbor pick that points at nobody", () => {
    const meta = matchRegistrantsToSeats(
      [registrant({ leftId: "99", rightId: NO_ID })],
      [{ id: "imp", name: "Ana" }],
    );
    expect(meta.get("imp")).toEqual({
      viaShared: false,
      leftName: undefined,
      rightName: undefined,
    });
  });

  it("has nothing to say without registrants", () => {
    expect(matchRegistrantsToSeats([], [{ id: "imp", name: "Ana" }]).size).toBe(
      0,
    );
  });
});

describe("assignedSeatNames", () => {
  const grimoire = new Map([
    ["washerwoman", "Alice"],
    ["chef", "Bob"],
    ["imp", ""],
  ]);

  it("collects the names of the seats in play", () => {
    expect([
      ...assignedSeatNames(["washerwoman", "chef", "imp"], grimoire),
    ]).toEqual(["Alice", "Bob"]);
  });

  // The heart of it: a name stranded on a character that left the script is not
  // assigned to anything, and the server's reveal takes the same view.
  it("ignores names on seats that are no longer in play", () => {
    expect([...assignedSeatNames(["chef"], grimoire)]).toEqual(["Bob"]);
    expect([...assignedSeatNames([], grimoire)]).toEqual([]);
  });

  it("ignores in-play seats with no name", () => {
    expect([...assignedSeatNames(["imp"], grimoire)]).toEqual([]);
  });

  it("keeps the original spelling, so the caller can normalize it", () => {
    expect([
      ...assignedSeatNames(["imp"], new Map([["imp", "  ANA   b "]])),
    ]).toEqual(["  ANA   b "]);
  });
});
