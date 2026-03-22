<script lang="ts">
	import { untrack } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { client } from '~/lib/api';
	import { invalidateSidebar } from '~/lib/sidebar-data.svelte';
	import { getErrorMessage } from '~/lib/errors';
	import type { Game, Character, Script } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import { Team, GameState, PhaseType, TravellerAlignment } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import { teamLabels } from '~/lib/team-styles';
	import CharacterCard from '~/lib/components/CharacterCard.svelte';
	import CharacterPickerModal from '~/lib/components/CharacterPickerModal.svelte';
	import ConfirmDialog from '~/lib/components/ConfirmDialog.svelte';
	import DeathTracker from '~/lib/components/DeathTracker.svelte';
	import DistributionBar from '~/lib/components/DistributionBar.svelte';
	import NightOrder from '~/lib/components/NightOrder.svelte';
	import PhaseHeader from '~/lib/components/PhaseHeader.svelte';
	import ReminderToken from '~/lib/components/ReminderToken.svelte';
	import SetupSidebar from '~/lib/components/SetupSidebar.svelte';
	import TeamSection from '~/lib/components/TeamSection.svelte';

	// --- Tab definitions (setup only) ---
	type GameTab = 'setup' | 'nightorder' | 'grimoire';

	const setupTabs: { id: GameTab; label: string }[] = [
		{ id: 'setup', label: 'Setup' },
		{ id: 'nightorder', label: 'Night Order' },
		{ id: 'grimoire', label: 'Grimoire' },
	];

	const validTabs = new Set<GameTab>(['setup', 'nightorder', 'grimoire']);
	const initialTab = page.url.searchParams.get('tab') as GameTab | null;
	let activeTab = $state<GameTab>(initialTab && validTabs.has(initialTab) ? initialTab : 'setup');

	function setTab(tab: GameTab) {
		activeTab = tab;
		const url = new URL(window.location.href);
		url.searchParams.set('tab', tab);
		goto(url.toString(), { replaceState: true, noScroll: true });
	}

	let game = $state<Game | undefined>();
	let script = $state<Script | undefined>();
	let loading = $state(true);
	let error = $state('');
	let randomizing = $state(false);

	// Confirm dialog state.
	let confirmDialog = $state<{ title: string; message: string; confirmLabel: string; cancelLabel: string; onconfirm: () => void; oncancel: () => void } | null>(null);

	// Picker state.
	let showCharacterPicker = $state(false);
	let pickerTeam = $state<Team | undefined>();
	let allCharacters = $state<Character[]>([]);

	const teamOrder = [Team.TOWNSFOLK, Team.OUTSIDER, Team.MINION, Team.DEMON] as const;

	// Characters grouped by team — includes both script and extra characters.
	const charactersByTeam = $derived.by(() => {
		const grouped: Record<number, Character[]> = {};
		const skip = new Set([Team.TRAVELLER, Team.FABLED, Team.LORIC]);
		for (const char of script?.characters ?? []) {
			if (skip.has(char.team)) continue;
			if (!grouped[char.team]) grouped[char.team] = [];
			grouped[char.team].push(char);
		}
		for (const char of game?.extraCharacterDetails ?? []) {
			if (skip.has(char.team)) continue;
			if (!grouped[char.team]) grouped[char.team] = [];
			grouped[char.team].push(char);
		}
		return grouped;
	});

	// Selected = script roles + extra characters (both show as "selected" in the grid).
	const selectedRoleIdSet = $derived(
		new Set([...(game?.selectedRoleIds ?? []), ...(game?.extraCharacterIds ?? [])])
	);

	// Track which IDs belong to the script vs extra (for toggle behavior).
	const scriptCharIdSet = $derived(new Set(script?.characters?.map((c) => c.id) ?? []));
	const extraCharIdSet = $derived(new Set(game?.extraCharacterIds ?? []));

	const selectedTravellerIdSet = $derived(
		new Set(game?.selectedTravellerIds ?? [])
	);

	const fabledCharacters = $derived(
		(game?.extraCharacterDetails ?? []).filter((c) => c.team === Team.FABLED)
	);
	const loricCharacters = $derived(
		(game?.extraCharacterDetails ?? []).filter((c) => c.team === Team.LORIC)
	);

	const optionalTeams = $derived([
		{ team: Team.TRAVELLER, label: 'Travellers', singular: 'Traveller', chars: game?.selectedTravellerCharacters ?? [], remove: removeTraveller },
		{ team: Team.FABLED, label: 'Fabled', singular: 'Fabled', chars: fabledCharacters, remove: removeExtraChar },
		{ team: Team.LORIC, label: 'Lorics', singular: 'Loric', chars: loricCharacters, remove: removeExtraChar },
	]);
	const emptyOptionals = $derived(optionalTeams.filter((o) => o.chars.length === 0));

	// Combined selectedIds for the character picker modal.
	const pickerSelectedIds = $derived(
		new Set([...(game?.selectedRoleIds ?? []), ...(game?.extraCharacterIds ?? []), ...(script?.characterIds ?? []), ...(game?.selectedTravellerIds ?? [])])
	);

	const currentDist = $derived.by(() => {
		if (!game) return { townsfolk: 0, outsiders: 0, minions: 0, demons: 0 };
		const d = { townsfolk: 0, outsiders: 0, minions: 0, demons: 0 };
		// Count from all characters (script + extra) that are selected.
		for (const [, chars] of Object.entries(charactersByTeam)) {
			for (const c of chars) {
				if (!selectedRoleIdSet.has(c.id)) continue;
				if (c.team === Team.TOWNSFOLK) d.townsfolk++;
				else if (c.team === Team.OUTSIDER) d.outsiders++;
				else if (c.team === Team.MINION) d.minions++;
				else if (c.team === Team.DEMON) d.demons++;
			}
		}
		return d;
	});

	const characterById = $derived.by(() => {
		const map = new Map<string, Character>();
		for (const char of script?.characters ?? []) {
			map.set(char.id, char);
		}
		for (const char of game?.selectedTravellerCharacters ?? []) {
			map.set(char.id, char);
		}
		for (const char of game?.extraCharacterDetails ?? []) {
			map.set(char.id, char);
		}
		return map;
	});

	// --- Game state derived values ---
	const isSetup = $derived(game?.state === GameState.SETUP);
	const isInProgress = $derived(game?.state === GameState.IN_PROGRESS);
	const isCompleted = $derived(game?.state === GameState.COMPLETED);
	const canStartGame = $derived(isSetup && (game?.selectedRoleIds?.length ?? 0) > 0);

	// --- Phase navigation state (in-progress) ---
	let viewingPhaseIndex = $state(0);
	let prevPhaseCount = $state(0);

	// Reset to latest only when new phases are created (not when phase content updates)
	$effect(() => {
		const count = game?.playState?.phases?.length ?? 0;
		if (count !== prevPhaseCount) {
			prevPhaseCount = count;
			viewingPhaseIndex = Math.max(0, count - 1);
		}
	});

	const phases = $derived(game?.playState?.phases ?? []);
	const viewingPhase = $derived(phases[viewingPhaseIndex] ?? phases[phases.length - 1]);
	const isViewingCurrent = $derived(viewingPhaseIndex === phases.length - 1);
	const viewingRound = $derived(viewingPhase?.roundNumber ?? 1);
	const isViewingNight = $derived(viewingPhase?.type === PhaseType.NIGHT);

	// Dead characters in the viewed phase (per-phase death records with propagation).
	const deadRoleIds = $derived(new Set((viewingPhase?.deaths ?? []).map((d) => d.roleId)));

	// Deaths that are NEW in this phase (not present in the previous phase).
	const newDeathsThisPhase = $derived.by(() => {
		const currentDeaths = viewingPhase?.deaths ?? [];
		if (viewingPhaseIndex <= 0) return currentDeaths;
		const prevDeadRoleIds = new Set((phases[viewingPhaseIndex - 1]?.deaths ?? []).map((d) => d.roleId));
		return currentDeaths.filter((d) => !prevDeadRoleIds.has(d.roleId));
	});

	const totalRoundsPlayed = $derived.by(() => {
		const phases = game?.playState?.phases ?? [];
		if (phases.length === 0) return 0;
		return phases[phases.length - 1].roundNumber;
	});

	// --- Load game ---
	async function loadGame(gameId: bigint) {
		loading = true;
		error = '';
		game = undefined;
		script = undefined;
		try {
			const resp = await client.getGame({ id: gameId });
			game = resp.game;
			if (game) {
				const scriptResp = await client.getScript({ id: game.scriptId });
				script = scriptResp.script;
			}
		} catch (err) {
			error = getErrorMessage(err, 'Failed to load game');
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		const id = page.params.id;
		untrack(() => {
			if (!id) {
				error = 'Invalid game ID';
				loading = false;
				return;
			}
			let gameId: bigint;
			try {
				gameId = BigInt(id);
			} catch {
				error = 'Invalid game ID';
				loading = false;
				return;
			}
			loadGame(gameId);
		});
	});

	// --- Setup actions ---
	async function randomize() {
		if (!game) return;
		randomizing = true;
		error = '';
		try {
			const resp = await client.randomizeRoles({ gameId: game.id });
			game = resp.game;
		} catch (err) {
			error = getErrorMessage(err, 'Failed to randomize roles');
		} finally {
			randomizing = false;
		}
	}

	async function toggleRole(id: string) {
		if (!game || !isSetup) return;
		error = '';

		// If it's an extra character, toggle via the extra characters API.
		if (extraCharIdSet.has(id)) {
			const newIds = (game.extraCharacterIds ?? []).filter((eid) => eid !== id);
			try {
				const resp = await client.updateGameExtraCharacters({
					gameId: game.id,
					extraCharacterIds: newIds
				});
				game = resp.game;
			} catch (err) {
				error = getErrorMessage(err, 'Failed to update roles');
			}
			return;
		}

		// Otherwise toggle via the normal roles API.
		const newIds = selectedRoleIdSet.has(id)
			? game.selectedRoleIds.filter((rid) => rid !== id)
			: [...game.selectedRoleIds, id];
		try {
			const resp = await client.updateGameRoles({
				gameId: game.id,
				selectedRoleIds: newIds
			});
			game = resp.game;
		} catch (err) {
			error = getErrorMessage(err, 'Failed to update roles');
		}
	}

	async function openCharacterPicker(forTeam?: Team) {
		error = '';
		if (allCharacters.length === 0) {
			try {
				const resp = await client.listCharacters({});
				allCharacters = resp.characters;
			} catch (err) {
				error = getErrorMessage(err, 'Failed to load characters');
				return;
			}
		}
		pickerTeam = forTeam;
		showCharacterPicker = true;
	}

	async function addExtraChar(char: Character) {
		if (!game) return;
		error = '';
		const newIds = [...(game.extraCharacterIds ?? []), char.id];
		try {
			const resp = await client.updateGameExtraCharacters({
				gameId: game.id,
				extraCharacterIds: newIds
			});
			game = resp.game;
		} catch (err) {
			error = getErrorMessage(err, 'Failed to add character');
		}
	}

	async function removeExtraChar(charId: string) {
		if (!game) return;
		error = '';
		const newIds = (game.extraCharacterIds ?? []).filter((eid) => eid !== charId);
		try {
			const resp = await client.updateGameExtraCharacters({
				gameId: game.id,
				extraCharacterIds: newIds
			});
			game = resp.game;
		} catch (err) {
			error = getErrorMessage(err, 'Failed to remove character');
		}
	}

	function handlePickerSelect(char: Character) {
		if (char.team === Team.TRAVELLER) {
			addTraveller(char);
		} else if (scriptCharIdSet.has(char.id)) {
			toggleRole(char.id);
		} else {
			addExtraChar(char);
		}
	}

	function handlePickerDeselect(charId: string) {
		if (selectedTravellerIdSet.has(charId)) {
			removeTraveller(charId);
		} else if (scriptCharIdSet.has(charId)) {
			toggleRole(charId);
		} else {
			removeExtraChar(charId);
		}
	}

	async function addTraveller(char: Character) {
		if (!game) return;
		error = '';
		const newIds = [...game.selectedTravellerIds, char.id];
		try {
			const resp = await client.updateGameTravellers({
				gameId: game.id,
				selectedTravellerIds: newIds
			});
			game = resp.game;
		} catch (err) {
			error = getErrorMessage(err, 'Failed to add traveller');
		}
	}

	async function removeTraveller(charId: string) {
		if (!game) return;
		error = '';
		const newIds = game.selectedTravellerIds.filter((tid) => tid !== charId);
		try {
			const resp = await client.updateGameTravellers({
				gameId: game.id,
				selectedTravellerIds: newIds
			});
			game = resp.game;
		} catch (err) {
			error = getErrorMessage(err, 'Failed to remove traveller');
		}
	}

	// --- Traveller alignment ---
	async function updateTravellerAlignment(roleId: string, alignment: TravellerAlignment) {
		if (!game) return;
		error = '';
		try {
			const resp = await client.updateTravellerAlignment({
				gameId: game.id,
				roleId,
				alignment,
			});
			game = resp.game;
		} catch (err) {
			error = getErrorMessage(err, 'Failed to update traveller alignment');
		}
	}

	// --- Game lifecycle actions ---
	async function startGame() {
		if (!game) return;
		error = '';
		try {
			const resp = await client.startGame({ gameId: game.id });
			game = resp.game;
		} catch (err) {
			error = getErrorMessage(err, 'Failed to start game');
		}
	}

	async function advancePhase() {
		if (!game) return;
		error = '';
		try {
			const resp = await client.advancePhase({ gameId: game.id });
			game = resp.game;
		} catch (err) {
			error = getErrorMessage(err, 'Failed to advance phase');
		}
	}

	function endGame() {
		if (!game) return;
		confirmDialog = {
			title: 'End Game',
			message: 'Are you sure you want to end this game? This cannot be undone.',
			confirmLabel: 'End Game',
			cancelLabel: 'Cancel',
			onconfirm: async () => {
				confirmDialog = null;
				if (!game) return;
				error = '';
				try {
					const resp = await client.endGame({ gameId: game.id });
					game = resp.game;
				} catch (err) {
					error = getErrorMessage(err, 'Failed to end game');
				}
			},
			oncancel: () => { confirmDialog = null; },
		};
	}

	// --- Night action tracking ---
	const completedActions = $derived(new Set(viewingPhase?.completedActions ?? []));

	async function toggleNightAction(actionId: string, done: boolean) {
		if (!game || !viewingPhase) return;
		error = '';
		try {
			const resp = await client.toggleNightAction({ gameId: game.id, actionId, done, phaseId: viewingPhase.id });
			game = resp.game;
		} catch (err) {
			error = getErrorMessage(err, 'Failed to toggle night action');
		}
	}

	// --- Death tracking ---
	async function doRecordDeath(roleId: string, propagate: boolean) {
		if (!game || !viewingPhase) return;
		error = '';
		try {
			const resp = await client.recordDeath({ gameId: game.id, roleId, phaseId: viewingPhase.id, propagate });
			game = resp.game;
		} catch (err) {
			error = getErrorMessage(err, 'Failed to record death');
		}
	}

	function recordDeath(roleId: string) {
		if (!game || !viewingPhase) return;
		if (isViewingCurrent) {
			doRecordDeath(roleId, true);
			return;
		}
		const charName = characterById.get(roleId)?.name ?? roleId;
		confirmDialog = {
			title: `Mark ${charName} as dead`,
			message: `Apply to later phases as well?`,
			confirmLabel: 'All later phases',
			cancelLabel: 'This phase only',
			onconfirm: () => { confirmDialog = null; doRecordDeath(roleId, true); },
			oncancel: () => { confirmDialog = null; doRecordDeath(roleId, false); },
		};
	}

	async function removeDeath(deathId: bigint, propagate = false) {
		if (!game) return;
		error = '';
		try {
			const resp = await client.removeDeath({ gameId: game.id, deathId, propagate });
			game = resp.game;
		} catch (err) {
			error = getErrorMessage(err, 'Failed to remove death');
		}
	}

	function undoDeathByRole(roleId: string) {
		const phaseDeath = (viewingPhase?.deaths ?? []).find((d) => d.roleId === roleId);
		if (!phaseDeath) return;
		if (isViewingCurrent) {
			removeDeath(phaseDeath.id, true);
			return;
		}
		const charName = characterById.get(roleId)?.name ?? roleId;
		confirmDialog = {
			title: `Revive ${charName}`,
			message: `Also revive in all later phases?`,
			confirmLabel: 'All later phases',
			cancelLabel: 'This phase only',
			onconfirm: () => { confirmDialog = null; removeDeath(phaseDeath.id, true); },
			oncancel: () => { confirmDialog = null; removeDeath(phaseDeath.id, false); },
		};
	}

	async function useGhostVote(deathId: bigint) {
		if (!game) return;
		error = '';
		try {
			const resp = await client.useGhostVote({ gameId: game.id, deathId });
			game = resp.game;
		} catch (err) {
			error = getErrorMessage(err, 'Failed to use ghost vote');
		}
	}

	// --- Editable game name ---
	let editingName = $state(false);
	let nameInput = $state('');

	async function updateGameName() {
		if (!game || !nameInput.trim() || nameInput === game.name) {
			editingName = false;
			return;
		}
		error = '';
		try {
			const resp = await client.updateGameName({ gameId: game.id, name: nameInput.trim() });
			game = resp.game;
			invalidateSidebar();
		} catch (err) {
			error = getErrorMessage(err, 'Failed to update game name');
		}
		editingName = false;
	}

	// --- State badge ---
	const stateBadge = $derived.by(() => {
		if (!game) return { label: '', class: '' };
		switch (game.state) {
			case GameState.IN_PROGRESS:
				return { label: 'In Progress', class: 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-300' };
			case GameState.COMPLETED:
				return { label: 'Completed', class: 'bg-element text-muted' };
			default:
				return { label: '', class: '' };
		}
	});
</script>

{#if loading}
	<p class="text-secondary">Loading...</p>
{:else if error && !game}
	<div class="rounded-lg bg-error-bg border border-error-border px-4 py-2 text-sm text-error-text">{error}</div>
{:else if game}
	<div class="space-y-6 pb-16 2xl:pb-0">
		<!-- Header -->
		<div class="no-print flex items-center justify-between">
			<div>
				<div class="flex items-center gap-3">
					{#if editingName}
						<input
							type="text"
							bind:value={nameInput}
							onblur={updateGameName}
							onkeydown={(e) => { if (e.key === 'Enter') updateGameName(); if (e.key === 'Escape') editingName = false; }}
							class="text-2xl font-bold text-primary bg-transparent border-b-2 border-indigo-500 outline-none w-full max-w-md"
							autofocus
						/>
					{:else}
						<button
							onclick={() => { nameInput = game?.name ?? ''; editingName = true; }}
							class="flex items-center gap-2 text-2xl font-bold text-primary hover:text-indigo-500 transition-colors text-left"
							title="Click to edit name"
						>
							{game.name || 'Untitled Game'}
							<svg class="h-5 w-5 shrink-0 text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0115.75 21H5.25A2.25 2.25 0 013 18.75V8.25A2.25 2.25 0 015.25 6H10" />
							</svg>
						</button>
					{/if}
					{#if stateBadge.label}
						<span class="rounded-full px-2.5 py-0.5 text-xs font-medium {stateBadge.class}">{stateBadge.label}</span>
					{/if}
				</div>
				<p class="mt-1 text-secondary">
					{game.playerCount} players{#if game.travellerCount > 0}
						+ {game.travellerCount} {game.travellerCount === 1 ? 'traveller' : 'travellers'}
						= {game.playerCount + game.travellerCount} total{/if}
				</p>
			</div>
			{#if canStartGame}
				<button
					onclick={startGame}
					class="rounded-lg bg-green-600 px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-green-500"
				>
					Start Game
				</button>
			{/if}
		</div>

		<!-- Completed game banner -->
		{#if isCompleted}
			<div class="rounded-lg border border-border bg-surface p-6 text-center">
				<svg class="mx-auto mb-3 h-12 w-12 text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
				</svg>
				<h2 class="text-xl font-bold text-primary">Game Complete</h2>
				<p class="mt-1 text-sm text-secondary">
					{totalRoundsPlayed} {totalRoundsPlayed === 1 ? 'round' : 'rounds'} played
					{#if (game.playState?.allDeaths ?? []).length > 0}
						&middot; {game.playState?.allDeaths.length} {game.playState?.allDeaths.length === 1 ? 'death' : 'deaths'}
					{/if}
				</p>

				<!-- Phase history -->
				{#if (game.playState?.phases ?? []).length > 0}
					<div class="mt-4 flex flex-wrap items-center justify-center gap-1">
						{#each game.playState?.phases ?? [] as phase (phase.id)}
							<span class="rounded px-2 py-0.5 text-xs font-medium bg-element text-secondary">
								{phase.type === PhaseType.NIGHT ? 'Night' : 'Day'} {phase.roundNumber}
								{#if phase.deaths.length > 0}
									({phase.deaths.length} {phase.deaths.length === 1 ? 'death' : 'deaths'})
								{/if}
							</span>
						{/each}
					</div>
				{/if}

				<!-- Deaths summary (read-only) -->
				{#if (game.playState?.allDeaths ?? []).length > 0}
					<div class="mt-6 max-w-lg mx-auto text-left">
						<DeathTracker
							{game}
							onrecord={() => {}}
							onremove={() => {}}
							onuseghostvote={() => {}}
							readonly
						/>
					</div>
				{/if}

				<!-- Setup info (read-only) -->
				<div class="mt-6 max-w-lg mx-auto text-left">
					<h3 class="mb-2 text-sm font-semibold uppercase tracking-wide text-secondary">Roles in Play</h3>
					<div class="flex flex-wrap gap-2">
						{#each game.selectedCharacters as char (char.id)}
							{@const isDead = deadRoleIds.has(char.id)}
							<span class="inline-flex items-center gap-1.5 rounded-full border border-border px-2.5 py-1 text-xs font-medium {isDead ? 'text-muted line-through' : 'text-primary'}">
								{char.name}
							</span>
						{/each}
					</div>
				</div>
			</div>
		{/if}

		<!-- Tab bar (setup only) -->
		{#if isSetup}
			<div class="no-print flex gap-1 rounded-lg bg-element p-1">
				{#each setupTabs as t}
					<button
						onclick={() => setTab(t.id)}
						class="rounded-md px-4 py-2 text-sm font-medium transition-colors {activeTab === t.id
							? 'bg-surface text-primary shadow-sm'
							: 'text-secondary hover:text-medium'}"
					>
						{t.label}
					</button>
				{/each}
			</div>
		{/if}

		{#if error}
			<div class="rounded-lg bg-error-bg border border-error-border px-4 py-2 text-sm text-error-text">{error}</div>
		{/if}

		<!-- ===== IN-PROGRESS (no tabs — single scrollable page) ===== -->
		{#if isInProgress && game.playState}
			<div class="space-y-6">
				<PhaseHeader
					{game}
					{viewingPhaseIndex}
					onadvance={advancePhase}
					onend={endGame}
					onnavigate={(i) => viewingPhaseIndex = i}
				/>

				<!-- Night phase: show night order -->
				{#if isViewingNight}
					<NightOrder
						{game}
						scriptCharacters={script?.characters ?? []}
						deadRoleIds={deadRoleIds}
						activeRound={viewingRound}
						{completedActions}
						ontoggle={toggleNightAction}
						ondeath={recordDeath}
						onundodeath={undoDeathByRole}
					/>
				{/if}

				<!-- Death tracker -->
				<DeathTracker
					{game}
					viewedPhaseDeaths={newDeathsThisPhase}
					onrecord={recordDeath}
					onremove={removeDeath}
					onuseghostvote={useGhostVote}
					readonly={!isViewingCurrent}
				/>

				<!-- Travellers (in-progress) -->
				{#if (game.selectedTravellerCharacters ?? []).length > 0}
					<TeamSection
						team={Team.TRAVELLER}
						characters={game.selectedTravellerCharacters}
						travellerAlignments={game.travellerAlignments}
						onalignmentchange={updateTravellerAlignment}
					/>
				{/if}
			</div>

		<!-- ===== SETUP TABS (setup state only) ===== -->
		{:else if isSetup}
			{#if activeTab === 'setup'}
				<div class="space-y-6">
					<div class="flex items-center justify-end">
						<button
							onclick={randomize}
							disabled={randomizing}
							class="btn-primary rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-400 disabled:opacity-50"
						>
							{randomizing ? 'Randomizing...' : 'Randomize Roles'}
						</button>
					</div>

					<!-- Distribution -->
					<div class="rounded-lg border border-border bg-surface p-4">
						<DistributionBar current={currentDist} expected={game.distribution} travellers={game.selectedTravellerCharacters.length} />
					</div>

					<!-- Characters — click to toggle selection (script + extra merged) -->
					{#if script}
						<div class="space-y-6">
							{#each teamOrder as team}
								{@const chars = charactersByTeam[team]}
								{#if chars && chars.length > 0}
									<TeamSection
										{team}
										characters={chars}
										selectedIds={selectedRoleIdSet}
										onclick={toggleRole}
										onadd={() => openCharacterPicker(team)}
									/>
								{/if}
							{/each}
						</div>
					{/if}

					<!-- Optional teams: Travellers, Fabled, Lorics -->
					{#each optionalTeams as opt}
						{#if opt.chars.length > 0}
							<TeamSection
								team={opt.team}
								characters={opt.chars}
								removable
								onremove={opt.remove}
								onadd={() => openCharacterPicker(opt.team)}
								addLabel="Add {opt.singular}"
								travellerAlignments={opt.team === Team.TRAVELLER ? game.travellerAlignments : undefined}
								onalignmentchange={opt.team === Team.TRAVELLER ? updateTravellerAlignment : undefined}
							/>
						{/if}
					{/each}

					<!-- Compact row for empty teams -->
					{#if emptyOptionals.length > 0}
						<div class="grid gap-2" style="grid-template-columns: repeat({emptyOptionals.length}, 1fr)">
							{#each emptyOptionals as opt}
								<TeamSection
									team={opt.team}
									characters={[]}
									compact
									onadd={() => openCharacterPicker(opt.team)}
									addLabel={opt.label}
								/>
							{/each}
						</div>
					{/if}

					<!-- Reminder tokens -->
					{#if game.reminderTokens.length > 0}
						<section>
							<h2 class="mb-3 text-lg font-semibold text-medium">Reminder Tokens</h2>
							<div class="flex flex-wrap gap-4">
								{#each game.reminderTokens as token}
									{@const char = characterById.get(token.characterId)}
									<ReminderToken
										characterId={token.characterId}
										characterName={token.characterName}
										text={token.text}
										edition={char?.edition ?? ''}
										team={char?.team ?? Team.UNSPECIFIED}
									/>
								{/each}
							</div>
						</section>
					{/if}
				</div>

			{:else if activeTab === 'nightorder'}
				<NightOrder
					{game}
					scriptCharacters={script?.characters ?? []}
				/>

			{:else if activeTab === 'grimoire'}
				<div class="flex flex-col items-center justify-center py-16 text-center">
					<svg class="mb-4 h-16 w-16 text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
					</svg>
					<h2 class="text-lg font-semibold text-primary">Grimoire</h2>
					<p class="mt-2 max-w-md text-sm text-secondary">
						The Grimoire will let you track tokens, player states, and game progression. Coming soon.
					</p>
				</div>
			{/if}
		{/if}
	</div>

	<!-- Character picker modal (setup only) -->
	{#if showCharacterPicker && isSetup}
		<CharacterPickerModal
			title={pickerTeam ? `Add ${teamLabels[pickerTeam] ?? 'Character'}` : 'Add Character'}
			characters={allCharacters}
			selectedIds={pickerSelectedIds}
			team={pickerTeam}
			onselect={handlePickerSelect}
			ondeselect={handlePickerDeselect}
			onclose={() => (showCharacterPicker = false)}
		/>
	{/if}

	<!-- Setup sidebar (setup tab + setup state only) -->
	{#if activeTab === 'setup' && isSetup}
		<SetupSidebar gameId={game.id} selectedIds={[...(game.selectedRoleIds ?? []), ...(game.extraCharacterIds ?? [])]} />
	{/if}

	<!-- Confirm dialog -->
	{#if confirmDialog}
		<ConfirmDialog
			title={confirmDialog.title}
			message={confirmDialog.message}
			confirmLabel={confirmDialog.confirmLabel}
			cancelLabel={confirmDialog.cancelLabel}
			onconfirm={confirmDialog.onconfirm}
			oncancel={confirmDialog.oncancel}
		/>
	{/if}
{/if}
