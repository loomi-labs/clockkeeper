import { describe, it, expect } from "vitest";
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import {
  GameSchema,
  CharacterSchema,
  RoleDistributionSchema,
  type Character,
  type Game,
} from "./gen/clockkeeper/v1/clockkeeper_pb";
import {
  getStartGameWarnings,
  bluffCharactersInPlay,
  type CurrentDistribution,
} from "./game-warnings";

function char(id: string, name: string, edition = "tb"): Character {
  return create(CharacterSchema, { id, name, edition });
}

function makeGame(overrides: MessageInitShape<typeof GameSchema> = {}): Game {
  return create(GameSchema, { playerCount: 5, ...overrides });
}

const matchingDist: CurrentDistribution = {
  townsfolk: 0,
  outsiders: 0,
  minions: 0,
  demons: 0,
};

describe("bluffCharactersInPlay", () => {
  it("returns bluffs that appear in selected roles / extras / travellers", () => {
    const game = makeGame({
      selectedRoleIds: ["chef"],
      extraCharacterIds: ["empath"],
      selectedTravellerIds: ["scapegoat"],
      selectedBluffCharacters: [
        char("chef", "Chef"),
        char("empath", "Empath"),
        char("scapegoat", "Scapegoat"),
        char("virgin", "Virgin"),
      ],
    });
    const inPlay = bluffCharactersInPlay(game).map((c) => c.id);
    expect(inPlay).toEqual(["chef", "empath", "scapegoat"]);
  });

  it("returns nothing when no bluffs are in play", () => {
    const game = makeGame({
      selectedRoleIds: ["baron"],
      selectedBluffCharacters: [char("virgin", "Virgin")],
    });
    expect(bluffCharactersInPlay(game)).toEqual([]);
  });
});

describe("getStartGameWarnings", () => {
  it("returns no warnings for a well-formed setup", () => {
    const game = makeGame({
      playerCount: 1,
      selectedRoleIds: ["imp"],
      distribution: create(RoleDistributionSchema, {
        townsfolk: 0,
        outsiders: 0,
        minions: 0,
        demons: 1,
      }),
    });
    const dist: CurrentDistribution = { ...matchingDist, demons: 1 };
    expect(getStartGameWarnings(game, dist)).toEqual([]);
  });

  it("warns when fewer than 3 bluffs are selected for 7+ players", () => {
    const game = makeGame({
      playerCount: 7,
      selectedRoleIds: Array.from({ length: 7 }, (_, i) => `role${i}`),
      selectedBluffIds: ["a", "b"],
    });
    const warnings = getStartGameWarnings(game, matchingDist);
    expect(warnings).toContain("Only 2 of 3 demon bluffs selected.");
  });

  it("does not warn about bluff count below the 7-player gate", () => {
    const game = makeGame({ playerCount: 6, selectedBluffIds: [] });
    const warnings = getStartGameWarnings(game, matchingDist);
    expect(warnings.some((w) => w.includes("demon bluffs selected"))).toBe(
      false,
    );
  });

  it("adds a per-bluff warning for each in-play bluff", () => {
    const game = makeGame({
      playerCount: 2,
      selectedRoleIds: ["chef", "imp"],
      selectedBluffCharacters: [char("chef", "Chef"), char("virgin", "Virgin")],
    });
    const warnings = getStartGameWarnings(game, matchingDist);
    expect(warnings).toContain("Demon bluff Chef is in play.");
    expect(warnings).not.toContain("Demon bluff Virgin is in play.");
  });

  it("warns when the role count differs from the player count", () => {
    const game = makeGame({ playerCount: 5, selectedRoleIds: ["a", "b"] });
    const warnings = getStartGameWarnings(game, matchingDist);
    expect(warnings).toContain("2 roles selected but 5 players expected.");
  });

  it("warns on distribution mismatches", () => {
    const game = makeGame({
      playerCount: 1,
      selectedRoleIds: ["imp"],
      distribution: create(RoleDistributionSchema, {
        townsfolk: 3,
        outsiders: 1,
        minions: 1,
        demons: 1,
      }),
    });
    const dist: CurrentDistribution = {
      townsfolk: 2,
      outsiders: 0,
      minions: 1,
      demons: 1,
    };
    const warnings = getStartGameWarnings(game, dist);
    expect(warnings).toContain("Townsfolk: 2 selected, 3 expected.");
    expect(warnings).toContain("Outsiders: 0 selected, 1 expected.");
    expect(warnings.some((w) => w.startsWith("Minions:"))).toBe(false);
    expect(warnings.some((w) => w.startsWith("Demons:"))).toBe(false);
  });

  it("warns about an unpicked bag substitution token", () => {
    const game = makeGame({
      playerCount: 1,
      selectedRoleIds: ["imp"],
      bagSubstitutions: [
        { causedById: "drunk", causedByName: "Drunk", characterId: "" },
      ],
    });
    const warnings = getStartGameWarnings(game, matchingDist);
    expect(warnings).toContain("Drunk has not picked a substitute token.");
  });
});
