import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import {
  DeathCause,
  DeathSchema,
  PhaseSchema,
  Team,
} from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
import type { Death, Phase } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
import {
  computeEmpath,
  computeChef,
  computeFortuneTeller,
  chefRangeCauses,
  classifyRegistration,
  findExecutedToday,
  type HelperPlayer,
} from "./helpers";

function player(
  id: string,
  overrides: Partial<HelperPlayer> = {},
): HelperPlayer {
  return {
    id,
    name: id,
    characterId: id,
    characterName: id,
    team: Team.TOWNSFOLK,
    edition: "tb",
    isDead: false,
    alignment: "good",
    ...overrides,
  };
}

function playerMap(players: HelperPlayer[]): Map<string, HelperPlayer> {
  return new Map(players.map((p) => [p.id, p]));
}

function death(roleId: string, cause?: DeathCause): Death {
  return create(DeathSchema, { roleId, cause });
}

function phase(deaths: Death[]): Phase {
  return create(PhaseSchema, { deaths });
}

describe("computeEmpath", () => {
  const order = ["emp", "left", "right", "x", "y"];

  it("returns undefined when the empath is missing", () => {
    expect(computeEmpath(order, playerMap([]), "emp")).toBeUndefined();
  });

  it("returns undefined when the empath is dead", () => {
    const players = playerMap([player("emp", { isDead: true })]);
    expect(computeEmpath(order, players, "emp")).toBeUndefined();
  });

  it("counts 0, 1 and 2 evil neighbours (exact range)", () => {
    const good = playerMap([
      player("emp"),
      player("left", { alignment: "good" }),
      player("y", { alignment: "good" }),
    ]);
    expect(computeEmpath(order, good, "emp")).toEqual({
      min: 0,
      max: 0,
      unknown: false,
    });

    const one = playerMap([
      player("emp"),
      player("left", { alignment: "evil" }),
      player("y", { alignment: "good" }),
    ]);
    expect(computeEmpath(order, one, "emp")).toEqual({
      min: 1,
      max: 1,
      unknown: false,
    });

    const two = playerMap([
      player("emp"),
      player("left", { alignment: "evil" }),
      player("y", { alignment: "evil" }),
    ]);
    expect(computeEmpath(order, two, "emp")).toEqual({
      min: 2,
      max: 2,
      unknown: false,
    });
  });

  it("respects an alignment override that flips a townsfolk to evil", () => {
    // 'left' is on the Townsfolk team but its effective alignment is evil
    // (e.g. Bounty Hunter / phase override) — the Empath should see it.
    const players = playerMap([
      player("emp"),
      player("left", { team: Team.TOWNSFOLK, alignment: "evil" }),
      player("y", { alignment: "good" }),
    ]);
    expect(computeEmpath(order, players, "emp")).toEqual({
      min: 1,
      max: 1,
      unknown: false,
    });
  });

  it("skips dead neighbours", () => {
    const players = playerMap([
      player("emp"),
      player("left", { isDead: true }),
      player("right", { alignment: "evil" }),
      player("y", { alignment: "good" }),
    ]);
    // clockwise neighbour is 'right' (left is dead) -> 1 evil.
    expect(computeEmpath(order, players, "emp")).toEqual({
      min: 1,
      max: 1,
      unknown: false,
    });
  });

  it("flags unknown for an undefined-alignment neighbour", () => {
    const players = playerMap([
      player("emp"),
      player("left", { team: Team.TRAVELLER, alignment: undefined }),
      player("y", { alignment: "good" }),
    ]);
    expect(computeEmpath(order, players, "emp")).toEqual({
      min: 0,
      max: 0,
      unknown: true,
    });
  });

  it("counts a shared neighbour once with only two alive", () => {
    const players = playerMap([
      player("emp"),
      player("x", { alignment: "evil" }),
      player("y", { isDead: true }),
    ]);
    expect(computeEmpath(["emp", "x", "y"], players, "emp")).toEqual({
      min: 1,
      max: 1,
      unknown: false,
    });
  });

  it("gives a 0–1 range for a Recluse neighbour (may register evil)", () => {
    const order = ["emp", "recluse", "x", "y", "good1"];
    const players = playerMap([
      player("emp"),
      player("recluse", { team: Team.OUTSIDER, alignment: "good" }),
      player("good1", { alignment: "good" }),
    ]);
    // cw neighbour recluse (ambiguous), ccw neighbour good1 (good).
    expect(computeEmpath(order, players, "emp")).toEqual({
      min: 0,
      max: 1,
      unknown: false,
    });
  });

  it("gives a 0–1 range for a Spy neighbour (may register good)", () => {
    const order = ["emp", "spy", "x", "y", "good1"];
    const players = playerMap([
      player("emp"),
      player("spy", { team: Team.MINION, alignment: "evil" }),
      player("good1", { alignment: "good" }),
    ]);
    expect(computeEmpath(order, players, "emp")).toEqual({
      min: 0,
      max: 1,
      unknown: false,
    });
  });

  it("gives a 0–2 range with both a Recluse and a Spy neighbour", () => {
    const order = ["emp", "recluse", "x", "y", "spy"];
    const players = playerMap([
      player("emp"),
      player("recluse", { team: Team.OUTSIDER, alignment: "good" }),
      player("spy", { team: Team.MINION, alignment: "evil" }),
    ]);
    expect(computeEmpath(order, players, "emp")).toEqual({
      min: 0,
      max: 2,
      unknown: false,
    });
  });

  it("skips a dead Recluse neighbour entirely", () => {
    const order = ["emp", "recluse", "good1"];
    const players = playerMap([
      player("emp"),
      player("recluse", {
        team: Team.OUTSIDER,
        alignment: "good",
        isDead: true,
      }),
      player("good1", { alignment: "good" }),
    ]);
    // Both neighbours resolve to good1 (recluse dead, skipped) -> exact 0.
    expect(computeEmpath(order, players, "emp")).toEqual({
      min: 0,
      max: 0,
      unknown: false,
    });
  });

  it("skips a dead evil neighbour, walking on to a good one (=> 0)", () => {
    const order = ["emp", "deadevil", "good1"];
    const players = playerMap([
      player("emp"),
      player("deadevil", { alignment: "evil", isDead: true }),
      player("good1", { alignment: "good" }),
    ]);
    expect(computeEmpath(order, players, "emp")).toEqual({
      min: 0,
      max: 0,
      unknown: false,
    });
  });

  it("skips a dead evil neighbour, walking on to an evil one (=> 1)", () => {
    const order = ["emp", "deadevil", "aliveevil"];
    const players = playerMap([
      player("emp"),
      player("deadevil", { alignment: "evil", isDead: true }),
      player("aliveevil", { alignment: "evil" }),
    ]);
    expect(computeEmpath(order, players, "emp")).toEqual({
      min: 1,
      max: 1,
      unknown: false,
    });
  });

  it("treats an alignment-flipped Recluse as ambiguous, not definite evil", () => {
    const order = ["emp", "recluse", "x", "y", "good1"];
    const players = playerMap([
      player("emp"),
      // Flipped to evil by an override, but a Recluse is still a Recluse.
      player("recluse", { team: Team.OUTSIDER, alignment: "evil" }),
      player("good1", { alignment: "good" }),
    ]);
    expect(computeEmpath(order, players, "emp")).toEqual({
      min: 0,
      max: 1,
      unknown: false,
    });
  });
});

