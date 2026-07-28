// Reactive token bag state.
//
// Three factories, one per device role: a player's phone, a shared device passed
// around the table, and the Storyteller's panel. Factories rather than module
// singletons because they can coexist (the panel watches the same bag the
// Storyteller's own shared-device tab is using) and because each instance is tied
// to one code and must be torn down with its page.
//
// Everything here is thin: proto -> plain state, view decisions and name matching
// all live in `tokenbag.ts`. What this layer owns is the stream lifecycle, the
// stored credential and error messages.

import { Code, ConnectError } from "@connectrpc/connect";
import { client as ownerRpc, rawClient } from "./api";
import { getErrorMessage } from "./errors";
import {
  watchLoop,
  isFatalStreamError,
  type WatchLoopHandle,
  type WatchStatus,
} from "./stream-retry";
import {
  applyOwnerBag,
  applySnapshot,
  emptySnapshot,
  NO_ID,
  type BagPlayer,
} from "./tokenbag";
import {
  clearCredential,
  isDismissed,
  loadCredential,
  saveCredential,
  setDismissed as persistDismissed,
} from "./tokenbag-credentials";
import { TokenBagPhase } from "./gen/clockkeeper/v1/clockkeeper_pb";
import type {
  Character,
  GetTokenBagSeatingResponse,
  RevealTokenSharedResponse,
  TokenBag,
  WatchTokenBagResponse,
} from "./gen/clockkeeper/v1/clockkeeper_pb";

/**
 * The public, unauthenticated RPCs. Players have no session at all, so these
 * must go through `rawClient` — the auth interceptor would try to mint an
 * anonymous Storyteller session off a 401.
 */
export type PublicBagClient = Pick<
  typeof rawClient,
  | "watchTokenBag"
  | "joinTokenBag"
  | "setTokenBagNeighbors"
  | "getMyToken"
  | "joinTokenBagShared"
  | "revealTokenShared"
>;

/** The owner-only RPCs, which need the Storyteller's bearer token. */
export type OwnerBagClient = Pick<
  typeof ownerRpc,
  | "getTokenBag"
  | "openTokenBagRegistration"
  | "closeTokenBagRegistration"
  | "addTokenBagRegistration"
  | "removeTokenBagRegistration"
  | "revealTokenBag"
  | "resetTokenBag"
  | "getTokenBagSeating"
>;

/** Ids cross the wire as int64; `"0"`, `""` and junk all mean "unset". */
function toProtoId(value: string | bigint): bigint {
  if (typeof value === "bigint") return value;
  if (value === "") return 0n;
  try {
    return BigInt(value);
  } catch {
    return 0n;
  }
}

// ---------------------------------------------------------------------------
// Player device
// ---------------------------------------------------------------------------

export type PlayerBagState = {
  status: WatchStatus;
  phase: TokenBagPhase;
  gameName: string;
  players: BagPlayer[];
  /** `"0"` while this device is unregistered (or was removed). */
  selfId: string;
  selfToken: Character | null;
  /** The game has left setup: the bag is over and the token display is gone. */
  gameStarted: boolean;
  /** Mirrors localStorage so the view state reacts to registering. */
  hasCredential: boolean;
  /** Mirrors localStorage: the player hid their revealed token. */
  dismissed: boolean;
  /** The code is unknown to the server — deleted game or stale QR link. */
  gone: boolean;
  error: string | null;
};

/**
 * One player's phone at `/join/<code>`.
 *
 * `api` is injectable for tests; production always uses the real raw client.
 */
