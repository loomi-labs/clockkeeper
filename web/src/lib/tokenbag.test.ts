import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import {
  TokenBagPhase,
  TokenBagSchema,
  WatchTokenBagResponseSchema,
} from "./gen/clockkeeper/v1/clockkeeper_pb";
import {
  applyOwnerBag,
  applySnapshot,
  derivePlayerView,
  deviceUrl,
  emptySnapshot,
  hasBothNeighbors,
  joinUrl,
  neighborOptions,
  normalizeName,
  unassignedRegistrants,
  type BagPlayer,
  type PlayerViewInput,
} from "./tokenbag";
import type { WatchStatus } from "./stream-retry";

function player(id: string, extra: Partial<BagPlayer> = {}): BagPlayer {
  return {
    id,
    name: `P${id}`,
    viaSharedDevice: false,
    leftId: "0",
    rightId: "0",
    ...extra,
  };
}

describe("applySnapshot", () => {
  it("normalizes int64 ids to strings", () => {
    const snapshot = applySnapshot(
      create(WatchTokenBagResponseSchema, {
        phase: TokenBagPhase.CLOSED,
        gameName: "Trouble Brewing",
        selfRegistrationId: 7n,
        players: [
          {
            registrationId: 7n,
            name: "Alice",
            viaSharedDevice: false,
            leftNeighborId: 8n,
            rightNeighborId: 0n,
          },
          {
            registrationId: 8n,
            name: "Bob",
            viaSharedDevice: true,
            leftNeighborId: 0n,
            rightNeighborId: 0n,
          },
        ],
      }),
    );

    expect(snapshot.phase).toBe(TokenBagPhase.CLOSED);
    expect(snapshot.gameName).toBe("Trouble Brewing");
    expect(snapshot.selfId).toBe("7");
    expect(snapshot.selfToken).toBeNull();
    expect(snapshot.players).toEqual([
      {
        id: "7",
        name: "Alice",
        viaSharedDevice: false,
        leftId: "8",
        rightId: "0",
      },
      {
        id: "8",
        name: "Bob",
        viaSharedDevice: true,
        leftId: "0",
        rightId: "0",
      },
    ]);
    for (const value of snapshot.players.flatMap((p) => [
      p.id,
      p.leftId,
      p.rightId,
    ])) {
      expect(typeof value).toBe("string");
    }
  });

  it("falls back to an empty snapshot", () => {
    expect(applySnapshot(undefined)).toEqual(emptySnapshot());
    expect(emptySnapshot().phase).toBe(TokenBagPhase.UNSPECIFIED);
    expect(emptySnapshot().selfId).toBe("0");
  });
});

describe("applyOwnerBag", () => {
  it("keeps the owner-only codes and normalizes players", () => {
    const bag = applyOwnerBag(
      create(TokenBagSchema, {
        phase: TokenBagPhase.OPEN,
        joinCode: "join-1",
        sharedCode: "shared-1",
        players: [{ registrationId: 3n, name: "Cleo" }],
      }),
    );
    expect(bag).toEqual({
      phase: TokenBagPhase.OPEN,
      joinCode: "join-1",
      sharedCode: "shared-1",
      players: [
        {
          id: "3",
          name: "Cleo",
          viaSharedDevice: false,
          leftId: "0",
          rightId: "0",
        },
      ],
    });
  });

  it("falls back to an inactive empty bag", () => {
    expect(applyOwnerBag(undefined)).toEqual({
      phase: TokenBagPhase.UNSPECIFIED,
      joinCode: "",
      sharedCode: "",
      players: [],
    });
  });
});

describe("URL builders", () => {
  it("builds join and device URLs", () => {
    expect(joinUrl("https://clock.example", "abc")).toBe(
      "https://clock.example/join/abc",
    );
    expect(deviceUrl("https://clock.example", "xyz")).toBe(
      "https://clock.example/device/xyz",
    );
  });

  it("does not double the slash", () => {
    expect(joinUrl("https://clock.example/", "abc")).toBe(
      "https://clock.example/join/abc",
    );
  });

  it("escapes the code", () => {
    expect(joinUrl("https://clock.example", "a/b c")).toBe(
      "https://clock.example/join/a%2Fb%20c",
    );
  });
});

