import { ConnectError, Code } from "@connectrpc/connect";
import { client } from "./api";
import { getErrorMessage } from "./errors";
import type { SpotifyStatus } from "./gen/clockkeeper/v1/clockkeeper_pb";

export type SlotKey = "day" | "night" | "nominations";

export type PlaylistRef = {
  uri: string;
  name: string;
  imageUrl: string;
};

export type SpotifyDevice = {
  id: string;
  name: string;
  type: string;
  isActive: boolean;
  volumePercent: number;
};

export type SpotifyErrorKind =
  | "not_connected"
  | "no_device"
  | "premium_required"
  | "rate_limited"
  | "slot_unconfigured"
  | "api";

export type SpotifyError = {
  kind: SpotifyErrorKind;
  message?: string;
  retryAfterSec?: number;
  slot?: SlotKey;
};

const DEFAULT_VOLUME = 60;
const API_BASE = "https://api.spotify.com/v1";
const TOKEN_SKEW_MS = 60_000;
const VOLUME_DEBOUNCE_MS = 250;
const POLL_INTERVAL_MS = 5_000;

const OAUTH_STATE_KEY = "spotify_oauth_state";
const RETURN_TO_KEY = "spotify_return_to";

export const SPOTIFY_SCOPES =
  "user-read-playback-state user-modify-playback-state playlist-read-private playlist-read-collaborative user-read-private";

function emptyPlaylists(): Record<SlotKey, PlaylistRef | null> {
  return { day: null, night: null, nominations: null };
}

export const spotify = $state({
  /** Feature flag — server has a Spotify client id configured. */
  available: false,
  clientId: "",
  connected: false,
  premium: false,
  displayName: "",
  playlists: emptyPlaylists(),
  // Transient playback session (never persisted).
  /** True once the Storyteller has started music — gates phase auto-switch. */
  sessionActive: false,
  currentSlot: null as SlotKey | null,
  isPlaying: false,
  volume: DEFAULT_VOLUME,
  devices: [] as SpotifyDevice[],
  activeDeviceId: null as string | null,
  needsDevicePick: false,
  error: null as SpotifyError | null,
});

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

function isSpotifyError(err: unknown): err is SpotifyError {
  return (
    typeof err === "object" &&
    err !== null &&
    typeof (err as { kind?: unknown }).kind === "string"
  );
}

/** Normalize anything thrown by this module (or the network) into a SpotifyError. */
export function toSpotifyError(err: unknown): SpotifyError {
  if (isSpotifyError(err)) return err;
  return {
    kind: "api",
    message: getErrorMessage(err, "Spotify request failed"),
  };
}

export function clearSpotifyError() {
  spotify.error = null;
}

/**
 * The device picker has served its purpose — playback reached a device by a
 * route that does not clear the flag itself (e.g. a plain resume).
 */
export function clearDevicePick() {
  spotify.needsDevicePick = false;
}

// ---------------------------------------------------------------------------
// Status (ConnectRPC)
// ---------------------------------------------------------------------------

function toPlaylistRef(
  slot: { uri: string; name: string; imageUrl: string } | undefined,
): PlaylistRef | null {
  if (!slot || !slot.uri) return null;
  return { uri: slot.uri, name: slot.name, imageUrl: slot.imageUrl };
}

/** Apply a server-side SpotifyStatus onto the rune state. */
export function applySpotifyStatus(status: SpotifyStatus | undefined) {
  spotify.connected = status?.connected ?? false;
  spotify.premium = status?.premium ?? false;
  spotify.displayName = status?.displayName ?? "";
  spotify.playlists = {
    day: toPlaylistRef(status?.day),
    night: toPlaylistRef(status?.night),
    nominations: toPlaylistRef(status?.nominations),
  };
}

/**
 * Lazily called when the panel opens — not app-wide. Resolves to whether the
 * status RPC succeeded, so the caller can retry a failed attempt.
 */
export async function initSpotifyStatus(): Promise<boolean> {
  try {
    const resp = await client.getSpotifyStatus({});
    applySpotifyStatus(resp.status);
    spotify.error = null;
    return true;
  } catch (err) {
    // A transient RPC failure says nothing about the account link — clobbering
    // the state here would flip a connected Storyteller to the connect CTA.
    // Keep what we have and only surface the error.
    spotify.error = {
      kind: "api",
      message: getErrorMessage(err, "Could not load Spotify status"),
    };
    return false;
  }
}

export async function disconnect() {
  await client.disconnectSpotify({});
  resetSpotifyState();
}

