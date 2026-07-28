import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { Code, ConnectError } from "@connectrpc/connect";
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import {
  CharacterSchema,
  TokenBagPhase,
  TokenBagSchema,
  WatchTokenBagResponseSchema,
  type WatchTokenBagResponse,
} from "./gen/clockkeeper/v1/clockkeeper_pb";
import {
  createPlayerBag,
  createDeviceBag,
  createStorytellerBag,
  type OwnerBagClient,
  type PublicBagClient,
} from "./tokenbag.svelte";
import {
  isDismissed,
  loadCredential,
  saveCredential,
} from "./tokenbag-credentials";

const CODE = "code-1";

type StreamOptions = { signal: AbortSignal };

/** A stream we can feed from the test; ends when the loop aborts it. */
function channel() {
  const queued: WatchTokenBagResponse[] = [];
  let wake: (() => void) | null = null;

  return {
    push(resp: WatchTokenBagResponse) {
      queued.push(resp);
      wake?.();
      wake = null;
    },
    async *open(signal: AbortSignal): AsyncIterable<WatchTokenBagResponse> {
      while (true) {
        while (queued.length > 0) {
          yield queued.shift() as WatchTokenBagResponse;
        }
        if (signal.aborted) throw new ConnectError("canceled", Code.Canceled);
        await new Promise<void>((resolve) => {
          wake = resolve;
          signal.addEventListener("abort", () => resolve(), { once: true });
        });
        if (signal.aborted) throw new ConnectError("canceled", Code.Canceled);
      }
    },
  };
}

/** A stream that fails on open with the given error. */
function failingStream(err: unknown) {
  return vi.fn(() =>
    (async function* (): AsyncIterable<WatchTokenBagResponse> {
      throw err;
    })(),
  );
}

function snapshot(
  over: MessageInitShape<typeof WatchTokenBagResponseSchema> = {},
) {
  return create(WatchTokenBagResponseSchema, {
    phase: TokenBagPhase.OPEN,
    gameName: "Trouble Brewing",
    ...over,
  });
}

const CHARACTER = create(CharacterSchema, {
  id: "washerwoman",
  name: "Washerwoman",
});

/** Lets the watch loop's microtasks settle. */
async function flush() {
  await vi.advanceTimersByTimeAsync(0);
}

