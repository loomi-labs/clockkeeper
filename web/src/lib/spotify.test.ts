import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { ConnectError, Code } from "@connectrpc/connect";

const rpc = vi.hoisted(() => ({
  getSpotifyAccessToken: vi.fn(),
  getSpotifyStatus: vi.fn(),
  disconnectSpotify: vi.fn(),
  updateSpotifyPlaylists: vi.fn(),
  connectSpotify: vi.fn(),
}));

vi.mock("./api", () => ({ client: rpc, rawClient: rpc }));

import {
  spotify,
  applySpotifyStatus,
  resetSpotifyState,
  getAccessToken,
  spotifyFetch,
  switchToSlot,
  syncPhase,
  stopMusic,
  setShufflePref,
  setVolumeDebounced,
  listPlaylists,
  startPlaybackPolling,
  stopPlaybackPolling,
  getSpotifyOAuthURL,
  validateSpotifyOAuthState,
  takeSpotifyReturnTo,
  SPOTIFY_SCOPES,
  type SpotifyError,
} from "./spotify.svelte";

const NOW_MS = 1_700_000_000_000;
const NOW_SEC = 1_700_000_000;
const HOUR = 3600;

function tokenResponse(accessToken: string, ttlSec = HOUR) {
  return { accessToken, expiresAtUnix: BigInt(NOW_SEC + ttlSec) };
}

/** Minimal Response stand-in — spotifyFetch only uses status/headers/text. */
function makeResponse(
  status: number,
  body?: unknown,
  headers: Record<string, string> = {},
): Response {
  return {
    status,
    ok: status >= 200 && status < 300,
    headers: {
      get: (name: string) => headers[name] ?? null,
    },
    text: async () => (body === undefined ? "" : JSON.stringify(body)),
  } as unknown as Response;
}

let fetchMock: ReturnType<typeof vi.fn>;

type FetchCall = [string, RequestInit];

function fetchArgs(index: number): FetchCall {
  return fetchMock.mock.calls[index] as unknown as FetchCall;
}

/** Every fetch whose URL contains `fragment`, in call order. */
function callsMatching(fragment: string): FetchCall[] {
  return (fetchMock.mock.calls as unknown as FetchCall[]).filter(([url]) =>
    url.includes(fragment),
  );
}

function playCalls(): FetchCall[] {
  return callsMatching("/me/player/play");
}

function bodyOf(call: FetchCall): Record<string, unknown> {
  return JSON.parse(String(call[1].body)) as Record<string, unknown>;
}

function offsetURI(call: FetchCall): string | undefined {
  return (bodyOf(call).offset as { uri?: string } | undefined)?.uri;
}

type Route = {
  match: string;
  resp: Response | (() => Response | Promise<Response>);
};

/**
 * Routes the fetch stub by URL substring, first match wins — anything
 * unmatched falls through to a bare 204. Order the specific paths first
 * (`/me/player/play` before `/me/player`).
 */
function routeFetch(routes: Route[]) {
  fetchMock.mockImplementation((url: string) => {
    const route = routes.find((r) => url.includes(r.match));
    if (!route) return Promise.resolve(makeResponse(204));
    return Promise.resolve(
      typeof route.resp === "function" ? route.resp() : route.resp,
    );
  });
}

function tracksResponse(uris: string[], total = uris.length): Response {
  return makeResponse(200, {
    items: uris.map((uri) => ({ track: { uri } })),
    total,
  });
}

/** Flushes fire-and-forget follow-ups (the shuffle PUT after a switch). */
async function flush() {
  await vi.advanceTimersByTimeAsync(0);
}

function playbackResponse(contextUri: string, trackUri: string): Response {
  return makeResponse(200, {
    is_playing: true,
    context: { uri: contextUri },
    item: { uri: trackUri },
  });
}

function authHeader(index: number): string | undefined {
  const init = fetchArgs(index)[1] as { headers?: Record<string, string> };
  return init.headers?.["Authorization"];
}

async function captureError(fn: () => Promise<unknown>): Promise<SpotifyError> {
  try {
    await fn();
  } catch (err) {
    return err as SpotifyError;
  }
  throw new Error("expected the call to reject");
}

