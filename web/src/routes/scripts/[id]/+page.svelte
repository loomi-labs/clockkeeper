<script lang="ts">
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { client } from '~/lib/api';
	import type { Script, Character } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import { Team } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import TeamSection from '~/lib/components/TeamSection.svelte';

	let script = $state<Script | undefined>();
	let name = $state('');
	let loading = $state(true);
	let error = $state('');
	let showAddCharacter = $state(false);
	let allCharacters = $state.raw<Character[]>([]);
	let searchQuery = $state('');
	let editionFilter = $state('');
	let teamFilter = $state(Team.UNSPECIFIED);
	let lastSavedName = '';
	let lastSavedIds: string[] = [];

	const teamOrder = [Team.TOWNSFOLK, Team.OUTSIDER, Team.MINION, Team.DEMON, Team.TRAVELLER, Team.FABLED];

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
		{ id: Team.DEMON, label: 'Demons', active: 'bg-red-600 text-white' }
	];

	const teamCardColors: Record<number, string> = {
		[Team.TOWNSFOLK]: 'border-blue-500/40 bg-blue-950/30',
		[Team.OUTSIDER]: 'border-cyan-500/40 bg-cyan-950/30',
		[Team.MINION]: 'border-orange-500/40 bg-orange-950/30',
		[Team.DEMON]: 'border-red-500/40 bg-red-950/30',
		[Team.TRAVELLER]: 'border-yellow-500/40 bg-yellow-950/30',
		[Team.FABLED]: 'border-amber-500/40 bg-amber-950/30'
	};

	const teamCardColorsSelected: Record<number, string> = {
		[Team.TOWNSFOLK]: 'border-blue-500 bg-blue-900/50',
		[Team.OUTSIDER]: 'border-cyan-500 bg-cyan-900/50',
		[Team.MINION]: 'border-orange-500 bg-orange-900/50',
		[Team.DEMON]: 'border-red-500 bg-red-900/50',
		[Team.TRAVELLER]: 'border-yellow-500 bg-yellow-900/50',
		[Team.FABLED]: 'border-amber-500 bg-amber-900/50'
	};

	const teamNameColors: Record<number, string> = {
		[Team.TOWNSFOLK]: 'text-blue-300',
		[Team.OUTSIDER]: 'text-cyan-300',
		[Team.MINION]: 'text-orange-300',
		[Team.DEMON]: 'text-red-300',
		[Team.TRAVELLER]: 'text-yellow-300',
		[Team.FABLED]: 'text-amber-300'
	};

	const teamCheckColors: Record<number, string> = {
		[Team.TOWNSFOLK]: 'text-blue-400',
		[Team.OUTSIDER]: 'text-cyan-400',
		[Team.MINION]: 'text-orange-400',
		[Team.DEMON]: 'text-red-400',
		[Team.TRAVELLER]: 'text-yellow-400',
		[Team.FABLED]: 'text-amber-400'
	};

	const teamNames: Record<number, string> = {
		[Team.TOWNSFOLK]: 'Townsfolk',
		[Team.OUTSIDER]: 'Outsider',
		[Team.MINION]: 'Minion',
		[Team.DEMON]: 'Demon',
		[Team.TRAVELLER]: 'Traveller',
		[Team.FABLED]: 'Fabled'
	};

	const charactersByTeam = $derived.by(() => {
		if (!script?.characters) return {};
		const grouped: Record<number, Character[]> = {};
		for (const char of script.characters) {
			if (!grouped[char.team]) grouped[char.team] = [];
			grouped[char.team].push(char);
		}
		return grouped;
	});

	const scriptIdSet = $derived(new Set(script?.characterIds ?? []));

	const filteredCharacters = $derived.by(() => {
		let chars = [...allCharacters];
		if (editionFilter) {
			chars = chars.filter((c) => c.edition === editionFilter);
		}
		if (teamFilter !== Team.UNSPECIFIED) {
			chars = chars.filter((c) => c.team === teamFilter);
		}
		if (searchQuery) {
			const q = searchQuery.toLowerCase();
			chars = chars.filter(
				(c) => c.name.toLowerCase().includes(q) || c.ability.toLowerCase().includes(q)
			);
		}
		return chars;
	});

	// Auto-save with debounce.
	let saveTimer: ReturnType<typeof setTimeout> | undefined;
	const characterIds = $derived(script?.characterIds ?? []);

	$effect(() => {
		const _name = name;
		const _ids = characterIds;

		if (script?.isSystem) return;
		if (_name === lastSavedName && JSON.stringify(_ids) === JSON.stringify(lastSavedIds)) return;

		clearTimeout(saveTimer);
		saveTimer = setTimeout(() => {
			untrack(() => autoSave(_name, _ids));
		}, 800);

		return () => clearTimeout(saveTimer);
	});

	async function autoSave(currentName: string, currentIds: string[]) {
		if (!script) return;
		try {
			await client.updateScript({
				id: script.id,
				name: currentName,
				characterIds: currentIds
			});
			lastSavedName = currentName;
			lastSavedIds = currentIds;
		} catch (err: any) {
			error = err.message || 'Failed to save';
		}
	}

	async function loadScript(id: bigint) {
		loading = true;
		error = '';
		try {
			const resp = await client.getScript({ id });
			script = resp.script;
			name = script?.name ?? '';
			lastSavedName = name;
			lastSavedIds = script?.characterIds ?? [];
		} catch (err: any) {
			error = err.message || 'Failed to load script';
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		const id = page.params.id;
		untrack(() => loadScript(BigInt(id)));
	});

	async function openAddCharacter() {
		showAddCharacter = true;
		if (allCharacters.length === 0) {
			const resp = await client.listCharacters({});
			allCharacters = resp.characters;
		}
	}

	function removeCharacter(charId: string) {
		if (!script) return;
		script = {
			...script,
			characterIds: script.characterIds.filter((id) => id !== charId),
			characters: script.characters.filter((c) => c.id !== charId)
		} as Script;
	}

	function addCharacter(char: Character) {
		if (!script) return;
		script = {
			...script,
			characterIds: [...script.characterIds, char.id],
			characters: [...script.characters, char]
		} as Script;
	}

	async function createFromEdition(editionId: string) {
		try {
			const resp = await client.createScriptFromEdition({ editionId, name: '' });
			if (resp.script) {
				goto(`/scripts/${resp.script.id}`);
			}
		} catch (err: any) {
			error = err.message || 'Failed to duplicate script';
		}
	}

	async function deleteScript() {
		if (!script) return;
		try {
			await client.deleteScript({ id: script.id });
			goto('/scripts');
		} catch (err: any) {
			error = err.message || 'Failed to delete script';
		}
	}

	function iconSuffix(team: Team): string {
		if (team === Team.TOWNSFOLK || team === Team.OUTSIDER) return '_g';
		if (team === Team.MINION || team === Team.DEMON) return '_e';
		return '';
	}