/** Replaces ALL three slots server-side — absent slot clears it. */
export async function savePlaylists(
  slots: Record<SlotKey, PlaylistRef | null>,
) {
  const resp = await client.updateSpotifyPlaylists({
    day: slots.day ?? undefined,
    night: slots.night ?? undefined,
    nominations: slots.nominations ?? undefined,
  });
  applySpotifyStatus(resp.status);
}

/** Resets connection + session state (keeps `available`/`clientId`). */
export function resetSpotifyState() {
  stopPlaybackPolling();
  cancelVolumeDebounce();
  clearTokenCache();
  spotify.connected = false;
  spotify.premium = false;
  spotify.displayName = "";
  spotify.playlists = emptyPlaylists();
  spotify.sessionActive = false;
  spotify.currentSlot = null;
  spotify.isPlaying = false;
  spotify.volume = DEFAULT_VOLUME;
  spotify.devices = [];
  spotify.activeDeviceId = null;
  spotify.needsDevicePick = false;
  spotify.error = null;
}

// ---------------------------------------------------------------------------
// OAuth
// ---------------------------------------------------------------------------

function generateOAuthState(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(32));
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

export function getSpotifyOAuthURL(): string {
  const redirectUri = `${window.location.origin}/auth/spotify/callback`;
  const state = generateOAuthState();
  sessionStorage.setItem(OAUTH_STATE_KEY, state);
  sessionStorage.setItem(RETURN_TO_KEY, window.location.pathname);
  const params = new URLSearchParams({
    client_id: spotify.clientId,
    response_type: "code",
    redirect_uri: redirectUri,
    state,
    scope: SPOTIFY_SCOPES,
  });
  return `https://accounts.spotify.com/authorize?${params}`;
}

/**
 * CSRF guard for account linking. The ConnectSpotify RPC carries no state
 * field, so this client-side check is the only protection.
 */
export function validateSpotifyOAuthState(state: string | null): boolean {
  const expected = sessionStorage.getItem(OAUTH_STATE_KEY);
  sessionStorage.removeItem(OAUTH_STATE_KEY);
  return !!state && !!expected && state === expected;
}

/** Reads and clears the pre-OAuth location so the callback can return there. */
export function takeSpotifyReturnTo(): string {
  const target = sessionStorage.getItem(RETURN_TO_KEY);
  sessionStorage.removeItem(RETURN_TO_KEY);
  return target || "/";
}

// ---------------------------------------------------------------------------
// Access token cache (module-level, deliberately not reactive)
// ---------------------------------------------------------------------------

let accessToken: string | null = null;
let expiresAtMs = 0;
let inflight: Promise<string> | null = null;
// Identifies the request currently owning `inflight`, so a superseded request
// cannot clear a newer one when it settles.
let tokenRequestSeq = 0;
let inflightId = 0;

function clearTokenCache() {
  accessToken = null;
  expiresAtMs = 0;
}

export async function getAccessToken(force = false): Promise<string> {
  if (!force) {
    if (accessToken && Date.now() <= expiresAtMs - TOKEN_SKEW_MS) {
      return accessToken;
    }
    // Single-flight: an in-progress fetch always yields a fresh token.
    if (inflight) return inflight;
  }

  // A forced call never joins an existing request — that request may predate
  // the 401 that triggered the force, so only a new RPC (with force_refresh,
  // which makes the server re-mint against Spotify) proves the grant is alive.
  // Later followers join this fresher promise.
  const requestId = ++tokenRequestSeq;
  inflightId = requestId;
  const request = (async () => {
    try {
      const resp = await client.getSpotifyAccessToken({ forceRefresh: force });
      accessToken = resp.accessToken;
      expiresAtMs = Number(resp.expiresAtUnix) * 1000;
      return resp.accessToken;
    } catch (err) {
      clearTokenCache();
      if (err instanceof ConnectError && err.code === Code.FailedPrecondition) {
        spotify.connected = false;
        throw { kind: "not_connected" } satisfies SpotifyError;
      }
      throw {
        kind: "api",
        message: getErrorMessage(err, "Could not get a Spotify token"),
      } satisfies SpotifyError;
    } finally {
      if (inflightId === requestId) inflight = null;
    }
  })();
  inflight = request;

  return request;
}

// ---------------------------------------------------------------------------
// Web API fetch
// ---------------------------------------------------------------------------

type FetchInit = {
  method?: string;
  body?: string;
  headers?: Record<string, string>;
};

async function readBody(
  resp: Response,
): Promise<Record<string, unknown> | null> {
  try {
    const text = await resp.text();
    if (!text) return null;
    return JSON.parse(text) as Record<string, unknown>;
  } catch {
    return null;
  }
}

