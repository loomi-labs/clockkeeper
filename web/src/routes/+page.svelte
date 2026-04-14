<script lang="ts">
  import { onMount } from "svelte";

  import { client } from "~/lib/api";
  import { GameState } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import type { GameSummary } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import GamesList from "~/lib/components/GamesList.svelte";

  let games = $state<GameSummary[]>([]);
  let loaded = $state(false);
  let showCompleted = $state(false);

  const activeGames = $derived(
    [...games]
      .filter(
        (g) => g.state === GameState.SETUP || g.state === GameState.IN_PROGRESS,
      )
      .sort((a, b) => {
        const order: Record<number, number> = {
          [GameState.IN_PROGRESS]: 0,
          [GameState.SETUP]: 1,
        };
        return (order[a.state] ?? 2) - (order[b.state] ?? 2);
      }),
  );

  const completedGames = $derived(
    games.filter((g) => g.state === GameState.COMPLETED),
  );

  const displayedGames = $derived(showCompleted ? completedGames : activeGames);

  onMount(async () => {
    try {
      const resp = await client.listGames({});
      games = resp.games;
    } catch {
      // Silently fail — games section simply won't appear
    } finally {
      loaded = true;
    }
  });
</script>

<div class="mx-auto max-w-2xl py-12">
  <div class="flex flex-col items-center text-center">
    <img src="/logo.webp" alt="" class="h-56 w-56 rounded-xl" />
    <h1 class="mt-4 font-[Goudy_Stout] text-3xl text-[#8b1520] dark:text-[#e04e5e]">Clock Keeper</h1>
    <p class="mt-1 text-secondary">
      Your digital companion for Blood on the Clocktower
    </p>
  </div>

  {#if loaded && games.length > 0}
    <section class="mt-8">
      <div class="flex items-center justify-between">
        <h2 class="font-[Goudy_Stout] text-base text-primary">Your Games</h2>
        {#if completedGames.length > 0}
          <button
            onclick={() => (showCompleted = !showCompleted)}
            class="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm transition-colors
							{showCompleted
              ? 'bg-hover text-primary font-medium'
              : 'text-secondary hover:bg-hover hover:text-primary'}"
          >
            {#if showCompleted}
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
                  d="M15.75 19.5L8.25 12l7.5-7.5"
                />
              </svg>
              Active games
            {:else}
              Past games
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
                  d="M8.25 4.5l7.5 7.5-7.5 7.5"
                />
              </svg>
            {/if}
          </button>
        {/if}
      </div>

      {#if displayedGames.length === 0}
        <p class="mt-3 text-sm text-muted">
          {showCompleted ? "No completed games yet." : "No active games."}
        </p>
      {:else}
        <div class="mt-3">
          <GamesList games={displayedGames} ondeleted={(id) => (games = games.filter((g) => g.id !== id))} />
        </div>
      {/if}
    </section>
  {/if}

  <div class="mt-8 grid gap-4">
    <a
      href="/games"
      class="card-slate group rounded-xl border border-border border-l-4 border-l-indigo-500 bg-surface p-8 transition-all hover:border-indigo-400 hover:border-l-indigo-500 hover:shadow-md"
    >
      <svg
        class="mb-3 h-10 w-10 text-indigo-500"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="1.5"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.347a1.125 1.125 0 010 1.972l-11.54 6.347a1.125 1.125 0 01-1.667-.986V5.653z"
        />
      </svg>
      <h2 class="font-[Goudy_Stout] text-base text-primary">New Game</h2>
      <p class="mt-1 text-sm text-secondary">Start a new game session</p>
    </a>

    <a
      href="/scripts"
      class="card-slate group rounded-xl border border-border bg-surface p-6 transition-all hover:border-indigo-400 hover:shadow-md"
    >
      <svg
        class="mb-3 h-8 w-8 text-indigo-500"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="1.5"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25"
        />
      </svg>
      <h2 class="font-[Goudy_Stout] text-base text-primary">Scripts</h2>
      <p class="mt-1 text-sm text-secondary">
        View editions, create or import scripts
      </p>
    </a>
    <a
      href="/almanac"
      class="card-slate group rounded-xl border border-border bg-surface p-6 transition-all hover:border-indigo-400 hover:shadow-md"
    >
      <svg
        class="mb-3 h-8 w-8 text-indigo-500"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="1.5"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25M15 6.75a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm-6 0a.75.75 0 11-1.5 0 .75.75 0 011.5 0z"
        />
      </svg>
      <h2 class="font-[Goudy_Stout] text-base text-primary">Almanac</h2>
      <p class="mt-1 text-sm text-secondary">
        Browse all characters, abilities, and night info
      </p>
    </a>
  </div>
</div>
