import { describe, it, expect } from "vitest";
import { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
import { bagSubDropTarget, bagSubDropHint } from "./bagsub";

describe("bagSubDropTarget", () => {
  const causedById = "drunk";
  const selected = new Set(["drunk", "empath", "washerwoman", "poisoner"]);
  const teamOf = (id: string): Team | undefined => {
    switch (id) {
      case "empath":
      case "washerwoman":
        return Team.TOWNSFOLK;
      case "poisoner":
        return Team.MINION;
      case "butler":
        return Team.OUTSIDER;
      default:
        return undefined;
    }
  };
  const requiredTeam = Team.TOWNSFOLK;

  it('returns "self" when dropped on the causing seat', () => {
    expect(
      bagSubDropTarget("drunk", causedById, selected, teamOf, requiredTeam),
    ).toBe("self");
  });

  it('returns "not-in-play" when the target role is not selected', () => {
    // e.g. a traveller seat: not in selected_roles -> cannot receive the sub.
    expect(
      bagSubDropTarget("scapegoat", causedById, selected, teamOf, requiredTeam),
    ).toBe("not-in-play");
  });

  it('returns "not-in-play" for a traveller target even before team check', () => {
    const withTraveller = new Set(selected);
    // Traveller is intentionally NOT added to selected roles; team lookup would
    // be TRAVELLER, but not-in-play must win.
    expect(
      bagSubDropTarget(
        "beggar",
        causedById,
        withTraveller,
        () => Team.TRAVELLER,
        requiredTeam,
      ),
    ).toBe("not-in-play");
  });

  it('returns "wrong-team" when the target team differs from required', () => {
    expect(
      bagSubDropTarget("poisoner", causedById, selected, teamOf, requiredTeam),
    ).toBe("wrong-team");
  });

  it('returns "ok" for a valid in-play, right-team target', () => {
    expect(
      bagSubDropTarget("empath", causedById, selected, teamOf, requiredTeam),
    ).toBe("ok");
    expect(
      bagSubDropTarget(
        "washerwoman",
        causedById,
        selected,
        teamOf,
        requiredTeam,
      ),
    ).toBe("ok");
  });

  it('treats an unknown team as "wrong-team"', () => {
    const sel = new Set(["drunk", "mystery"]);
    expect(
      bagSubDropTarget(
        "mystery",
        causedById,
        sel,
        () => undefined,
        requiredTeam,
      ),
    ).toBe("wrong-team");
  });
});

describe("bagSubDropHint", () => {
  it("gives a team hint for wrong-team", () => {
    expect(bagSubDropHint("wrong-team", "Drunk", "Townsfolk")).toBe(
      "The Drunk must think they're a Townsfolk — drop the token on a Townsfolk seat.",
    );
  });

  it("gives a not-in-play hint", () => {
    const hint = bagSubDropHint("not-in-play", "Drunk", "Townsfolk");
    expect(hint).toContain("Drunk");
    expect(hint.length).toBeGreaterThan(0);
  });

  it("returns empty string for ok and self", () => {
    expect(bagSubDropHint("ok", "Drunk", "Townsfolk")).toBe("");
    expect(bagSubDropHint("self", "Drunk", "Townsfolk")).toBe("");
  });
});
