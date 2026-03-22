<script lang="ts">
	import { onMount } from 'svelte';

	import { client } from '~/lib/api';
	import { invalidateSidebar } from '~/lib/sidebar-data.svelte';
	import { GameState, PhaseType } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import type { GameSummary } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';

	const PAGE_SIZE = 5;

	let games = $state<GameSummary[]>([]);
	let loaded = $state(false);
	let showCompleted = $state(false);
	let currentPage = $state(0);
	let confirmingDelete = $state<bigint | null>(null);
	let deleting = $state(false);

	const activeGames = $derived(
		[...games]
			.filter((g) => g.state === GameState.SETUP || g.state === GameState.IN_PROGRESS)
			.sort((a, b) => {
				const order: Record<number, number> = { [GameState.IN_PROGRESS]: 0, [GameState.SETUP]: 1 };
				return (order[a.state] ?? 2) - (order[b.state] ?? 2);
			})
	);

	const completedGames = $derived(
		games.filter((g) => g.state === GameState.COMPLETED)
	);

	const displayedGames = $derived(showCompleted ? completedGames : activeGames);
	const totalPages = $derived(Math.ceil(displayedGames.length / PAGE_SIZE));
	const pagedGames = $derived(
		displayedGames.slice(currentPage * PAGE_SIZE, (currentPage + 1) * PAGE_SIZE)
	);

	function stateBadge(game: GameSummary): { label: string; classes: string } {
		switch (game.state) {
			case GameState.SETUP:
				return { label: 'Setup', classes: 'bg-yellow-100 text-yellow-700' };
			case GameState.IN_PROGRESS: {
				const phase = game.currentPhaseType === PhaseType.DAY ? 'Day' : 'Night';
				return { label: `${phase} ${game.currentRound}`, classes: 'bg-green-100 text-green-700' };
			}
			case GameState.COMPLETED:
				return { label: 'Completed', classes: 'bg-element text-muted' };
			default:
				return { label: 'Unknown', classes: 'bg-element text-muted' };
		}
	}

	function toggleShowCompleted() {
		showCompleted = !showCompleted;
		currentPage = 0;
		confirmingDelete = null;
	}

	async function deleteGame(id: bigint) {
		deleting = true;
		try {
			await client.deleteGame({ id });
			games = games.filter((g) => g.id !== id);
			confirmingDelete = null;
			invalidateSidebar();
			// Adjust page if we deleted the last item on this page.
			if (currentPage > 0 && currentPage >= Math.ceil(displayedGames.length / PAGE_SIZE)) {
				currentPage--;
			}
		} catch {
			// Silently fail — user can retry
		} finally {
			deleting = false;
		}
	}

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
	<div class="flex items-center gap-4">
		<img src="/logo.webp" alt="" class="h-16 w-16 rounded-lg" />
		<div>
			<h1 class="font-[Goudy_Stout] text-3xl text-primary">Clock Keeper</h1>
			<p class="mt-1 text-secondary">Your digital companion for Blood on the Clocktower</p>
		</div>
	</div>

	{#if loaded && games.length > 0}
		<section class="mt-8">
			<div class="flex items-center justify-between">
				<h2 class="font-[Goudy_Stout] text-base text-primary">Your Games</h2>
				{#if completedGames.length > 0}
					<button
						onclick={toggleShowCompleted}
						class="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm transition-colors
							{showCompleted ? 'bg-hover text-primary font-medium' : 'text-secondary hover:bg-hover hover:text-primary'}"
					>
						{#if showCompleted}
							<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" />
							</svg>
							Active games
						{:else}
							Past games
							<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
							</svg>
						{/if}
					</button>
				{/if}
			</div>

			{#if displayedGames.length === 0}
				<p class="mt-3 text-sm text-muted">
					{showCompleted ? 'No completed games yet.' : 'No active games.'}
				</p>
			{:else}
				<div class="mt-3 grid gap-3">
					{#each pagedGames as game (game.id)}
						{@const badge = stateBadge(game)}
						{#if confirmingDelete === game.id}
							<div class="card-slate rounded-xl border border-red-300 bg-surface p-4 dark:border-red-800">
								<div class="flex items-center justify-between">
									<span class="text-sm text-primary">Delete <strong>{game.scriptName}</strong>?</span>
									<div class="flex gap-2">
										<button
											onclick={() => (confirmingDelete = null)}
											class="rounded-lg px-3 py-1 text-sm text-secondary transition-colors hover:bg-hover hover:text-primary"
										>
											Cancel
										</button>
										<button
											onclick={() => deleteGame(game.id)}
											disabled={deleting}
											class="rounded-lg bg-red-600 px-3 py-1 text-sm text-white transition-colors hover:bg-red-700 disabled:opacity-50"
										>
											{deleting ? 'Deleting...' : 'Delete'}
										</button>
									</div>
								</div>
							</div>
						{:else}
							<div class="card-slate group relative rounded-xl border border-border bg-surface transition-all hover:border-indigo-400 hover:shadow-md">
								<a href="/games/{game.id}" class="block p-4">
									<div class="flex items-center justify-between pr-8">
										<span class="font-medium text-primary">{game.name || game.scriptName}</span>
										<span class="rounded-full px-2.5 py-0.5 text-xs font-medium {badge.classes}">{badge.label}</span>
									</div>
									{#if game.name && game.scriptName}
										<p class="mt-0.5 text-xs text-muted">{game.scriptName}</p>
									{/if}
									<div class="mt-1.5 flex gap-3 text-sm text-secondary">
										<span>{game.playerCount} players</span>
										{#if game.deathCount > 0}
											<span>&middot;</span>
											<span>{game.deathCount} {game.deathCount === 1 ? 'death' : 'deaths'}</span>
										{/if}
									</div>
								</a>
								<button
									onclick={(e) => { e.preventDefault(); e.stopPropagation(); confirmingDelete = game.id; }}
									class="absolute top-3 right-3 rounded-lg p-1.5 text-muted opacity-0 transition-all hover:bg-hover hover:text-red-500 group-hover:opacity-100"
									aria-label="Delete game"
								>
									<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
										<path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
									</svg>
								</button>
							</div>
						{/if}
					{/each}
				</div>

				{#if totalPages > 1}
					<div class="mt-4 flex items-center justify-center gap-3">
						<button
							onclick={() => currentPage--}
							disabled={currentPage === 0}
							class="rounded-lg px-3 py-1.5 text-sm text-secondary transition-colors hover:bg-hover hover:text-primary disabled:opacity-40 disabled:hover:bg-transparent"
						>
							Previous
						</button>
						<span class="text-sm text-muted">{currentPage + 1} / {totalPages}</span>
						<button
							onclick={() => currentPage++}
							disabled={currentPage >= totalPages - 1}
							class="rounded-lg px-3 py-1.5 text-sm text-secondary transition-colors hover:bg-hover hover:text-primary disabled:opacity-40 disabled:hover:bg-transparent"
						>
							Next
						</button>
					</div>
				{/if}
			{/if}
		</section>
	{/if}

	<div class="mt-8 grid gap-4">
		<a
			href="/games/new"
			class="card-slate group rounded-xl border border-border border-l-4 border-l-indigo-500 bg-surface p-8 transition-all hover:border-indigo-400 hover:border-l-indigo-500 hover:shadow-md"
		>
			<svg class="mb-3 h-10 w-10 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path stroke-linecap="round" stroke-linejoin="round" d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.347a1.125 1.125 0 010 1.972l-11.54 6.347a1.125 1.125 0 01-1.667-.986V5.653z" />
			</svg>
			<h2 class="font-[Goudy_Stout] text-base text-primary">New Game</h2>
			<p class="mt-1 text-sm text-secondary">Start a new game session</p>
		</a>

		<a
			href="/scripts"
			class="card-slate group rounded-xl border border-border bg-surface p-6 transition-all hover:border-indigo-400 hover:shadow-md"
		>
			<svg class="mb-3 h-8 w-8 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25" />
			</svg>
			<h2 class="font-[Goudy_Stout] text-base text-primary">Scripts</h2>
			<p class="mt-1 text-sm text-secondary">View editions, create or import scripts</p>
		</a>
		<a
			href="/almanac"
			class="card-slate group rounded-xl border border-border bg-surface p-6 transition-all hover:border-indigo-400 hover:shadow-md"
		>
			<svg class="mb-3 h-8 w-8 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25M15 6.75a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm-6 0a.75.75 0 11-1.5 0 .75.75 0 011.5 0z" />
			</svg>
			<h2 class="font-[Goudy_Stout] text-base text-primary">Almanac</h2>
			<p class="mt-1 text-sm text-secondary">Browse all characters, abilities, and night info</p>
		</a>
	</div>
</div>