const DAY_PLAYLIST = {
  uri: "spotify:playlist:day",
  name: "Daytime",
  imageUrl: "https://img/day.jpg",
};
const NIGHT_PLAYLIST = {
  uri: "spotify:playlist:night",
  name: "Nighttime",
  imageUrl: "",
};

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(NOW_MS);
  vi.clearAllMocks();
  sessionStorage.clear();
  // resetSpotifyState re-reads the persisted shuffle pref, so clear first.
  localStorage.clear();
  resetSpotifyState();

  rpc.getSpotifyAccessToken.mockResolvedValue(tokenResponse("tok-1"));
  fetchMock = vi.fn().mockResolvedValue(makeResponse(204));
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  resetSpotifyState();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("getAccessToken", () => {
  it("caches the token until 60s before expiry, then refetches", async () => {
    expect(await getAccessToken()).toBe("tok-1");
    expect(await getAccessToken()).toBe("tok-1");
    expect(rpc.getSpotifyAccessToken).toHaveBeenCalledTimes(1);

    // 61s before expiry — still cached.
    vi.setSystemTime(NOW_MS + (HOUR - 61) * 1000);
    expect(await getAccessToken()).toBe("tok-1");
    expect(rpc.getSpotifyAccessToken).toHaveBeenCalledTimes(1);

    // 59s before expiry — inside the skew window, refetch.
    rpc.getSpotifyAccessToken.mockResolvedValue(
      tokenResponse("tok-2", 2 * HOUR),
    );
    vi.setSystemTime(NOW_MS + (HOUR - 59) * 1000);
    expect(await getAccessToken()).toBe("tok-2");
    expect(rpc.getSpotifyAccessToken).toHaveBeenCalledTimes(2);
  });

  it("force bypasses the cache and asks the server to refresh", async () => {
    expect(await getAccessToken()).toBe("tok-1");
    expect(rpc.getSpotifyAccessToken).toHaveBeenCalledWith({
      forceRefresh: false,
    });

    rpc.getSpotifyAccessToken.mockResolvedValue(tokenResponse("tok-forced"));
    expect(await getAccessToken(true)).toBe("tok-forced");
    expect(rpc.getSpotifyAccessToken).toHaveBeenCalledTimes(2);
    expect(rpc.getSpotifyAccessToken).toHaveBeenLastCalledWith({
      forceRefresh: true,
    });
  });

  it("shares a single in-flight RPC between concurrent non-forced callers", async () => {
    let release: (value: unknown) => void = () => {};
    rpc.getSpotifyAccessToken.mockReturnValue(
      new Promise((resolve) => {
        release = resolve;
      }),
    );

    const pending = Promise.all([getAccessToken(), getAccessToken()]);
    release(tokenResponse("tok-shared"));

    expect(await pending).toEqual(["tok-shared", "tok-shared"]);
    expect(rpc.getSpotifyAccessToken).toHaveBeenCalledTimes(1);
  });

  it("does not let a forced call join an older in-flight request", async () => {
    // The in-flight request may predate the 401 that triggered the force, so
    // it proves nothing about the grant still being valid.
    let releaseStale: (value: unknown) => void = () => {};
    rpc.getSpotifyAccessToken.mockReturnValueOnce(
      new Promise((resolve) => {
        releaseStale = resolve;
      }),
    );

    const stale = getAccessToken();
    const forced = getAccessToken(true);
    // A follower that arrives after the forced call joins the fresher promise.
    const follower = getAccessToken();

    releaseStale(tokenResponse("tok-stale"));

    expect(rpc.getSpotifyAccessToken).toHaveBeenCalledTimes(2);
    expect(rpc.getSpotifyAccessToken).toHaveBeenNthCalledWith(1, {
      forceRefresh: false,
    });
    expect(rpc.getSpotifyAccessToken).toHaveBeenNthCalledWith(2, {
      forceRefresh: true,
    });
    expect(await stale).toBe("tok-stale");
    expect(await forced).toBe("tok-1");
    expect(await follower).toBe("tok-1");
  });

  it("maps FailedPrecondition to not_connected and ends the session", async () => {
    spotify.connected = true;
    spotify.sessionActive = true;
    spotify.currentSlot = "night";
    spotify.isPlaying = true;
    rpc.getSpotifyAccessToken.mockRejectedValue(
      new ConnectError("spotify not linked", Code.FailedPrecondition),
    );

    const err = await captureError(() => getAccessToken());
    expect(err.kind).toBe("not_connected");
    expect(spotify.connected).toBe(false);
    expect(spotify.sessionActive).toBe(false);
    expect(spotify.currentSlot).toBeNull();
    expect(spotify.isPlaying).toBe(false);
  });
});