function rawFetch(
  path: string,
  init: FetchInit,
  token: string,
): Promise<Response> {
  const headers: Record<string, string> = { ...init.headers };
  if (init.body !== undefined && !headers["Content-Type"]) {
    headers["Content-Type"] = "application/json";
  }
  headers["Authorization"] = `Bearer ${token}`;
  return fetch(`${API_BASE}${path}`, { ...init, headers });
}

/**
 * Calls the Spotify Web API with a cached bearer token and normalizes
 * failures into SpotifyError. Returns null for empty bodies (204/202).
 */
export async function spotifyFetch<T = unknown>(
  path: string,
  init: FetchInit = {},
): Promise<T | null> {
  let resp = await rawFetch(path, init, await getAccessToken());

  if (resp.status === 401) {
    // Token rejected — force a refresh and retry exactly once.
    resp = await rawFetch(path, init, await getAccessToken(true));
    if (resp.status === 401) {
      clearTokenCache();
      spotify.connected = false;
      throw { kind: "not_connected" } satisfies SpotifyError;
    }
  }

  if (resp.status === 404 && path.startsWith("/me/player")) {
    spotify.needsDevicePick = true;
    throw { kind: "no_device" } satisfies SpotifyError;
  }

  if (resp.status === 403) {
    const body = await readBody(resp);
    const detail = (body?.error ?? body) as
      | { reason?: string; message?: string }
      | undefined;
    const message = detail?.message ?? "";
    if (detail?.reason === "PREMIUM_REQUIRED" || /premium/i.test(message)) {
      throw { kind: "premium_required", message } satisfies SpotifyError;
    }
    throw {
      kind: "api",
      message: message || "Spotify rejected the request",
    } satisfies SpotifyError;
  }

  if (resp.status === 429) {
    const retryAfter = Number(resp.headers.get("Retry-After") ?? "");
    throw {
      kind: "rate_limited",
      retryAfterSec:
        Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter : 1,
    } satisfies SpotifyError;
  }

  if (resp.status < 200 || resp.status >= 300) {
    const body = await readBody(resp);
    const detail = (body?.error ?? body) as { message?: string } | undefined;
    throw {
      kind: "api",
      message: detail?.message || `Spotify request failed (${resp.status})`,
    } satisfies SpotifyError;
  }

  if (resp.status === 204 || resp.status === 202) return null;
  return (await readBody(resp)) as T | null;
}

// ---------------------------------------------------------------------------
// Web API wrappers
// ---------------------------------------------------------------------------

function deviceQuery(deviceId?: string): string {
  return deviceId ? `?device_id=${encodeURIComponent(deviceId)}` : "";
}

export async function playContext(contextUri: string, deviceId?: string) {
  await spotifyFetch(`/me/player/play${deviceQuery(deviceId)}`, {
    method: "PUT",
    body: JSON.stringify({ context_uri: contextUri }),
  });
}

export async function pausePlayback() {
  await spotifyFetch("/me/player/pause", { method: "PUT" });
}

export async function resumePlayback() {
  await spotifyFetch("/me/player/play", { method: "PUT" });
}

export async function setVolume(pct: number, deviceId?: string) {
  const clamped = Math.max(0, Math.min(100, Math.round(pct)));
  const params = new URLSearchParams({ volume_percent: String(clamped) });
  if (deviceId) params.set("device_id", deviceId);
  await spotifyFetch(`/me/player/volume?${params}`, { method: "PUT" });
}

type RawDevice = {
  id: string | null;
  name: string;
  type: string;
  is_active: boolean;
  volume_percent: number | null;
};

export async function getDevices(): Promise<SpotifyDevice[]> {
  const resp = await spotifyFetch<{ devices?: RawDevice[] }>(
    "/me/player/devices",
  );
  const devices: SpotifyDevice[] = (resp?.devices ?? [])
    .filter((d) => !!d.id)
    .map((d) => ({
      id: d.id as string,
      name: d.name,
      type: d.type,
      isActive: !!d.is_active,
      volumePercent: d.volume_percent ?? 0,
    }));
  spotify.devices = devices;
  const active = devices.find((d) => d.isActive);
  if (active) spotify.activeDeviceId = active.id;
  return devices;
}

export async function transferPlayback(deviceId: string, play: boolean) {
  await spotifyFetch("/me/player", {
    method: "PUT",
    body: JSON.stringify({ device_ids: [deviceId], play }),
  });
  spotify.activeDeviceId = deviceId;
  spotify.needsDevicePick = false;
}

export type PlaybackState = {
  is_playing?: boolean;
  device?: RawDevice;
  context?: { uri?: string } | null;
};

