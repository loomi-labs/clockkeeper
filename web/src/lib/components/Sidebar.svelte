<script lang="ts">
	import { untrack } from 'svelte';
	import { page } from '$app/state';
	import { sidebar, toggleSidebar } from '~/lib/sidebar.svelte';
	import { sidebarData } from '~/lib/sidebar-data.svelte';
	import { client } from '~/lib/api';
	import { GameState } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import type { Script, GameSummary } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';

	let { mobileOpen = $bindable(false) }: { mobileOpen?: boolean } = $props();

	let scripts = $state<Script[]>([]);
	let games = $state<GameSummary[]>([]);

	const activeGames = $derived(
		games.filter((g) => g.state === GameState.SETUP || g.state === GameState.IN_PROGRESS)
	);

	const scriptsOpen = $derived(page.url.pathname.startsWith('/scripts'));
	const gamesOpen = $derived(page.url.pathname.startsWith('/games') || page.url.pathname === '/');

	function isItemActive(href: string): boolean {
		return page.url.pathname === href;
	}

	function closeMobile() {
		mobileOpen = false;
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') closeMobile();
	}

	function stateColor(state: number): string {
		switch (state) {
			case GameState.SETUP: return 'bg-yellow-400';
			case GameState.IN_PROGRESS: return 'bg-green-400';
			default: return 'bg-element';
		}
	}

	async function fetchData() {
		try {
			const [scriptResp, gameResp] = await Promise.all([
				client.listScripts({}),
				client.listGames({}),
			]);
			scripts = scriptResp.scripts;
			games = gameResp.games;
		} catch {
			// Sidebar items simply won't appear
		}
	}

	$effect(() => {
		void sidebarData.version;
		untrack(() => fetchData());
	});
</script>

