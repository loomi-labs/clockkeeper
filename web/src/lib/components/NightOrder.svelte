<script lang="ts">
	import { page } from '$app/state';
	import type { Character, Game } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import { Team } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import { formatReminder } from '~/lib/format';
	import { teamCardColors, teamNameColors, teamDataAttr, iconSuffix } from '~/lib/team-styles';

	let {
		game,
		scriptCharacters = [],
		deadRoleIds,
		activeRound,
		completedActions,
		ontoggle,
		ondeath,
		onundodeath,
	}: {
		game: Game;
		scriptCharacters?: Character[];
		deadRoleIds?: Set<string>;
		activeRound?: number;
		completedActions?: Set<string>;
		ontoggle?: (id: string, done: boolean) => void;
		ondeath?: (roleId: string) => void;
		onundodeath?: (roleId: string) => void;
	} = $props();

	import { usePan, type PanCustomEvent } from 'svelte-gestures';

	const SWIPE_THRESHOLD = 80;
	const NON_INTERACTIVE_SPECIALS = new Set(['dusk', 'dawn']);

	let panState = $state<{ id: string; startX: number; dx: number } | null>(null);

	function panProps(entryId: string, onLeftSwipe?: () => void) {
		const allowLeft = !!onLeftSwipe;
		if (!ontoggle && !allowLeft) return {};
		return usePan(
			(e: PanCustomEvent) => {
				if (!panState || panState.id !== entryId) {
					panState = { id: entryId, startX: e.detail.x, dx: 0 };
				} else {
					let dx = e.detail.x - panState.startX;
					if (!ontoggle) dx = Math.min(0, dx);
					if (!allowLeft) dx = Math.max(0, dx);
					panState = { ...panState, dx };
				}
			},
			() => ({ delay: 0, touchAction: 'pan-y' as const }),
			{
				onpanup: () => {
					if (panState && panState.id === entryId) {
						if (panState.dx > SWIPE_THRESHOLD) {
							const isDone = completedActions?.has(entryId) ?? false;
							ontoggle?.(entryId, !isDone);
						} else if (panState.dx < -SWIPE_THRESHOLD && allowLeft) {
							onLeftSwipe?.();
						}
					}
					panState = null;
				},
			},
		);
	}

	function swipeTransform(entryId: string): string {
		return `translate3d(${panState?.id === entryId ? panState.dx : 0}px, 0, 0)`;
	}

	function swipeTransition(entryId: string): string {
		return panState?.id === entryId ? 'none' : 'transform 250ms cubic-bezier(0.2, 0, 0, 1)';
	}

	interface NightEntry { id: string; name: string; reminder: string; team?: number; edition?: string; isSpecial: boolean; inPlay: boolean; isDead: boolean; }

	const SPECIAL_ENTRIES: Record<string, { name: string; reminder: string; position: { first: number; other: number }; minPlayers?: number }> = {
		dusk: { name: 'Dusk', reminder: 'Night begins. All players close their eyes.', position: { first: 0, other: 0 } },
		minioninfo: { name: 'Minion Info', reminder: 'Show the *THIS IS THE DEMON* token. Point to the Demon. Show the *THESE ARE YOUR MINIONS* token. Point to the other Minions.', position: { first: 20, other: -1 }, minPlayers: 7 },
		demoninfo: { name: 'Demon Info', reminder: 'Show the *THESE ARE YOUR MINIONS* token. Point to all Minions. Show the *THESE CHARACTERS ARE NOT IN PLAY* token. Show 3 not-in-play good character tokens.', position: { first: 25, other: -1 }, minPlayers: 7 },
		dawn: { name: 'Dawn', reminder: 'Night ends. All players open their eyes.', position: { first: 999, other: 999 } },
	};

	// NightOrder applies opacity separately, so Traveller doesn't need opacity-60 in unselected.
	const unselectedColors: Record<number, string> = {
		[Team.TOWNSFOLK]: 'border-blue-100 bg-blue-50/50 dark:border-blue-800/50 dark:bg-blue-950/20',
		[Team.OUTSIDER]: 'border-cyan-100 bg-cyan-50/50 dark:border-cyan-800/50 dark:bg-cyan-950/20',
		[Team.MINION]: 'border-orange-100 bg-orange-50/50 dark:border-orange-800/50 dark:bg-orange-950/20',
		[Team.DEMON]: 'border-red-100 bg-red-50/50 dark:border-red-800/50 dark:bg-red-950/20',
		[Team.TRAVELLER]: 'card-traveller',
		[Team.FABLED]: 'border-yellow-200 bg-yellow-50/50 dark:border-yellow-700/50 dark:bg-yellow-950/20',
		[Team.LORIC]: 'border-green-100 bg-green-50/50 dark:border-green-800/50 dark:bg-green-950/20',
	};
	const allSelectedChars = $derived([...(game.selectedCharacters ?? []), ...(game.selectedTravellerCharacters ?? []), ...(game.extraCharacterDetails ?? [])]);
	const selectedIdSet = $derived(new Set(allSelectedChars.map((c) => c.id)));
	const allScriptChars = $derived.by(() => {
		const seen = new Set<string>();
		const result: Character[] = [];
		for (const c of [...scriptCharacters, ...(game.selectedTravellerCharacters ?? []), ...(game.extraCharacterDetails ?? [])]) {
			if (!seen.has(c.id)) { seen.add(c.id); result.push(c); }
		}
		return result;
	});

	let showAll = $state(false);

	function buildNightOrder(night: 'first' | 'other'): NightEntry[] {
		const posField = night === 'first' ? 'firstNight' : 'otherNight';
		const reminderField = night === 'first' ? 'firstNightReminder' : 'otherNightReminder';
		const source = showAll ? allScriptChars : allSelectedChars;
		const charEntries: (NightEntry & { pos: number })[] = source
			.filter((c) => c[reminderField])
			.map((c) => ({ id: c.id, name: c.name, reminder: c[reminderField], team: c.team, edition: c.edition, isSpecial: false, inPlay: selectedIdSet.has(c.id), isDead: deadRoleIds?.has(c.id) ?? false, pos: c[posField] || 500 }));
		const specialEntries: (NightEntry & { pos: number })[] = [];
		for (const [id, entry] of Object.entries(SPECIAL_ENTRIES)) {
			const pos = night === 'first' ? entry.position.first : entry.position.other;
			if (pos < 0) continue;
			if (entry.minPlayers && game.playerCount < entry.minPlayers) continue;
			specialEntries.push({ id, name: entry.name, reminder: entry.reminder, isSpecial: true, inPlay: true, isDead: false, pos });
		}
		const all = [...charEntries, ...specialEntries];
		all.sort((a, b) => a.pos - b.pos);
		return all;
	}

	const firstNightOrder = $derived(buildNightOrder('first'));
	const otherNightOrder = $derived(buildNightOrder('other'));
	let manualNight = $state<'first' | 'other'>('first');
	const activeNight = $derived(activeRound !== undefined ? (activeRound === 1 ? 'first' : 'other') : manualNight);
	const nightToggleDisabled = $derived(activeRound !== undefined);
	const activeOrder = $derived(activeNight === 'first' ? firstNightOrder : otherNightOrder);
	const specialIcons: Record<string, string> = { dusk: '/night-dusk.webp', dawn: '/night-dawn.webp', minioninfo: '/night-minioninfo.webp', demoninfo: '/night-demoninfo.webp' };
	function handlePrint() { window.print(); }