describe("computeChef", () => {
  it("counts adjacent evil pairs with wrap-around (exact range)", () => {
    const order = ["a", "b", "c", "d"];
    const players = playerMap([
      player("a", { alignment: "evil" }),
      player("b", { alignment: "good" }),
      player("c", { alignment: "good" }),
      player("d", { alignment: "evil" }),
    ]);
    expect(computeChef(order, players)).toEqual({
      min: 1,
      max: 1,
      unknown: false,
    });
  });

  it("counts three evil in a row as two pairs", () => {
    const order = ["a", "b", "c", "d", "e"];
    const players = playerMap([
      player("a", { alignment: "evil" }),
      player("b", { alignment: "evil" }),
      player("c", { alignment: "evil" }),
      player("d", { alignment: "good" }),
      player("e", { alignment: "good" }),
    ]);
    expect(computeChef(order, players)).toEqual({
      min: 2,
      max: 2,
      unknown: false,
    });
  });

  it("flags unknown with an undefined-alignment player", () => {
    const order = ["a", "b", "c"];
    const players = playerMap([
      player("a", { alignment: "evil" }),
      player("b", { alignment: "evil" }),
      player("c", { team: Team.TRAVELLER, alignment: undefined }),
    ]);
    expect(computeChef(order, players)).toEqual({
      min: 1,
      max: 1,
      unknown: true,
    });
  });

  it("gives 0–2 for evil / Recluse / evil (Recluse between two evils)", () => {
    // Recluse sits between two evils who are not otherwise adjacent.
    const order = ["evil1", "recluse", "evil2", "good1", "good2"];
    const players = playerMap([
      player("evil1", { alignment: "evil" }),
      player("recluse", { team: Team.OUTSIDER, alignment: "good" }),
      player("evil2", { alignment: "evil" }),
      player("good1", { alignment: "good" }),
      player("good2", { alignment: "good" }),
    ]);
    // min 0 (no two definite evils adjacent), max 2 (evil1-recluse, recluse-evil2).
    expect(computeChef(order, players)).toEqual({
      min: 0,
      max: 2,
      unknown: false,
    });
  });

  it("is unaffected by a Recluse not adjacent to any evil-registering player", () => {
    const order = ["evil1", "evil2", "good1", "recluse", "good2"];
    const players = playerMap([
      player("evil1", { alignment: "evil" }),
      player("evil2", { alignment: "evil" }),
      player("good1", { alignment: "good" }),
      player("recluse", { team: Team.OUTSIDER, alignment: "good" }),
      player("good2", { alignment: "good" }),
    ]);
    // evil1-evil2 is a definite pair; the Recluse sits between two goods.
    expect(computeChef(order, players)).toEqual({
      min: 1,
      max: 1,
      unknown: false,
    });
  });
});

