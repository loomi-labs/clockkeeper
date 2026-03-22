<script lang="ts">
	import type { Game } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import { PhaseType } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';

	let {
		game,
		viewingPhaseIndex,
		onadvance,
		onend,
		onnavigate,
	}: {
		game: Game;
		viewingPhaseIndex: number;
		onadvance: () => void;
		onend: () => void;
		onnavigate: (index: number) => void;
	} = $props();

	const phases = $derived(game.playState?.phases ?? []);
	const viewingPhase = $derived(phases[viewingPhaseIndex]);
	const isViewingCurrent = $derived(viewingPhaseIndex === phases.length - 1);

	const phaseLabel = $derived(
		viewingPhase?.type === PhaseType.NIGHT
			? `Night ${viewingPhase?.roundNumber}`
			: `Day ${viewingPhase?.roundNumber}`
	);

	const nextPhaseLabel = $derived.by(() => {
		const current = phases[phases.length - 1];
		if (!current) return 'Next Phase';
		if (current.type === PhaseType.NIGHT) {
			return 'Start Day';
		}
		return `Start Night ${current.roundNumber + 1}`;
	});

	const canGoBack = $derived(viewingPhaseIndex > 0);
	const canGoForward = $derived(viewingPhaseIndex < phases.length - 1);

	function phaseDisplayName(type: PhaseType, round: number): string {
		if (type === PhaseType.NIGHT) return `N${round}`;
		return `D${round}`;
	}
</script>

<div class="rounded-lg border border-border bg-surface p-4">
	<div class="flex items-center justify-between gap-4">
		<!-- Phase display with arrow navigation -->
		<div>
			<div class="flex items-center gap-2">
				<button
					onclick={() => onnavigate(viewingPhaseIndex - 1)}
					disabled={!canGoBack}
					class="rounded p-1 text-secondary transition-colors hover:bg-hover disabled:opacity-30 disabled:cursor-default"
					aria-label="Previous phase"
				>
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
					</svg>
				</button>
				<h2 class="text-2xl font-bold text-primary">{phaseLabel}</h2>
				<button
					onclick={() => onnavigate(viewingPhaseIndex + 1)}
					disabled={!canGoForward}
					class="rounded p-1 text-secondary transition-colors hover:bg-hover disabled:opacity-30 disabled:cursor-default"
					aria-label="Next phase"
				>
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
					</svg>
				</button>
			</div>
			<!-- Phase history breadcrumbs (clickable) -->
			{#if phases.length > 1}
				<div class="mt-1 flex items-center gap-1">
					{#each phases as phase, i (phase.id)}
						<button
							onclick={() => onnavigate(i)}
							class="rounded px-1.5 py-0.5 text-xs font-medium transition-colors {i === viewingPhaseIndex
								? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-500/20 dark:text-indigo-300'
								: 'bg-element text-muted hover:bg-hover hover:text-medium'}"
						>
							{phaseDisplayName(phase.type, phase.roundNumber)}
						</button>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Action buttons (only when viewing current phase) -->
		{#if isViewingCurrent}
			<div class="flex items-center gap-2">
				<button
					onclick={onadvance}
					class="rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-400"
				>
					{nextPhaseLabel}
				</button>
				<button
					onclick={onend}
					class="rounded-lg border border-red-300 px-4 py-2 text-sm font-medium text-red-600 transition-colors hover:bg-red-50 dark:border-red-700 dark:text-red-400 dark:hover:bg-red-950/30"
				>
					End Game
				</button>
			</div>
		{:else}
			<span class="text-sm text-muted italic">Viewing past phase</span>
		{/if}
	</div>
</div>
