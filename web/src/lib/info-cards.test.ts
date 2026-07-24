import { describe, it, expect } from "vitest";
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import {
  GameSchema,
  CharacterSchema,
  InfoCardSchema,
  Team,
  type Character,
  type Game,
} from "./gen/clockkeeper/v1/clockkeeper_pb";
import {
  generateStandardCards,
  customCardToDisplay,
  type StandardCardId,
} from "./info-cards";

function char(
  id: string,
  name: string,
  team: Team = Team.TOWNSFOLK,
  edition = "tb",
): Character {
  return create(CharacterSchema, { id, name, team, edition });
}

function makeGame(overrides: MessageInitShape<typeof GameSchema> = {}): Game {
  return create(GameSchema, { playerCount: 5, ...overrides });
}

function ids(cards: { id: StandardCardId | `custom:${string}` }[]): string[] {
  return cards.map((c) => c.id);
}

describe("generateStandardCards", () => {
  it("includes the bluff card when demon bluffs are selected", () => {
    const game = makeGame({
      selectedBluffCharacters: [
        char("chef", "Chef"),
        char("empath", "Empath"),
        char("virgin", "Virgin"),
      ],
    });
    const cards = generateStandardCards(game);
    const bluff = cards.find((c) => c.id === "std:notinplay");
    expect(bluff).toBeDefined();
    expect(bluff?.accent).toBe("blue");
    expect(bluff?.characters.map((c) => c.id)).toEqual([
      "chef",
      "empath",
      "virgin",
    ]);
    // Good-team bluffs resolve to the `_g` icon variant.
    expect(bluff?.characters.map((c) => c.iconSuffix)).toEqual([
      "_g",
      "_g",
      "_g",
    ]);
  });

  it("derives the icon suffix from each bluff character's team", () => {
    const game = makeGame({
      selectedBluffCharacters: [
        char("recluse", "Recluse", Team.OUTSIDER),
        char("imp", "Imp", Team.DEMON),
        char("scapegoat", "Scapegoat", Team.TRAVELLER),
      ],
    });
    const bluff = generateStandardCards(game).find(
      (c) => c.id === "std:notinplay",
    );
    expect(bluff?.characters.map((c) => c.iconSuffix)).toEqual([
      "_g",
      "_e",
      "",
    ]);
  });

  it("omits the bluff card when no demon bluffs are selected", () => {
    const cards = generateStandardCards(
      makeGame({ selectedBluffCharacters: [] }),
    );
    expect(ids(cards)).not.toContain("std:notinplay");
  });

  it("omits the minions card below the 7-player gate", () => {
    const cards = generateStandardCards(makeGame({ playerCount: 6 }));
    expect(ids(cards)).not.toContain("std:minions");
  });

  it("includes the minions card at 7+ players", () => {
    const cards = generateStandardCards(makeGame({ playerCount: 7 }));
    const minions = cards.find((c) => c.id === "std:minions");
    expect(minions).toBeDefined();
    expect(minions?.accent).toBe("red");
    expect(minions?.characters).toHaveLength(0);
  });

  it("flags exactly the three pick cards as needsCharacterPick", () => {
    const cards = generateStandardCards(makeGame({ playerCount: 7 }));
    const pickers = cards.filter((c) => c.needsCharacterPick).map((c) => c.id);
    expect(pickers.sort()).toEqual(
      ["std:selectedyou", "std:thisplayeris", "std:youare"].sort(),
    );
  });

  it("assigns the expected accents to the fixed cards", () => {
    const byId = new Map(
      generateStandardCards(makeGame({ playerCount: 7 })).map((c) => [c.id, c]),
    );
    expect(byId.get("std:thisisthedemon")?.accent).toBe("red");
    expect(byId.get("std:youare")?.accent).toBe("purple");
    expect(byId.get("std:thisplayeris")?.accent).toBe("purple");
    expect(byId.get("std:selectedyou")?.accent).toBe("green");
    expect(byId.get("std:vote")?.accent).toBe("gold");
    expect(byId.get("std:nominate")?.accent).toBe("gold");
  });

  it("marks every standard card kind as standard with uppercase titles", () => {
    const cards = generateStandardCards(makeGame({ playerCount: 7 }));
    for (const c of cards) {
      expect(c.kind).toBe("standard");
      expect(c.title).toBe(c.title.toUpperCase());
    }
  });
});

describe("customCardToDisplay", () => {
  it("maps a stored info card to a neutral custom display card", () => {
    const card = create(InfoCardSchema, {
      id: 42n,
      title: "Secret Note",
      body: "You feel uneasy.",
      characterIds: ["imp"],
      characters: [char("imp", "Imp", Team.DEMON)],
    });
    const display = customCardToDisplay(card);
    expect(display.id).toBe("custom:42");
    expect(display.title).toBe("Secret Note");
    expect(display.body).toBe("You feel uneasy.");
    expect(display.kind).toBe("custom");
    expect(display.accent).toBe("neutral");
    expect(display.needsCharacterPick).toBeUndefined();
    expect(display.characters).toEqual([
      { id: "imp", name: "Imp", edition: "tb", iconSuffix: "_e" },
    ]);
  });

  it("maps a card with no characters", () => {
    const card = create(InfoCardSchema, { id: 1n, title: "T", body: "" });
    const display = customCardToDisplay(card);
    expect(display.characters).toEqual([]);
    expect(display.id).toBe("custom:1");
  });
});
