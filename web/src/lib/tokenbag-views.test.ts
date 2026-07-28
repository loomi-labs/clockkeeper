import { describe, it, expect } from "vitest";
import { TokenBagPhase } from "./gen/clockkeeper/v1/clockkeeper_pb";
import type { PlayerView } from "./tokenbag";
import { deriveDeviceView, refinePlayerView } from "./tokenbag-views";

const FRESH = { settled: false, editing: false, streamDead: false };

describe("refinePlayerView", () => {
  it("shows the picker while nothing has been submitted or skipped", () => {
    const view: PlayerView = { kind: "neighbor_pick", hasNeighbors: false };
    expect(refinePlayerView(view, FRESH)).toEqual(view);
  });

  it("switches to waiting after submitting", () => {
    const view: PlayerView = { kind: "neighbor_pick", hasNeighbors: true };
    expect(refinePlayerView(view, { ...FRESH, settled: true })).toEqual({
      kind: "waiting_reveal",
    });
  });

  it("switches to waiting after skipping, with no neighbors saved", () => {
    const view: PlayerView = { kind: "neighbor_pick", hasNeighbors: false };
    expect(refinePlayerView(view, { ...FRESH, settled: true })).toEqual({
      kind: "waiting_reveal",
    });
  });

  it("waits when the server already knows this player's neighbors", () => {
    // A reload during the closed phase: nothing was settled in this session, but
    // the answer is already in.
    const view: PlayerView = { kind: "neighbor_pick", hasNeighbors: true };
    expect(refinePlayerView(view, FRESH)).toEqual({ kind: "waiting_reveal" });
  });

  it("reopens the picker while editing, even once settled", () => {
    const view: PlayerView = { kind: "neighbor_pick", hasNeighbors: true };
    expect(
      refinePlayerView(view, { ...FRESH, settled: true, editing: true }),
    ).toEqual(view);
  });

  it("passes every other view through untouched", () => {
    for (const view of [
      { kind: "loading" },
      { kind: "enter_name" },
      { kind: "waiting_open" },
      { kind: "revealed_shown" },
      { kind: "revealed_hidden" },
      { kind: "removed" },
      { kind: "game_started" },
      { kind: "gone" },
    ] satisfies PlayerView[]) {
      expect(
        refinePlayerView(view, { ...FRESH, settled: true, editing: true }),
      ).toBe(view);
    }
  });

  it("is gone once the stream is dead, whatever the last snapshot said", () => {
    // The Storyteller deleted or reset the game: the phase in hand is stale, so
    // no screen derived from it may keep claiming to be live.
    for (const view of [
      { kind: "enter_name" },
      { kind: "waiting_open" },
      { kind: "neighbor_pick", hasNeighbors: false },
      { kind: "waiting_reveal" },
      { kind: "revealed_shown" },
      { kind: "revealed_hidden" },
      { kind: "removed" },
    ] satisfies PlayerView[]) {
      expect(refinePlayerView(view, { ...FRESH, streamDead: true })).toEqual({
        kind: "gone",
      });
    }
  });

  it("lets a dead stream win over an open picker", () => {
    const view: PlayerView = { kind: "neighbor_pick", hasNeighbors: false };
    expect(
      refinePlayerView(view, { ...FRESH, editing: true, streamDead: true }),
    ).toEqual({ kind: "gone" });
  });
});

describe("deriveDeviceView", () => {
  it("loads until the first snapshot arrives", () => {
    expect(
      deriveDeviceView(TokenBagPhase.UNSPECIFIED, "connecting", false),
    ).toBe("loading");
    expect(
      deriveDeviceView(TokenBagPhase.UNSPECIFIED, "reconnecting", false),
    ).toBe("loading");
  });

  it("is gone when the stream gave up before any snapshot", () => {
    expect(deriveDeviceView(TokenBagPhase.UNSPECIFIED, "stopped", false)).toBe(
      "gone",
    );
  });

  it("is gone when the stream dies after a snapshot", () => {
    // Otherwise the tablet keeps offering a tappable name grid for a game that
    // no longer exists.
    for (const phase of [
      TokenBagPhase.OPEN,
      TokenBagPhase.CLOSED,
      TokenBagPhase.REVEALED,
      TokenBagPhase.INACTIVE,
    ]) {
      expect(deriveDeviceView(phase, "stopped", false)).toBe("gone");
    }
  });

  it("maps each phase to its screen", () => {
    expect(deriveDeviceView(TokenBagPhase.OPEN, "live", false)).toBe(
      "add_names",
    );
    expect(deriveDeviceView(TokenBagPhase.CLOSED, "live", false)).toBe(
      "closed",
    );
    expect(deriveDeviceView(TokenBagPhase.REVEALED, "live", false)).toBe(
      "reveal_list",
    );
    expect(deriveDeviceView(TokenBagPhase.INACTIVE, "live", false)).toBe(
      "gone",
    );
  });

  // The phase stays REVEALED when the game starts, so only the flag can take the
  // tappable name grid away — and the server refuses every reveal from then on.
  it("says the game has started, whatever the phase says", () => {
    for (const phase of [
      TokenBagPhase.OPEN,
      TokenBagPhase.CLOSED,
      TokenBagPhase.REVEALED,
    ]) {
      expect(deriveDeviceView(phase, "live", true)).toBe("game_started");
    }
  });

  it("lets a dead stream win over a started game", () => {
    expect(deriveDeviceView(TokenBagPhase.REVEALED, "stopped", true)).toBe(
      "gone",
    );
  });
});