{#snippet navIcon(pattern: string)}
	{#if pattern === '/'}
		<!-- Home -->
		<svg class="h-5 w-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
			<path stroke-linecap="round" stroke-linejoin="round" d="m2.25 12 8.954-8.955c.44-.439 1.152-.439 1.591 0L21.75 12M4.5 9.75v10.125c0 .621.504 1.125 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21h4.125c.621 0 1.125-.504 1.125-1.125V9.75M8.25 21h8.25" />
		</svg>
	{:else if pattern === '/scripts'}
		<!-- Book/Scripts -->
		<svg class="h-5 w-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
			<path stroke-linecap="round" stroke-linejoin="round" d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25" />
		</svg>
	{:else if pattern === '/games'}
		<!-- Play/New Game -->
		<svg class="h-5 w-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
			<path stroke-linecap="round" stroke-linejoin="round" d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.347a1.125 1.125 0 010 1.972l-11.54 6.347a1.125 1.125 0 01-1.667-.986V5.653z" />
		</svg>
	{:else if pattern === '/almanac'}
		<!-- Almanac -->
		<svg class="h-5 w-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
			<path stroke-linecap="round" stroke-linejoin="round" d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25M15 6.75a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm-6 0a.75.75 0 11-1.5 0 .75.75 0 011.5 0z" />
		</svg>
	{/if}
{/snippet}

{#snippet navContent(showLabel: boolean, onclick?: () => void)}
	<!-- Home -->
	<a
		href="/"
		onclick={onclick}
		class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors
			{isItemActive('/') ? 'bg-hover text-primary font-medium' : 'text-secondary hover:bg-hover hover:text-primary'}
			{showLabel ? '' : 'justify-center'}"
	>
		{@render navIcon('/')}
		{#if showLabel}<span>Home</span>{/if}
	</a>

	<!-- Games (collapsible) -->
	<div>
		<a
			href="/games/new"
			onclick={onclick}
			class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors
				{gamesOpen ? 'bg-hover text-primary font-medium' : 'text-secondary hover:bg-hover hover:text-primary'}
				{showLabel ? '' : 'justify-center'}"
		>
			{@render navIcon('/games')}
			{#if showLabel}<span>Games</span>{/if}
		</a>
		{#if showLabel && gamesOpen && activeGames.length > 0}
			<div class="ml-5 space-y-0.5 border-l border-border pl-3">
				{#each activeGames as game (game.id)}
					{@const href = `/games/${game.id}`}
					<a
						{href}
						onclick={onclick}
						class="flex items-center gap-1.5 rounded-lg px-2 py-1 text-xs transition-colors
							{isItemActive(href) ? 'bg-hover text-primary font-medium' : 'text-secondary hover:bg-hover hover:text-primary'}"
					>
						<span class="h-1.5 w-1.5 shrink-0 rounded-full {stateColor(game.state)}"></span>
						<span class="truncate">{game.name || game.scriptName}</span>
					</a>
				{/each}
			</div>
		{/if}
	</div>

	<!-- Scripts (collapsible) -->
	<div>
		<a
			href="/scripts"
			onclick={onclick}
			class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors
				{scriptsOpen ? 'bg-hover text-primary font-medium' : 'text-secondary hover:bg-hover hover:text-primary'}
				{showLabel ? '' : 'justify-center'}"
		>
			{@render navIcon('/scripts')}
			{#if showLabel}<span>Scripts</span>{/if}
		</a>
		{#if showLabel && scriptsOpen && scripts.length > 0}
			<div class="ml-5 space-y-0.5 border-l border-border pl-3">
				{#each scripts as script (script.id)}
					{@const href = `/scripts/${script.id}`}
					<a
						{href}
						onclick={onclick}
						class="block truncate rounded-lg px-2 py-1 text-xs transition-colors
							{isItemActive(href) ? 'bg-hover text-primary font-medium' : 'text-secondary hover:bg-hover hover:text-primary'}"
					>
						{script.name}
					</a>
				{/each}
			</div>
		{/if}
	</div>

	<!-- Almanac -->
	<a
		href="/almanac"
		onclick={onclick}
		class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors
			{page.url.pathname.startsWith('/almanac') ? 'bg-hover text-primary font-medium' : 'text-secondary hover:bg-hover hover:text-primary'}
			{showLabel ? '' : 'justify-center'}"
	>
		{@render navIcon('/almanac')}
		{#if showLabel}<span>Almanac</span>{/if}
	</a>
{/snippet}

<!-- Desktop sidebar (md+) -->
<aside
	class="nav-sidebar card-slate fixed top-[57px] left-0 bottom-0 z-30 hidden flex-col border-r border-border bg-surface transition-[width] duration-200 md:flex
		{sidebar.expanded ? 'w-48' : 'w-14'}"
>
	<nav class="flex-1 space-y-1 overflow-y-auto p-2">
		{@render navContent(sidebar.expanded)}
	</nav>

	<button
		onclick={toggleSidebar}
		class="flex items-center justify-center border-t border-border p-3 text-secondary transition-colors hover:bg-hover hover:text-primary"
		aria-label={sidebar.expanded ? 'Collapse sidebar' : 'Expand sidebar'}
	>
		<svg
			class="h-4 w-4 transition-transform duration-200 {sidebar.expanded ? '' : 'rotate-180'}"
			fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
		>
			<path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" />
		</svg>
	</button>
</aside>

<!-- Mobile overlay (below md) -->
{#if mobileOpen}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-40 md:hidden"
		onkeydown={handleKeydown}
	>
		<!-- Backdrop -->
		<button
			class="absolute inset-0 bg-black/40"
			onclick={closeMobile}
			aria-label="Close menu"
			tabindex="-1"
		></button>

		<!-- Sidebar panel -->
		<aside class="nav-sidebar card-slate absolute top-0 left-0 bottom-0 w-56 overflow-y-auto border-r border-border bg-surface shadow-lg">
			<div class="flex items-center justify-between border-b border-border px-4 py-3">
				<span class="text-sm font-semibold text-primary">Menu</span>
				<button
					onclick={closeMobile}
					class="rounded-lg p-1 text-secondary transition-colors hover:bg-hover hover:text-primary"
					aria-label="Close menu"
				>
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
					</svg>
				</button>
			</div>
			<nav class="space-y-1 p-2">
				{@render navContent(true, closeMobile)}
			</nav>
		</aside>
	</div>
{/if}
