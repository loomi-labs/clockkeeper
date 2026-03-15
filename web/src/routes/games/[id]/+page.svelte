<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { client } from '~/lib/api';
	import type { Game, Character, Script } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import { Team } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import CharacterCard from '~/lib/components/CharacterCard.svelte';
	import DistributionBar from '~/lib/components/DistributionBar.svelte';

	let game = $state<Game | undefined>();
	let script = $state<Script | undefined>();
	let loading = $state(true);
	let error = $state('');
	let randomizing = $state(false);

	// Traveller picker state.
	let showTravellerPicker = $state(false);
	let allTravellers = $state<Character[]>([]);
	let travellerSearch = $state('');

	const teamOrder = [Team.TOWNSFOLK, Team.OUTSIDER, Team.MINION, Team.DEMON] as const;

	const teamLabels: Record<number, string> = {
		[Team.TOWNSFOLK]: 'Townsfolk',
		[Team.OUTSIDER]: 'Outsiders',
		[Team.MINION]: 'Minions',
		[Team.DEMON]: 'Demons'
	};

	const teamHeaderColors: Record<number, string> = {
		[Team.TOWNSFOLK]: 'text-blue-400',
		[Team.OUTSIDER]: 'text-cyan-400',
		[Team.MINION]: 'text-orange-400',
		[Team.DEMON]: 'text-red-400'
	};

	// All script characters grouped by team (non-traveller).
	const scriptCharactersByTeam = $derived.by(() => {
		if (!script?.characters) return {};
		const grouped: Record<number, Character[]> = {};
		for (const char of script.characters) {
			if (char.team === Team.TRAVELLER || char.team === Team.FABLED) continue;
			if (!grouped[char.team]) grouped[char.team] = [];
			grouped[char.team].push(char);
		}
		return grouped;
	});

	const selectedRoleIdSet = $derived(new Set(game?.selectedRoleIds ?? []));

	const currentDist = $derived.by(() => {
		if (!game?.selectedRoleIds) return { townsfolk: 0, outsiders: 0, minions: 0, demons: 0 };
		const d = { townsfolk: 0, outsiders: 0, minions: 0, demons: 0 };
		// Count from script characters that are selected.
		for (const c of script?.characters ?? []) {
			if (!selectedRoleIdSet.has(c.id)) continue;
			if (c.team === Team.TOWNSFOLK) d.townsfolk++;
			else if (c.team === Team.OUTSIDER) d.outsiders++;
			else if (c.team === Team.MINION) d.minions++;
			else if (c.team === Team.DEMON) d.demons++;
		}
		return d;
	});

	const selectedTravellerIdSet = $derived(
		new Set(game?.selectedTravellerIds ?? [])
	);

	const filteredTravellers = $derived.by(() => {
		const q = travellerSearch.toLowerCase();
		return allTravellers.filter(
			(c) => !selectedTravellerIdSet.has(c.id) && (!q || c.name.toLowerCase().includes(q))
		);
	});

	onMount(async () => {
		try {
			const id = BigInt(page.params.id);
			const resp = await client.getGame({ id });
			game = resp.game;
			if (game) {
				const scriptResp = await client.getScript({ id: game.scriptId });
				script = scriptResp.script;
			}
		} catch (err: any) {
			error = err.message || 'Failed to load game';
		} finally {
			loading = false;
		}
	});

	async function randomize() {
		if (!game) return;
		randomizing = true;
		error = '';
		try {
			const resp = await client.randomizeRoles({ gameId: game.id });
			game = resp.game;
		} catch (err: any) {
			error = err.message || 'Failed to randomize roles';
		} finally {
			randomizing = false;
		}
	}

	async function toggleRole(id: string) {
		if (!game) return;
		const newIds = selectedRoleIdSet.has(id)
			? game.selectedRoleIds.filter((rid) => rid !== id)
			: [...game.selectedRoleIds, id];
		error = '';
		try {
			const resp = await client.updateGameRoles({
				gameId: game.id,
				selectedRoleIds: newIds
			});
			game = resp.game;
		} catch (err: any) {
			error = err.message || 'Failed to update roles';
		}
	}

	async function openTravellerPicker() {
		if (allTravellers.length === 0) {
			try {
				const resp = await client.listCharacters({ edition: script?.edition ?? '', team: Team.TRAVELLER });
				allTravellers = resp.characters;
			} catch (err: any) {
				error = err.message || 'Failed to load travellers';
				return;
			}
		}
		showTravellerPicker = true;
	}

	async function addTraveller(id: string) {
		if (!game) return;
		const newIds = [...game.selectedTravellerIds, id];
		try {
			const resp = await client.updateGameTravellers({
				gameId: game.id,
				selectedTravellerIds: newIds
			});
			game = resp.game;
		} catch (err: any) {
			error = err.message || 'Failed to add traveller';
		}
	}

	async function removeTraveller(id: string) {
		if (!game) return;
		const newIds = game.selectedTravellerIds.filter((tid) => tid !== id);
		try {
			const resp = await client.updateGameTravellers({
				gameId: game.id,
				selectedTravellerIds: newIds
			});
			game = resp.game;
		} catch (err: any) {
			error = err.message || 'Failed to remove traveller';
		}
	}
</script>