describe("createPlayerBag", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("applies stream snapshots and status", async () => {
    const stream = channel();
    const api = {
      watchTokenBag: vi.fn((_req: unknown, opts: StreamOptions) =>
        stream.open(opts.signal),
      ),
    } as unknown as PublicBagClient;

    const bag = createPlayerBag(CODE, api);
    bag.start();
    await flush();
    expect(bag.state.status).toBe("connecting");

    stream.push(
      snapshot({
        phase: TokenBagPhase.CLOSED,
        selfRegistrationId: 5n,
        players: [{ registrationId: 5n, name: "Alice" }],
      }),
    );
    await flush();

    expect(bag.state.status).toBe("live");
    expect(bag.state.phase).toBe(TokenBagPhase.CLOSED);
    expect(bag.state.gameName).toBe("Trouble Brewing");
    expect(bag.state.selfId).toBe("5");
    expect(bag.state.players).toEqual([
      {
        id: "5",
        name: "Alice",
        viaSharedDevice: false,
        leftId: "0",
        rightId: "0",
      },
    ]);

    bag.stop();
    expect(bag.state.status).toBe("stopped");
  });

  it("watches without a secret until one is stored", async () => {
    const stream = channel();
    const watchTokenBag = vi.fn((_req: unknown, opts: StreamOptions) =>
      stream.open(opts.signal),
    );
    const bag = createPlayerBag(CODE, {
      watchTokenBag,
    } as unknown as PublicBagClient);

    expect(bag.state.hasCredential).toBe(false);
    bag.start();
    await flush();
    expect(watchTokenBag).toHaveBeenCalledTimes(1);
    expect(watchTokenBag.mock.calls[0][0]).toEqual({
      code: CODE,
      registrationSecret: "",
    });
    bag.stop();
  });

  it("register stores the credential and restarts the stream with the secret", async () => {
    const stream = channel();
    const watchTokenBag = vi.fn((_req: unknown, opts: StreamOptions) =>
      stream.open(opts.signal),
    );
    const joinTokenBag = vi.fn(async () => ({
      registrationId: 9n,
      registrationSecret: "sekrit",
      snapshot: snapshot({
        selfRegistrationId: 9n,
        players: [{ registrationId: 9n, name: "Alice" }],
      }),
    }));
    const bag = createPlayerBag(CODE, {
      watchTokenBag,
      joinTokenBag,
    } as unknown as PublicBagClient);

    bag.start();
    await flush();

    expect(await bag.register("Alice")).toBe(true);
    await flush();

    expect(joinTokenBag).toHaveBeenCalledWith({
      joinCode: CODE,
      name: "Alice",
    });
    expect(loadCredential(CODE)).toEqual({
      registrationId: "9",
      secret: "sekrit",
      name: "Alice",
    });
    expect(bag.state.hasCredential).toBe(true);
    // The response snapshot is applied without waiting for the stream.
    expect(bag.state.selfId).toBe("9");
    // Reopened, this time carrying the secret.
    expect(watchTokenBag).toHaveBeenCalledTimes(2);
    expect(watchTokenBag.mock.calls[1][0]).toEqual({
      code: CODE,
      registrationSecret: "sekrit",
    });

    bag.stop();
  });

  it("register surfaces a rejected name and stores nothing", async () => {
    const joinTokenBag = vi.fn(async () => {
      throw new ConnectError("name already taken", Code.AlreadyExists);
    });
    const bag = createPlayerBag(CODE, {
      joinTokenBag,
    } as unknown as PublicBagClient);

    expect(await bag.register("Alice")).toBe(false);
    expect(bag.state.error).toContain("name already taken");
    expect(loadCredential(CODE)).toBeNull();
    expect(bag.state.hasCredential).toBe(false);
  });

  it("setNeighbors sends the stored secret and applies the snapshot", async () => {
    saveCredential(CODE, {
      registrationId: "9",
      secret: "sekrit",
      name: "Alice",
    });
    const setTokenBagNeighbors = vi.fn(async () => ({
      snapshot: snapshot({
        phase: TokenBagPhase.CLOSED,
        selfRegistrationId: 9n,
        players: [
          {
            registrationId: 9n,
            name: "Alice",
            leftNeighborId: 3n,
            rightNeighborId: 4n,
          },
        ],
      }),
    }));
    const bag = createPlayerBag(CODE, {
      setTokenBagNeighbors,
    } as unknown as PublicBagClient);

    expect(await bag.setNeighbors("3", "4")).toBe(true);
    expect(setTokenBagNeighbors).toHaveBeenCalledWith({
      registrationSecret: "sekrit",
      leftRegistrationId: 3n,
      rightRegistrationId: 4n,
    });
    expect(bag.state.players[0].leftId).toBe("3");
    expect(bag.state.players[0].rightId).toBe("4");
  });

  it("setNeighbors maps empty and junk ids to the 0 sentinel", async () => {
    saveCredential(CODE, { registrationId: "9", secret: "s", name: "Alice" });
    const setTokenBagNeighbors = vi.fn(async () => ({
      snapshot: snapshot(),
    }));
    const bag = createPlayerBag(CODE, {
      setTokenBagNeighbors,
    } as unknown as PublicBagClient);

    expect(await bag.setNeighbors("", "nope")).toBe(true);
    expect(setTokenBagNeighbors).toHaveBeenCalledWith({
      registrationSecret: "s",
      leftRegistrationId: 0n,
      rightRegistrationId: 0n,
    });
  });

  it("setNeighbors refuses without a credential", async () => {
    const setTokenBagNeighbors = vi.fn();
    const bag = createPlayerBag(CODE, {
      setTokenBagNeighbors,
    } as unknown as PublicBagClient);

    expect(await bag.setNeighbors("1", "2")).toBe(false);
    expect(setTokenBagNeighbors).not.toHaveBeenCalled();
    expect(bag.state.error).toContain("not registered");
  });

  it("fetchMyToken caches the character in state", async () => {
    saveCredential(CODE, { registrationId: "9", secret: "s", name: "Alice" });
    const getMyToken = vi.fn(async () => ({ character: CHARACTER }));
    const bag = createPlayerBag(CODE, {
      getMyToken,
    } as unknown as PublicBagClient);

    expect(await bag.fetchMyToken()).toBe(CHARACTER);
    expect(getMyToken).toHaveBeenCalledWith({ registrationSecret: "s" });
    // Reactive state hands back a proxy, so compare by value.
    expect(bag.state.selfToken).toEqual(CHARACTER);
  });

  it("fetchMyToken refuses without a credential", async () => {
    const getMyToken = vi.fn();
    const bag = createPlayerBag(CODE, {
      getMyToken,
    } as unknown as PublicBagClient);

    expect(await bag.fetchMyToken()).toBeNull();
    expect(getMyToken).not.toHaveBeenCalled();
  });

  it("keeps the revealed token across snapshots but drops it on reset", async () => {
    const stream = channel();
    const bag = createPlayerBag(CODE, {
      watchTokenBag: vi.fn((_req: unknown, opts: StreamOptions) =>
        stream.open(opts.signal),
      ),
    } as unknown as PublicBagClient);

    bag.start();
    await flush();

    stream.push(
      snapshot({
        phase: TokenBagPhase.REVEALED,
        selfRegistrationId: 1n,
        selfToken: { id: "washerwoman", name: "Washerwoman" },
      }),
    );
    await flush();
    expect(bag.state.selfToken?.id).toBe("washerwoman");

    // A later revealed snapshot without the token (a plain re-send) keeps it.
    stream.push(
      snapshot({ phase: TokenBagPhase.REVEALED, selfRegistrationId: 1n }),
    );
    await flush();
    expect(bag.state.selfToken?.id).toBe("washerwoman");

    // A reset wipes it.
    stream.push(snapshot({ phase: TokenBagPhase.INACTIVE }));
    await flush();
    expect(bag.state.selfToken).toBeNull();

    bag.stop();
  });

  it("marks the bag gone on a fatal NotFound", async () => {
    const watchTokenBag = failingStream(
      new ConnectError("token bag not found", Code.NotFound),
    );
    const bag = createPlayerBag(CODE, {
      watchTokenBag,
    } as unknown as PublicBagClient);

    bag.start();
    await flush();

    expect(bag.state.gone).toBe(true);
    expect(bag.state.status).toBe("stopped");
    expect(bag.state.error).toContain("token bag not found");
    // Fatal means no retry.
    await vi.advanceTimersByTimeAsync(60_000);
    expect(watchTokenBag).toHaveBeenCalledTimes(1);
  });

  it("can start a fresh stream after a fatal stop", async () => {
    const stream = channel();
    let attempt = 0;
    const watchTokenBag = vi.fn((_req: unknown, opts: StreamOptions) => {
      attempt++;
      if (attempt === 1) {
        return (async function* (): AsyncIterable<WatchTokenBagResponse> {
          throw new ConnectError("token bag not found", Code.NotFound);
        })();
      }
      return stream.open(opts.signal);
    });
    const bag = createPlayerBag(CODE, {
      watchTokenBag,
    } as unknown as PublicBagClient);

    bag.start();
    await flush();
    expect(bag.state.status).toBe("stopped");
    expect(bag.state.gone).toBe(true);

    // The loop terminated itself; start() must not be a silent no-op.
    bag.start();
    await flush();
    expect(watchTokenBag).toHaveBeenCalledTimes(2);

    stream.push(snapshot({ selfRegistrationId: 1n }));
    await flush();
    expect(bag.state.status).toBe("live");
    bag.stop();
  });

  it("does not mark the bag gone on a transport failure", async () => {
    const bag = createPlayerBag(CODE, {
      watchTokenBag: failingStream(
        new ConnectError("offline", Code.Unavailable),
      ),
    } as unknown as PublicBagClient);

    bag.start();
    await flush();
    expect(bag.state.gone).toBe(false);
    expect(bag.state.status).toBe("reconnecting");
    bag.stop();
  });

  it("tracks the dismissed flag through localStorage", () => {
    const empty = {} as unknown as PublicBagClient;
    const bag = createPlayerBag(CODE, empty);

    expect(bag.state.dismissed).toBe(false);
    bag.dismissToken(true);
    expect(bag.state.dismissed).toBe(true);
    expect(isDismissed(CODE)).toBe(true);
    // A fresh instance picks up what was persisted.
    expect(createPlayerBag(CODE, empty).state.dismissed).toBe(true);
    bag.dismissToken(false);
    expect(isDismissed(CODE)).toBe(false);
  });

  it("forget clears the credential and the dismissed flag", () => {
    saveCredential(CODE, { registrationId: "9", secret: "s", name: "Alice" });
    const bag = createPlayerBag(CODE, {} as unknown as PublicBagClient);
    bag.dismissToken(true);

    bag.forget();
    expect(loadCredential(CODE)).toBeNull();
    expect(isDismissed(CODE)).toBe(false);
    expect(bag.state.hasCredential).toBe(false);
    expect(bag.state.selfId).toBe("0");
  });
});

