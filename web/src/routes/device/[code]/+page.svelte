<script lang="ts">
  // The tablet passed around the table, for players without a phone.
  //
  // Public route, no credential: the shared code in the URL is the authority.
  // Every reveal is a one-shot call whose payload lives in a local variable and
  // is thrown away on "Done" or when the countdown runs out — nothing that
  // identifies a character may survive for the next person who picks this up.
  import { onMount } from "svelte";
  import { page } from "$app/state";
  import ConfirmDialog from "~/lib/components/ConfirmDialog.svelte";
  import RoleCard from "~/lib/components/tokenbag/RoleCard.svelte";
  import { createCountdown } from "~/lib/countdown";
  import type { Character } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { TokenBagPhase } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { initTheme } from "~/lib/theme";
  import type { BagPlayer } from "~/lib/tokenbag";
  import { createDeviceBag } from "~/lib/tokenbag.svelte";
  import { deriveDeviceView } from "~/lib/tokenbag-views";

  /** How long a revealed character stays on screen before it hides itself. */
  const REVEAL_SECONDS = 45;

  const code = page.params.code ?? "";
  const bag = createDeviceBag(code);

  let nameInput = $state("");
  let adding = $state(false);
  /** Tapped a name, waiting for the confirm. */
  let pending = $state<BagPlayer | null>(null);
  let revealing = $state(false);
  /** The only place the revealed character exists on this device. */
  let shown = $state<{ name: string; character: Character } | null>(null);
  let remaining = $state(0);

  const countdown = createCountdown({
    seconds: REVEAL_SECONDS,
    onTick: (value) => (remaining = value),
    onExpire: () => hideRole(),
  });

  function hideRole() {
    countdown.stop();
    shown = null;
  }

  onMount(() => {
    initTheme();
  });

  $effect(() => {
    bag.start();
    return () => {
      bag.stop();
      // Drop the character with the page, not just the timer: nothing about a
      // reveal may outlive the screen that showed it.
      hideRole();
    };
  });

  const view = $derived(
    deriveDeviceView(bag.state.phase, bag.state.status, bag.state.gameStarted),
  );

  // A reset or reopened bag must not leave a character on screen — and neither
  // may a game that has just started, which leaves the phase on REVEALED.
  $effect(() => {
    if (bag.state.gameStarted || bag.state.phase !== TokenBagPhase.REVEALED) {
      hideRole();
      // The confirm dialog goes with it: the server would refuse the reveal
      // behind it anyway.
      pending = null;
    }
  });

  async function addName(event: SubmitEvent) {
    event.preventDefault();
    const name = nameInput.trim();
    if (name === "" || adding) return;
    adding = true;
    const id = await bag.addName(name);
    adding = false;
    if (id) nameInput = "";
  }

  async function confirmReveal() {
    const target = pending;
    pending = null;
    if (!target || revealing) return;
    revealing = true;
    const resp = await bag.revealFor(target.id);
    revealing = false;
    if (!resp?.character) return;
    shown = { name: resp.name || target.name, character: resp.character };
    countdown.start();
  }
</script>

<svelte:head>
  <title>Shared Device — Clock Keeper</title>
</svelte:head>

