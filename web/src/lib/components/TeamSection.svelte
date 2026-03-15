<script lang="ts">
	import type { Character } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import CharacterCard from './CharacterCard.svelte';

	let {
		team,
		characters,
		removable = false,
		onremove
	}: {
		team: string;
		characters: Character[];
		removable?: boolean;
		onremove?: (id: string) => void;
	} = $props();

	const teamLabels: Record<string, string> = {
		townsfolk: 'Townsfolk',
		outsider: 'Outsiders',
		minion: 'Minions',
		demon: 'Demons',
		traveller: 'Travellers',
		fabled: 'Fabled'
	};

	const teamHeaderColors: Record<string, string> = {
		townsfolk: 'text-blue-400',
		outsider: 'text-cyan-400',
		minion: 'text-orange-400',
		demon: 'text-red-400',
		traveller: 'text-yellow-400',
		fabled: 'text-amber-400'
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