describe("createDeviceBag", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("watches the shared code without a secret", async () => {
    const stream = channel();
    const watchTokenBag = vi.fn((_req: unknown, opts: StreamOptions) =>
      stream.open(opts.signal),
    );
    const bag = createDeviceBag(CODE, {
      watchTokenBag,
    } as unknown as PublicBagClient);

    bag.start();
    await flush();
    expect(watchTokenBag.mock.calls[0][0]).toEqual({ code: CODE });

    stream.push(snapshot({ players: [{ registrationId: 2n, name: "Bob" }] }));
    await flush();
    expect(bag.state.players.map((p) => p.id)).toEqual(["2"]);
    expect(bag.state.gameName).toBe("Trouble Brewing");
    expect(bag.state.status).toBe("live");
    bag.stop();
  });

  it("addName returns the new registration id as a string", async () => {
    const joinTokenBagShared = vi.fn(async () => ({ registrationId: 12n }));
    const bag = createDeviceBag(CODE, {
      joinTokenBagShared,
    } as unknown as PublicBagClient);

    expect(await bag.addName("Bob")).toBe("12");
    expect(joinTokenBagShared).toHaveBeenCalledWith({
      sharedCode: CODE,
      name: "Bob",
    });
    expect(bag.state.error).toBeNull();
  });

  it("addName reports a rejection", async () => {
    const bag = createDeviceBag(CODE, {
      joinTokenBagShared: vi.fn(async () => {
        throw new ConnectError("name already taken", Code.AlreadyExists);
      }),
    } as unknown as PublicBagClient);

    expect(await bag.addName("Bob")).toBeNull();
    expect(bag.state.error).toContain("name already taken");
  });

  it("revealFor hands the payload to the caller and keeps none of it", async () => {
    const payload = { name: "Bob", character: CHARACTER };
    const revealTokenShared = vi.fn(async () => payload);
    const bag = createDeviceBag(CODE, {
      revealTokenShared,
    } as unknown as PublicBagClient);

    const revealed = await bag.revealFor("12");

    expect(revealed).toBe(payload);
    expect(revealTokenShared).toHaveBeenCalledWith({
      sharedCode: CODE,
      registrationId: 12n,
    });
    // Nothing about the character may linger in reactive state for whoever
    // takes the device next.
    expect(JSON.stringify(bag.state)).not.toContain("washerwoman");
    expect(Object.values(bag.state)).not.toContain(payload);
  });

  it("revealFor reports a failure without throwing", async () => {
    const bag = createDeviceBag(CODE, {
      revealTokenShared: vi.fn(async () => {
        throw new ConnectError("not revealed yet", Code.FailedPrecondition);
      }),
    } as unknown as PublicBagClient);

    expect(await bag.revealFor("12")).toBeNull();
    expect(bag.state.error).toContain("not revealed yet");
  });
});