describe("derivePlayerView", () => {
  function view(over: Partial<PlayerViewInput> = {}) {
    const input: PlayerViewInput = {
      phase: TokenBagPhase.OPEN,
      players: [],
      selfId: "0",
      hasCredential: false,
      dismissed: false,
      streamStatus: "live",
      ...over,
    };
    return derivePlayerView(input);
  }

  it("is loading until the first snapshot arrives", () => {
    for (const status of [
      "connecting",
      "live",
      "reconnecting",
    ] as WatchStatus[]) {
      expect(
        view({ phase: TokenBagPhase.UNSPECIFIED, streamStatus: status }),
      ).toEqual({ kind: "loading" });
    }
  });

  it("is gone when the stream died before any snapshot", () => {
    expect(
      view({ phase: TokenBagPhase.UNSPECIFIED, streamStatus: "stopped" }),
    ).toEqual({ kind: "gone" });
  });

  // phase x credential, with no self registration in the snapshot.
  const withoutCredential: [TokenBagPhase, string][] = [
    [TokenBagPhase.INACTIVE, "gone"],
    [TokenBagPhase.OPEN, "enter_name"],
    [TokenBagPhase.CLOSED, "gone"],
    [TokenBagPhase.REVEALED, "gone"],
  ];

  for (const [phase, kind] of withoutCredential) {
    it(`phase ${TokenBagPhase[phase]} without a credential -> ${kind}`, () => {
      expect(view({ phase, hasCredential: false }).kind).toBe(kind);
      // A dismissed flag left over from a previous game changes nothing.
      expect(view({ phase, hasCredential: false, dismissed: true }).kind).toBe(
        kind,
      );
    });
  }

  const registered = {
    hasCredential: true,
    selfId: "5",
    players: [player("5"), player("6")],
  };

  it("phase open with a credential -> waiting_open", () => {
    expect(view({ ...registered, phase: TokenBagPhase.OPEN }).kind).toBe(
      "waiting_open",
    );
  });

  it("phase closed with a credential -> neighbor_pick", () => {
    expect(view({ ...registered, phase: TokenBagPhase.CLOSED })).toEqual({
      kind: "neighbor_pick",
      hasNeighbors: false,
    });
    expect(
      view({
        ...registered,
        phase: TokenBagPhase.CLOSED,
        players: [player("5", { leftId: "6", rightId: "6" }), player("6")],
      }),
    ).toEqual({ kind: "neighbor_pick", hasNeighbors: true });
  });

  it("phase revealed respects the dismissed flag", () => {
    expect(view({ ...registered, phase: TokenBagPhase.REVEALED }).kind).toBe(
      "revealed_shown",
    );
    expect(
      view({ ...registered, phase: TokenBagPhase.REVEALED, dismissed: true })
        .kind,
    ).toBe("revealed_hidden");
  });

  it("phase inactive with a credential -> gone (the bag was reset)", () => {
    expect(view({ ...registered, phase: TokenBagPhase.INACTIVE }).kind).toBe(
      "gone",
    );
    expect(
      view({ ...registered, phase: TokenBagPhase.INACTIVE, dismissed: true })
        .kind,
    ).toBe("gone");
  });

  it("a credential the server no longer knows -> removed (kicked)", () => {
    for (const phase of [
      TokenBagPhase.OPEN,
      TokenBagPhase.CLOSED,
      TokenBagPhase.REVEALED,
    ]) {
      expect(
        view({
          phase,
          hasCredential: true,
          selfId: "0",
          players: [player("6")],
        }).kind,
      ).toBe("removed");
      expect(
        view({ phase, hasCredential: true, selfId: "", players: [player("6")] })
          .kind,
      ).toBe("removed");
    }
  });
});

describe("hasBothNeighbors", () => {
  it("needs both sides claimed", () => {
    expect(hasBothNeighbors([player("1")], "1")).toBe(false);
    expect(hasBothNeighbors([player("1", { leftId: "2" })], "1")).toBe(false);
    expect(hasBothNeighbors([player("1", { rightId: "2" })], "1")).toBe(false);
    expect(
      hasBothNeighbors([player("1", { leftId: "2", rightId: "3" })], "1"),
    ).toBe(true);
  });

  it("is false when self is not in the list", () => {
    expect(hasBothNeighbors([player("2")], "1")).toBe(false);
  });
});

describe("neighborOptions", () => {
  it("excludes self and keeps everyone else in order", () => {
    const players = [player("1"), player("2"), player("3")];
    expect(neighborOptions(players, "2").map((p) => p.id)).toEqual(["1", "3"]);
  });

  it("returns everyone when self is unknown", () => {
    const players = [player("1"), player("2")];
    expect(neighborOptions(players, "0").map((p) => p.id)).toEqual(["1", "2"]);
  });
});

describe("normalizeName", () => {
  it("lowercases, trims and collapses whitespace", () => {
    expect(normalizeName("  Ana   Belle  ")).toBe("ana belle");
    expect(normalizeName("ANA\tBELLE")).toBe("ana belle");
    expect(normalizeName("Ana\nBelle")).toBe("ana belle");
  });

  it("drops control characters", () => {
    expect(normalizeName("An\u0001a")).toBe("ana");
    expect(normalizeName("An \u0001 a")).toBe("an a");
  });
});

describe("unassignedRegistrants", () => {
  it("removes assigned names case- and whitespace-insensitively", () => {
    const registrants = ["Ana Belle", "Bob", "Cleo"];
    const assigned = new Set(["  ana   belle ", "CLEO"]);
    expect(unassignedRegistrants(registrants, assigned)).toEqual(["Bob"]);
  });

  it("keeps everything when nothing is assigned", () => {
    expect(unassignedRegistrants(["Ana", "Bob"], new Set())).toEqual([
      "Ana",
      "Bob",
    ]);
  });

  it("preserves registration order and the original spelling", () => {
    expect(
      unassignedRegistrants(["Zoe", "Ana Belle"], new Set(["zoe"])),
    ).toEqual(["Ana Belle"]);
  });
});