describe("spotifyFetch", () => {
  it("refreshes the token once and retries on 401", async () => {
    rpc.getSpotifyAccessToken
      .mockResolvedValueOnce(tokenResponse("stale"))
      .mockResolvedValueOnce(tokenResponse("fresh"));
    fetchMock
      .mockResolvedValueOnce(makeResponse(401))
      .mockResolvedValueOnce(makeResponse(200, { ok: true }));

    const body = await spotifyFetch<{ ok: boolean }>("/me/player/devices");

    expect(body).toEqual({ ok: true });
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(rpc.getSpotifyAccessToken).toHaveBeenCalledTimes(2);
    expect(authHeader(0)).toBe("Bearer stale");
    expect(authHeader(1)).toBe("Bearer fresh");
    // The retry must ask the server to re-mint, so a revoked grant surfaces
    // instead of the server handing back its still-unexpired cached token.
    expect(rpc.getSpotifyAccessToken).toHaveBeenNthCalledWith(1, {
      forceRefresh: false,
    });
    expect(rpc.getSpotifyAccessToken).toHaveBeenNthCalledWith(2, {
      forceRefresh: true,
    });
  });

  it("treats a second 401 as not_connected and ends the session", async () => {
    spotify.connected = true;
    spotify.sessionActive = true;
    spotify.currentSlot = "day";
    spotify.isPlaying = true;
    fetchMock.mockResolvedValue(makeResponse(401));

    const err = await captureError(() => spotifyFetch("/me/player/devices"));

    expect(err.kind).toBe("not_connected");
    expect(spotify.connected).toBe(false);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    // A dead grant cannot pause, resume or switch anything — the session is
    // over, so nothing may stay armed on it.
    expect(spotify.sessionActive).toBe(false);
    expect(spotify.currentSlot).toBeNull();
    expect(spotify.isPlaying).toBe(false);
  });

  it("maps 404 on player paths to no_device and flags the device picker", async () => {
    fetchMock.mockResolvedValue(makeResponse(404));

    const err = await captureError(() => spotifyFetch("/me/player/play"));

    expect(err.kind).toBe("no_device");
    expect(spotify.needsDevicePick).toBe(true);
  });

  it("maps 403 PREMIUM_REQUIRED to premium_required", async () => {
    fetchMock.mockResolvedValue(
      makeResponse(403, {
        error: {
          status: 403,
          reason: "PREMIUM_REQUIRED",
          message: "Player command failed: Premium required",
        },
      }),
    );

    const err = await captureError(() => spotifyFetch("/me/player/play"));
    expect(err.kind).toBe("premium_required");
  });

  it("maps 429 to rate_limited with Retry-After seconds", async () => {
    fetchMock.mockResolvedValue(
      makeResponse(429, undefined, { "Retry-After": "7" }),
    );

    const err = await captureError(() => spotifyFetch("/me/player/play"));
    expect(err.kind).toBe("rate_limited");
    expect(err.retryAfterSec).toBe(7);
  });

  it("treats 204 as success with an empty body", async () => {
    fetchMock.mockResolvedValue(makeResponse(204));
    await expect(spotifyFetch("/me/player/pause")).resolves.toBeNull();
  });
});

describe("switchToSlot", () => {
  it("plays the configured playlist on the active device", async () => {
    spotify.playlists.day = DAY_PLAYLIST;
    spotify.activeDeviceId = "device-42";

    await switchToSlot("day");

    const plays = playCalls();
    expect(plays).toHaveLength(1);
    expect(plays[0][0]).toBe(
      "https://api.spotify.com/v1/me/player/play?device_id=device-42",
    );
    expect(plays[0][1].method).toBe("PUT");
    expect(bodyOf(plays[0])).toEqual({ context_uri: "spotify:playlist:day" });
    expect(spotify.currentSlot).toBe("day");
    expect(spotify.sessionActive).toBe(true);
    expect(spotify.isPlaying).toBe(true);
    expect(spotify.error).toBeNull();
  });

  it("omits device_id when no device is active", async () => {
    spotify.playlists.night = NIGHT_PLAYLIST;
    await switchToSlot("night");
    expect(playCalls()[0][0]).toBe("https://api.spotify.com/v1/me/player/play");
  });

  it("no-ops with slot_unconfigured when the slot is empty", async () => {
    await switchToSlot("nominations");

    expect(fetchMock).not.toHaveBeenCalled();
    expect(spotify.error).toEqual({
      kind: "slot_unconfigured",
      slot: "nominations",
    });
    expect(spotify.sessionActive).toBe(false);
  });

  it("stores the error instead of throwing when playback fails", async () => {
    spotify.playlists.day = DAY_PLAYLIST;
    fetchMock.mockResolvedValue(makeResponse(404));

    await switchToSlot("day");

    expect(spotify.error?.kind).toBe("no_device");
    expect(spotify.needsDevicePick).toBe(true);
    expect(spotify.sessionActive).toBe(false);
  });
});