describe("createStorytellerBag", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  function ownerBag(over: MessageInitShape<typeof TokenBagSchema> = {}) {
    return create(TokenBagSchema, {
      phase: TokenBagPhase.OPEN,
      joinCode: "join-1",
      sharedCode: "shared-1",
      players: [{ registrationId: 1n, name: "Alice" }],
      ...over,
    });
  }

  function ownerClient() {
    return {
      getTokenBag: vi.fn(async () => ({
        tokenBag: ownerBag({ phase: TokenBagPhase.INACTIVE, players: [] }),
      })),
      openTokenBagRegistration: vi.fn(async () => ({ tokenBag: ownerBag() })),
      closeTokenBagRegistration: vi.fn(async () => ({
        tokenBag: ownerBag({ phase: TokenBagPhase.CLOSED }),
      })),
      addTokenBagRegistration: vi.fn(async () => ({
        tokenBag: ownerBag({
          players: [
            { registrationId: 1n, name: "Alice" },
            { registrationId: 2n, name: "Bob", viaSharedDevice: true },
          ],
        }),
      })),
      removeTokenBagRegistration: vi.fn(async () => ({
        tokenBag: ownerBag({ players: [] }),
      })),
      revealTokenBag: vi.fn(async () => ({
        tokenBag: ownerBag({ phase: TokenBagPhase.REVEALED }),
      })),
      resetTokenBag: vi.fn(async () => ({
        tokenBag: ownerBag({
          phase: TokenBagPhase.INACTIVE,
          joinCode: "",
          sharedCode: "",
          players: [],
        }),
      })),
      getTokenBagSeating: vi.fn(async () => ({
        orderedRegistrationIds: [1n, 2n],
        complete: true,
        conflicts: [] as string[],
      })),
    };
  }

  it("load reads the owner view including the codes", async () => {
    const owner = ownerClient();
    const bag = createStorytellerBag("42", {
      owner: owner as unknown as OwnerBagClient,
    });

    expect(await bag.load()).toBe(true);
    expect(owner.getTokenBag).toHaveBeenCalledWith({ gameId: 42n });
    expect(bag.state.phase).toBe(TokenBagPhase.INACTIVE);
    expect(bag.state.joinCode).toBe("join-1");
    expect(bag.state.sharedCode).toBe("shared-1");
  });

  it("accepts a bigint game id", async () => {
    const owner = ownerClient();
    await createStorytellerBag(42n, {
      owner: owner as unknown as OwnerBagClient,
    }).load();
    expect(owner.getTokenBag).toHaveBeenCalledWith({ gameId: 42n });
  });

  it("each action applies the returned bag", async () => {
    const owner = ownerClient();
    const bag = createStorytellerBag("42", {
      owner: owner as unknown as OwnerBagClient,
    });

    expect(await bag.open()).toBe(true);
    expect(bag.state.phase).toBe(TokenBagPhase.OPEN);
    expect(bag.state.players.map((p) => p.name)).toEqual(["Alice"]);
    expect(owner.openTokenBagRegistration).toHaveBeenCalledWith({
      gameId: 42n,
    });

    expect(await bag.close()).toBe(true);
    expect(bag.state.phase).toBe(TokenBagPhase.CLOSED);

    expect(await bag.reveal()).toBe(true);
    expect(bag.state.phase).toBe(TokenBagPhase.REVEALED);

    expect(await bag.addPlayer("Bob")).toBe(true);
    expect(owner.addTokenBagRegistration).toHaveBeenCalledWith({
      gameId: 42n,
      name: "Bob",
    });
    expect(bag.state.players.map((p) => p.name)).toEqual(["Alice", "Bob"]);

    expect(await bag.remove("1")).toBe(true);
    expect(owner.removeTokenBagRegistration).toHaveBeenCalledWith({
      gameId: 42n,
      registrationId: 1n,
    });
    expect(bag.state.players).toEqual([]);

    expect(await bag.reset()).toBe(true);
    expect(bag.state.phase).toBe(TokenBagPhase.INACTIVE);
    expect(bag.state.joinCode).toBe("");
  });

  it("an action failure lands in error and leaves state alone", async () => {
    const bag = createStorytellerBag("42", {
      owner: {
        ...ownerClient(),
        openTokenBagRegistration: vi.fn(async () => {
          throw new ConnectError("nope", Code.PermissionDenied);
        }),
      } as unknown as OwnerBagClient,
    });

    expect(await bag.open()).toBe(false);
    expect(bag.state.error).toContain("nope");
    expect(bag.state.phase).toBe(TokenBagPhase.UNSPECIFIED);
  });

  it("addPlayer reports a refused name without touching the players", async () => {
    const bag = createStorytellerBag("42", {
      owner: {
        ...ownerClient(),
        addTokenBagRegistration: vi.fn(async () => {
          throw new ConnectError("name already taken", Code.AlreadyExists);
        }),
      } as unknown as OwnerBagClient,
    });

    await bag.load();
    expect(await bag.addPlayer("Alice")).toBe(false);
    expect(bag.state.error).toContain("name already taken");
    expect(bag.state.players).toEqual([]);
  });

  it("seating hands the raw response to the panel", async () => {
    const bag = createStorytellerBag("42", {
      owner: ownerClient() as unknown as OwnerBagClient,
    });
    const resp = await bag.seating();
    expect(resp?.complete).toBe(true);
    expect(resp?.orderedRegistrationIds).toEqual([1n, 2n]);
  });

  it("the stream updates phase and players but never the codes", async () => {
    const stream = channel();
    const watchTokenBag = vi.fn((_req: unknown, opts: StreamOptions) =>
      stream.open(opts.signal),
    );
    const bag = createStorytellerBag("42", {
      owner: ownerClient() as unknown as OwnerBagClient,
      publicApi: { watchTokenBag } as unknown as PublicBagClient,
    });

    await bag.load();
    bag.start(bag.state.joinCode);
    await flush();
    expect(watchTokenBag.mock.calls[0][0]).toEqual({ code: "join-1" });

    stream.push(
      snapshot({
        phase: TokenBagPhase.OPEN,
        players: [
          { registrationId: 1n, name: "Alice" },
          { registrationId: 2n, name: "Bob", viaSharedDevice: true },
        ],
      }),
    );
    await flush();

    expect(bag.state.phase).toBe(TokenBagPhase.OPEN);
    expect(bag.state.players.map((p) => p.name)).toEqual(["Alice", "Bob"]);
    expect(bag.state.joinCode).toBe("join-1");
    expect(bag.state.sharedCode).toBe("shared-1");

    bag.stop();
    expect(bag.state.status).toBe("stopped");
  });

  it("reset stops the stream and reports no error", async () => {
    const stream = channel();
    const watchTokenBag = vi.fn((_req: unknown, opts: StreamOptions) =>
      stream.open(opts.signal),
    );
    const owner = ownerClient();
    const bag = createStorytellerBag("42", {
      owner: owner as unknown as OwnerBagClient,
      publicApi: { watchTokenBag } as unknown as PublicBagClient,
    });

    await bag.load();
    bag.start("join-1");
    await flush();
    expect(watchTokenBag).toHaveBeenCalledTimes(1);

    expect(await bag.reset()).toBe(true);
    await vi.advanceTimersByTimeAsync(60_000);

    // The code the loop was watching is gone, so the loop is too — no retry
    // into a NotFound, and no error banner for an action that succeeded.
    expect(bag.state.status).toBe("stopped");
    expect(bag.state.error).toBeNull();
    expect(bag.state.joinCode).toBe("");
    expect(watchTokenBag).toHaveBeenCalledTimes(1);
  });

  it("can re-arm the stream with a new join code after a fatal stop", async () => {
    const stream = channel();
    let attempt = 0;
    const watchTokenBag = vi.fn((_req: unknown, opts: StreamOptions) => {
      attempt++;
      if (attempt === 1) {
        return (async function* (): AsyncIterable<WatchTokenBagResponse> {
          throw new ConnectError("token bag not found", Code.NotFound);
        })();
      }
      return stream.open(opts.signal);
    });
    const bag = createStorytellerBag("42", {
      owner: ownerClient() as unknown as OwnerBagClient,
      publicApi: { watchTokenBag } as unknown as PublicBagClient,
    });

    await bag.load();
    bag.start("dead-code");
    await flush();
    expect(bag.state.status).toBe("stopped");

    // Registration re-opened behind a fresh code.
    await bag.open();
    bag.start("join-2");
    await flush();
    expect(watchTokenBag).toHaveBeenCalledTimes(2);
    expect(watchTokenBag.mock.calls[1][0]).toEqual({ code: "join-2" });
    bag.stop();
  });

  it("keeps quiet about a fatal stream error once the code is gone", async () => {
    const bag = createStorytellerBag("42", {
      owner: ownerClient() as unknown as OwnerBagClient,
      publicApi: {
        watchTokenBag: failingStream(
          new ConnectError("token bag not found", Code.NotFound),
        ),
      } as unknown as PublicBagClient,
    });

    // No load(), so state holds no join code — nothing to lose track of.
    bag.start("stale-code");
    await flush();
    expect(bag.state.status).toBe("stopped");
    expect(bag.state.error).toBeNull();
  });

  it("does not start a stream without a join code", () => {
    const watchTokenBag = vi.fn();
    const bag = createStorytellerBag("42", {
      owner: ownerClient() as unknown as OwnerBagClient,
      publicApi: { watchTokenBag } as unknown as PublicBagClient,
    });

    bag.start("");
    expect(watchTokenBag).not.toHaveBeenCalled();
  });
});