</script>

{#if loading}
	<p class="text-gray-400">Loading...</p>
{:else if error && !script}
	<div class="rounded-lg bg-red-900/50 px-4 py-2 text-sm text-red-300">{error}</div>
{:else if script}
	<div class="space-y-6">
		<!-- Top bar -->
		<div class="flex items-center justify-between gap-4">
			<div class="flex min-w-0 flex-1 items-center gap-3">
				<a href="/scripts" aria-label="Back to scripts" class="text-gray-400 transition-colors hover:text-gray-200">
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
					</svg>
				</a>
				{#if script.isSystem}
					<h2 class="min-w-0 flex-1 text-lg font-medium text-white">{script.name}</h2>
				{:else}
					<input
						bind:value={name}
						class="min-w-0 flex-1 rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-lg font-medium text-white focus:border-indigo-400 focus:outline-none"
					/>
				{/if}
			</div>
			<div class="flex items-center gap-2">
				{#if !script.isSystem}
					<button
						onclick={openAddCharacter}
						class="rounded-lg bg-indigo-500 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-400"
					>
						Add Characters
					</button>
				{/if}
				<a
					href="/games/new?script={script.id}"
					class="rounded-lg border border-gray-700 px-3 py-2 text-sm text-gray-300 transition-colors hover:bg-gray-800"
				>
					Start Game
				</a>
				{#if script.isSystem}
					<button
						onclick={() => createFromEdition(script.edition)}
						class="rounded-lg border border-gray-700 px-3 py-2 text-sm text-gray-300 transition-colors hover:bg-gray-800"
					>
						Duplicate
					</button>
				{:else}
					<button
						onclick={deleteScript}
						aria-label="Delete script"
						class="rounded p-2 text-gray-500 transition-colors hover:bg-gray-800 hover:text-red-400"
					>
						<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
						</svg>
					</button>
				{/if}
			</div>
		</div>

		{#if error}
			<div class="rounded-lg bg-red-900/50 px-4 py-2 text-sm text-red-300">{error}</div>
		{/if}

		<!-- Character list grouped by team -->
		<div class="space-y-6">
			{#each teamOrder as team}
				{@const chars = charactersByTeam[team]}
				{#if chars && chars.length > 0}
					<TeamSection {team} characters={chars} removable={!script.isSystem} onremove={removeCharacter} />
				{/if}
			{/each}
		</div>

		{#if script.characterIds.length === 0}
			<div class="rounded-lg border border-dashed border-gray-600 p-8 text-center">
				<p class="text-gray-400">No characters yet. Click "Add Characters" to get started.</p>
			</div>
		{/if}
	</div>

	<!-- Add character modal -->
	{#if showAddCharacter}
		<div class="fixed inset-0 z-50 flex items-start justify-center pt-16">
			<!-- Backdrop -->
			<button
				class="absolute inset-0 bg-black/60"
				onclick={() => (showAddCharacter = false)}
				aria-label="Close"
			></button>

			<!-- Modal -->
			<div class="relative mx-4 flex max-h-[80vh] w-full max-w-2xl flex-col rounded-xl border border-gray-700 bg-gray-900 shadow-2xl">
				<!-- Header -->
				<div class="flex items-center justify-between border-b border-gray-700 px-4 py-3">
					<h2 class="text-lg font-semibold text-white">Add Characters</h2>
					<button
						onclick={() => (showAddCharacter = false)}
						aria-label="Close"
						class="rounded p-1 text-gray-400 transition-colors hover:bg-gray-800 hover:text-gray-200"
					>
						<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
						</svg>
					</button>
				</div>

				<!-- Filters -->
				<div class="space-y-3 border-b border-gray-700 px-4 py-3">
					<input
						bind:value={searchQuery}
						placeholder="Search characters..."
						class="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white placeholder-gray-500 focus:border-indigo-400 focus:outline-none"
					/>
					<div class="flex flex-wrap items-center gap-1.5">
						<span class="text-xs text-gray-500">Edition:</span>
						{#each editions as ed}
							<button
								onclick={() => (editionFilter = ed.id)}
								class="rounded-lg px-3 py-1 text-xs font-medium transition-colors {editionFilter === ed.id
									? ed.active
									: 'bg-gray-800 text-gray-400 hover:text-gray-200'}"
							>
								{ed.label}
							</button>
						{/each}
					</div>
					<div class="flex flex-wrap items-center gap-1.5">
						<span class="text-xs text-gray-500">Type:</span>
						{#each teams as t}
							<button
								onclick={() => (teamFilter = t.id)}
								class="rounded-lg px-3 py-1 text-xs font-medium transition-colors {teamFilter === t.id
									? t.active
									: 'bg-gray-800 text-gray-400 hover:text-gray-200'}"
							>
								{t.label}
							</button>
						{/each}
					</div>
				</div>

				<!-- Character list -->
				<div class="overflow-y-auto p-4">
					<div class="grid gap-2 sm:grid-cols-2">
						{#each filteredCharacters as char (char.id)}
							{@const added = scriptIdSet.has(char.id)}
							<button
								onclick={() => added ? removeCharacter(char.id) : addCharacter(char)}
								class="rounded-lg border p-2.5 text-left transition-colors {added
									? (teamCardColorsSelected[char.team] ?? 'border-gray-500 bg-gray-800') + ' hover:brightness-90'
									: (teamCardColors[char.team] ?? 'border-gray-700 bg-gray-800') + ' hover:brightness-110'}"
							>
								<div class="flex items-center gap-2.5">
									<img
										src="/characters/{char.edition}/{char.id}{iconSuffix(char.team)}.webp"
										alt=""
										class="h-8 w-8 shrink-0 rounded-full"
										onerror={(e: Event) => (e.target as HTMLImageElement).style.display = 'none'}
									/>
									<div class="min-w-0 flex-1">
										<div class="flex items-center gap-1.5">
											<span class="text-sm font-medium {added ? (teamNameColors[char.team] ?? 'text-white') : 'text-white'}">{char.name}</span>
											<span class="text-xs text-gray-500">{teamNames[char.team] ?? ''}</span>
										</div>
										<p class="text-xs text-gray-400 line-clamp-1">{char.ability}</p>
									</div>
									{#if added}
										<svg class="h-4 w-4 shrink-0 {teamCheckColors[char.team] ?? 'text-gray-400'}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
										</svg>
									{/if}
								</div>
							</button>
						{/each}
					</div>
					{#if filteredCharacters.length === 0}
						<p class="py-4 text-center text-sm text-gray-500">No matching characters.</p>
					{/if}
				</div>
			</div>
		</div>
	{/if}
{/if}

