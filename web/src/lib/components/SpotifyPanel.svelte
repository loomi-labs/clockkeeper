<script lang="ts">
  import { untrack } from "svelte";
  import {
    spotify,
    initSpotifyStatus,
    getSpotifyOAuthURL,
    switchToSlot,
    setVolumeDebounced,
    pausePlayback,
    resumePlayback,
    getDevices,
    transferPlayback,
    startPlaybackPolling,
    stopPlaybackPolling,
    clearDevicePick,
    clearSpotifyError,
    toSpotifyError,
    type SlotKey,
    type SpotifyError,
  } from "~/lib/spotify.svelte";
  import SpotifyConfigModal from "./SpotifyConfigModal.svelte";

  let { activeIsDay }: { activeIsDay: boolean } = $props();

  // Panel order puts Night first — that is where a game starts.
  const slots: { slot: SlotKey; label: string }[] = [
    { slot: "night", label: "Night" },
    { slot: "day", label: "Day" },
    { slot: "nominations", label: "Nominations" },
  ];

  const slotLabels: Record<SlotKey, string> = {
    day: "Day",
    night: "Night",
    nominations: "Nominations",
  };

  let open = $state(false);
  let statusRequested = false;
  let configOpen = $state(false);
  let devicesOpen = $state(false);
  let devicesLoading = $state(false);
  let busy = $state(false);

  const activeDevice = $derived(
    spotify.devices.find((d) => d.id === spotify.activeDeviceId),
  );

  /**
   * Warms `spotify.devices` so the device row can name the device the poll
   * reports as active. Single-flight, and near-silent: a background prefetch
   * must not paint the error strip for a transient failure. The exception
   * mirrors pollPlaybackOnce — a lost grant flips the panel back to the
   * connect CTA as a side effect, so it has to say why.
   */
  let devicePrefetch: Promise<void> | null = null;
  function prefetchDevices(): Promise<void> {
    devicePrefetch ??= getDevices()
      .then(() => {})
      .catch((err: unknown) => {
        const error = toSpotifyError(err);
        if (error.kind === "not_connected") spotify.error = error;
      })
      .finally(() => {
        devicePrefetch = null;
      });
    return devicePrefetch;
  }

  function toggle() {
    open = !open;
    if (!open) return;
    if (spotify.connected) void prefetchDevices();
    if (!statusRequested) {
      statusRequested = true;
      // The status call may be what first reveals the connection, so retry the
      // prefetch once it lands (single-flight dedupes the overlap above).
      void initSpotifyStatus().then((ok) => {
        // A failed status call leaves the panel showing stale state — release
        // the latch so reopening retries instead of dead-ending.
        if (!ok) {
          statusRequested = false;
          return;
        }
        if (open && spotify.connected) void prefetchDevices();
      });
    }
  }

  function handleClickOutside(event: MouseEvent) {
    // The config modal is portalled to <body>, so its clicks land outside the
    // panel — they must not be read as a dismissal.
    if (configOpen) return;
    const target = event.target as HTMLElement;
    if (!target.closest(".spotify-panel")) open = false;
  }

  $effect(() => {
    if (!open) return;
    document.addEventListener("click", handleClickOutside, true);
    return () =>
      document.removeEventListener("click", handleClickOutside, true);
  });

  // Playback state is only worth polling while the Storyteller is looking at it.
  $effect(() => {
    if (!open || !spotify.connected) return;
    startPlaybackPolling();
    return () => stopPlaybackPolling();
  });

  // A failed play because nothing is listening — surface the picker right away.
  $effect(() => {
    const needsPick = spotify.needsDevicePick;
    const isOpen = open;
    untrack(() => {
      if (isOpen && needsPick && !devicesOpen) {
        devicesOpen = true;
        void refreshDevices();
      }
    });
  });

  function connect() {
    window.location.href = getSpotifyOAuthURL();
  }

  /**
   * Runs an action that throws SpotifyError, routing failures into panel state.
   * Resolves to whether it succeeded so callers can gate follow-up UI.
   */
  async function run(action: () => Promise<void>): Promise<boolean> {
    busy = true;
    try {
      await action();
      spotify.error = null;
      return true;
    } catch (err) {
      spotify.error = toSpotifyError(err);
      return false;
    } finally {
      busy = false;
    }
  }

  // switchToSlot resolves its own errors into spotify.error, so it must not go
  // through run() (which would clear the error it just set).
  async function selectSlot(slot: SlotKey) {
    busy = true;
    try {
      await switchToSlot(slot);
    } finally {
      busy = false;
    }
  }

  async function togglePlayback() {
    if (spotify.sessionActive && spotify.isPlaying) {
      await run(async () => {
        await pausePlayback();
        // Optimistic — the 5s poll would otherwise lag the icon.
        spotify.isPlaying = false;
      });
    } else if (spotify.currentSlot) {
      await run(async () => {
        await resumePlayback();
        spotify.isPlaying = true;
        spotify.sessionActive = true;
        clearDevicePick();
      });
    } else {
      await selectSlot(activeIsDay ? "day" : "night");
    }
  }

  // Deliberately does not clear spotify.error: listing devices does not resolve
  // the condition that produced it (typically "no active device").
  async function refreshDevices() {
    devicesLoading = true;
    try {
      await getDevices();
    } catch (err) {
      spotify.error = toSpotifyError(err);
    } finally {
      devicesLoading = false;
    }
  }

  function toggleDevices() {
    devicesOpen = !devicesOpen;
    if (devicesOpen) void refreshDevices();
  }

  async function pickDevice(deviceId: string) {
    const ok = await run(async () => {
      await transferPlayback(deviceId, spotify.sessionActive);
      await getDevices();
    });
    if (ok) devicesOpen = false;
  }

  function errorText(err: SpotifyError): string {
    switch (err.kind) {
      case "slot_unconfigured":
        return `No playlist set for ${err.slot ? slotLabels[err.slot] : "this slot"} — configure it`;
      case "no_device":
        return "No active Spotify device";
      case "premium_required":
        return "Spotify Premium required";
      case "not_connected":
        return "Spotify connection lost — reconnect";
      case "rate_limited":
        return `Spotify is rate-limiting, retry in ${err.retryAfterSec ?? 1}s`;
      default:
        return err.message || "Spotify request failed";
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    // The config modal owns Escape while it is up.
    if (e.key === "Escape" && open && !configOpen) open = false;
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="spotify-panel relative">
  <button
    type="button"
    onclick={toggle}
    class="rounded-lg border border-border px-3 py-2.5 text-sm font-medium transition-colors {spotify.sessionActive &&
    spotify.isPlaying
      ? 'border-green-300 bg-green-100 text-green-600 dark:border-green-600 dark:bg-green-500/20 dark:text-green-400'
      : 'text-secondary hover:bg-hover hover:text-primary'}"
    title="Phase music"
    aria-label="Phase music"
    aria-expanded={open}
  >
    <svg
      class="h-4 w-4"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="2"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        d="M9 9l10.5-3m0 6.553v3.75a2.25 2.25 0 01-1.632 2.163l-1.32.377a1.803 1.803 0 11-.99-3.467l2.31-.66a2.25 2.25 0 001.632-2.163zm0 0V2.25L9 5.25v10.303m0 0v3.75a2.25 2.25 0 01-1.632 2.163l-1.32.377a1.803 1.803 0 01-.99-3.467l2.31-.66A2.25 2.25 0 009 15.553z"
      />
    </svg>
  </button>

  {#if open}
    <div
      class="absolute right-0 top-full z-50 mt-1 w-72 rounded-lg border border-border bg-surface p-3 shadow-lg"
    >
      <div class="flex items-center justify-between">
        <span class="text-sm font-semibold text-primary">Phase Music</span>
        {#if spotify.connected}
          <span
            class="flex items-center gap-1 rounded-full bg-element px-2 py-0.5 text-[11px] font-medium text-secondary"
          >
            {#if activeIsDay}
              <svg
                class="h-3 w-3"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="1.5"
                ><path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M12 3v2.25m6.364.386l-1.591 1.591M21 12h-2.25m-.386 6.364l-1.591-1.591M12 18.75V21m-4.773-4.227l-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0z"
                /></svg
              >
              Day
            {:else}
              <svg
                class="h-3 w-3"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="1.5"
                ><path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M21.752 15.002A9.718 9.718 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z"
                /></svg
              >
              Night
            {/if}
          </span>
        {/if}
      </div>

      {#if !spotify.connected}
        <p class="mt-2 text-xs text-secondary">
          Play phase music on a Spotify device.
        </p>
        <button
          type="button"
          onclick={connect}
          class="mt-3 w-full rounded-lg bg-green-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-green-500"
        >
          Connect Spotify
        </button>
        <p class="mt-2 text-[11px] text-muted">Requires Spotify Premium</p>
      {:else}
        {#if !spotify.premium}
          <p
            class="mt-2 rounded-lg border border-warning-border bg-warning-bg px-2 py-1.5 text-[11px] text-warning-text"
          >
            Spotify Premium is required for playback control. Upgraded recently?
            Disconnect and reconnect.
          </p>
        {/if}

        <!-- Slots -->
        <div class="mt-3 grid grid-cols-3 gap-1.5">
          {#each slots as s (s.slot)}
            {@const playlist = spotify.playlists[s.slot]}
            <button
              type="button"
              onclick={() => selectSlot(s.slot)}
              disabled={busy}
              class="min-w-0 rounded-lg border px-2 py-1.5 text-left transition-colors disabled:opacity-60 {spotify.currentSlot ===
              s.slot
                ? 'border-green-300 bg-green-100 dark:border-green-600 dark:bg-green-500/20'
                : 'border-border hover:bg-hover'}"
              title={playlist ? playlist.name : `${s.label} — no playlist set`}
            >
              <span
                class="block truncate text-xs font-semibold {spotify.currentSlot ===
                s.slot
                  ? 'text-green-700 dark:text-green-300'
                  : 'text-primary'}"
              >
                {s.label}
              </span>
              <span class="mt-0.5 block truncate text-[11px] text-muted">
                {playlist ? playlist.name : "Not set"}
              </span>
            </button>
          {/each}
        </div>

        <!-- Transport -->
        <div class="mt-3 flex items-center gap-2">
          <button
            type="button"
            onclick={togglePlayback}
            disabled={busy}
            class="shrink-0 rounded-lg border border-border p-2 text-secondary transition-colors hover:bg-hover hover:text-primary disabled:opacity-60"
            title={spotify.sessionActive && spotify.isPlaying
              ? "Pause"
              : "Play"}
            aria-label={spotify.sessionActive && spotify.isPlaying
              ? "Pause"
              : "Play"}
          >
            {#if spotify.sessionActive && spotify.isPlaying}
              <svg
                class="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
                ><path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M15.75 5.25v13.5m-7.5-13.5v13.5"
                /></svg
              >
            {:else}
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="currentColor"
                ><path
                  d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.348a1.125 1.125 0 010 1.971l-11.54 6.347a1.125 1.125 0 01-1.667-.985V5.653z"
                /></svg
              >
            {/if}
          </button>
          <svg
            class="h-4 w-4 shrink-0 text-muted"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="1.5"
            ><path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M19.114 5.636a9 9 0 010 12.728M16.463 8.288a5.25 5.25 0 010 7.424M6.75 8.25l4.72-4.72a.75.75 0 011.28.53v15.88a.75.75 0 01-1.28.53l-4.72-4.72H4.51c-.88 0-1.704-.507-1.938-1.354A9.01 9.01 0 012.25 12c0-.83.112-1.633.322-2.396C2.806 8.756 3.63 8.25 4.51 8.25H6.75z"
            /></svg
          >
          <input
            type="range"
            min="0"
            max="100"
            value={spotify.volume}
            oninput={(e) => setVolumeDebounced(Number(e.currentTarget.value))}
            aria-label="Volume"
            class="min-w-0 flex-1 accent-green-600"
          />
          <span class="w-7 shrink-0 text-right text-[11px] text-muted"
            >{spotify.volume}</span
          >
        </div>

        <!-- Device -->
        <div class="mt-3 border-t border-border pt-2">
          <button
            type="button"
            onclick={toggleDevices}
            class="flex w-full items-center gap-2 rounded-lg px-1 py-1 text-left transition-colors hover:bg-hover"
            aria-expanded={devicesOpen}
          >
            <svg
              class="h-4 w-4 shrink-0 text-muted"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="1.5"
              ><path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M9 17.25v1.007a3 3 0 01-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0115 18.257V17.25m6-12V15a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 15V5.25m18 0A2.25 2.25 0 0018.75 3H5.25A2.25 2.25 0 003 5.25m18 0V12a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 12V5.25"
              /></svg
            >
            <span class="min-w-0 flex-1 truncate text-xs text-secondary">
              {activeDevice ? activeDevice.name : "No device"}
            </span>
            <svg
              class="h-3 w-3 shrink-0 text-muted transition-transform {devicesOpen
                ? 'rotate-180'
                : ''}"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
              ><path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M19 9l-7 7-7-7"
              /></svg
            >
          </button>

          {#if devicesOpen}
            {#if spotify.needsDevicePick}
              <p class="mt-1 px-1 text-[11px] text-muted">
                Open Spotify on a device, then pick it here.
              </p>
            {/if}
            <div class="mt-1 space-y-0.5">
              {#if devicesLoading}
                <p class="px-1 py-1 text-[11px] text-muted">
                  Loading devices...
                </p>
              {:else if spotify.devices.length === 0}
                <p class="px-1 py-1 text-[11px] text-muted">
                  No devices available.
                </p>
              {:else}
                {#each spotify.devices as device (device.id)}
                  <button
                    type="button"
                    onclick={() => pickDevice(device.id)}
                    disabled={busy}
                    class="flex w-full items-center gap-2 rounded-lg px-2 py-1 text-left transition-colors hover:bg-hover disabled:opacity-60 {device.id ===
                    spotify.activeDeviceId
                      ? 'bg-hover'
                      : ''}"
                  >
                    <span class="min-w-0 flex-1 truncate text-xs text-primary"
                      >{device.name}</span
                    >
                    <span class="shrink-0 text-[10px] text-muted"
                      >{device.type}</span
                    >
                  </button>
                {/each}
              {/if}
            </div>
            <button
              type="button"
              onclick={refreshDevices}
              disabled={devicesLoading}
              class="mt-1 flex items-center gap-1 rounded-lg px-1 py-1 text-[11px] text-muted transition-colors hover:text-primary disabled:opacity-60"
            >
              <svg
                class="h-3 w-3"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
                ><path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182m0-4.991v4.99"
                /></svg
              >
              Refresh
            </button>
          {/if}
        </div>

        <!-- Footer -->
        <div
          class="mt-2 flex items-center justify-end border-t border-border pt-2"
        >
          <button
            type="button"
            onclick={() => (configOpen = true)}
            class="rounded-lg p-1.5 text-muted transition-colors hover:bg-hover hover:text-primary"
            title="Configure playlists"
            aria-label="Configure playlists"
          >
            <svg
              class="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="1.5"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M10.343 3.94c.09-.542.56-.94 1.11-.94h1.093c.55 0 1.02.398 1.11.94l.149.894c.07.424.384.764.78.93.398.164.855.142 1.205-.108l.737-.527a1.125 1.125 0 011.45.12l.773.774c.39.389.44 1.002.12 1.45l-.527.737c-.25.35-.272.806-.107 1.204.165.397.505.71.93.78l.893.15c.543.09.94.56.94 1.109v1.094c0 .55-.397 1.02-.94 1.11l-.893.149c-.425.07-.765.383-.93.78-.165.398-.143.854.107 1.204l.527.738c.32.447.269 1.06-.12 1.45l-.774.773a1.125 1.125 0 01-1.449.12l-.738-.527c-.35-.25-.806-.272-1.204-.107-.397.165-.71.505-.78.929l-.15.894c-.09.542-.56.94-1.11.94h-1.094c-.55 0-1.019-.398-1.11-.94l-.148-.894c-.071-.424-.384-.764-.781-.93-.398-.164-.854-.142-1.204.108l-.738.527c-.447.32-1.06.269-1.45-.12l-.773-.774a1.125 1.125 0 01-.12-1.45l.527-.737c.25-.35.272-.806.107-1.204-.165-.397-.505-.71-.93-.78l-.894-.15c-.542-.09-.94-.56-.94-1.109v-1.094c0-.55.398-1.02.94-1.11l.894-.149c.424-.07.765-.383.93-.78.165-.398.143-.854-.107-1.204l-.527-.738a1.125 1.125 0 01.12-1.45l.773-.773a1.125 1.125 0 011.45-.12l.737.527c.35.25.807.272 1.204.107.397-.165.71-.505.78-.929l.15-.894z"
              />
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
              />
            </svg>
          </button>
        </div>
      {/if}

      {#if spotify.error}
        <div
          class="mt-2 flex items-start gap-2 rounded-lg border border-error-border bg-error-bg px-2 py-1.5"
        >
          <span class="min-w-0 flex-1 text-[11px] text-error-text">
            {errorText(spotify.error)}
          </span>
          <button
            type="button"
            onclick={clearSpotifyError}
            class="shrink-0 text-error-text/70 transition-colors hover:text-error-text"
            aria-label="Dismiss error"
          >
            <svg
              class="h-3 w-3"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
              ><path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M6 18L18 6M6 6l12 12"
              /></svg
            >
          </button>
        </div>
      {/if}
    </div>
  {/if}

  {#if configOpen}
    <SpotifyConfigModal onclose={() => (configOpen = false)} />
  {/if}
</div>
