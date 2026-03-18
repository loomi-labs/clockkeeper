<script lang="ts">
	import type { Character } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import { Team } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import CharacterCard from './CharacterCard.svelte';

	let {
		team,
		characters,
		removable = false,
		onremove
	}: {
		team: Team;
		characters: Character[];
		removable?: boolean;
		onremove?: (id: string) => void;
	} = $props();

	const teamLabels: Record<number, string> = {
		[Team.TOWNSFOLK]: 'Townsfolk',
		[Team.OUTSIDER]: 'Outsiders',
		[Team.MINION]: 'Minions',
		[Team.DEMON]: 'Demons',
		[Team.TRAVELLER]: 'Travellers',
		[Team.FABLED]: 'Fabled'
	};

	const teamHeaderColors: Record<number, string> = {
		[Team.TOWNSFOLK]: 'text-blue-400',
		[Team.OUTSIDER]: 'text-cyan-400',
		[Team.MINION]: 'text-orange-400',
		[Team.DEMON]: 'text-red-400',
		[Team.TRAVELLER]: 'text-yellow-400',
		[Team.FABLED]: 'text-amber-400'
	};
</script>

<div>
	<h3 class="mb-3 text-sm font-semibold uppercase tracking-wide {teamHeaderColors[team] ?? 'text-gray-400'}">
		{teamLabels[team] ?? team} ({characters.length})
	</h3>
	<div class="grid gap-2 sm:grid-cols-2">
		{#each characters as char (char.id)}
			<CharacterCard
				character={char}
				{removable}
				onremove={onremove ? () => onremove(char.id) : undefined}
			/>
		{/each}
	</div>
</div>