describe("chefRangeCauses", () => {
  it("reports a Recluse that widens the range", () => {
    const order = ["evil1", "recluse", "evil2", "good1", "good2"];
    const players = playerMap([
      player("evil1", { alignment: "evil" }),
      player("recluse", { team: Team.OUTSIDER, alignment: "good" }),
      player("evil2", { alignment: "evil" }),
      player("good1", { alignment: "good" }),
      player("good2", { alignment: "good" }),
    ]);
    expect(chefRangeCauses(order, players)).toEqual({
      recluse: true,
      spy: false,
    });
  });

  it("does not report a Recluse with only good neighbours", () => {
    const order = ["evil1", "evil2", "good1", "recluse", "good2"];
    const players = playerMap([
      player("evil1", { alignment: "evil" }),
      player("evil2", { alignment: "evil" }),
      player("good1", { alignment: "good" }),
      player("recluse", { team: Team.OUTSIDER, alignment: "good" }),
      player("good2", { alignment: "good" }),
    ]);
    expect(chefRangeCauses(order, players)).toEqual({
      recluse: false,
      spy: false,
    });
  });

  it("reports a Spy adjacent to an evil-registering player", () => {
    const order = ["evil1", "spy", "good1", "good2"];
    const players = playerMap([
      player("evil1", { alignment: "evil" }),
      player("spy", { team: Team.MINION, alignment: "evil" }),
      player("good1", { alignment: "good" }),
      player("good2", { alignment: "good" }),
    ]);
    // spy (ambiguous) adjacent to evil1 -> spy widens the range.
    expect(chefRangeCauses(order, players)).toEqual({
      recluse: false,
      spy: true,
    });
  });
});