export function createPlayerBag(
  code: string,
  api: PublicBagClient = rawClient,
) {
  const state = $state<PlayerBagState>({
    status: "connecting",
    ...emptySnapshot(),
    hasCredential: loadCredential(code) !== null,
    dismissed: isDismissed(code),
    gone: false,
    error: null,
  });

  let loop: WatchLoopHandle | null = null;

  function apply(resp: WatchTokenBagResponse | undefined) {
    const snapshot = applySnapshot(resp);
    state.phase = snapshot.phase;
    state.gameName = snapshot.gameName;
    state.players = snapshot.players;
    state.selfId = snapshot.selfId;
    state.gameStarted = snapshot.gameStarted;
    // Every snapshot is complete, so the token follows it without exception: a
    // snapshot that carries none means this device has no token to show any more
    // — the reveal was reset, this player was removed from the bag, or the game
    // has started. Holding on to the last one would leave a character on a
    // screen the server has already stopped vouching for.
    state.selfToken = snapshot.selfToken;
    // The hidden flag belongs to the token that was hidden. Once the reveal is
    // withdrawn that token is history, so the NEXT one shows itself without
    // asking, exactly as the first reveal did.
    if (state.dismissed && snapshot.phase !== TokenBagPhase.REVEALED) {
      persistDismissed(code, false);
      state.dismissed = false;
    }
  }

  async function guard<T>(
    fallback: string,
    fn: () => Promise<T>,
  ): Promise<T | null> {
    try {
      state.error = null;
      return await fn();
    } catch (err) {
      state.error = getErrorMessage(err, fallback);
      return null;
    }
  }

  function start() {
    if (loop) return;
    // The loop can end on its own (fatal error), not just through stop(). The
    // handle has to be dropped either way, or `if (loop) return` above would
    // make every later start() a silent no-op. `terminated` covers the case of
    // a loop that ends before the assignment below.
    let terminated = false;
    // Read the credential per attempt, not once: register() restarts the loop
    // and the reconnect after it must carry the new secret.
    const handle = watchLoop<WatchTokenBagResponse>({
      open: (signal) =>
        api.watchTokenBag(
          { code, registrationSecret: loadCredential(code)?.secret ?? "" },
          { signal },
        ),
      onMessage: apply,
      onStatus: (status) => {
        state.status = status;
        if (status === "stopped") {
          terminated = true;
          loop = null;
        }
      },
      isFatal: (err) => {
        if (!isFatalStreamError(err)) return false;
        state.error = getErrorMessage(err, "This game is no longer available");
        // NotFound is about the code itself: the game behind it was deleted, so
        // this device can never reconnect.
        if (ConnectError.from(err).code === Code.NotFound) state.gone = true;
        return true;
      },
    });
    loop = terminated ? null : handle;
  }

  function stop() {
    loop?.stop();
    loop = null;
  }

  function restart() {
    stop();
    start();
  }

  /** Claims a name. Returns false and sets `error` when the server refuses. */
  async function register(name: string): Promise<boolean> {
    const ok = await guard("Could not join the game", async () => {
      const resp = await api.joinTokenBag({ joinCode: code, name });
      saveCredential(code, {
        registrationId: resp.registrationId,
        secret: resp.registrationSecret,
        name,
      });
      state.hasCredential = true;
      state.gone = false;
      // A fresh registration is not "already seen", even if this device
      // dismissed a token for the same code before.
      persistDismissed(code, false);
      state.dismissed = false;
      apply(resp.snapshot);
      return true;
    });
    // The stream that is open right now was started without a secret, so it
    // cannot report self_* fields. Reopen it with the credential.
    if (ok && loop) restart();
    return ok === true;
  }

  async function setNeighbors(
    leftId: string,
    rightId: string,
  ): Promise<boolean> {
    const cred = loadCredential(code);
    if (!cred) {
      state.error = "This device is not registered for this game";
      return false;
    }
    const ok = await guard("Could not save your neighbors", async () => {
      const resp = await api.setTokenBagNeighbors({
        registrationSecret: cred.secret,
        leftRegistrationId: toProtoId(leftId),
        rightRegistrationId: toProtoId(rightId),
      });
      apply(resp.snapshot);
      return true;
    });
    return ok === true;
  }

  /** Re-fetches the character, for showing it again after hiding it. */
  async function fetchMyToken(): Promise<Character | null> {
    const cred = loadCredential(code);
    if (!cred) {
      state.error = "This device is not registered for this game";
      return null;
    }
    const character = await guard("Could not load your character", async () => {
      const resp = await api.getMyToken({ registrationSecret: cred.secret });
      return resp.character ?? null;
    });
    if (character) state.selfToken = character;
    return character ?? null;
  }

  /** Hides / re-shows the revealed token on this device only. */
  function dismissToken(dismissed: boolean) {
    persistDismissed(code, dismissed);
    state.dismissed = dismissed;
  }

  /** Forgets this device's registration (leaves the server-side one alone). */
  function forget() {
    clearCredential(code);
    state.hasCredential = false;
    state.dismissed = false;
    state.selfId = NO_ID;
    state.selfToken = null;
    // The open stream was dialed with the old secret; without a redial the
    // next snapshot would repopulate selfId and undo the forget.
    if (loop) restart();
  }

  return {
    state,
    start,
    stop,
    register,
    setNeighbors,
    fetchMyToken,
    dismissToken,
    forget,
  };
}

