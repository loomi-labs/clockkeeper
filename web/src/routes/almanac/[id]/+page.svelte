<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { client } from '~/lib/api';
	import { Team } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import type { Character } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import ReminderToken from '~/lib/components/ReminderToken.svelte';
	import { formatReminder } from '~/lib/format';

	let character = $state<Character | null>(null);
	let loading = $state(true);
	let error = $state('');

	const backUrl = $derived.by(() => {
		const from = page.url.searchParams.get('from');
		if (from && from.startsWith('/') && !from.startsWith('//')) return from;
		return '/almanac';
	});
	const backLabel = $derived(backUrl.startsWith('/games/') ? 'Back to Game' : 'Back to Almanac');

	const teamLabels: Record<number, string> = {
		[Team.TOWNSFOLK]: 'Townsfolk',
		[Team.OUTSIDER]: 'Outsider',
		[Team.MINION]: 'Minion',
		[Team.DEMON]: 'Demon',
		[Team.TRAVELLER]: 'Traveller',
		[Team.FABLED]: 'Fabled',
		[Team.LORIC]: 'Loric',
	};

	const teamBadgeColors: Record<number, string> = {
		[Team.TOWNSFOLK]: 'bg-blue-100 text-blue-700 dark:bg-blue-500/20 dark:text-blue-300',
		[Team.OUTSIDER]: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-500/20 dark:text-cyan-300',
		[Team.MINION]: 'bg-orange-100 text-orange-700 dark:bg-orange-500/20 dark:text-orange-300',
		[Team.DEMON]: 'bg-red-100 text-red-700 dark:bg-red-500/20 dark:text-red-300',
		[Team.TRAVELLER]: 'bg-gradient-to-r from-blue-100 to-red-100 text-blue-700 dark:from-blue-500/20 dark:to-red-500/20 dark:text-blue-300',
		[Team.FABLED]: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-500/20 dark:text-yellow-300',
		[Team.LORIC]: 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-300',
	};

	const teamHeaderColors: Record<number, string> = {
		[Team.TOWNSFOLK]: 'text-blue-600 dark:text-blue-400',
		[Team.OUTSIDER]: 'text-cyan-600 dark:text-cyan-400',
		[Team.MINION]: 'text-orange-600 dark:text-orange-400',
		[Team.DEMON]: 'text-red-600 dark:text-red-400',
		[Team.TRAVELLER]: 'bg-gradient-to-r from-blue-600 to-red-600 bg-clip-text text-transparent dark:from-blue-400 dark:to-red-400',
		[Team.FABLED]: 'text-yellow-500 dark:text-yellow-400',
		[Team.LORIC]: 'text-green-600 dark:text-green-400',
	};

	const editionLabels: Record<string, string> = {
		tb: 'Trouble Brewing',
		bmr: 'Bad Moon Rising',
		snv: 'Sects & Violets',
		carousel: 'Carousel',
		fabled: 'Fabled',
		loric: 'Loric',
	};

	function iconSuffix(team: number): string {
		if (team === Team.TOWNSFOLK || team === Team.OUTSIDER) return '_g';
		if (team === Team.MINION || team === Team.DEMON) return '_e';
		return '';
	}

	function wikiUrl(name: string): string {
		return `https://wiki.bloodontheclocktower.com/${encodeURIComponent(name.replace(/ /g, '_'))}`;
	}

	onMount(async () => {
		try {
			const resp = await client.getCharacter({ id: page.params.id });
			character = resp.character ?? null;
		} catch {
			error = 'Character not found.';
		}
		loading = false;
	});
</script>

<svelte:head>
	<title>{character?.name ?? 'Character'} — Almanac — Clock Keeper</title>
</svelte:head>

