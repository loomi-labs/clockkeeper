import { describe, it, expect } from "vitest";
import { deriveNameSource, type NameSourceInput } from "./tokenbag-names";

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