{#if loading}
	<p class="text-gray-400">Loading...</p>
{:else if error && !game}
	<div class="rounded-lg bg-red-900/50 px-4 py-2 text-sm text-red-300">{error}</div>
{:else if game}
	<div class="space-y-6">
		<div class="flex items-center justify-between">
			<div>
				<h1 class="text-2xl font-bold">Game Setup</h1>
				<p class="mt-1 text-gray-400">
					{game.playerCount} players{#if game.travellerCount > 0}
						+ {game.travellerCount} {game.travellerCount === 1 ? 'traveller' : 'travellers'}
						= {game.playerCount + game.travellerCount} total{/if}
				</p>
			</div>
			<div class="flex gap-2">
				<button
					onclick={randomize}
					disabled={randomizing}
					class="rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-400 disabled:opacity-50"
				>
					{randomizing ? 'Randomizing...' : 'Randomize Roles'}
				</button>
				{#if game.selectedRoleIds.length > 0}
					<a
						href="/games/{game.id}/setup"
						class="rounded-lg border border-gray-700 px-4 py-2 text-sm text-gray-300 transition-colors hover:bg-gray-800"
					>
						Setup Checklist
					</a>
				{/if}
			</div>
		</div>

		{#if error}
			<div class="rounded-lg bg-red-900/50 px-4 py-2 text-sm text-red-300">{error}</div>
		{/if}

		<!-- Distribution -->
		<div class="rounded-lg border border-gray-700 bg-gray-900 p-4">
			<DistributionBar current={currentDist} expected={game.distribution} travellers={game.selectedTravellerCharacters.length} />
		</div>

		<!-- Script characters — click to toggle selection -->
		{#if script}
			<div class="space-y-6">
				{#each teamOrder as team}
					{@const chars = scriptCharactersByTeam[team]}
					{#if chars && chars.length > 0}
						<div>
							<h3 class="mb-3 text-sm font-semibold uppercase tracking-wide {teamHeaderColors[team] ?? 'text-gray-400'}">
								{teamLabels[team] ?? team} ({chars.filter((c) => selectedRoleIdSet.has(c.id)).length}/{chars.length})
							</h3>
							<div class="grid gap-2 sm:grid-cols-2">
								{#each chars as char (char.id)}
									<button
										onclick={() => toggleRole(char.id)}
										class="w-full text-left transition-transform active:scale-[0.98]"
									>
										<CharacterCard
											character={char}
											selected={selectedRoleIdSet.has(char.id)}
										/>
									</button>
								{/each}
							</div>
						</div>
					{/if}
				{/each}
			</div>
		{/if}

		<!-- Travellers -->
		<section class="space-y-3">
			<div class="flex items-center justify-between">
				<h2 class="text-sm font-semibold uppercase tracking-wide text-yellow-400">
					Travellers ({game.selectedTravellerCharacters.length})
				</h2>
				<button
					onclick={openTravellerPicker}
					class="rounded-lg border border-yellow-700/50 px-3 py-1.5 text-sm text-yellow-300 transition-colors hover:bg-yellow-900/30"
				>
					Add Traveller
				</button>
			</div>

			{#if game.selectedTravellerCharacters.length > 0}
				<div class="grid gap-2 sm:grid-cols-2">
					{#each game.selectedTravellerCharacters as char (char.id)}
						<CharacterCard
							character={char}
							removable
							onremove={() => removeTraveller(char.id)}
						/>
					{/each}
				</div>
			{:else}
				<p class="text-sm text-gray-500">No travellers added yet.</p>
			{/if}
		</section>

		<!-- Traveller picker modal -->
		{#if showTravellerPicker}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
				onkeydown={(e) => e.key === 'Escape' && (showTravellerPicker = false)}
			>
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<div class="absolute inset-0" onclick={() => (showTravellerPicker = false)}></div>
				<div class="relative max-h-[80vh] w-full max-w-lg overflow-hidden rounded-xl border border-gray-700 bg-gray-900 shadow-xl">
					<div class="flex items-center justify-between border-b border-gray-700 px-4 py-3">
						<h3 class="font-semibold text-white">Add Traveller</h3>
						<button
							onclick={() => (showTravellerPicker = false)}
							class="rounded p-1 text-gray-400 hover:text-white"
						>
							<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
							</svg>
						</button>
					</div>
					<div class="border-b border-gray-700 px-4 py-2">
						<input
							type="text"
							placeholder="Search travellers..."
							bind:value={travellerSearch}
							class="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white placeholder-gray-500 focus:border-indigo-400 focus:outline-none"
						/>
					</div>
					<div class="max-h-[60vh] overflow-y-auto p-4">
						{#if filteredTravellers.length === 0}
							<p class="text-center text-sm text-gray-500">No travellers found.</p>
						{:else}
							<div class="space-y-2">
								{#each filteredTravellers as char (char.id)}
									<button
										onclick={() => { addTraveller(char.id); }}
										class="w-full text-left"
									>
										<CharacterCard character={char} />
									</button>
								{/each}
							</div>
						{/if}
					</div>
				</div>
			</div>
		{/if}

		<!-- Reminder tokens -->
		{#if game.reminderTokens.length > 0}
			<section>
				<h2 class="mb-3 text-lg font-semibold text-gray-300">Reminder Tokens</h2>
				<div class="flex flex-wrap gap-2">
					{#each game.reminderTokens as token}
						<span class="rounded-lg border border-gray-700 bg-gray-900 px-3 py-1.5 text-sm">
							<span class="text-gray-400">{token.characterName}:</span>
							<span class="text-gray-200">{token.text}</span>
						</span>
					{/each}
				</div>
			</section>
		{/if}
	</div>
{/if}

