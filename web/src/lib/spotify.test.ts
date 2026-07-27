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
  resetSpotifyState,
  getAccessToken,
  spotifyFetch,
  switchToSlot,
  syncPhase,
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

function fetchArgs(index: number): [string, RequestInit] {
  return fetchMock.mock.calls[index] as unknown as [string, RequestInit];
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
  resetSpotifyState();
  sessionStorage.clear();

  rpc.getSpotifyAccessToken.mockResolvedValue(tokenResponse("tok-1"));
  fetchMock = vi.fn().mockResolvedValue(makeResponse(204));
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  resetSpotifyState();
  vi.useRealTimers();
  vi.unstubAllGlobals();
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

  it("maps FailedPrecondition to not_connected and clears connected", async () => {
    spotify.connected = true;
    rpc.getSpotifyAccessToken.mockRejectedValue(
      new ConnectError("spotify not linked", Code.FailedPrecondition),
    );

    const err = await captureError(() => getAccessToken());
    expect(err.kind).toBe("not_connected");
    expect(spotify.connected).toBe(false);
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

  it("treats a second 401 as not_connected", async () => {
    spotify.connected = true;
    fetchMock.mockResolvedValue(makeResponse(401));

    const err = await captureError(() => spotifyFetch("/me/player/devices"));

    expect(err.kind).toBe("not_connected");
    expect(spotify.connected).toBe(false);
    expect(fetchMock).toHaveBeenCalledTimes(2);
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

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchArgs(0);
    expect(url).toBe(
      "https://api.spotify.com/v1/me/player/play?device_id=device-42",
    );
    expect(init.method).toBe("PUT");
    expect(init.body).toBe(
      JSON.stringify({ context_uri: "spotify:playlist:day" }),
    );
    expect(spotify.currentSlot).toBe("day");
    expect(spotify.sessionActive).toBe(true);
    expect(spotify.isPlaying).toBe(true);
    expect(spotify.error).toBeNull();
  });

  it("omits device_id when no device is active", async () => {
    spotify.playlists.night = NIGHT_PLAYLIST;
    await switchToSlot("night");
    expect(fetchArgs(0)[0]).toBe("https://api.spotify.com/v1/me/player/play");
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
    expect(fetchArgs(0)[1].body).toBe(
      JSON.stringify({ context_uri: "spotify:playlist:day" }),
    );

    await syncPhase(false);
    expect(spotify.currentSlot).toBe("night");
    expect(fetchArgs(1)[1].body).toBe(
      JSON.stringify({ context_uri: "spotify:playlist:night" }),
    );
  });

  it("overrides a manual nominations selection on a phase flip", async () => {
    spotify.connected = true;
    spotify.sessionActive = true;
    spotify.currentSlot = "nominations";

    await syncPhase(true);

    expect(spotify.currentSlot).toBe("day");
    expect(fetchMock).toHaveBeenCalledTimes(1);
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
