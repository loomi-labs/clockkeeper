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
const SHUFFLE_KEY = "clockkeeper_spotify_shuffle";

export const SPOTIFY_SCOPES =
  "user-read-playback-state user-modify-playback-state playlist-read-private playlist-read-collaborative user-read-private";

function emptyPlaylists(): Record<SlotKey, PlaylistRef | null> {
  return { day: null, night: null, nominations: null };
}

/** Shuffle is on unless the Storyteller explicitly turned it off. */
function storedShuffle(): boolean {
  if (typeof localStorage === "undefined") return true;
  return localStorage.getItem(SHUFFLE_KEY) !== "false";
}

export const spotify = $state({
  /** Feature flag — server has a Spotify client id configured. */
  available: false,
  clientId: "",
  connected: false,
  premium: false,
  displayName: "",
  playlists: emptyPlaylists(),
  /** Persisted preference, pushed to the device on every slot switch. */
  shuffle: storedShuffle(),
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
 * The Spotify grant is gone. Beyond flipping the connect CTA back on, this ends
 * the playback session: nothing can be paused, resumed or switched without a
 * token, so leaving `sessionActive` up would keep phase auto-switching armed
 * and keep offering session-gated UI for a session that cannot exist.
 */
function markGrantLost() {
  spotify.connected = false;
  spotify.sessionActive = false;
  spotify.currentSlot = null;
  spotify.isPlaying = false;
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
  // A server-reported disconnect (revoked elsewhere, row deleted) ends the
  // playback session too — otherwise transient flags like sessionActive keep
  // session-gated UI (e.g. the nominations step) alive with no connection.
  if (!spotify.connected) {
    spotify.sessionActive = false;
    spotify.currentSlot = null;
    spotify.isPlaying = false;
  }
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
  cancelPendingSwitch();
  clearTrackSelection();
  spotify.connected = false;
  spotify.premium = false;
  spotify.displayName = "";
  spotify.playlists = emptyPlaylists();
  // Not session state — re-read so memory matches what was persisted.
  spotify.shuffle = storedShuffle();
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
        // Same dead grant as the double-401 below, just discovered a step
        // earlier — the server has no usable refresh token for this account.
        markGrantLost();
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
      markGrantLost();
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

/** Where inside the context playback should start. */
export type PlayOffset = { uri: string } | { position: number };

export async function playContext(
  contextUri: string,
  deviceId?: string,
  offset?: PlayOffset,
) {
  const body: Record<string, unknown> = { context_uri: contextUri };
  if (offset) body.offset = offset;
  await spotifyFetch(`/me/player/play${deviceQuery(deviceId)}`, {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

export async function setShuffle(state: boolean, deviceId?: string) {
  const params = new URLSearchParams({ state: String(state) });
  if (deviceId) params.set("device_id", deviceId);
  await spotifyFetch(`/me/player/shuffle?${params}`, { method: "PUT" });
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
  item?: { uri?: string } | null;
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
// Playlist track lists
// ---------------------------------------------------------------------------

const TRACK_PAGE_LIMIT = 100;
/** 5 x 100 tracks. Beyond that we stop paging and fall back to a position. */
const MAX_TRACK_PAGES = 5;

export type PlaylistTracks = {
  /** Track URIs read so far — only trustworthy as a whole when `complete`. */
  uris: string[];
  /** Playlist length reported by the API, null when no page succeeded. */
  total: number | null;
  /** False when the list is truncated (>500 tracks) or a page failed. */
  complete: boolean;
};

// Spotify's February 2026 Web API migration renamed the playlist contents
// endpoint from /playlists/{id}/tracks to /playlists/{id}/items and the
// per-entry field from `track` to `item`; the old endpoint returns 403.
type RawTrackPage = {
  items?: ({ item?: { uri?: string } | null } | null)[];
  total?: number;
};

/** `spotify:playlist:37i9dQ` -> `37i9dQ`. */
function playlistId(playlistUri: string): string {
  const parts = playlistUri.split(":");
  return parts[parts.length - 1] ?? "";
}

const trackCache = new Map<string, PlaylistTracks>();

/**
 * Pages a playlist's track URIs so a slot can start on a specific song.
 * Never throws: a failure just means the caller loses URI-precise selection
 * and falls back to a position offset (or none) — music still has to start.
 * Successful reads are cached for the page session.
 */
export async function getPlaylistTrackURIs(
  playlistUri: string,
): Promise<PlaylistTracks> {
  const cached = trackCache.get(playlistUri);
  if (cached) return cached;

  const id = playlistId(playlistUri);
  const uris: string[] = [];
  let total: number | null = null;
  let complete = false;

  try {
    for (let page = 0; page < MAX_TRACK_PAGES; page++) {
      const params = new URLSearchParams({
        fields: "items(item(uri)),total",
        limit: String(TRACK_PAGE_LIMIT),
        offset: String(page * TRACK_PAGE_LIMIT),
      });
      const resp = await spotifyFetch<RawTrackPage>(
        `/playlists/${encodeURIComponent(id)}/items?${params}`,
      );
      if (typeof resp?.total === "number") total = resp.total;
      const items = resp?.items ?? [];
      for (const item of items) {
        const uri = item?.item?.uri;
        if (uri) uris.push(uri);
      }
      if (items.length < TRACK_PAGE_LIMIT) {
        complete = true;
        break;
      }
      if (total !== null && uris.length >= total) {
        complete = true;
        break;
      }
    }
  } catch {
    // Partial reads are still useful for their `total`.
    return { uris, total, complete: false };
  }

  const tracks: PlaylistTracks = { uris, total, complete };
  trackCache.set(playlistUri, tracks);
  return tracks;
}

// ---------------------------------------------------------------------------
// Played-aware track selection (module-level, transient per page load)
// ---------------------------------------------------------------------------

const playedBySlot: Partial<Record<SlotKey, Set<string>>> = {};
const lastPickBySlot: Partial<Record<SlotKey, string>> = {};

function playedSet(slot: SlotKey): Set<string> {
  return (playedBySlot[slot] ??= new Set<string>());
}

function markPlayed(slot: SlotKey, trackUri: string | undefined | null) {
  if (trackUri) playedSet(slot).add(trackUri);
}

function clearTrackSelection() {
  for (const key of Object.keys(playedBySlot) as SlotKey[]) {
    delete playedBySlot[key];
  }
  for (const key of Object.keys(lastPickBySlot) as SlotKey[]) {
    delete lastPickBySlot[key];
  }
  trackCache.clear();
}

function randomIndex(length: number): number {
  return Math.floor(Math.random() * length);
}

/**
 * Picks a track the slot has not played yet. Once the slot has been through
 * the whole playlist the set is wiped and every track is fair game again —
 * except the one it just started on, which would be an audible repeat.
 */
function pickUnplayed(slot: SlotKey, uris: string[]): string | null {
  if (uris.length === 0) return null;
  const played = playedSet(slot);
  let candidates = uris.filter((uri) => !played.has(uri));
  if (candidates.length === 0) {
    played.clear();
    const last = lastPickBySlot[slot];
    const fresh = uris.filter((uri) => uri !== last);
    candidates = fresh.length > 0 ? fresh : uris;
  }
  const pick = candidates[randomIndex(candidates.length)] ?? null;
  if (pick) lastPickBySlot[slot] = pick;
  return pick;
}

/** Records the track a playback snapshot shows, if it belongs to the slot. */
function notePlayingTrack(slot: SlotKey | null, state: PlaybackState | null) {
  if (!slot || !state) return;
  const playlist = spotify.playlists[slot];
  if (!playlist || state.context?.uri !== playlist.uri) return;
  markPlayed(slot, state.item?.uri);
}

// ---------------------------------------------------------------------------
// Session logic
// ---------------------------------------------------------------------------

/**
 * Ticket for the most recent switch request. Overlapping switches take
 * different numbers of round trips (the snapshot is conditional, track lists
 * may be cached), so without this the slower earlier call can issue its play
 * last and leave the device on a phase the Storyteller already moved past.
 */
let switchSeq = 0;

function isCurrentSwitch(seq: number): boolean {
  return seq === switchSeq;
}

/** Invalidates every in-flight switch so none of them can write state. */
function cancelPendingSwitch() {
  switchSeq++;
}

/**
 * Remembers what the outgoing slot was playing so switching back does not land
 * on it again. Best effort — a failed snapshot must never block the switch.
 */
async function snapshotOutgoingTrack(seq: number) {
  const slot = spotify.currentSlot;
  if (!slot) return;
  // spotifyFetch raises the device picker on a 404 as a side effect. A
  // best-effort read must not open it — the play that follows decides that.
  const hadDevicePick = spotify.needsDevicePick;
  try {
    notePlayingTrack(slot, await getPlaybackState());
  } catch {
    // Losing one snapshot only risks a repeat later. Undo the flag only while
    // this switch is still the current one: a newer switch may legitimately
    // have raised the picker while this rejection was in flight.
    if (isCurrentSwitch(seq)) spotify.needsDevicePick = hadDevicePick;
  }
}

/** Keeps the device in sync with the pref. Never fails the slot switch. */
async function applyShufflePref(deviceId?: string) {
  try {
    await setShuffle(spotify.shuffle, deviceId);
  } catch (err) {
    // Shuffle is a nicety — a device that refuses it still plays the slot.
    const error = toSpotifyError(err);
    if (error.kind === "not_connected") spotify.error = error;
  }
}

/**
 * Starts the slot's playlist on a track it has not played yet, so cycling
 * phases does not replay the same song. Falls back to a random position, then
 * to plain context playback, as the information available shrinks.
 *
 * Resolves to false when a newer switch superseded this one — the caller must
 * then leave the state alone.
 */
async function startSlotPlayback(
  slot: SlotKey,
  playlistUri: string,
  seq: number,
  deviceId?: string,
): Promise<boolean> {
  const tracks = await getPlaylistTrackURIs(playlistUri);
  // Never issue a play for a slot the Storyteller has already switched off.
  if (!isCurrentSwitch(seq)) return false;

  const pick = tracks.complete ? pickUnplayed(slot, tracks.uris) : null;
  let offset: PlayOffset | undefined;
  if (pick) {
    offset = { uri: pick };
  } else if (tracks.total !== null && tracks.total > 0) {
    offset = { position: randomIndex(tracks.total) };
  }

  try {
    await playContext(playlistUri, deviceId, offset);
  } catch (err) {
    if (!isCurrentSwitch(seq)) return false;
    // A stale or unavailable track breaks only the offset — the context is
    // still playable, so drop it and try once more. Device, plan and rate
    // limit failures are about the request itself; a retry cannot help.
    if (!offset || toSpotifyError(err).kind !== "api") throw err;
    await playContext(playlistUri, deviceId);
    return isCurrentSwitch(seq);
  }
  // The pick did start playing, so it counts as played either way.
  if (pick) markPlayed(slot, pick);
  return isCurrentSwitch(seq);
}

export async function switchToSlot(slot: SlotKey) {
  const playlist = spotify.playlists[slot];
  if (!playlist) {
    spotify.error = { kind: "slot_unconfigured", slot };
    return;
  }
  const seq = ++switchSeq;
  if (spotify.currentSlot && spotify.currentSlot !== slot) {
    await snapshotOutgoingTrack(seq);
    if (!isCurrentSwitch(seq)) return;
  }
  const deviceId = spotify.activeDeviceId ?? undefined;
  try {
    if (!(await startSlotPlayback(slot, playlist.uri, seq, deviceId))) return;
    spotify.currentSlot = slot;
    spotify.sessionActive = true;
    spotify.isPlaying = true;
    spotify.needsDevicePick = false;
    spotify.error = null;
  } catch (err) {
    if (!isCurrentSwitch(seq)) return;
    // no_device leaves needsDevicePick set — the UI opens the device picker
    // and re-invokes this after a device is chosen.
    spotify.error = toSpotifyError(err);
    return;
  }
  // Not awaited: the panel's busy flag should release once audio starts, not
  // after a follow-up call that cannot change the outcome.
  void applyShufflePref(deviceId);
}

/**
 * Ends the playback session: pauses the device and disarms the phase
 * auto-switch. Deliberately NOT a disconnect — playlists, devices, shuffle and
 * the token cache all survive, so the play button can restart the current
 * phase's slot (it falls through to switchToSlot once currentSlot is null).
 */
export async function stopMusic() {
  // Invalidate up front: the tail of this function overwrites whatever a
  // concurrent switch writes *during* the pause, but a switch that resolves
  // after stopMusic returns would otherwise re-arm the session behind it.
  cancelPendingSwitch();
  // A pause that finds no device raises the picker as a side effect of
  // spotifyFetch. The Storyteller is stopping, not trying to start something,
  // so that flag has to be put back exactly as it was.
  const hadDevicePick = spotify.needsDevicePick;
  try {
    await pausePlayback();
    spotify.error = null;
  } catch (err) {
    const error = toSpotifyError(err);
    // Nothing playing, a sleeping device, a rate limit — none of it changes
    // the intent to stop, so the failure is swallowed rather than left on
    // screen. A lost grant is the one exception the whole module surfaces.
    spotify.error = error.kind === "not_connected" ? error : null;
    spotify.needsDevicePick = hadDevicePick;
  }
  spotify.sessionActive = false;
  spotify.currentSlot = null;
  spotify.isPlaying = false;
}

/** Persists the shuffle preference and pushes it to a live session. */
export function setShufflePref(on: boolean) {
  spotify.shuffle = on;
  if (typeof localStorage !== "undefined") {
    localStorage.setItem(SHUFFLE_KEY, String(on));
  }
  if (!spotify.sessionActive) return;
  void setShuffle(on, spotify.activeDeviceId ?? undefined).catch(
    (err: unknown) => {
      const error = toSpotifyError(err);
      if (error.kind === "not_connected") spotify.error = error;
    },
  );
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
    notePlayingTrack(spotify.currentSlot, state);
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
    // disconnected and has to say why — and there is nothing left to poll,
    // so stop the timer instead of hammering the dead grant every tick.
    const error = toSpotifyError(err);
    if (error.kind === "not_connected") {
      spotify.error = error;
      stopPlaybackPolling();
    }
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