describe("computeFortuneTeller", () => {
  const players = playerMap([
    player("ft"),
    player("imp", { team: Team.DEMON, alignment: "evil" }),
    player("herring", { team: Team.TOWNSFOLK, alignment: "good" }),
    player("townsfolk", { team: Team.TOWNSFOLK, alignment: "good" }),
    player("recluse", { team: Team.OUTSIDER, alignment: "good" }),
  ]);

  it("returns undefined until two players are picked", () => {
    expect(computeFortuneTeller([], players, "herring")).toBeUndefined();
    expect(computeFortuneTeller(["ft"], players, "herring")).toBeUndefined();
  });

  it("says yes when a picked player is the Demon", () => {
    expect(
      computeFortuneTeller(["imp", "townsfolk"], players, "herring"),
    ).toEqual({ answer: "yes", viaRedHerring: false, recluseMayYes: false });
  });

  it("says yes via the red herring when no demon is picked", () => {
    expect(
      computeFortuneTeller(["herring", "townsfolk"], players, "herring"),
    ).toEqual({ answer: "yes", viaRedHerring: true, recluseMayYes: false });
  });

  it("does not flag red herring when a real demon is also picked", () => {
    expect(
      computeFortuneTeller(["imp", "herring"], players, "herring"),
    ).toEqual({ answer: "yes", viaRedHerring: false, recluseMayYes: false });
  });

  it("says no when neither a demon nor the red herring is picked", () => {
    expect(
      computeFortuneTeller(["townsfolk", "ft"], players, "herring"),
    ).toEqual({ answer: "no", viaRedHerring: false, recluseMayYes: false });
  });

  it("says no when there is no red herring and no demon", () => {
    expect(
      computeFortuneTeller(["townsfolk", "ft"], players, undefined),
    ).toEqual({ answer: "no", viaRedHerring: false, recluseMayYes: false });
  });

  it("flags recluseMayYes on a NO answer when a Recluse is picked", () => {
    expect(
      computeFortuneTeller(["recluse", "townsfolk"], players, "herring"),
    ).toEqual({ answer: "no", viaRedHerring: false, recluseMayYes: true });
  });

  it("does not flag recluseMayYes when the answer is already YES", () => {
    // Recluse + real Demon -> answer is yes, so no "may register" caveat.
    expect(
      computeFortuneTeller(["recluse", "imp"], players, "herring"),
    ).toEqual({ answer: "yes", viaRedHerring: false, recluseMayYes: false });
  });
});

describe("classifyRegistration", () => {
  it("classifies definite evil / good by effective alignment", () => {
    expect(classifyRegistration(player("x", { alignment: "evil" }))).toBe(
      "evil",
    );
    expect(classifyRegistration(player("x", { alignment: "good" }))).toBe(
      "good",
    );
  });

  it("classifies a Recluse/Spy as ambiguous regardless of alignment", () => {
    expect(classifyRegistration(player("recluse", { alignment: "good" }))).toBe(
      "ambiguous",
    );
    expect(classifyRegistration(player("spy", { alignment: "evil" }))).toBe(
      "ambiguous",
    );
    // Alignment-flipped Recluse is still ambiguous.
    expect(classifyRegistration(player("recluse", { alignment: "evil" }))).toBe(
      "ambiguous",
    );
  });

  it("classifies an unset alignment as unknown", () => {
    expect(
      classifyRegistration(
        player("trav", { team: Team.TRAVELLER, alignment: undefined }),
      ),
    ).toBe("unknown");
    expect(classifyRegistration(undefined)).toBe("unknown");
  });
});

describe("findExecutedToday", () => {
  it("returns undefined with no previous round", () => {
    expect(findExecutedToday(undefined)).toBeUndefined();
    expect(findExecutedToday({})).toBeUndefined();
  });

  it("returns an exact match for an EXECUTION-cause day death", () => {
    const prev = {
      night: phase([death("imp", DeathCause.DEMON)]),
      day: phase([
        death("imp", DeathCause.DEMON),
        death("saint", DeathCause.EXECUTION),
      ]),
    };
    expect(findExecutedToday(prev)).toEqual({
      roleId: "saint",
      heuristic: false,
    });
  });

  it("falls back to a heuristic for legacy rows (no cause)", () => {
    // Legacy day death not present in the night -> assumed execution.
    const prev = {
      night: phase([death("empath")]),
      day: phase([death("empath"), death("virgin")]),
    };
    expect(findExecutedToday(prev)).toEqual({
      roleId: "virgin",
      heuristic: true,
    });
  });

  it("excludes night deaths propagated into the day", () => {
    // Only the night death appears in the day -> no execution today.
    const prev = {
      night: phase([death("empath")]),
      day: phase([death("empath")]),
    };
    expect(findExecutedToday(prev)).toBeUndefined();
  });

  it("returns undefined when the day has no deaths", () => {
    expect(
      findExecutedToday({ night: phase([death("empath")]), day: phase([]) }),
    ).toBeUndefined();
  });
});
