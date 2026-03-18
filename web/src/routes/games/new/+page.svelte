<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { client } from '~/lib/api';
	import { getErrorMessage } from '~/lib/errors';
	import type { Script, Edition, RoleDistribution } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import DistributionBar from '~/lib/components/DistributionBar.svelte';

	let scripts = $state<Script[]>([]);
	let editions = $state<Edition[]>([]);
	let selectedScriptId = $state<bigint | undefined>();
	let totalCount = $state(8);
	let travellerCount = $state(0);
	let loading = $state(true);
	let creating = $state(false);
	let error = $state('');
	let currentDist = $state<RoleDistribution | undefined>();

	const editionStyles: Record<string, { border: string; bg: string; activeBorder: string; activeBg: string }> = {
		tb: { border: 'border-rose-800/60', bg: 'bg-rose-950/40', activeBorder: 'border-rose-500', activeBg: 'bg-rose-900/50' },
		bmr: { border: 'border-orange-800/60', bg: 'bg-orange-950/40', activeBorder: 'border-orange-500', activeBg: 'bg-orange-900/50' },
		snv: { border: 'border-violet-700/60', bg: 'bg-violet-950/40', activeBorder: 'border-violet-500', activeBg: 'bg-violet-900/50' }
	};

	const playerCount = $derived(Math.min(totalCount, 15));
	const minTravellers = $derived(Math.max(0, totalCount - 15));
	const selectedScript = $derived(scripts.find((s) => s.id === selectedScriptId));
	const totalPeople = $derived(playerCount + travellerCount);
	const showTotalWarning = $derived(totalPeople > 20);

	// When totalCount changes, ensure travellerCount is at least the minimum.
	$effect(() => {
		if (travellerCount < minTravellers) {
			travellerCount = minTravellers;
		}
	});

	// Fetch distribution from API when player count changes.
	$effect(() => {
		const pc = playerCount;
		client.getDistribution({ playerCount: pc }).then((resp) => {
			currentDist = resp.distribution;
		});
	});

	onMount(async () => {
		try {
			const [scriptsResp, editionsResp] = await Promise.all([
				client.listScripts({}),
				client.listEditions({})
			]);
			scripts = scriptsResp.scripts;
			editions = editionsResp.editions;

			// Pre-select from query param.
			const scriptParam = page.url.searchParams.get('script');
			if (scriptParam) {
				selectedScriptId = BigInt(scriptParam);
			}
		} catch (err) {
			error = getErrorMessage(err, 'Failed to load');
		} finally {
			loading = false;
		}
	});

	function selectEdition(editionId: string) {
		const existing = scripts.find((s) => s.edition === editionId && s.isSystem);
		if (existing) {
			selectedScriptId = existing.id;
		}
	}

	async function createGame() {
		if (!selectedScriptId) return;
		creating = true;
		error = '';
		try {
			const resp = await client.createGame({
				scriptId: selectedScriptId,
				playerCount,
				travellerCount
			});
			if (resp.game) {
				goto(`/games/${resp.game.id}`);
			}
		} catch (err) {
			error = getErrorMessage(err, 'Failed to create game');
		} finally {
			creating = false;
		}
	}
</script>

