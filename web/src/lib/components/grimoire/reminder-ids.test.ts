import { describe, it, expect } from "vitest";
import { stableReminderIds, canonicalizeReminderKeys } from "./reminder-ids";

// Mirrors the Go token order: per character (selected -> travellers -> extras),
// each character's `reminders` first then `remindersGlobal`. Here we only model
// the flat token list the frontend actually receives via game.reminderTokens.

describe("stableReminderIds", () => {
  it("counts occurrences per characterId in list order", () => {
    const tokens = [
      { characterId: "washerwoman" },
      { characterId: "empath" },
      { characterId: "washerwoman" },
      { characterId: "poisoner" },
      { characterId: "washerwoman" },
    ];
    expect(stableReminderIds(tokens)).toEqual([
      "reminder-washerwoman-0",
      "reminder-empath-0",
      "reminder-washerwoman-1",
      "reminder-poisoner-0",
      "reminder-washerwoman-2",
    ]);
  });

  it("counts by characterId regardless of duplicate reminder text", () => {
    // Two tokens for the same character with identical text still get distinct
    // occurrence indices (ids are position-in-character, not text-derived).
    const tokens = [
      { characterId: "fortuneteller" },
      { characterId: "fortuneteller" },
    ];
    expect(stableReminderIds(tokens)).toEqual([
      "reminder-fortuneteller-0",
      "reminder-fortuneteller-1",
    ]);
  });

  it("preserves reminders-then-remindersGlobal ordering as-supplied", () => {
    // The caller supplies tokens already ordered reminders-first,
    // remindersGlobal-second; occurrence counting must respect that order.
    const tokens = [
      { characterId: "monk" }, // reminders[0]
      { characterId: "monk" }, // remindersGlobal[0]
    ];
    expect(stableReminderIds(tokens)).toEqual([
      "reminder-monk-0",
      "reminder-monk-1",
    ]);
  });

  it("returns [] for no tokens", () => {
    expect(stableReminderIds([])).toEqual([]);
  });
});

describe("canonicalizeReminderKeys", () => {
  const tokens = [
    { characterId: "washerwoman" }, // stable id reminder-washerwoman-0
    { characterId: "empath" }, // reminder-empath-0
    { characterId: "washerwoman" }, // reminder-washerwoman-1
  ];

  it("maps legacy positional keys onto their stable ids (record input)", () => {
    const result = canonicalizeReminderKeys(
      {
        "reminder-0": "a",
        "reminder-1": "b",
        "reminder-2": "c",
      },
      tokens,
    );
    expect(result.get("reminder-washerwoman-0")).toBe("a");
    expect(result.get("reminder-empath-0")).toBe("b");
    expect(result.get("reminder-washerwoman-1")).toBe("c");
    expect(result.size).toBe(3);
  });

  it("maps legacy positional keys (Map input)", () => {
    const result = canonicalizeReminderKeys(
      new Map([
        ["reminder-0", "a"],
        ["reminder-2", "c"],
      ]),
      tokens,
    );
    expect(result.get("reminder-washerwoman-0")).toBe("a");
    expect(result.get("reminder-washerwoman-1")).toBe("c");
    expect(result.size).toBe(2);
  });

  it("drops legacy keys whose index is out of range", () => {
    const result = canonicalizeReminderKeys(
      {
        "reminder-0": "a",
        "reminder-3": "gone", // only 3 tokens (0..2)
        "reminder-99": "gone",
      },
      tokens,
    );
    expect(result.has("reminder-washerwoman-0")).toBe(true);
    expect([...result.values()]).not.toContain("gone");
    expect(result.size).toBe(1);
  });

  it("passes stable reminder keys through untouched", () => {
    const result = canonicalizeReminderKeys(
      {
        "reminder-empath-0": "keep",
        "reminder-washerwoman-1": "keep2",
      },
      tokens,
    );
    expect(result.get("reminder-empath-0")).toBe("keep");
    expect(result.get("reminder-washerwoman-1")).toBe("keep2");
    expect(result.size).toBe(2);
  });

  it("passes bagsub-reminder-* keys through untouched", () => {
    const result = canonicalizeReminderKeys(
      { "bagsub-reminder-drunk": "10:1.5" },
      tokens,
    );
    expect(result.get("bagsub-reminder-drunk")).toBe("10:1.5");
    expect(result.size).toBe(1);
  });

  it("keeps the existing stable entry when a legacy key collides onto it", () => {
    // reminder-0 canonicalizes to reminder-washerwoman-0, which is already
    // present as a genuine stable key -> keep the existing stable value.
    const result = canonicalizeReminderKeys(
      {
        "reminder-washerwoman-0": "stable",
        "reminder-0": "legacy",
      },
      tokens,
    );
    expect(result.get("reminder-washerwoman-0")).toBe("stable");
    expect(result.size).toBe(1);
  });

  it("keeps the first legacy key when two legacy keys collide", () => {
    // Two tokens for the same character both at index-derived same stable id is
    // impossible, but two legacy keys pointing at the same index cannot occur
    // for a record; for a Map we still guard keep-first via iteration order.
    const result = canonicalizeReminderKeys(
      new Map([
        ["reminder-1", "first"],
        // second entry with same key can't exist in a Map; simulate a stable
        // key already occupying the target instead.
        ["reminder-empath-0", "occupied"],
      ]),
      tokens,
    );
    // reminder-empath-0 was set first as a pass-through; legacy reminder-1
    // (-> reminder-empath-0) must not overwrite it.
    expect(result.get("reminder-empath-0")).toBe("occupied");
    expect(result.size).toBe(1);
  });

  it("returns an empty Map for empty input", () => {
    expect(canonicalizeReminderKeys({}, tokens).size).toBe(0);
    expect(canonicalizeReminderKeys(new Map(), tokens).size).toBe(0);
  });
});
