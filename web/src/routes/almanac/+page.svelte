<script lang="ts">
	import { onMount } from 'svelte';
	import { client } from '~/lib/api';
	import { Team } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import type { Character } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import AlmanacView from '~/lib/components/AlmanacView.svelte';

	let allCharacters = $state<Character[]>([]);
	let loading = $state(true);
	let error = $state('');
	let searchQuery = $state('');
	let editionFilter = $state('');
	let teamFilter = $state(Team.UNSPECIFIED);

	const editions = [
		{ id: '', label: 'All', active: 'bg-indigo-500 text-white' },
		{ id: 'tb', label: 'Trouble Brewing', active: 'bg-rose-700 text-white' },
		{ id: 'bmr', label: 'Bad Moon Rising', active: 'bg-orange-700 text-white' },
		{ id: 'snv', label: 'Sects & Violets', active: 'bg-violet-700 text-white' }
	];

	const teams = [
		{ id: Team.UNSPECIFIED, label: 'All', active: 'bg-indigo-500 text-white' },
		{ id: Team.TOWNSFOLK, label: 'Townsfolk', active: 'bg-blue-600 text-white' },
		{ id: Team.OUTSIDER, label: 'Outsiders', active: 'bg-cyan-600 text-white' },
		{ id: Team.MINION, label: 'Minions', active: 'bg-orange-600 text-white' },
		{ id: Team.DEMON, label: 'Demons', active: 'bg-red-600 text-white' },
		{ id: Team.TRAVELLER, label: 'Travellers', active: 'bg-gradient-to-r from-blue-600 to-red-600 text-white' },
		{ id: Team.FABLED, label: 'Fabled', active: 'bg-yellow-600 text-white' },
		{ id: Team.LORIC, label: 'Lorics', active: 'bg-green-600 text-white' }
	];

	const filtered = $derived.by(() => {
		let result = allCharacters;
		if (editionFilter) {
			result = result.filter((c) => c.edition === editionFilter);
		}
		if (teamFilter !== Team.UNSPECIFIED) {
			result = result.filter((c) => c.team === teamFilter);
		}
		if (searchQuery.trim()) {
			const q = searchQuery.trim().toLowerCase();
			result = result.filter(
				(c) => c.name.toLowerCase().includes(q) || c.ability.toLowerCase().includes(q)
			);
		}
		return result;
	});

	onMount(async () => {
		try {
			const resp = await client.listCharacters({});
			allCharacters = resp.characters;
		} catch {
			error = 'Failed to load characters.';
		}
		loading = false;
	});
</script>

<svelte:head>
	<title>Almanac — Clock Keeper</title>
</svelte:head>

<div class="py-4">
	<h1 class="text-2xl font-bold text-primary">Almanac</h1>
	<p class="mt-1 text-sm text-secondary">Browse all characters, abilities, and night info</p>

	<!-- Filters -->
	<div class="mt-4 space-y-3">
		<input
			type="text"
			bind:value={searchQuery}
			placeholder="Search by name or ability..."
			class="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-primary placeholder:text-muted focus:border-indigo-400 focus:outline-none"
		/>

		<div class="flex flex-wrap gap-1.5">
			{#each editions as ed}
				<button
					onclick={() => (editionFilter = ed.id)}
					class="rounded-full px-3 py-1 text-xs font-medium transition-colors {editionFilter === ed.id ? ed.active : 'bg-element text-secondary hover:text-primary'}"
				>
					{ed.label}
				</button>
			{/each}
		</div>

		<div class="flex flex-wrap gap-1.5">
			{#each teams as t}
				<button
					onclick={() => (teamFilter = t.id)}
					class="rounded-full px-3 py-1 text-xs font-medium transition-colors {teamFilter === t.id ? t.active : 'bg-element text-secondary hover:text-primary'}"
				>
					{t.label}
				</button>
			{/each}
		</div>
	</div>

	<!-- Count -->
	<p class="mt-3 text-xs text-muted">
		{#if loading}
			Loading characters...
		{:else if error}
			&nbsp;
		{:else}
			Showing {filtered.length} of {allCharacters.length} characters
		{/if}
	</p>

	<!-- Character list -->
	<div class="mt-4">
		{#if loading}
			<p class="py-8 text-center text-sm text-muted">Loading...</p>
		{:else if error}
			<p class="py-8 text-center text-sm text-red-500">{error}</p>
		{:else}
			<AlmanacView characters={filtered} />
		{/if}
	</div>
</div>
