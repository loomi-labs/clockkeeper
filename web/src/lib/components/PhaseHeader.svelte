<script lang="ts">
  import type { Game, Phase } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";

  interface Round {
    night?: Phase;
    day?: Phase;
    roundNumber: number;
  }

  let {
    game,
    viewingRoundIndex,
    rounds,
    onadvance,
    onend,
    onnavigate,
  }: {
    game: Game;
    viewingRoundIndex: number;
    rounds: Round[];
    onadvance: () => void;
    onend: () => void;
    onnavigate: (index: number) => void;
  } = $props();

  const viewingRound = $derived(rounds[viewingRoundIndex]);
  const isViewingCurrent = $derived(viewingRoundIndex === rounds.length - 1);
  const roundNumber = $derived(viewingRound?.roundNumber ?? 1);

  const canGoBack = $derived(viewingRoundIndex > 0);
  const canGoForward = $derived(viewingRoundIndex < rounds.length - 1);
</script>

<div class="rounded-lg border border-border bg-surface p-4">
  <div class="flex items-center justify-between gap-4">
    <!-- Round display with arrow navigation -->
    <div>
      <div class="flex items-center gap-2">
        <button
          onclick={() => onnavigate(viewingRoundIndex - 1)}
          disabled={!canGoBack}
          class="rounded p-1 text-secondary transition-colors hover:bg-hover disabled:opacity-30 disabled:cursor-default"
          aria-label="Previous round"
        >
          <svg
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M15 19l-7-7 7-7"
            />
          </svg>
        </button>
        <h2 class="text-2xl font-bold text-primary">Night {roundNumber}</h2>
        <button
          onclick={() => onnavigate(viewingRoundIndex + 1)}
          disabled={!canGoForward}
          class="rounded p-1 text-secondary transition-colors hover:bg-hover disabled:opacity-30 disabled:cursor-default"
          aria-label="Next round"
        >
          <svg
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M9 5l7 7-7 7"
            />
          </svg>
        </button>
      </div>
      <!-- Round breadcrumbs (clickable) -->
      {#if rounds.length > 1}
        <div class="mt-1 flex items-center gap-1">
          {#each rounds as round, i (round.roundNumber)}
            {@const isFirst = round.roundNumber === 1}
            {@const isActive = i === viewingRoundIndex}
            <button
              onclick={() => onnavigate(i)}
              class="rounded px-1.5 py-0.5 text-xs font-medium transition-colors {isActive
                ? isFirst
                  ? 'bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-300'
                  : 'bg-indigo-100 text-indigo-700 dark:bg-indigo-500/20 dark:text-indigo-300'
                : isFirst
                  ? 'bg-amber-50 text-amber-600/70 hover:bg-amber-100 hover:text-amber-700 dark:bg-amber-500/10 dark:text-amber-400/70 dark:hover:bg-amber-500/20 dark:hover:text-amber-300'
                  : 'bg-element text-muted hover:bg-hover hover:text-medium'}"
            >
              Night {round.roundNumber}
            </button>
          {/each}
        </div>
      {/if}
    </div>

    <!-- Action buttons (only when viewing current round) -->
    {#if isViewingCurrent}
      <div class="flex items-center gap-2">
        <button
          onclick={onadvance}
          class="rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-400"
        >
          Finish Night
        </button>
        <button
          onclick={onend}
          class="rounded-lg border border-red-300 px-4 py-2 text-sm font-medium text-red-600 transition-colors hover:bg-red-50 dark:border-red-700 dark:text-red-400 dark:hover:bg-red-950/30"
        >
          End Game
        </button>
      </div>
    {:else}
      <span class="text-sm text-muted italic">Viewing past round</span>
    {/if}
  </div>
</div>