export type PlayerBag = ReturnType<typeof createPlayerBag>;

// ---------------------------------------------------------------------------
// Shared device
// ---------------------------------------------------------------------------

export type DeviceBagState = {
  status: WatchStatus;
  phase: TokenBagPhase;
  gameName: string;
  players: BagPlayer[];
  /** The game has left setup: the server refuses every further reveal. */
  gameStarted: boolean;
  error: string | null;
};

/**
 * The tablet passed around the table at `/device/<code>`. It holds no
 * credential: the shared code itself is the authority, and every reveal is a
 * one-shot unary call.
 */
export function createDeviceBag(
  code: string,
  api: PublicBagClient = rawClient,
) {
  const state = $state<DeviceBagState>({
    status: "connecting",
    phase: TokenBagPhase.UNSPECIFIED,
    gameName: "",
    players: [],
    gameStarted: false,
    error: null,
  });

  let loop: WatchLoopHandle | null = null;

  function start() {
    if (loop) return;
    // See createPlayerBag.start(): a self-terminating loop must release the
    // handle so a later start() can dial again.
    let terminated = false;
    const handle = watchLoop<WatchTokenBagResponse>({
      // No secret: a shared device has no registration of its own.
      open: (signal) => api.watchTokenBag({ code }, { signal }),
      onMessage: (resp) => {
        const snapshot = applySnapshot(resp);
        state.phase = snapshot.phase;
        state.gameName = snapshot.gameName;
        state.players = snapshot.players;
        state.gameStarted = snapshot.gameStarted;
      },
      onStatus: (status) => {
        state.status = status;
        if (status === "stopped") {
          terminated = true;
          loop = null;
        }
      },
      isFatal: (err) => {
        if (!isFatalStreamError(err)) return false;
        state.error = getErrorMessage(err, "This game is no longer available");
        return true;
      },
    });
    loop = terminated ? null : handle;
  }

  function stop() {
    loop?.stop();
    loop = null;
  }

  /** Registers a player who has no phone. Returns their id, or null on error. */
  async function addName(name: string): Promise<string | null> {
    try {
      state.error = null;
      const resp = await api.joinTokenBagShared({ sharedCode: code, name });
      return String(resp.registrationId);
    } catch (err) {
      state.error = getErrorMessage(err, "Could not add that name");
      return null;
    }
  }

  /**
   * Shows one player their character. The payload is returned, never stored:
   * a character left in reactive state would still be there when the next
   * player takes the device.
   */
  async function revealFor(
    registrationId: string,
  ): Promise<RevealTokenSharedResponse | null> {
    try {
      state.error = null;
      return await api.revealTokenShared({
        sharedCode: code,
        registrationId: toProtoId(registrationId),
      });
    } catch (err) {
      state.error = getErrorMessage(err, "Could not reveal that character");
      return null;
    }
  }

  return { state, start, stop, addName, revealFor };
}

export type DeviceBag = ReturnType<typeof createDeviceBag>;

// ---------------------------------------------------------------------------
// Storyteller panel
// ---------------------------------------------------------------------------

export type StorytellerBagState = {
  status: WatchStatus;
  phase: TokenBagPhase;
  /** Owner-only, from the unary calls — the stream never carries the codes. */
  joinCode: string;
  sharedCode: string;
  players: BagPlayer[];
  error: string | null;
};

/**
 * The Storyteller's control panel for one game. Actions go through the authed
 * client; the live registrant list comes from the same public stream the players
 * use (with the join code and no secret).
 */
