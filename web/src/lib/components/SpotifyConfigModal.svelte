<script lang="ts">
  import { getErrorMessage } from "~/lib/errors";
  import {
    spotify,
    listPlaylists,
    savePlaylists,
    disconnect,
    type PlaylistRef,
    type SlotKey,
  } from "~/lib/spotify.svelte";

  let { onclose }: { onclose: () => void } = $props();

  const rows: { slot: SlotKey; label: string; hint: string }[] = [
    { slot: "day", label: "Day", hint: "Plays during the day phase" },
    { slot: "night", label: "Night", hint: "Plays during the night phase" },
    {
      slot: "nominations",
      label: "Nominations",
      hint: "Manual only — never auto-selected",
    },
  ];

  // Edits stay local until Save; cancel/escape simply drops this draft.
  let draft = $state<Record<SlotKey, PlaylistRef | null>>({
    day: spotify.playlists.day,
    night: spotify.playlists.night,
    nominations: spotify.playlists.nominations,
  });

  let chooserSlot = $state<SlotKey | null>(null);
  let playlists = $state<PlaylistRef[]>([]);
  let playlistsLoaded = $state(false);
  let loading = $state(false);
  let filter = $state("");
  let saving = $state(false);
  let error = $state("");
  let confirmDisconnect = $state(false);
  let disconnecting = $state(false);

  const filtered = $derived.by(() => {
    const needle = filter.trim().toLowerCase();
    if (!needle) return playlists;
    return playlists.filter((p) => p.name.toLowerCase().includes(needle));
  });

  async function openChooser(slot: SlotKey) {
    if (chooserSlot === slot) {
      chooserSlot = null;
      return;
    }
    chooserSlot = slot;
    filter = "";
    if (playlistsLoaded || loading) return;
    loading = true;
    error = "";
    try {
      playlists = await listPlaylists();
      playlistsLoaded = true;
    } catch (err) {
      error = getErrorMessage(err, "Could not load your Spotify playlists");
    } finally {
      loading = false;
    }
  }

  function assign(slot: SlotKey, playlist: PlaylistRef) {
    draft[slot] = playlist;
    chooserSlot = null;
    filter = "";
  }

  function clearSlot(slot: SlotKey) {
    draft[slot] = null;
  }

  async function save() {
    saving = true;
    error = "";
    try {
      await savePlaylists(draft);
      onclose();
    } catch (err) {
      error = getErrorMessage(err, "Could not save your playlists");
    } finally {
      saving = false;
    }
  }

  async function doDisconnect() {
    disconnecting = true;
    error = "";
    try {
      await disconnect();
      onclose();
    } catch (err) {
      error = getErrorMessage(err, "Could not disconnect Spotify");
    } finally {
      disconnecting = false;
    }
  }

  // The panel that owns this modal lives inside the game header, whose setup
  // branch is `sticky ... z-10` — a stacking context that would trap a `fixed`
  // overlay underneath the nav (z-40) and sidebar (z-30). Escaping to <body>
  // is the only way the backdrop can dim them. Svelte 5 attaches delegated
  // listeners to both the mount container and `document`, so onclick handlers
  // keep working after the node is moved outside the app root.
  function portalToBody(node: HTMLElement) {
    document.body.appendChild(node);
    return () => node.remove();
  }

  let closeBtn: HTMLButtonElement | undefined = $state();

  // Move focus into the dialog when it opens, so Escape/Tab act on the modal
  // rather than the panel button that launched it.
  $effect(() => {
    closeBtn?.focus();
  });

  // No stopPropagation here: this listener is on window, where sibling window
  // listeners fire regardless. The panel not closing behind this modal is the
  // work of its own `!configOpen` guard in handleKeydown.
  function handleKeydown(e: KeyboardEvent) {
    if (e.key !== "Escape") return;
    if (chooserSlot) {
      chooserSlot = null;
    } else if (confirmDisconnect) {
      confirmDisconnect = false;
    } else {
      onclose();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div
  {@attach portalToBody}
  class="fixed inset-0 z-[60] flex items-center justify-center"
>
  <!-- Backdrop -->
  <button
    type="button"
    tabindex="-1"
    class="absolute inset-0 bg-black/50"
    onclick={onclose}
    aria-label="Close"
  ></button>

  <div
    role="dialog"
    aria-modal="true"
    aria-label="Spotify playlists"
    class="relative z-10 mx-4 flex max-h-[85vh] w-full max-w-md flex-col rounded-xl border border-border bg-surface p-6 shadow-2xl"
  >
    <div class="flex items-center justify-between">
      <h2 class="text-lg font-semibold text-primary">Phase Music</h2>
      <button
        type="button"
        bind:this={closeBtn}
        onclick={onclose}
        class="rounded-lg p-1 text-muted transition-colors hover:bg-hover hover:text-primary"
        aria-label="Close"
      >
        <svg
          class="h-5 w-5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M6 18L18 6M6 6l12 12"
          />
        </svg>
      </button>
    </div>

    {#if spotify.displayName}
      <p class="mt-1 text-xs text-muted">
        Connected as {spotify.displayName}
      </p>
    {/if}

    {#if error}
      <div
        class="mt-3 rounded-lg border border-error-border bg-error-bg px-3 py-2 text-sm text-error-text"
      >
        {error}
      </div>
    {/if}

    <div class="mt-4 min-h-0 flex-1 space-y-2 overflow-y-auto">
      {#each rows as row (row.slot)}
        {@const assigned = draft[row.slot]}
        <div class="rounded-lg border border-border p-3">
          <div class="flex items-center gap-3">
            {#if assigned?.imageUrl}
              <img
                src={assigned.imageUrl}
                alt=""
                class="h-10 w-10 shrink-0 rounded object-cover"
              />
            {:else}
              <div
                class="flex h-10 w-10 shrink-0 items-center justify-center rounded bg-element text-muted"
              >
                <svg
                  class="h-5 w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="1.5"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M9 9l10.5-3m0 6.553v3.75a2.25 2.25 0 01-1.632 2.163l-1.32.377a1.803 1.803 0 11-.99-3.467l2.31-.66a2.25 2.25 0 001.632-2.163zm0 0V2.25L9 5.25v10.303m0 0v3.75a2.25 2.25 0 01-1.632 2.163l-1.32.377a1.803 1.803 0 01-.99-3.467l2.31-.66A2.25 2.25 0 009 15.553z"
                  />
                </svg>
              </div>
            {/if}
            <div class="min-w-0 flex-1">
              <div class="text-sm font-medium text-primary">{row.label}</div>
              {#if assigned}
                <div class="truncate text-xs text-secondary">
                  {assigned.name}
                </div>
              {:else}
                <div class="text-xs text-muted">Not set</div>
              {/if}
            </div>
            <div class="flex shrink-0 items-center gap-1">
              <button
                type="button"
                onclick={() => openChooser(row.slot)}
                class="rounded-lg border border-border px-2 py-1 text-xs font-medium text-secondary transition-colors hover:bg-hover hover:text-primary"
              >
                {chooserSlot === row.slot ? "Close" : "Change"}
              </button>
              <button
                type="button"
                onclick={() => clearSlot(row.slot)}
                disabled={!assigned}
                class="rounded-lg border border-border px-2 py-1 text-xs font-medium text-muted transition-colors hover:text-red-500 disabled:opacity-40"
              >
                Clear
              </button>
            </div>
          </div>
          <p class="mt-1 text-[11px] text-muted">{row.hint}</p>

          {#if chooserSlot === row.slot}
            <div class="mt-3 border-t border-border pt-3">
              <input
                type="text"
                bind:value={filter}
                placeholder="Filter playlists..."
                aria-label="Filter playlists"
                class="w-full rounded-lg border border-border bg-transparent px-3 py-1.5 text-sm text-primary outline-none focus:border-indigo-500"
              />
              {#if loading}
                <p class="mt-3 text-sm text-secondary">Loading playlists...</p>
              {:else if playlists.length === 0}
                <p class="mt-3 text-sm text-muted">No playlists found.</p>
              {:else if filtered.length === 0}
                <p class="mt-3 text-sm text-muted">No playlists match.</p>
              {:else}
                <div class="mt-2 max-h-48 space-y-0.5 overflow-y-auto">
                  {#each filtered as playlist (playlist.uri)}
                    <button
                      type="button"
                      onclick={() => assign(row.slot, playlist)}
                      class="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left transition-colors hover:bg-hover"
                    >
                      {#if playlist.imageUrl}
                        <img
                          src={playlist.imageUrl}
                          alt=""
                          class="h-7 w-7 shrink-0 rounded object-cover"
                        />
                      {:else}
                        <div class="h-7 w-7 shrink-0 rounded bg-element"></div>
                      {/if}
                      <span class="truncate text-sm text-primary"
                        >{playlist.name}</span
                      >
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
          {/if}
        </div>
      {/each}
    </div>

    <div class="mt-4 flex items-center justify-end gap-2">
      <button
        type="button"
        onclick={onclose}
        class="rounded-lg border border-border px-4 py-2 text-sm font-medium text-secondary transition-colors hover:bg-hover hover:text-medium"
      >
        Cancel
      </button>
      <button
        type="button"
        onclick={save}
        disabled={saving}
        class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-500 disabled:opacity-50"
      >
        {saving ? "Saving..." : "Save"}
      </button>
    </div>

    <div class="mt-4 border-t border-border pt-3">
      {#if confirmDisconnect}
        <p class="text-xs text-secondary">
          Disconnect Spotify? Your saved playlists will be forgotten.
        </p>
        <div class="mt-2 flex items-center gap-2">
          <button
            type="button"
            onclick={() => (confirmDisconnect = false)}
            class="rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-secondary transition-colors hover:bg-hover hover:text-medium"
          >
            Keep connected
          </button>
          <button
            type="button"
            onclick={doDisconnect}
            disabled={disconnecting}
            class="rounded-lg bg-red-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-red-500 disabled:opacity-50"
          >
            {disconnecting ? "Disconnecting..." : "Yes, disconnect"}
          </button>
        </div>
      {:else}
        <button
          type="button"
          onclick={() => (confirmDisconnect = true)}
          class="text-xs font-medium text-muted transition-colors hover:text-red-500"
        >
          Disconnect Spotify
        </button>
      {/if}
    </div>
  </div>
</div>
