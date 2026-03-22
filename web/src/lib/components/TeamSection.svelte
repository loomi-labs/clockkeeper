<script lang="ts">
	import type { Character } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import { Team, TravellerAlignment } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import CharacterCard from './CharacterCard.svelte';
	import { teamLabels, teamSingulars, addCardColors } from '~/lib/team-styles';

	let {
		team,
		characters,
		removable = false,
		onremove,
		onadd,
		addLabel,
		selectedIds,
		onclick,
		compact = false,
		travellerAlignments,
		onalignmentchange
	}: {
		team: Team;
		characters: Character[];
		removable?: boolean;
		onremove?: (id: string) => void;
		onadd?: () => void;
		addLabel?: string;
		selectedIds?: Set<string>;
		onclick?: (id: string) => void;
		compact?: boolean;
		travellerAlignments?: { [key: string]: TravellerAlignment };
		onalignmentchange?: (roleId: string, alignment: TravellerAlignment) => void;
	} = $props();

	// TeamSection uses different gradient weights (500) for Traveller header, no dark variant.
	const teamHeaderColors: Record<number, string> = {
		[Team.TOWNSFOLK]: 'text-blue-600 dark:text-blue-400',
		[Team.OUTSIDER]: 'text-cyan-600 dark:text-cyan-400',
		[Team.MINION]: 'text-orange-600 dark:text-orange-400',
		[Team.DEMON]: 'text-red-600 dark:text-red-400',
		[Team.TRAVELLER]: 'bg-gradient-to-r from-blue-500 to-red-500 bg-clip-text text-transparent',
		[Team.FABLED]: 'text-yellow-500 dark:text-yellow-400',
		[Team.LORIC]: 'text-green-600 dark:text-green-400'
	};

	const countLabel = $derived.by(() => {
		if (selectedIds) {
			const selected = characters.filter((c) => selectedIds.has(c.id)).length;
			return `${selected}/${characters.length}`;
		}
		return `${characters.length}`;
	});

	const resolvedAddLabel = $derived(addLabel ?? `Add ${teamSingulars[team] ?? 'Character'}`);
</script>

<div class={compact ? 'h-full' : ''}>
	{#if !compact}
		<h3 class="mb-3 text-sm font-semibold uppercase tracking-wide {teamHeaderColors[team] ?? 'text-secondary'}">
			{teamLabels[team] ?? team} ({countLabel})
		</h3>
	{/if}
	<div class={compact ? 'h-full' : 'grid gap-2 sm:grid-cols-2'}>
		{#each characters as char (char.id)}
			{#if onclick}
				<button
					onclick={() => onclick(char.id)}
					class="h-full w-full text-left transition-transform active:scale-[0.98]"
				>
					<CharacterCard
						character={char}
						selected={selectedIds ? selectedIds.has(char.id) : true}
						{removable}
						onremove={onremove ? () => onremove(char.id) : undefined}
						travellerAlignment={travellerAlignments?.[char.id]}
						onalignmentchange={onalignmentchange ? (a) => onalignmentchange(char.id, a) : undefined}
					/>
				</button>
			{:else}
				<CharacterCard
					character={char}
					{removable}
					onremove={onremove ? () => onremove(char.id) : undefined}
					travellerAlignment={travellerAlignments?.[char.id]}
					onalignmentchange={onalignmentchange ? (a) => onalignmentchange(char.id, a) : undefined}
				/>
			{/if}
		{/each}
		{#if onadd}
			<button
				onclick={onadd}
				class="flex h-full w-full min-h-[4rem] items-center justify-center gap-2 rounded-lg border-2 border-dashed transition-colors {addCardColors[team] ?? 'border-border text-muted hover:bg-hover'}"
			>
				<svg class="h-5 w-5 {team === Team.TRAVELLER ? 'text-blue-400' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
				</svg>
				<span class="text-sm font-medium {team === Team.TRAVELLER ? 'text-gradient-traveller' : ''}">{resolvedAddLabel}</span>
			</button>
		{/if}
	</div>
</div>