<div class="mx-auto max-w-2xl space-y-8">
	<h1 class="text-2xl font-bold">New Game</h1>

	{#if error}
		<div class="rounded-lg bg-red-900/50 px-4 py-2 text-sm text-red-300">{error}</div>
	{/if}

	{#if loading}
		<p class="text-gray-400">Loading...</p>
	{:else}
		<!-- Step 1: Pick script -->
		<section class="space-y-3">
			<h2 class="text-lg font-semibold text-gray-300">1. Choose a Script</h2>

			<!-- Edition templates -->
			{#if editions.length > 0}
				<div class="grid gap-3 sm:grid-cols-3">
					{#each editions as edition}
						{@const style = editionStyles[edition.id] ?? { border: 'border-gray-700', bg: 'bg-gray-900', activeBorder: 'border-indigo-500', activeBg: 'bg-indigo-500/10' }}
						{@const editionCharSet = new Set(edition.characterIds)}
					{@const isSelected = selectedScript?.characterIds.some((id) => editionCharSet.has(id)) ?? false}
						<button
							onclick={() => selectEdition(edition.id)}
							class="flex flex-col items-center rounded-lg border p-5 transition-all hover:scale-[1.02] hover:brightness-110 {isSelected
								? `${style.activeBorder} ${style.activeBg}`
								: `${style.border} ${style.bg}`}"
						>
							<img
								src="/editions/{edition.id}.png"
								alt={edition.name}
								class="h-16 object-contain"
							/>
							<p class="mt-3 text-sm text-gray-400">{edition.characterIds.length} characters</p>
						</button>
					{/each}
				</div>
			{/if}

			<!-- Saved scripts -->
			{#if scripts.filter((s) => !s.isSystem).length > 0}
				<p class="text-sm text-gray-500">Or choose a saved script:</p>
				<div class="grid gap-2 sm:grid-cols-2">
					{#each scripts.filter((s) => !s.isSystem) as script (script.id)}
						<button
							onclick={() => (selectedScriptId = script.id)}
							class="rounded-lg border p-3 text-left transition-colors {selectedScriptId === script.id
								? 'border-indigo-500 bg-indigo-500/10'
								: 'border-gray-700 bg-gray-900 hover:border-gray-600'}"
						>
							<span class="font-medium text-white">{script.name}</span>
							<span class="ml-2 text-sm text-gray-400">{script.characterIds.length} chars</span>
						</button>
					{/each}
				</div>
			{/if}
		</section>

		<!-- Step 2: Player count -->
		{#if selectedScriptId}
			<section class="space-y-3">
				<h2 class="text-lg font-semibold text-gray-300">2. How many people?</h2>
				<div class="flex flex-wrap gap-2">
					{#each Array.from({ length: 16 }, (_, i) => i + 5) as n}
						<button
							onclick={() => (totalCount = n)}
							class="h-10 w-10 rounded-lg text-sm font-medium transition-colors {totalCount === n
								? 'bg-indigo-500 text-white'
								: 'border border-gray-700 bg-gray-900 text-gray-300 hover:bg-gray-800'}"
						>
							{n}
						</button>
					{/each}
				</div>

				{#if totalCount > 15}
					<p class="text-sm text-gray-400">
						Max 15 players — {totalCount - 15} will be {totalCount - 15 === 1 ? 'a traveller' : 'travellers'}
					</p>
				{/if}

				{#if currentDist}
					<div class="rounded-lg border border-gray-700 bg-gray-900 p-4">
						<p class="mb-2 text-sm text-gray-400">Expected distribution for {playerCount} players:</p>
						<DistributionBar current={{ townsfolk: currentDist.townsfolk, outsiders: currentDist.outsiders, minions: currentDist.minions, demons: currentDist.demons }} travellers={travellerCount} />
					</div>
				{/if}
			</section>

			<!-- Step 3: Travellers -->
			<section class="space-y-3">
				<h2 class="text-lg font-semibold text-gray-300">3. Travellers</h2>
				<div class="flex items-center gap-3">
					<button
						onclick={() => (travellerCount = Math.max(minTravellers, travellerCount - 1))}
						disabled={travellerCount <= minTravellers}
						class="flex h-10 w-10 items-center justify-center rounded-lg border border-gray-700 bg-gray-900 text-lg font-medium text-gray-300 transition-colors hover:bg-gray-800 disabled:opacity-30"
					>
						-
					</button>
					<span class="w-8 text-center text-lg font-medium text-white">{travellerCount}</span>
					<button
						onclick={() => (travellerCount = travellerCount + 1)}
						disabled={totalPeople >= 25}
						class="flex h-10 w-10 items-center justify-center rounded-lg border border-gray-700 bg-gray-900 text-lg font-medium text-gray-300 transition-colors hover:bg-gray-800 disabled:opacity-30"
					>
						+
					</button>
					<span class="text-sm text-gray-400">
						Total: {totalPeople} {totalPeople === 1 ? 'person' : 'people'}
					</span>
				</div>

				{#if showTotalWarning}
					<div class="rounded-lg border border-yellow-700/50 bg-yellow-900/30 px-4 py-2 text-sm text-yellow-300">
						The recommended maximum is 20 players. Games with more players may be harder to manage.
					</div>
				{/if}
			</section>

			<!-- Step 4: Create -->
			<section>
				<button
					onclick={createGame}
					disabled={creating || !selectedScriptId}
					class="rounded-lg bg-indigo-500 px-6 py-2.5 font-medium text-white transition-colors hover:bg-indigo-400 disabled:opacity-50"
				>
					{creating ? 'Creating...' : 'Create Game'}
				</button>
			</section>
		{/if}
	{/if}
</div>
