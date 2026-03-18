<script lang="ts">
	import type { Character } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import { Team } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';

	let {
		character,
		selected = true,
		removable = false,
		onremove
	}: {
		character: Character;
		selected?: boolean;
		removable?: boolean;
		onremove?: () => void;
	} = $props();

	const iconSuffix = $derived(
		character.team === Team.TOWNSFOLK || character.team === Team.OUTSIDER ? '_g' :
		character.team === Team.MINION || character.team === Team.DEMON ? '_e' : ''
	);
	const iconUrl = $derived(`/characters/${character.edition}/${character.id}${iconSuffix}.webp`);

	const teamColors: Record<number, string> = {
		[Team.TOWNSFOLK]: 'border-blue-500/40 bg-blue-950/30',
		[Team.OUTSIDER]: 'border-cyan-500/40 bg-cyan-950/30',
		[Team.MINION]: 'border-orange-500/40 bg-orange-950/30',
		[Team.DEMON]: 'border-red-500/40 bg-red-950/30',
		[Team.TRAVELLER]: 'border-yellow-500/40 bg-yellow-950/30',
		[Team.FABLED]: 'border-amber-500/40 bg-amber-950/30'
	};

	const unselectedColors: Record<number, string> = {
		[Team.TOWNSFOLK]: 'border-blue-500/20 bg-blue-950/10',
		[Team.OUTSIDER]: 'border-cyan-500/20 bg-cyan-950/10',
		[Team.MINION]: 'border-orange-500/20 bg-orange-950/10',
		[Team.DEMON]: 'border-red-500/20 bg-red-950/10',
		[Team.TRAVELLER]: 'border-yellow-500/20 bg-yellow-950/10',
		[Team.FABLED]: 'border-amber-500/20 bg-amber-950/10'
	};

	const teamBadgeColors: Record<number, string> = {
		[Team.TOWNSFOLK]: 'bg-blue-500/20 text-blue-300',
		[Team.OUTSIDER]: 'bg-cyan-500/20 text-cyan-300',
		[Team.MINION]: 'bg-orange-500/20 text-orange-300',
		[Team.DEMON]: 'bg-red-500/20 text-red-300',
		[Team.TRAVELLER]: 'bg-yellow-500/20 text-yellow-300',
		[Team.FABLED]: 'bg-amber-500/20 text-amber-300'
	};

	const teamNames: Record<number, string> = {
		[Team.TOWNSFOLK]: 'Townsfolk',
		[Team.OUTSIDER]: 'Outsider',
		[Team.MINION]: 'Minion',
		[Team.DEMON]: 'Demon',
		[Team.TRAVELLER]: 'Traveller',
		[Team.FABLED]: 'Fabled'
	};

	const colorClass = $derived(
		selected
			? (teamColors[character.team] ?? 'border-gray-700 bg-gray-800')
			: (unselectedColors[character.team] ?? 'border-gray-700/50 bg-gray-800/30')
	);

	let imgError = $state(false);
</script>

<div class="rounded-lg border p-3 transition-opacity {colorClass}" class:opacity-40={!selected} class:border-dashed={!selected}>
	<div class="flex items-start gap-3">
		{#if !imgError}
			<img
				src={iconUrl}
				alt={character.name}
				class="h-14 w-14 shrink-0 rounded-full"
				onerror={() => (imgError = true)}
			/>
		{:else}
			<div class="flex h-14 w-14 shrink-0 items-center justify-center rounded-full bg-gray-700 text-sm text-gray-400">
				{character.name.charAt(0)}
			</div>
		{/if}
		<div class="min-w-0 flex-1">
			<div class="flex items-center gap-2">
				<span class="font-medium text-white">{character.name}</span>
				<span class="rounded px-1.5 py-0.5 text-xs {teamBadgeColors[character.team] ?? 'bg-gray-700 text-gray-300'}">
					{teamNames[character.team] ?? 'Unknown'}
				</span>
				{#if character.setup}
					<span class="rounded bg-yellow-500/20 px-1.5 py-0.5 text-xs text-yellow-300">setup</span>
				{/if}
			</div>
			<p class="mt-1 text-sm text-gray-400">{character.ability}</p>
		</div>
		{#if removable && onremove}
			<button
				onclick={onremove}
				aria-label="Remove {character.name}"
				class="shrink-0 rounded p-1 text-gray-500 transition-colors hover:bg-gray-700 hover:text-gray-300"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
				</svg>
			</button>
		{/if}
	</div>
</div>