export function createStorytellerBag(
  gameId: bigint | string,
  deps: { owner?: OwnerBagClient; publicApi?: PublicBagClient } = {},
) {
  const owner = deps.owner ?? ownerRpc;
  const api = deps.publicApi ?? rawClient;
  const id = toProtoId(gameId);

  const state = $state<StorytellerBagState>({
    status: "stopped",
    phase: TokenBagPhase.UNSPECIFIED,
    joinCode: "",
    sharedCode: "",
    players: [],
    error: null,
  });

  let loop: WatchLoopHandle | null = null;

  function applyBag(bag: TokenBag | undefined) {
    const next = applyOwnerBag(bag);
    state.phase = next.phase;
    state.joinCode = next.joinCode;
    state.sharedCode = next.sharedCode;
    state.players = next.players;
  }

  async function guard<T>(
    fallback: string,
    fn: () => Promise<T>,
  ): Promise<T | null> {
    try {
      state.error = null;
      return await fn();
    } catch (err) {
      state.error = getErrorMessage(err, fallback);
      return null;
    }
  }

  /** Initial (and only) read of the owner view, including the codes. */
  async function load(): Promise<boolean> {
    const ok = await guard("Could not load the token bag", async () => {
      const resp = await owner.getTokenBag({ gameId: id });
      applyBag(resp.tokenBag);
      return true;
    });
    return ok === true;
  }

  function start(joinCode: string) {
    if (loop || !joinCode) return;
    // See createPlayerBag.start(): the handle must be released when the loop
    // ends by itself, or the "Reconnect" affordance would be a silent no-op.
    let terminated = false;
    const handle = watchLoop<WatchTokenBagResponse>({
      open: (signal) => api.watchTokenBag({ code: joinCode }, { signal }),
      onMessage: (resp) => {
        const snapshot = applySnapshot(resp);
        // The stream is the public view: it has no codes to apply, and the
        // ones already loaded stay valid.
        state.phase = snapshot.phase;
        state.players = snapshot.players;
      },
      onStatus: (status) => {
        state.status = status;
        if (status === "stopped") {
          terminated = true;
          loop = null;
        }
      },
      isFatal: (err) => {
        if (!isFatalStreamError(err)) return false;
        state.error = getErrorMessage(err, "Lost track of the token bag");
        return true;
      },
    });
    loop = terminated ? null : handle;
  }

  function stop() {
    loop?.stop();
    loop = null;
  }

  /** Every action returns the full bag; the panel's state follows it. */
  function action(
    fallback: string,
    call: () => Promise<{ tokenBag?: TokenBag }>,
  ) {
    return async (): Promise<boolean> => {
      const ok = await guard(fallback, async () => {
        applyBag((await call()).tokenBag);
        return true;
      });
      return ok === true;
    };
  }

  const openRegistration = action("Could not open registration", () =>
    owner.openTokenBagRegistration({ gameId: id }),
  );
  const closeRegistration = action("Could not close registration", () =>
    owner.closeTokenBagRegistration({ gameId: id }),
  );
  const reveal = action("Could not reveal the characters", () =>
    owner.revealTokenBag({ gameId: id }),
  );
  const reset = action("Could not reset the reveal", () =>
    owner.resetTokenBag({ gameId: id }),
  );

  /** Puts a player in the bag for someone who has no device of their own. */
  async function addPlayer(name: string): Promise<boolean> {
    const ok = await guard("Could not add that player", async () => {
      const resp = await owner.addTokenBagRegistration({ gameId: id, name });
      applyBag(resp.tokenBag);
      return true;
    });
    return ok === true;
  }

  async function remove(registrationId: string | bigint): Promise<boolean> {
    const ok = await guard("Could not remove that player", async () => {
      const resp = await owner.removeTokenBagRegistration({
        gameId: id,
        registrationId: toProtoId(registrationId),
      });
      applyBag(resp.tokenBag);
      return true;
    });
    return ok === true;
  }

  /** Seating is advisory (it can be incomplete), so the panel interprets it. */
  async function seating(): Promise<GetTokenBagSeatingResponse | null> {
    return await guard("Could not work out the seating", () =>
      owner.getTokenBagSeating({ gameId: id }),
    );
  }

  return {
    state,
    load,
    start,
    stop,
    open: openRegistration,
    close: closeRegistration,
    addPlayer,
    remove,
    reveal,
    reset,
    seating,
  };
}

export type StorytellerBag = ReturnType<typeof createStorytellerBag>;
