import { describe, it, expect } from "vitest";
import {
  isPoisonReminderText,
  isDrunkReminderText,
  derivePlayerStatuses,
  type StatusReminderInput,
} from "./status";

describe("isPoisonReminderText", () => {
  it("matches the Poisoned token case/space-insensitively", () => {
    expect(isPoisonReminderText("Poisoned")).toBe(true);
    expect(isPoisonReminderText("  poisoned  ")).toBe(true);
    expect(isPoisonReminderText("POISONED")).toBe(true);
  });

  it("does not match other tokens", () => {
    expect(isPoisonReminderText("Drunk")).toBe(false);
    expect(isPoisonReminderText("Poison")).toBe(false);
    expect(isPoisonReminderText("")).toBe(false);
  });
});

describe("isDrunkReminderText", () => {
  it("matches the Drunk token family", () => {
    expect(isDrunkReminderText("Drunk")).toBe(true);
    expect(isDrunkReminderText("drunk")).toBe(true);
    expect(isDrunkReminderText("Drunk 1")).toBe(true);
    expect(isDrunkReminderText("Drunk 2")).toBe(true);
    expect(isDrunkReminderText("Drunk 3")).toBe(true);
    expect(isDrunkReminderText("Is The Drunk")).toBe(true);
    expect(isDrunkReminderText("is the drunk")).toBe(true);
    // Bag substitution synthesizes "Is the <name>".
    expect(isDrunkReminderText("Is the Drunk")).toBe(true);
  });

  it("does not match the global Everyone Is Drunk token", () => {
    expect(isDrunkReminderText("Everyone Is Drunk")).toBe(false);
    expect(isDrunkReminderText("everyone is drunk")).toBe(false);
  });

  it("does not match unrelated text", () => {
    expect(isDrunkReminderText("Poisoned")).toBe(false);
    expect(isDrunkReminderText("Red Herring")).toBe(false);
    expect(isDrunkReminderText("Safe")).toBe(false);
    expect(isDrunkReminderText("Drunkard")).toBe(false);
    expect(isDrunkReminderText("Drunk one")).toBe(false);
    expect(isDrunkReminderText("")).toBe(false);
  });
});

describe("derivePlayerStatuses", () => {
  it("ignores unattached tokens", () => {
    const reminders: StatusReminderInput[] = [
      { text: "Poisoned", characterName: "Poisoner" }, // no attachedTo
      { text: "Drunk", attachedTo: undefined, characterName: "Sailor" },
    ];
    const map = derivePlayerStatuses(reminders, []);
    expect(map.size).toBe(0);
  });

  it("derives poisoned and drunk per attached player", () => {
    const reminders: StatusReminderInput[] = [
      { text: "Poisoned", attachedTo: "p1", characterName: "Poisoner" },
      { text: "Drunk", attachedTo: "p2", characterName: "Sailor" },
      {
        text: "Red Herring",
        attachedTo: "p3",
        characterName: "Fortune Teller",
      },
    ];
    const map = derivePlayerStatuses(reminders, []);
    expect(map.get("p1")).toEqual({
      poisoned: true,
      drunk: false,
      sources: ["Poisoner"],
    });
    expect(map.get("p2")).toEqual({
      poisoned: false,
      drunk: true,
      sources: ["Sailor"],
    });
    // Red Herring is not a status token.
    expect(map.has("p3")).toBe(false);
  });

  it("accumulates multiple sources without duplicates", () => {
    const reminders: StatusReminderInput[] = [
      { text: "Poisoned", attachedTo: "p1", characterName: "Poisoner" },
      { text: "Poisoned", attachedTo: "p1", characterName: "Widow" },
      { text: "Drunk", attachedTo: "p1", characterName: "Poisoner" }, // dup name
    ];
    const map = derivePlayerStatuses(reminders, []);
    const status = map.get("p1");
    expect(status?.poisoned).toBe(true);
    expect(status?.drunk).toBe(true);
    expect(status?.sources).toEqual(["Poisoner", "Widow"]);
  });

  it("aliases bag substitutions onto the shown character row", () => {
    // The Drunk (causedById "drunk") plays as the Empath (characterId "empath").
    // The synthesized "Is the Drunk" token is attached under "drunk".
    const reminders: StatusReminderInput[] = [
      { text: "Is the Drunk", attachedTo: "drunk", characterName: "Drunk" },
    ];
    const bagSubs = [{ characterId: "empath", causedById: "drunk" }];
    const map = derivePlayerStatuses(reminders, bagSubs);

    const underlying = map.get("drunk");
    expect(underlying?.drunk).toBe(true);
    // The night row keyed by the shown character resolves to the same status.
    expect(map.get("empath")).toBe(underlying);
  });

  it("does not alias when the underlying player has no status", () => {
    const bagSubs = [{ characterId: "empath", causedById: "drunk" }];
    const map = derivePlayerStatuses([], bagSubs);
    expect(map.has("empath")).toBe(false);
  });
});