<div class="py-4">
	<a href={backUrl} class="inline-flex items-center gap-1 text-sm text-secondary hover:text-primary">
		<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" /></svg>
		{backLabel}
	</a>

	{#if loading}
		<p class="py-12 text-center text-sm text-muted">Loading...</p>
	{:else if error}
		<p class="py-12 text-center text-sm text-red-500">{error}</p>
	{:else if character}
		{@const c = character}
		<div class="mt-6 space-y-6">
			<!-- Header -->
			<div class="flex items-start gap-5">
				<img
					src="/characters/{c.edition}/{c.id}{iconSuffix(c.team)}.webp"
					alt=""
					class="h-64 w-64 shrink-0 rounded-full"
					onerror={(e: Event) => ((e.target as HTMLImageElement).style.display = 'none')}
				/>
				<div>
					<h1 class="text-2xl font-bold {teamHeaderColors[c.team] ?? 'text-primary'}">{c.name}</h1>
					<div class="mt-1.5 flex flex-wrap items-center gap-2">
						<span class="rounded px-2 py-0.5 text-xs font-medium {teamBadgeColors[c.team] ?? 'bg-element text-secondary'}">
							{teamLabels[c.team] ?? ''}
						</span>
						<span class="rounded bg-element px-2 py-0.5 text-xs text-secondary">
							{editionLabels[c.edition] ?? c.edition}
						</span>
						{#if c.setup}
							<span class="rounded bg-yellow-100 px-2 py-0.5 text-xs text-yellow-700 dark:bg-yellow-500/20 dark:text-yellow-300">setup</span>
						{/if}
					</div>
				</div>
			</div>

			<!-- Flavor -->
			{#if c.flavor}
				<p class="text-sm italic text-muted">&ldquo;{c.flavor}&rdquo;</p>
			{/if}

			<!-- Ability -->
			<div>
				<h2 class="text-xs font-semibold uppercase tracking-wide text-secondary">Ability</h2>
				<p class="mt-1 text-primary">{@html formatReminder(c.ability)}</p>
			</div>

			<!-- Night Info -->
			{#if c.firstNightReminder || c.otherNightReminder}
				<div>
					<h2 class="text-xs font-semibold uppercase tracking-wide text-secondary">Night Actions</h2>
					<div class="mt-2 space-y-2">
						{#if c.firstNightReminder}
							<div class="rounded-lg border border-border bg-surface-alt p-3">
								<div class="flex items-center gap-2">
									<svg class="h-4 w-4 shrink-0 text-indigo-400" fill="currentColor" viewBox="0 0 20 20"><path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z" /></svg>
									<span class="text-sm font-medium text-secondary">First Night</span>
									{#if c.firstNight > 0}
										<span class="rounded bg-element px-1.5 py-0.5 text-xs text-muted">#{c.firstNight}</span>
									{/if}
								</div>
								<p class="mt-1 text-sm text-primary">{@html formatReminder(c.firstNightReminder)}</p>
							</div>
						{/if}
						{#if c.otherNightReminder}
							<div class="rounded-lg border border-border bg-surface-alt p-3">
								<div class="flex items-center gap-2">
									<svg class="h-4 w-4 shrink-0 text-amber-400" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4 8a4 4 0 11-8 0 4 4 0 018 0zm-.464 4.95l.707.707a1 1 0 001.414-1.414l-.707-.707a1 1 0 00-1.414 1.414zm2.12-10.607a1 1 0 010 1.414l-.706.707a1 1 0 11-1.414-1.414l.707-.707a1 1 0 011.414 0zM17 11a1 1 0 100-2h-1a1 1 0 100 2h1zm-7 4a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM5.05 6.464A1 1 0 106.465 5.05l-.708-.707a1 1 0 00-1.414 1.414l.707.707zm1.414 8.486l-.707.707a1 1 0 01-1.414-1.414l.707-.707a1 1 0 011.414 1.414zM4 11a1 1 0 100-2H3a1 1 0 000 2h1z" clip-rule="evenodd" /></svg>
									<span class="text-sm font-medium text-secondary">Other Nights</span>
									{#if c.otherNight > 0}
										<span class="rounded bg-element px-1.5 py-0.5 text-xs text-muted">#{c.otherNight}</span>
									{/if}
								</div>
								<p class="mt-1 text-sm text-primary">{@html formatReminder(c.otherNightReminder)}</p>
							</div>
						{/if}
					</div>
				</div>
			{/if}

			<!-- Reminders -->
			{#if c.reminders.length > 0 || c.remindersGlobal.length > 0}
				<div>
					<h2 class="text-xs font-semibold uppercase tracking-wide text-secondary">Reminder Tokens</h2>
					<div class="mt-2 flex flex-wrap gap-4">
						{#each c.reminders as reminder}
							<ReminderToken
								characterId={c.id}
								characterName={c.name}
								text={reminder}
								edition={c.edition}
								team={c.team}
							/>
						{/each}
						{#each c.remindersGlobal as reminder}
							<ReminderToken
								characterId={c.id}
								characterName={c.name}
								text="{reminder} (global)"
								edition={c.edition}
								team={c.team}
							/>
						{/each}
					</div>
				</div>
			{/if}

			<!-- Jinxes -->
			{#if c.jinxes.length > 0}
				<div>
					<h2 class="text-xs font-semibold uppercase tracking-wide text-secondary">Jinxes</h2>
					<div class="mt-2 space-y-2">
						{#each c.jinxes as jinx}
							<div class="flex items-start gap-3 rounded-lg border border-border bg-surface-alt p-3">
								<a href="/almanac/{jinx.characterId}?from={encodeURIComponent(backUrl)}" class="font-medium text-primary hover:underline shrink-0">{jinx.characterName}</a>
								<p class="text-sm text-secondary">{jinx.reason}</p>
							</div>
						{/each}
					</div>
				</div>
			{/if}

			<!-- Wiki link -->
			<div class="border-t border-border pt-4">
				<a
					href={wikiUrl(c.name)}
					target="_blank"
					rel="noopener noreferrer"
					class="inline-flex items-center gap-1.5 text-sm text-indigo-600 hover:text-indigo-500 dark:text-indigo-400 dark:hover:text-indigo-300"
				>
					View on Wiki
					<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
				</a>
			</div>
		</div>
	{/if}
</div>
