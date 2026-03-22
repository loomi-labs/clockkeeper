<script lang="ts">
	import type { Character } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import { Team, TravellerAlignment } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import { teamCardColors, teamCardColorsUnselected, teamDataAttr } from '~/lib/team-styles';

	let {
		character,
		selected = true,
		removable = false,
		onremove,
		travellerAlignment,
		onalignmentchange
	}: {
		character: Character;
		selected?: boolean;
		removable?: boolean;
		onremove?: () => void;
		travellerAlignment?: TravellerAlignment;
		onalignmentchange?: (alignment: TravellerAlignment) => void;
	} = $props();

	const iconSuffix = $derived.by(() => {
		// For travellers with an alignment set, override the icon suffix.
		if (character.team === Team.TRAVELLER && travellerAlignment !== undefined) {
			if (travellerAlignment === TravellerAlignment.GOOD) return '_g';
			if (travellerAlignment === TravellerAlignment.EVIL) return '_e';
		}
		// Default behavior for all other teams.
		if (character.team === Team.TOWNSFOLK || character.team === Team.OUTSIDER) return '_g';
		if (character.team === Team.MINION || character.team === Team.DEMON) return '_e';
		return '';
	});
	const iconUrl = $derived(`/characters/${character.edition}/${character.id}${iconSuffix}.webp`);


	// Resolve the effective team for styling: travellers with alignment use townsfolk (good) or minion (evil) colors.
	const effectiveTeam = $derived.by(() => {
		if (character.team === Team.TRAVELLER && travellerAlignment !== undefined) {
			if (travellerAlignment === TravellerAlignment.GOOD) return Team.TOWNSFOLK;
			if (travellerAlignment === TravellerAlignment.EVIL) return Team.MINION;
		}
		return character.team;
	});

	const colorClass = $derived(
		selected
			? (teamCardColors[effectiveTeam] ?? 'border-border bg-surface-alt')
			: (teamCardColorsUnselected[effectiveTeam] ?? 'border-border/50 bg-surface-alt/30')
	);

	let imgError = $state(false);
</script>

<div class="card-slate h-full rounded-lg border p-2 transition-opacity {colorClass}" class:opacity-40={!selected} class:border-dashed={!selected} data-team={teamDataAttr[character.team] ?? ''}>
	<div class="flex items-center gap-2">
		{#if !imgError}
			<img
				src={iconUrl}
				alt={character.name}
				class="h-24 w-24 shrink-0 rounded-full"
				onerror={() => (imgError = true)}
			/>
		{:else}
			<div class="flex h-24 w-24 shrink-0 items-center justify-center rounded-full bg-element text-sm text-secondary">
				{character.name.charAt(0)}
			</div>
		{/if}
		<div class="min-w-0 flex-1">
			<div class="flex items-center gap-2">
				<span class="font-medium text-primary">{character.name}</span>
				{#if character.setup}
					<span class="rounded bg-yellow-100 px-1.5 py-0.5 text-xs text-yellow-700 dark:bg-yellow-500/20 dark:text-yellow-300">setup</span>
				{/if}
			</div>
			<p class="mt-0.5 text-sm text-secondary">{character.ability}</p>
			{#if onalignmentchange}
				<div class="mt-1.5 flex items-center gap-1.5">
					<button
						onclick={(e: MouseEvent) => { e.stopPropagation(); onalignmentchange(TravellerAlignment.GOOD); }}
						aria-label="Set good alignment"
						class="h-5 w-5 rounded-full border-2 transition-colors {travellerAlignment === TravellerAlignment.GOOD
							? 'border-blue-500 bg-blue-500'
							: 'border-blue-300 bg-transparent hover:bg-blue-100 dark:border-blue-600 dark:hover:bg-blue-900/40'}"
					></button>
					<button
						onclick={(e: MouseEvent) => { e.stopPropagation(); onalignmentchange(TravellerAlignment.UNSPECIFIED); }}
						aria-label="Clear alignment"
						class="h-5 w-5 rounded-full border-2 transition-colors {travellerAlignment === undefined || travellerAlignment === TravellerAlignment.UNSPECIFIED
							? 'border-gray-400 bg-gray-400 dark:border-gray-500 dark:bg-gray-500'
							: 'border-gray-300 bg-transparent hover:bg-gray-100 dark:border-gray-600 dark:hover:bg-gray-800'}"
					></button>
					<button
						onclick={(e: MouseEvent) => { e.stopPropagation(); onalignmentchange(TravellerAlignment.EVIL); }}
						aria-label="Set evil alignment"
						class="h-5 w-5 rounded-full border-2 transition-colors {travellerAlignment === TravellerAlignment.EVIL
							? 'border-orange-500 bg-orange-500'
							: 'border-orange-300 bg-transparent hover:bg-orange-100 dark:border-orange-600 dark:hover:bg-orange-900/40'}"
					></button>
				</div>
			{/if}
		</div>
		{#if removable && onremove}
			<button
				onclick={onremove}
				aria-label="Remove {character.name}"
				class="shrink-0 rounded p-1 text-muted transition-colors hover:bg-hover hover:text-label"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
				</svg>
			</button>
		{/if}
	</div>
</div>