</script>

<div class="space-y-4">
	<h2 class="print-title hidden text-xl font-bold">{activeNight === 'first' ? 'First Night' : 'Other Nights'}</h2>
	<div class="no-print flex items-center justify-between">
		<div class="flex items-center gap-3">
			<div class="flex gap-1 rounded-lg bg-element p-1">
				<button onclick={() => (manualNight = 'first')} disabled={nightToggleDisabled} class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors {activeNight === 'first' ? 'bg-surface text-primary shadow-sm' : 'text-secondary hover:text-medium'} {nightToggleDisabled ? 'cursor-default' : ''}">First Night</button>
				<button onclick={() => (manualNight = 'other')} disabled={nightToggleDisabled} class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors {activeNight === 'other' ? 'bg-surface text-primary shadow-sm' : 'text-secondary hover:text-medium'} {nightToggleDisabled ? 'cursor-default' : ''}">Other Nights</button>
			</div>
			<button onclick={() => (showAll = !showAll)} class="flex items-center gap-1.5 text-xs text-secondary">
				<div class="flex h-5 w-5 shrink-0 items-center justify-center rounded border transition-colors {showAll ? 'border-green-500 bg-green-500' : 'border-border-strong'}">
					{#if showAll}<svg class="h-3 w-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" /></svg>{/if}
				</div>
				Show all
			</button>
		</div>
		<button onclick={handlePrint} class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-sm text-secondary transition-colors hover:bg-hover hover:text-medium">
			<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 17h2a2 2 0 002-2v-4a2 2 0 00-2-2H5a2 2 0 00-2 2v4a2 2 0 002 2h2m2 4h6a2 2 0 002-2v-4a2 2 0 00-2-2H9a2 2 0 00-2 2v4a2 2 0 002 2zm8-12V5a2 2 0 00-2-2H9a2 2 0 00-2 2v4h10z" /></svg>
			Print
		</button>
	</div>

	<div class="space-y-1">
		{#if activeOrder.length === 0}
			<p class="py-8 text-center text-sm text-muted">No characters with night actions selected.</p>
		{:else}
			{#each activeOrder as entry, i (entry.id)}
				{@const isDone = completedActions?.has(entry.id) ?? false}
				{#if entry.isSpecial}
					{@const isInteractive = !NON_INTERACTIVE_SPECIALS.has(entry.id)}
					<div class="overflow-hidden rounded-lg" data-entry={entry.id}>
						<div
							{...(isInteractive ? panProps(entry.id) : {})}
							style="transform: {isInteractive ? swipeTransform(entry.id) : 'translate3d(0,0,0)'}; transition: {isInteractive ? swipeTransition(entry.id) : 'none'}"
							class="relative flex items-center gap-3 bg-element/50 px-3 py-2.5 {isInteractive && isDone ? 'opacity-50 border-l-4 border-l-green-500' : ''}"
						>
							<img src={specialIcons[entry.id]} alt="" class="h-20 w-20 shrink-0 object-contain" onerror={(e: Event) => ((e.target as HTMLImageElement).style.display = 'none')} />
							<div class="min-w-0 flex-1">
								<span class="text-base font-bold text-primary {isInteractive && isDone ? 'line-through' : ''}">{entry.name}</span>
								<p class="text-sm text-muted">{@html formatReminder(entry.reminder)}</p>
							</div>
							{#if ontoggle && isInteractive}
								<button onclick={() => { const done = completedActions?.has(entry.id) ?? false; ontoggle?.(entry.id, !done); }} class="no-print flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-2 transition-colors {isDone ? 'border-green-500 bg-green-500 text-white' : 'border-border-strong text-transparent hover:border-green-400'}" title={isDone ? 'Mark as not done' : 'Mark as done'}>
									<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" /></svg>
								</button>
							{/if}
							<span class="w-6 shrink-0 text-center text-xs font-bold text-muted">{i + 1}</span>
						</div>
					</div>
				{:else}
					{@const leftSwipeAction = ondeath ? (entry.isDead ? () => onundodeath?.(entry.id) : () => ondeath?.(entry.id)) : undefined}
					<div class="relative overflow-hidden rounded-lg" data-entry={entry.id}>
						{#if panState?.id === entry.id && panState.dx !== 0}
							{#if panState.dx > 0}
								<div class="absolute inset-0 flex items-center rounded-lg bg-green-500/20 pl-4">
									<svg class="h-6 w-6 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" /></svg>
								</div>
							{:else if entry.isDead}
								<div class="absolute inset-0 flex items-center justify-end rounded-lg bg-green-500/20 pr-4">
									<svg class="h-6 w-6 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z" /></svg>
								</div>
							{:else}
								<div class="absolute inset-0 flex items-center justify-end rounded-lg bg-red-500/20 pr-4">
									<svg class="h-6 w-6 text-red-500" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C7.58 2 4 5.58 4 10c0 2.76 1.34 5.2 3.4 6.72V20a1 1 0 001 1h7.2a1 1 0 001-1v-3.28C18.66 15.2 20 12.76 20 10c0-4.42-3.58-8-8-8zm-1 15v-2h2v2h-2zm4-7a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm-5 0a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0z" /></svg>
								</div>
							{/if}
						{/if}
						<div
							{...panProps(entry.id, leftSwipeAction)}
							style="transform: {swipeTransform(entry.id)}; transition: {swipeTransition(entry.id)}"
							class="card-slate relative flex items-center gap-3 border px-3 py-2.5 {isDone ? 'opacity-50 border-l-4 border-l-green-500' : ''} {entry.isDead ? (unselectedColors[entry.team ?? 0] ?? 'border-border/50') + ' opacity-40 border-dashed' : entry.inPlay ? (teamCardColors[entry.team ?? 0] ?? 'border-border') : (unselectedColors[entry.team ?? 0] ?? 'border-border/50') + ' opacity-40 border-dashed'}"
							data-team={teamDataAttr[entry.team ?? 0] ?? ''}
						>
							<img src="/characters/{entry.edition}/{entry.id}{iconSuffix(entry.team ?? 0)}.webp" alt="" class="h-20 w-20 shrink-0 rounded-full {entry.isDead ? 'grayscale' : ''}" onerror={(e: Event) => ((e.target as HTMLImageElement).style.display = 'none')} />
							<div class="min-w-0 flex-1">
								<span class="text-base font-medium {isDone ? 'line-through ' : ''}{entry.isDead ? 'line-through text-muted' : (teamNameColors[entry.team ?? 0] ?? 'text-primary')}">{entry.name}</span>
								{#if entry.isDead}<span class="ml-2 text-xs text-red-500 dark:text-red-400">Dead</span>{/if}
								<p class="text-sm {entry.isDead ? 'text-muted' : 'text-secondary'}">{@html formatReminder(entry.reminder)}</p>
							</div>
							<div class="no-print flex shrink-0 items-center gap-1">
								{#if ondeath && !entry.isDead}
									<button onclick={() => ondeath?.(entry.id)} class="rounded p-1 text-muted transition-colors hover:bg-hover hover:text-red-500" title="Mark as dead">
										<svg class="h-4 w-4" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C7.58 2 4 5.58 4 10c0 2.76 1.34 5.2 3.4 6.72V20a1 1 0 001 1h7.2a1 1 0 001-1v-3.28C18.66 15.2 20 12.76 20 10c0-4.42-3.58-8-8-8zm-1 15v-2h2v2h-2zm4-7a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm-5 0a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0z" /></svg>
									</button>
								{:else if onundodeath && entry.isDead}
									<button onclick={() => onundodeath?.(entry.id)} class="rounded p-1 text-red-400 transition-colors hover:bg-hover hover:text-green-500" title="Undo death">
										<svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 2C7.58 2 4 5.58 4 10c0 2.76 1.34 5.2 3.4 6.72V20a1 1 0 001 1h7.2a1 1 0 001-1v-3.28C18.66 15.2 20 12.76 20 10c0-4.42-3.58-8-8-8zm-2 15v-1h4v1h-4zm0-3h1v2h2v-2h1v2h-4zm5.6-2.08l-.6.46V17h-6v-2.62l-.6-.46A5.94 5.94 0 016 10c0-3.31 2.69-6 6-6s6 2.69 6 6a5.94 5.94 0 01-2.4 3.92z" /><line x1="4" y1="4" x2="20" y2="20" stroke-width="2" /></svg>
									</button>
								{/if}
								<a href="/almanac/{entry.id}?from={encodeURIComponent(page.url.pathname + page.url.search)}" class="rounded p-1 text-muted transition-colors hover:bg-hover hover:text-medium" title="Almanac">
									<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" /></svg>
								</a>
								<a href="https://wiki.bloodontheclocktower.com/{entry.name.replace(/ /g, '_')}" target="_blank" rel="noopener" class="rounded p-1 text-muted transition-colors hover:bg-hover hover:text-medium" title="Wiki">
									<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
								</a>
							</div>
							{#if ontoggle}
								<button onclick={() => { const done = completedActions?.has(entry.id) ?? false; ontoggle?.(entry.id, !done); }} class="no-print flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-2 transition-colors {isDone ? 'border-green-500 bg-green-500 text-white' : 'border-border-strong text-transparent hover:border-green-400'}" title={isDone ? 'Mark as not done' : 'Mark as done'}>
									<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" /></svg>
								</button>
							{/if}
							<span class="w-6 shrink-0 text-center text-xs font-medium text-muted">{i + 1}</span>
						</div>
					</div>
				{/if}
			{/each}
		{/if}
	</div>
</div>
