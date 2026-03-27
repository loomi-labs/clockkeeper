<script lang="ts">
  import type { Game, Phase } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import type { Snippet } from "svelte";

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
    activeView = "nightsheet",
    onviewchange,
    isFullscreen = false,
    ontogglefullscreen,
  }: {
    game: Game;
    viewingRoundIndex: number;
    rounds: Round[];
    onadvance: () => void;
    onend: () => void;
    onnavigate: (index: number) => void;
    activeView?: "nightsheet" | "grimoire";
    onviewchange?: (view: "nightsheet" | "grimoire") => void;
    isFullscreen?: boolean;
    ontogglefullscreen?: () => void;
  } = $props();

  const viewingRound = $derived(rounds[viewingRoundIndex]);
  const isViewingCurrent = $derived(viewingRoundIndex === rounds.length - 1);
  const roundNumber = $derived(viewingRound?.roundNumber ?? 1);

  const canGoBack = $derived(viewingRoundIndex > 0);
  const canGoForward = $derived(viewingRoundIndex < rounds.length - 1);

  let breadcrumbsEl: HTMLDivElement | undefined = $state();
  let headerHidden = $state(false);

  $effect(() => {
    if (!breadcrumbsEl) return;
    const activeEl = breadcrumbsEl.querySelector<HTMLButtonElement>(
      `button[data-round-index="${viewingRoundIndex}"]`,
    );
    activeEl?.scrollIntoView({ behavior: "smooth", block: "nearest", inline: "nearest" });
  });

  $effect(() => {
    if (!isFullscreen) headerHidden = false;
  });
</script>

{#if isFullscreen && headerHidden}
  <button
    onclick={() => (headerHidden = false)}
    class="no-print rounded-lg border border-border bg-surface/80 p-1.5 text-secondary transition-colors hover:bg-hover hover:text-medium"
    title="Show header"
    aria-label="Show header"
  >
    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
    </svg>
  </button>
{:else}
<div class="no-print rounded-lg border border-border bg-surface p-3 sm:p-4">
  <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
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
        <h2 class="text-xl font-bold text-primary sm:text-2xl">Night {roundNumber}</h2>
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
        <div bind:this={breadcrumbsEl} class="mt-1 flex items-center gap-1 overflow-x-auto [scrollbar-width:none]">
          {#each rounds as round, i (round.roundNumber)}
            {@const isFirst = round.roundNumber === 1}
            {@const isActive = i === viewingRoundIndex}
            <button
              data-round-index={i}
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

    <!-- Right side: view toggle + actions -->
    <div class="flex flex-wrap items-center gap-2 sm:gap-3">
      <!-- View toggle: Night Sheet / Grimoire + Fullscreen -->
      {#if onviewchange}
        <div class="flex items-center gap-1.5">
          <div class="flex gap-1 rounded-lg bg-element p-1">
            <button
              onclick={() => onviewchange?.("nightsheet")}
              class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors {activeView ===
              'nightsheet'
                ? 'bg-surface text-primary shadow-sm'
                : 'text-secondary hover:text-medium'}"
            >
              Night Sheet
            </button>
            <button
              onclick={() => onviewchange?.("grimoire")}
              class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors {activeView ===
              'grimoire'
                ? 'bg-surface text-primary shadow-sm'
                : 'text-secondary hover:text-medium'}"
            >
              Grimoire
            </button>
          </div>
          {#if isFullscreen}
            <button
              onclick={() => (headerHidden = true)}
              class="rounded-lg border border-border p-2 text-secondary transition-colors hover:bg-hover hover:text-medium"
              title="Hide header"
              aria-label="Hide header"
            >
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M5 15l7-7 7 7" />
              </svg>
            </button>
          {/if}
          {#if ontogglefullscreen}
            <button
              onclick={ontogglefullscreen}
              class="rounded-lg border border-border p-2 text-secondary transition-colors hover:bg-hover hover:text-medium"
              title={isFullscreen ? "Exit fullscreen" : "Fullscreen"}
            >
              {#if isFullscreen}
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
                    d="M9 9V4.5M9 9H4.5M9 9L3.75 3.75M9 15v4.5M9 15H4.5M9 15l-5.25 5.25M15 9h4.5M15 9V4.5M15 9l5.25-5.25M15 15h4.5M15 15v4.5m0-4.5l5.25 5.25"
                  />
                </svg>
              {:else}
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
                    d="M3.75 3.75v4.5m0-4.5h4.5m-4.5 0L9 9M3.75 20.25v-4.5m0 4.5h4.5m-4.5 0L9 15M20.25 3.75h-4.5m4.5 0v4.5m0-4.5L15 9m5.25 11.25h-4.5m4.5 0v-4.5m0 4.5L15 15"
                  />
                </svg>
              {/if}
            </button>
          {/if}
        </div>
      {/if}

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
</div>
{/if}