{#snippet errorBox()}
  {#if bag.state.error}
    <p
      class="rounded-lg border border-error-border bg-error-bg px-3 py-2 text-sm text-error-text"
    >
      {bag.state.error}
    </p>
  {/if}
{/snippet}

{#snippet nameList(players: BagPlayer[])}
  {#if players.length === 0}
    <p class="text-sm text-muted">Nobody has joined yet.</p>
  {:else}
    <ul class="divide-y divide-border">
      {#each players as player (player.id)}
        <li class="flex items-center gap-2 py-2 text-sm text-primary">
          <span class="min-w-0 flex-1 truncate">{player.name}</span>
          {#if player.viaSharedDevice}
            <!-- Added here rather than from the player's own phone. -->
            <svg
              class="h-4 w-4 shrink-0 text-muted"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
              aria-label="Added on this device"
              role="img"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M9.75 17h4.5m-6.75 4h9a1.5 1.5 0 001.5-1.5v-15A1.5 1.5 0 0016.5 3h-9A1.5 1.5 0 006 4.5v15A1.5 1.5 0 007.5 21z"
              />
            </svg>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
{/snippet}

<div class="min-h-dvh bg-surface-alt text-primary">
  {#if bag.state.status === "reconnecting"}
    <div
      class="sticky top-0 z-40 bg-yellow-100 px-4 py-1 text-center text-xs font-medium text-yellow-800 dark:bg-yellow-500/20 dark:text-yellow-200"
    >
      Reconnecting…
    </div>
  {/if}

  <div class="mx-auto flex min-h-dvh w-full max-w-xl flex-col gap-6 p-4 pt-8">
    <header class="text-center">
      <h1 class="text-xl font-bold text-primary">Shared Device</h1>
      {#if bag.state.gameName}
        <p class="mt-0.5 text-xs text-secondary">{bag.state.gameName}</p>
      {/if}
    </header>

    <main
      class="card-slate flex flex-1 flex-col rounded-xl bg-surface p-5 shadow-sm"
    >
      {#if shown}
        <div class="flex flex-1 flex-col items-center justify-between gap-6">
          <p class="text-sm font-medium text-secondary">{shown.name}</p>
          <RoleCard character={shown.character} />
          <div class="flex w-full flex-col items-center gap-3">
            <p
              class="flex h-12 w-12 items-center justify-center rounded-full border-2 border-border text-base font-semibold text-secondary"
            >
              <span aria-hidden="true">{remaining}</span>
              <span class="sr-only">Hides in {remaining} seconds</span>
            </p>
            <button
              type="button"
              onclick={hideRole}
              class="w-full rounded-lg bg-indigo-600 px-4 py-3 text-base font-medium text-white transition-colors hover:bg-indigo-500"
            >
              Done
            </button>
          </div>
        </div>
      {:else if view === "loading"}
        <div class="flex flex-1 flex-col items-center justify-center gap-3">
          <div
            class="h-8 w-8 animate-spin rounded-full border-2 border-border border-t-indigo-500"
          ></div>
          <p class="text-sm text-secondary">Connecting…</p>
        </div>
      {:else if view === "add_names"}
        <div class="space-y-4">
          <p class="text-sm text-secondary">
            Add players without a phone. They come back to this device for the
            reveal.
          </p>
          <form class="flex gap-2" onsubmit={addName}>
            <input
              type="text"
              maxlength="50"
              bind:value={nameInput}
              placeholder="Player name"
              aria-label="Player name"
              class="min-w-0 flex-1 rounded-lg border border-border bg-surface px-3 py-2.5 text-base text-primary placeholder:text-muted"
            />
            <button
              type="submit"
              disabled={adding || nameInput.trim() === ""}
              class="shrink-0 rounded-lg bg-indigo-600 px-4 py-2.5 text-base font-medium text-white transition-colors hover:bg-indigo-500 disabled:opacity-50"
            >
              {adding ? "Adding…" : "Add player"}
            </button>
          </form>
          {@render errorBox()}
          {@render nameList(bag.state.players)}
        </div>
      {:else if view === "closed"}
        <div class="space-y-4">
          <p class="text-sm text-secondary">
            Registration closed — waiting for the reveal.
          </p>
          {@render errorBox()}
          {@render nameList(bag.state.players)}
        </div>
      {:else if view === "reveal_list"}
        <div class="space-y-4">
          <p class="text-sm text-secondary">Tap your name to see your role.</p>
          {@render errorBox()}
          {#if bag.state.players.length === 0}
            <p class="text-sm text-muted">Nobody joined this game.</p>
          {:else}
            <div class="grid grid-cols-2 gap-3">
              {#each bag.state.players as player (player.id)}
                <button
                  type="button"
                  disabled={revealing}
                  onclick={() => (pending = player)}
                  class="min-h-16 rounded-lg border border-border bg-surface-alt px-3 py-4 text-base font-medium text-primary transition-colors hover:bg-hover disabled:opacity-50"
                >
                  <span class="line-clamp-2 break-words">{player.name}</span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {:else if view === "game_started"}
        <div class="flex flex-1 flex-col items-center justify-center gap-3">
          <p class="text-lg font-semibold text-primary">The game has started</p>
          <p class="text-center text-sm text-secondary">
            Everyone's role stays with them — good luck!
          </p>
        </div>
      {:else}
        <div class="flex flex-1 flex-col items-center justify-center gap-2">
          <p class="text-center text-base text-primary">
            This game's token bag is closed.
          </p>
          <p class="text-center text-xs text-muted">
            Ask the Storyteller for a fresh link.
          </p>
        </div>
      {/if}
    </main>
  </div>
</div>

{#if pending}
  <ConfirmDialog
    title="You are {pending.name}"
    message="Reveal this role? Make sure nobody else can see the screen."
    confirmLabel="Reveal"
    oncancel={() => (pending = null)}
    onconfirm={confirmReveal}
  />
{/if}