describe("syncPhase", () => {
  beforeEach(() => {
    spotify.playlists.day = DAY_PLAYLIST;
    spotify.playlists.night = NIGHT_PLAYLIST;
  });

  it("does nothing when the session has not started", async () => {
    spotify.connected = true;
    spotify.sessionActive = false;

    await syncPhase(true);

    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("does nothing when Spotify is not connected", async () => {
    spotify.connected = false;
    spotify.sessionActive = true;

    await syncPhase(false);

    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("switches to the day and night slots", async () => {
    spotify.connected = true;
    spotify.sessionActive = true;

    await syncPhase(true);
    expect(spotify.currentSlot).toBe("day");
    expect(bodyOf(playCalls()[0])).toEqual({
      context_uri: "spotify:playlist:day",
    });

    await syncPhase(false);
    expect(spotify.currentSlot).toBe("night");
    expect(bodyOf(playCalls()[1])).toEqual({
      context_uri: "spotify:playlist:night",
    });
  });

  it("overrides a manual nominations selection on a phase flip", async () => {
    spotify.connected = true;
    spotify.sessionActive = true;
    spotify.currentSlot = "nominations";

    await syncPhase(true);

    expect(spotify.currentSlot).toBe("day");
    expect(playCalls()).toHaveLength(1);
  });
});

describe("stopMusic", () => {
  it("pauses and ends the session without disconnecting", async () => {
    spotify.playlists.day = DAY_PLAYLIST;
    spotify.playlists.night = NIGHT_PLAYLIST;
    spotify.activeDeviceId = "device-42";
    spotify.connected = true;

    await switchToSlot("day");
    await flush();
    expect(spotify.sessionActive).toBe(true);
    expect(rpc.getSpotifyAccessToken).toHaveBeenCalledTimes(1);

    // A leftover strip from an earlier action must not outlive the session it
    // was complaining about.
    spotify.error = { kind: "slot_unconfigured", slot: "nominations" };

    await stopMusic();

    const pauses = callsMatching("/me/player/pause");
    expect(pauses).toHaveLength(1);
    expect(pauses[0][1].method).toBe("PUT");
    expect(spotify.sessionActive).toBe(false);
    expect(spotify.currentSlot).toBeNull();
    expect(spotify.isPlaying).toBe(false);
    expect(spotify.error).toBeNull();

    // Stop is not disconnect — everything the restart needs survives.
    expect(spotify.connected).toBe(true);
    expect(spotify.playlists.day).toEqual(DAY_PLAYLIST);
    expect(spotify.playlists.night).toEqual(NIGHT_PLAYLIST);
    expect(spotify.shuffle).toBe(true);
    expect(spotify.activeDeviceId).toBe("device-42");
    // The token cache is intact: the pause reused the switch's token.
    expect(rpc.getSpotifyAccessToken).toHaveBeenCalledTimes(1);
  });

  it("ends the session even when the pause fails, without an error strip", async () => {
    spotify.sessionActive = true;
    spotify.currentSlot = "night";
    spotify.isPlaying = true;
    fetchMock.mockResolvedValue(
      makeResponse(500, { error: { message: "server on fire" } }),
    );

    await stopMusic();

    expect(spotify.sessionActive).toBe(false);
    expect(spotify.currentSlot).toBeNull();
    expect(spotify.isPlaying).toBe(false);
    expect(spotify.error).toBeNull();
  });

  it("does not raise the device picker when the pause finds no device", async () => {
    spotify.sessionActive = true;
    spotify.currentSlot = "day";
    fetchMock.mockResolvedValue(makeResponse(404));

    await stopMusic();

    expect(spotify.needsDevicePick).toBe(false);
    expect(spotify.sessionActive).toBe(false);
    expect(spotify.error).toBeNull();
  });

  it("surfaces a lost connection", async () => {
    spotify.connected = true;
    spotify.sessionActive = true;
    spotify.currentSlot = "day";
    // Two 401s in a row is how spotifyFetch decides the grant is gone.
    fetchMock.mockResolvedValue(makeResponse(401));

    await stopMusic();

    expect(spotify.error?.kind).toBe("not_connected");
    expect(spotify.connected).toBe(false);
    expect(spotify.sessionActive).toBe(false);
    expect(spotify.currentSlot).toBeNull();
    expect(spotify.isPlaying).toBe(false);
  });

  it("keeps an in-flight switch from resurrecting the session", async () => {
    spotify.playlists.day = DAY_PLAYLIST;
    let releasePlay: (() => void) | undefined;
    routeFetch([
      {
        match: "/me/player/play",
        resp: () =>
          new Promise<Response>((resolve) => {
            releasePlay = () => resolve(makeResponse(204));
          }),
      },
    ]);

    const switching = switchToSlot("day");
    await flush();
    expect(releasePlay).toBeDefined();

    await stopMusic();
    releasePlay?.();
    await switching;
    await flush();

    expect(spotify.sessionActive).toBe(false);
    expect(spotify.currentSlot).toBeNull();
    expect(spotify.isPlaying).toBe(false);
  });
});

describe("shuffle preference", () => {
  it("defaults to on", () => {
    expect(spotify.shuffle).toBe(true);
  });

  it("persists the preference and pushes it to a live session", async () => {
    spotify.sessionActive = true;

    setShufflePref(false);

    expect(spotify.shuffle).toBe(false);
    expect(localStorage.getItem("clockkeeper_spotify_shuffle")).toBe("false");

    await vi.advanceTimersByTimeAsync(0);

    const calls = callsMatching("/me/player/shuffle");
    expect(calls).toHaveLength(1);
    expect(calls[0][0]).toBe(
      "https://api.spotify.com/v1/me/player/shuffle?state=false",
    );
    expect(calls[0][1].method).toBe("PUT");
  });

  it("only persists when no session is running", async () => {
    setShufflePref(false);
    await vi.advanceTimersByTimeAsync(0);

    expect(localStorage.getItem("clockkeeper_spotify_shuffle")).toBe("false");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("is applied to the device after a successful slot switch", async () => {
    spotify.playlists.day = DAY_PLAYLIST;
    spotify.activeDeviceId = "device-42";

    await switchToSlot("day");
    await flush();

    const calls = callsMatching("/me/player/shuffle");
    expect(calls).toHaveLength(1);
    expect(calls[0][0]).toBe(
      "https://api.spotify.com/v1/me/player/shuffle?state=true&device_id=device-42",
    );
  });

  it("does not fail the switch when the device refuses shuffle", async () => {
    spotify.playlists.day = DAY_PLAYLIST;
    routeFetch([
      {
        match: "/me/player/shuffle",
        resp: makeResponse(500, { error: { message: "no shuffle here" } }),
      },
    ]);

    await switchToSlot("day");
    await flush();

    expect(spotify.currentSlot).toBe("day");
    expect(spotify.sessionActive).toBe(true);
    expect(spotify.isPlaying).toBe(true);
    expect(spotify.error).toBeNull();
  });
});

describe("played-aware track selection", () => {
  const TRACKS = ["spotify:track:a", "spotify:track:b", "spotify:track:c"];

  beforeEach(() => {
    spotify.playlists.day = DAY_PLAYLIST;
    spotify.playlists.night = NIGHT_PLAYLIST;
  });

  it("starts on a playlist track and caches the track list", async () => {
    routeFetch([
      { match: "/playlists/day/tracks", resp: tracksResponse(TRACKS) },
    ]);

    await switchToSlot("day");
    await switchToSlot("day");

    const trackFetches = callsMatching("/playlists/day/tracks");
    expect(trackFetches).toHaveLength(1);
    expect(trackFetches[0][0]).toContain("limit=100");
    expect(trackFetches[0][0]).toContain("offset=0");

    const plays = playCalls();
    expect(plays).toHaveLength(2);
    for (const play of plays) {
      expect(bodyOf(play).context_uri).toBe("spotify:playlist:day");
      expect(TRACKS).toContain(offsetURI(play));
    }
    // A slot never repeats a track until the playlist is exhausted.
    expect(offsetURI(plays[0])).not.toBe(offsetURI(plays[1]));
  });

  it("never restarts on a track the poll saw playing in that slot", async () => {
    spotify.currentSlot = "day";
    routeFetch([
      { match: "/playlists/day/tracks", resp: tracksResponse(TRACKS) },
      { match: "/me/player/play", resp: makeResponse(204) },
      { match: "/me/player/shuffle", resp: makeResponse(204) },
      {
        match: "/me/player",
        resp: playbackResponse(DAY_PLAYLIST.uri, "spotify:track:a"),
      },
    ]);

    startPlaybackPolling();
    await vi.advanceTimersByTimeAsync(0);
    stopPlaybackPolling();

    await switchToSlot("day");
    await switchToSlot("day");

    const picked = playCalls().map(offsetURI);
    expect(picked).toHaveLength(2);
    expect(picked).not.toContain("spotify:track:a");
    expect(new Set(picked)).toEqual(
      new Set(["spotify:track:b", "spotify:track:c"]),
    );
  });

  it("remembers what the outgoing slot was playing", async () => {
    spotify.currentSlot = "day";
    spotify.sessionActive = true;
    routeFetch([
      { match: "/playlists/day/tracks", resp: tracksResponse(TRACKS) },
      { match: "/playlists/night/tracks", resp: tracksResponse(["n1"]) },
      { match: "/me/player/play", resp: makeResponse(204) },
      { match: "/me/player/shuffle", resp: makeResponse(204) },
      {
        match: "/me/player",
        resp: playbackResponse(DAY_PLAYLIST.uri, "spotify:track:a"),
      },
    ]);

    await switchToSlot("night");
    await switchToSlot("day");

    // Two snapshots — one per switch. Nothing polled in between, so the
    // snapshot before leaving Day is the only thing that recorded track a.
    const snapshots = (fetchMock.mock.calls as unknown as FetchCall[]).filter(
      ([url]) => url === "https://api.spotify.com/v1/me/player",
    );
    expect(snapshots).toHaveLength(2);
    const picked = playCalls().map(offsetURI);
    expect(picked[0]).toBe("n1");
    expect(picked[1]).not.toBe("spotify:track:a");
  });

  it("keeps the switch alive when the snapshot fails", async () => {
    spotify.currentSlot = "night";
    routeFetch([
      { match: "/playlists/day/tracks", resp: tracksResponse(TRACKS) },
      { match: "/me/player/play", resp: makeResponse(204) },
      { match: "/me/player/shuffle", resp: makeResponse(204) },
      { match: "/me/player", resp: makeResponse(404) },
    ]);

    await switchToSlot("day");

    expect(spotify.currentSlot).toBe("day");
    expect(spotify.error).toBeNull();
    expect(TRACKS).toContain(offsetURI(playCalls()[0]));
  });

  it("does not open the device picker for a failed snapshot", async () => {
    spotify.currentSlot = "night";
    routeFetch([
      { match: "/playlists/day/tracks", resp: tracksResponse(TRACKS) },
      // The play fails for an unrelated reason, so nothing after the snapshot
      // can clear a device-picker flag the snapshot should never have set.
      {
        match: "/me/player/play",
        resp: makeResponse(500, { error: { message: "server on fire" } }),
      },
      { match: "/me/player", resp: makeResponse(404) },
    ]);

    await switchToSlot("day");

    expect(spotify.error?.kind).toBe("api");
    expect(spotify.needsDevicePick).toBe(false);
  });

  it("does not let a stale snapshot undo a newer switch's device picker", async () => {
    spotify.currentSlot = "day";
    spotify.sessionActive = true;
    let releaseSnapshot: (resp: Response) => void = () => {};
    routeFetch([
      { match: "/playlists/night/tracks", resp: tracksResponse(["n1"]) },
      { match: "/playlists/day/tracks", resp: tracksResponse(TRACKS) },
      { match: "/me/player/play", resp: makeResponse(404) },
      { match: "/me/player/shuffle", resp: makeResponse(204) },
      {
        match: "/me/player",
        resp: () =>
          new Promise<Response>((resolve) => {
            releaseSnapshot = resolve;
          }),
      },
    ]);

    // Night's snapshot hangs, so its rejection lands after Day has already
    // failed with no_device and legitimately raised the picker.
    const stale = switchToSlot("night");
    await switchToSlot("day");
    expect(spotify.needsDevicePick).toBe(true);

    releaseSnapshot(makeResponse(404));
    await stale;

    expect(spotify.error?.kind).toBe("no_device");
    expect(spotify.needsDevicePick).toBe(true);
  });

  it("lets the last requested slot win overlapping switches", async () => {
    spotify.currentSlot = "day";
    spotify.sessionActive = true;
    routeFetch([
      { match: "/playlists/day/tracks", resp: tracksResponse(TRACKS) },
      { match: "/playlists/night/tracks", resp: tracksResponse(["n1"]) },
      { match: "/me/player/play", resp: makeResponse(204) },
      { match: "/me/player/shuffle", resp: makeResponse(204) },
      {
        match: "/me/player",
        resp: playbackResponse(DAY_PLAYLIST.uri, "spotify:track:a"),
      },
    ]);

    // Night takes the extra snapshot round trip (currentSlot is Day), so
    // without a guard its play PUT would land after Day's and win the device.
    const first = switchToSlot("night");
    const second = switchToSlot("day");
    await Promise.all([first, second]);
    await flush();

    const contexts = playCalls().map((call) => bodyOf(call).context_uri);
    expect(contexts[contexts.length - 1]).toBe("spotify:playlist:day");
    expect(contexts).not.toContain("spotify:playlist:night");
    expect(spotify.currentSlot).toBe("day");
  });

  it("resets the played set once the playlist is exhausted", async () => {
    // Always take the first candidate, except on the pick after the reset —
    // 0.9 lands on the just-played track unless it has been excluded.
    vi.spyOn(Math, "random")
      .mockReturnValueOnce(0)
      .mockReturnValueOnce(0)
      .mockReturnValueOnce(0)
      .mockReturnValue(0.9);
    routeFetch([
      { match: "/playlists/day/tracks", resp: tracksResponse(TRACKS) },
    ]);

    await switchToSlot("day");
    await switchToSlot("day");
    await switchToSlot("day");
    await switchToSlot("day");

    const picked = playCalls().map(offsetURI);
    expect(picked).toHaveLength(4);
    // The whole playlist is used before anything repeats.
    expect(picked.slice(0, 3)).toEqual(TRACKS);
    // Starting over must not immediately replay the track just started.
    expect(TRACKS).toContain(picked[3]);
    expect(picked[3]).not.toBe(picked[2]);
  });

  it("retries once without the offset when the offset is rejected", async () => {
    let plays = 0;
    routeFetch([
      { match: "/playlists/day/tracks", resp: tracksResponse(TRACKS) },
      {
        match: "/me/player/play",
        resp: () =>
          ++plays === 1
            ? makeResponse(400, { error: { message: "Invalid track uri" } })
            : makeResponse(204),
      },
    ]);

    await switchToSlot("day");

    const calls = playCalls();
    expect(calls).toHaveLength(2);
    expect(TRACKS).toContain(offsetURI(calls[0]));
    expect(bodyOf(calls[1])).toEqual({ context_uri: "spotify:playlist:day" });
    expect(spotify.currentSlot).toBe("day");
    expect(spotify.error).toBeNull();
  });

  it("does not retry a no_device failure", async () => {
    routeFetch([
      { match: "/playlists/day/tracks", resp: tracksResponse(TRACKS) },
      { match: "/me/player/play", resp: makeResponse(404) },
    ]);

    await switchToSlot("day");

    expect(playCalls()).toHaveLength(1);
    expect(spotify.error?.kind).toBe("no_device");
    expect(spotify.sessionActive).toBe(false);
  });

  it("falls back to a random position for a playlist past the page cap", async () => {
    vi.spyOn(Math, "random").mockReturnValue(0.5);
    const fullPage = tracksResponse(
      Array.from({ length: 100 }, (_, i) => `spotify:track:${i}`),
      900,
    );
    routeFetch([{ match: "/playlists/day/tracks", resp: () => fullPage }]);

    await switchToSlot("day");

    expect(callsMatching("/playlists/day/tracks")).toHaveLength(5);
    expect(bodyOf(playCalls()[0]).offset).toEqual({ position: 450 });
  });

  it("uses the total from a partial read when a later page fails", async () => {
    vi.spyOn(Math, "random").mockReturnValue(0.5);
    const page1 = tracksResponse(
      Array.from({ length: 100 }, (_, i) => `spotify:track:${i}`),
      250,
    );
    let pages = 0;
    routeFetch([
      {
        match: "/playlists/day/tracks",
        resp: () =>
          ++pages === 1
            ? page1
            : makeResponse(500, { error: { message: "boom" } }),
      },
    ]);

    await switchToSlot("day");

    // Page 1 gave a total but no usable full list, so start at a position.
    expect(bodyOf(playCalls()[0]).offset).toEqual({ position: 125 });
    // A partial read is never cached — the next start pages again.
    pages = 0;
    await switchToSlot("day");
    expect(pages).toBe(2);
  });

  it("falls back to plain context playback when the list cannot be read", async () => {
    routeFetch([
      {
        match: "/playlists/day/tracks",
        resp: makeResponse(500, { error: { message: "boom" } }),
      },
    ]);

    await switchToSlot("day");

    const calls = playCalls();
    expect(calls).toHaveLength(1);
    expect(bodyOf(calls[0])).toEqual({ context_uri: "spotify:playlist:day" });
    expect(spotify.currentSlot).toBe("day");
  });
});

describe("setVolumeDebounced", () => {
  it("updates local volume immediately and sends one trailing call", async () => {
    setVolumeDebounced(10);
    expect(spotify.volume).toBe(10);
    setVolumeDebounced(45);
    await vi.advanceTimersByTimeAsync(100);
    setVolumeDebounced(80);

    expect(spotify.volume).toBe(80);
    expect(fetchMock).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(250);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchArgs(0);
    expect(url).toBe(
      "https://api.spotify.com/v1/me/player/volume?volume_percent=80",
    );
    expect(init.method).toBe("PUT");
  });

  it("includes the active device id", async () => {
    spotify.activeDeviceId = "device-42";
    setVolumeDebounced(33);
    await vi.advanceTimersByTimeAsync(250);

    expect(fetchArgs(0)[0]).toBe(
      "https://api.spotify.com/v1/me/player/volume?volume_percent=33&device_id=device-42",
    );
  });
});

describe("listPlaylists", () => {
  it("follows next pagination and flattens the pages", async () => {
    fetchMock
      .mockResolvedValueOnce(
        makeResponse(200, {
          items: [
            {
              uri: "spotify:playlist:a",
              name: "A",
              images: [
                { url: "https://img/a.jpg" },
                { url: "https://img/a2.jpg" },
              ],
            },
            { uri: "spotify:playlist:b", name: "B", images: [] },
          ],
          next: "https://api.spotify.com/v1/me/playlists?limit=50&offset=50",
        }),
      )
      .mockResolvedValueOnce(
        makeResponse(200, {
          items: [{ uri: "spotify:playlist:c", name: "C", images: null }],
          next: null,
        }),
      );

    const playlists = await listPlaylists();

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchArgs(0)[0]).toBe(
      "https://api.spotify.com/v1/me/playlists?limit=50",
    );
    expect(fetchArgs(1)[0]).toBe(
      "https://api.spotify.com/v1/me/playlists?limit=50&offset=50",
    );
    expect(playlists).toEqual([
      { uri: "spotify:playlist:a", name: "A", imageUrl: "https://img/a.jpg" },
      { uri: "spotify:playlist:b", name: "B", imageUrl: "" },
      { uri: "spotify:playlist:c", name: "C", imageUrl: "" },
    ]);
  });
});

describe("applySpotifyStatus", () => {
  it("ends the playback session on a server-reported disconnect", () => {
    spotify.connected = true;
    spotify.sessionActive = true;
    spotify.currentSlot = "day";
    spotify.isPlaying = true;

    applySpotifyStatus(undefined);

    expect(spotify.connected).toBe(false);
    expect(spotify.sessionActive).toBe(false);
    expect(spotify.currentSlot).toBeNull();
    expect(spotify.isPlaying).toBe(false);
  });

  it("keeps a running session when the server still reports connected", () => {
    spotify.sessionActive = true;
    spotify.currentSlot = "night";
    spotify.isPlaying = true;

    applySpotifyStatus({ connected: true } as never);

    expect(spotify.sessionActive).toBe(true);
    expect(spotify.currentSlot).toBe("night");
    expect(spotify.isPlaying).toBe(true);
  });
});

describe("playback polling", () => {
  afterEach(() => {
    stopPlaybackPolling();
  });

  it("surfaces a lost connection from a poll tick", async () => {
    spotify.connected = true;
    // Two 401s in a row is how spotifyFetch decides the grant is gone.
    fetchMock.mockResolvedValue(makeResponse(401));

    startPlaybackPolling();
    await vi.advanceTimersByTimeAsync(0);

    expect(spotify.error?.kind).toBe("not_connected");
    expect(spotify.connected).toBe(false);

    // The grant is dead — the timer must stop instead of retrying every tick.
    const callsAfterFailure = fetchMock.mock.calls.length;
    await vi.advanceTimersByTimeAsync(15_000);
    expect(fetchMock.mock.calls.length).toBe(callsAfterFailure);
  });

  it("swallows other poll failures without painting the error strip", async () => {
    fetchMock.mockResolvedValue(
      makeResponse(500, { error: { message: "server on fire" } }),
    );

    startPlaybackPolling();
    await vi.advanceTimersByTimeAsync(0);

    expect(fetchMock).toHaveBeenCalled();
    expect(spotify.error).toBeNull();
  });
});

describe("OAuth state round-trip", () => {
  it("builds an authorize URL with all params and a stored state", () => {
    spotify.clientId = "client-abc";

    const url = new URL(getSpotifyOAuthURL());

    expect(`${url.origin}${url.pathname}`).toBe(
      "https://accounts.spotify.com/authorize",
    );
    expect(url.searchParams.get("client_id")).toBe("client-abc");
    expect(url.searchParams.get("response_type")).toBe("code");
    expect(url.searchParams.get("redirect_uri")).toBe(
      `${window.location.origin}/auth/spotify/callback`,
    );
    expect(url.searchParams.get("scope")).toBe(SPOTIFY_SCOPES);

    const state = url.searchParams.get("state");
    expect(state).toBeTruthy();
    expect(sessionStorage.getItem("spotify_oauth_state")).toBe(state);
    expect(sessionStorage.getItem("spotify_return_to")).toBe(
      window.location.pathname,
    );
  });

  it("validates the state once and rejects afterwards", () => {
    const state = new URL(getSpotifyOAuthURL()).searchParams.get("state");

    expect(validateSpotifyOAuthState(state)).toBe(true);
    expect(validateSpotifyOAuthState(state)).toBe(false);
    expect(sessionStorage.getItem("spotify_oauth_state")).toBeNull();
  });

  it("rejects a missing or mismatched state", () => {
    getSpotifyOAuthURL();
    expect(validateSpotifyOAuthState("not-the-state")).toBe(false);
    getSpotifyOAuthURL();
    expect(validateSpotifyOAuthState(null)).toBe(false);
  });

  it("returns and clears the stored return path", () => {
    getSpotifyOAuthURL();
    sessionStorage.setItem("spotify_return_to", "/games/7");

    expect(takeSpotifyReturnTo()).toBe("/games/7");
    expect(takeSpotifyReturnTo()).toBe("/");
  });
});
