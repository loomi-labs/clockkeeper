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

  it("counts 0, 1 and 2 evil neighbours", () => {
    const good = playerMap([
      player("emp"),
      player("left", { alignment: "good" }),
      player("y", { alignment: "good" }),
    ]);
    expect(computeEmpath(order, good, "emp")).toEqual({
      count: 0,
      unknown: false,
    });

    const one = playerMap([
      player("emp"),
      player("left", { alignment: "evil" }),
      player("y", { alignment: "good" }),
    ]);
    expect(computeEmpath(order, one, "emp")).toEqual({
      count: 1,
      unknown: false,
    });

    const two = playerMap([
      player("emp"),
      player("left", { alignment: "evil" }),
      player("y", { alignment: "evil" }),
    ]);
    expect(computeEmpath(order, two, "emp")).toEqual({
      count: 2,
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
      count: 1,
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
      count: 1,
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
      count: 0,
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
      count: 1,
      unknown: false,
    });
  });
});

describe("computeChef", () => {
  it("counts adjacent evil pairs with wrap-around", () => {
    const order = ["a", "b", "c", "d"];
    const players = playerMap([
      player("a", { alignment: "evil" }),
      player("b", { alignment: "good" }),
      player("c", { alignment: "good" }),
      player("d", { alignment: "evil" }),
    ]);
    expect(computeChef(order, players)).toEqual({ pairs: 1, unknown: false });
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
    expect(computeChef(order, players)).toEqual({ pairs: 2, unknown: false });
  });

  it("flags unknown with an undefined-alignment player", () => {
    const order = ["a", "b", "c"];
    const players = playerMap([
      player("a", { alignment: "evil" }),
      player("b", { alignment: "evil" }),
      player("c", { team: Team.TRAVELLER, alignment: undefined }),
    ]);
    expect(computeChef(order, players)).toEqual({ pairs: 1, unknown: true });
  });
});

describe("computeFortuneTeller", () => {
  const players = playerMap([
    player("ft"),
    player("imp", { team: Team.DEMON, alignment: "evil" }),
    player("herring", { team: Team.TOWNSFOLK, alignment: "good" }),
    player("townsfolk", { team: Team.TOWNSFOLK, alignment: "good" }),
  ]);

  it("returns undefined until two players are picked", () => {
    expect(computeFortuneTeller([], players, "herring")).toBeUndefined();
    expect(computeFortuneTeller(["ft"], players, "herring")).toBeUndefined();
  });

  it("says yes when a picked player is the Demon", () => {
    expect(
      computeFortuneTeller(["imp", "townsfolk"], players, "herring"),
    ).toEqual({ answer: "yes", viaRedHerring: false });
  });

  it("says yes via the red herring when no demon is picked", () => {
    expect(
      computeFortuneTeller(["herring", "townsfolk"], players, "herring"),
    ).toEqual({ answer: "yes", viaRedHerring: true });
  });

  it("does not flag red herring when a real demon is also picked", () => {
    expect(
      computeFortuneTeller(["imp", "herring"], players, "herring"),
    ).toEqual({ answer: "yes", viaRedHerring: false });
  });

  it("says no when neither a demon nor the red herring is picked", () => {
    expect(
      computeFortuneTeller(["townsfolk", "ft"], players, "herring"),
    ).toEqual({ answer: "no", viaRedHerring: false });
  });

  it("says no when there is no red herring and no demon", () => {
    expect(
      computeFortuneTeller(["townsfolk", "ft"], players, undefined),
    ).toEqual({ answer: "no", viaRedHerring: false });
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