/** GET /me/player — null when nothing is playing (204). */
export async function getPlaybackState(): Promise<PlaybackState | null> {
  return await spotifyFetch<PlaybackState>("/me/player");
}

type RawPlaylistPage = {
  items?: {
    uri: string;
    name: string;
    images?: { url: string }[] | null;
  }[];
  next?: string | null;
};

function toApiPath(url: string): string {
  if (url.startsWith(API_BASE)) return url.slice(API_BASE.length);
  try {
    const parsed = new URL(url);
    return `${parsed.pathname.replace(/^\/v1/, "")}${parsed.search}`;
  } catch {
    return url;
  }
}

/**
 * GET /me/playlists, following `next` until exhausted. The page cap is a
 * safety net: a malformed self-referencing `next` would otherwise spin
 * forever. 20 pages x 50 items is far more than any real library.
 */
const MAX_PLAYLIST_PAGES = 20;

export async function listPlaylists(): Promise<PlaylistRef[]> {
  const out: PlaylistRef[] = [];
  let path: string | null = "/me/playlists?limit=50";
  for (let fetched = 0; fetched < MAX_PLAYLIST_PAGES && path; fetched++) {
    const page: RawPlaylistPage | null =
      await spotifyFetch<RawPlaylistPage>(path);
    for (const item of page?.items ?? []) {
      out.push({
        uri: item.uri,
        name: item.name,
        imageUrl: item.images?.[0]?.url ?? "",
      });
    }
    path = page?.next ? toApiPath(page.next) : null;
  }
  return out;
}

// ---------------------------------------------------------------------------
// Session logic
// ---------------------------------------------------------------------------

export async function switchToSlot(slot: SlotKey) {
  const playlist = spotify.playlists[slot];
  if (!playlist) {
    spotify.error = { kind: "slot_unconfigured", slot };
    return;
  }
  try {
    await playContext(playlist.uri, spotify.activeDeviceId ?? undefined);
    spotify.currentSlot = slot;
    spotify.sessionActive = true;
    spotify.isPlaying = true;
    spotify.needsDevicePick = false;
    spotify.error = null;
  } catch (err) {
    // no_device leaves needsDevicePick set — the UI opens the device picker
    // and re-invokes this after a device is chosen.
    spotify.error = toSpotifyError(err);
  }
}

/** Auto-switch on day/night flip. Only runs inside an active session. */
export async function syncPhase(isDay: boolean) {
  if (!spotify.connected || !spotify.sessionActive) return;
  // A phase flip always wins, including over a manual "nominations" override.
  await switchToSlot(isDay ? "day" : "night");
}

let volumeTimer: ReturnType<typeof setTimeout> | null = null;
let pendingVolume: number | null = null;

function cancelVolumeDebounce() {
  if (volumeTimer !== null) clearTimeout(volumeTimer);
  volumeTimer = null;
  pendingVolume = null;
}

/** Updates the slider immediately, trailing-debounces the API call. */
export function setVolumeDebounced(pct: number) {
  spotify.volume = pct;
  pendingVolume = pct;
  if (volumeTimer !== null) clearTimeout(volumeTimer);
  volumeTimer = setTimeout(() => {
    volumeTimer = null;
    const value = pendingVolume;
    pendingVolume = null;
    if (value === null) return;
    void setVolume(value, spotify.activeDeviceId ?? undefined).catch((err) => {
      spotify.error = toSpotifyError(err);
    });
  }, VOLUME_DEBOUNCE_MS);
}

let pollTimer: ReturnType<typeof setInterval> | null = null;

async function pollPlaybackOnce() {
  try {
    const state = await getPlaybackState();
    if (!state) {
      spotify.isPlaying = false;
      return;
    }
    spotify.isPlaying = !!state.is_playing;
    if (state.device) {
      if (state.device.id) spotify.activeDeviceId = state.device.id;
      const midDebounce = volumeTimer !== null || pendingVolume !== null;
      if (!midDebounce && typeof state.device.volume_percent === "number") {
        spotify.volume = state.device.volume_percent;
      }
    }
  } catch (err) {
    // Polling errors are swallowed — no error-state spam while the panel is
    // open. The exception is a lost connection: the panel just flipped to
    // disconnected and has to say why.
    const error = toSpotifyError(err);
    if (error.kind === "not_connected") spotify.error = error;
  }
}

/** Panel-open only: refresh playback state every 5s. */
export function startPlaybackPolling() {
  if (pollTimer !== null) return;
  void pollPlaybackOnce();
  pollTimer = setInterval(() => void pollPlaybackOnce(), POLL_INTERVAL_MS);
}

export function stopPlaybackPolling() {
  if (pollTimer !== null) clearInterval(pollTimer);
  pollTimer = null;
}
