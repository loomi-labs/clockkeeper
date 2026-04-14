<script lang="ts">
  import { untrack } from "svelte";
  import { page } from "$app/state";
  import { sidebar, toggleSidebar } from "~/lib/sidebar.svelte";
  import { sidebarData } from "~/lib/sidebar-data.svelte";
  import { client } from "~/lib/api";
  import { GameState } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import type {
    Script,
    GameSummary,
  } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";

  let { mobileOpen = $bindable(false) }: { mobileOpen?: boolean } = $props();

  let scripts = $state<Script[]>([]);
  let games = $state<GameSummary[]>([]);

  const activeGames = $derived(
    games.filter(
      (g) => g.state === GameState.SETUP || g.state === GameState.IN_PROGRESS,
    ),
  );

  const scriptsOpen = $derived(page.url.pathname.startsWith("/scripts"));
  const gamesOpen = $derived(
    page.url.pathname.startsWith("/games") || page.url.pathname === "/",
  );

  function isItemActive(href: string): boolean {
    return page.url.pathname === href;
  }

  function closeMobile() {
    mobileOpen = false;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") closeMobile();
  }

  function stateColor(state: number): string {
    switch (state) {
      case GameState.SETUP:
        return "bg-yellow-400";
      case GameState.IN_PROGRESS:
        return "bg-green-400";
      default:
        return "bg-element";
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
  {#if pattern === "/"}
    <!-- Home -->
    <svg
      class="h-5 w-5 shrink-0"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="1.5"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        d="m2.25 12 8.954-8.955c.44-.439 1.152-.439 1.591 0L21.75 12M4.5 9.75v10.125c0 .621.504 1.125 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21h4.125c.621 0 1.125-.504 1.125-1.125V9.75M8.25 21h8.25"
      />
    </svg>
  {:else if pattern === "/scripts"}
    <!-- Book/Scripts -->
    <svg
      class="h-5 w-5 shrink-0"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="1.5"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25"
      />
    </svg>
  {:else if pattern === "/games"}
    <!-- Play/New Game -->
    <svg
      class="h-5 w-5 shrink-0"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="1.5"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.347a1.125 1.125 0 010 1.972l-11.54 6.347a1.125 1.125 0 01-1.667-.986V5.653z"
      />
    </svg>
  {:else if pattern === "/almanac"}
    <!-- Almanac -->
    <svg
      class="h-5 w-5 shrink-0"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="1.5"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25M15 6.75a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm-6 0a.75.75 0 11-1.5 0 .75.75 0 011.5 0z"
      />
    </svg>
  {/if}
{/snippet}

{#snippet navContent(showLabel: boolean, onclick?: () => void)}
  <!-- Home -->
  <a
    href="/"
    {onclick}
    class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors
			{isItemActive('/')
      ? 'bg-hover text-primary font-medium'
      : 'text-secondary hover:bg-hover hover:text-primary'}
			{showLabel ? '' : 'justify-center'}"
  >
    {@render navIcon("/")}
    {#if showLabel}<span>Home</span>{/if}
  </a>

  <!-- Games (collapsible) -->
  <div>
    <a
      href="/games"
      {onclick}
      class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors
				{gamesOpen
        ? 'bg-hover text-primary font-medium'
        : 'text-secondary hover:bg-hover hover:text-primary'}
				{showLabel ? '' : 'justify-center'}"
    >
      {@render navIcon("/games")}
      {#if showLabel}<span>Games</span>{/if}
    </a>
    {#if showLabel && gamesOpen && activeGames.length > 0}
      <div class="ml-5 space-y-0.5 border-l border-border pl-3">
        {#each activeGames as game (game.id)}
          {@const href = `/games/${game.id}`}
          <a
            {href}
            {onclick}
            class="flex items-center gap-1.5 rounded-lg px-2 py-1 text-xs transition-colors
							{isItemActive(href)
              ? 'bg-hover text-primary font-medium'
              : 'text-secondary hover:bg-hover hover:text-primary'}"
          >
            <span
              class="h-1.5 w-1.5 shrink-0 rounded-full {stateColor(game.state)}"
            ></span>
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
      {onclick}
      class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors
				{scriptsOpen
        ? 'bg-hover text-primary font-medium'
        : 'text-secondary hover:bg-hover hover:text-primary'}
				{showLabel ? '' : 'justify-center'}"
    >
      {@render navIcon("/scripts")}
      {#if showLabel}<span>Scripts</span>{/if}
    </a>
    {#if showLabel && scriptsOpen && scripts.length > 0}
      <div class="ml-5 space-y-0.5 border-l border-border pl-3">
        {#each scripts as script (script.id)}
          {@const href = `/scripts/${script.id}`}
          <a
            {href}
            {onclick}
            class="block truncate rounded-lg px-2 py-1 text-xs transition-colors
							{isItemActive(href)
              ? 'bg-hover text-primary font-medium'
              : 'text-secondary hover:bg-hover hover:text-primary'}"
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
    {onclick}
    class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors
			{page.url.pathname.startsWith('/almanac')
      ? 'bg-hover text-primary font-medium'
      : 'text-secondary hover:bg-hover hover:text-primary'}
			{showLabel ? '' : 'justify-center'}"
  >
    {@render navIcon("/almanac")}
    {#if showLabel}<span>Almanac</span>{/if}
  </a>
{/snippet}

<!-- Desktop sidebar (md+) -->
<aside
  class="nav-sidebar card-slate fixed top-0 left-0 bottom-0 z-30 hidden flex-col border-r border-border bg-surface transition-[width] duration-200 md:flex
		{sidebar.expanded ? 'w-48' : 'w-14'}"
>
  <nav class="flex-1 space-y-1 overflow-y-auto p-2">
    {@render navContent(sidebar.expanded)}
  </nav>

  <div class="border-t border-border px-3 py-2">
    <a
      href="https://github.com/loomi-labs/clockkeeper"
      target="_blank"
      rel="noopener noreferrer"
      class="flex items-center gap-2 rounded-lg px-1 py-1 text-secondary transition-colors hover:text-primary {sidebar.expanded ? '' : 'justify-center'}"
      aria-label="GitHub repository"
    >
      <svg class="h-5 w-5 shrink-0" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
      </svg>
      {#if sidebar.expanded}
        <span class="text-xs">GitHub</span>
      {/if}
    </a>
    {#if sidebar.expanded}
      <a
        href="https://bloodontheclocktower.com/pages/community-created-content-policy"
        target="_blank"
        rel="noopener noreferrer"
        class="mt-1 block px-1 text-[10px] leading-tight text-tertiary transition-colors hover:text-secondary"
      >
        Unofficial fan project. Not affiliated with The Pandemonium Institute.
      </a>
    {/if}
  </div>

  <button
    onclick={toggleSidebar}
    class="flex items-center justify-center border-t border-border p-3 text-secondary transition-colors hover:bg-hover hover:text-primary"
    aria-label={sidebar.expanded ? "Collapse sidebar" : "Expand sidebar"}
  >
    <svg
      class="h-4 w-4 transition-transform duration-200 {sidebar.expanded
        ? ''
        : 'rotate-180'}"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="2"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        d="M15.75 19.5L8.25 12l7.5-7.5"
      />
    </svg>
  </button>
</aside>

<!-- Mobile overlay (below md) -->
{#if mobileOpen}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-40 md:hidden" onkeydown={handleKeydown}>
    <!-- Backdrop -->
    <button
      class="absolute inset-0 bg-black/40"
      onclick={closeMobile}
      aria-label="Close menu"
      tabindex="-1"
    ></button>

    <!-- Sidebar panel -->
    <aside
      class="nav-sidebar card-slate absolute top-0 left-0 bottom-0 flex w-56 flex-col overflow-y-auto border-r border-border bg-surface shadow-lg"
    >
      <div
        class="flex items-center justify-between border-b border-border px-4 py-3"
      >
        <span class="text-sm font-semibold text-primary">Menu</span>
        <button
          onclick={closeMobile}
          class="rounded-lg p-1 text-secondary transition-colors hover:bg-hover hover:text-primary"
          aria-label="Close menu"
        >
          <svg
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="2"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      </div>
      <nav class="flex-1 space-y-1 p-2">
        {@render navContent(true, closeMobile)}
      </nav>
      <div class="border-t border-border px-4 py-3">
        <a
          href="https://github.com/loomi-labs/clockkeeper"
          target="_blank"
          rel="noopener noreferrer"
          class="flex items-center gap-2 rounded-lg px-1 py-1 text-secondary transition-colors hover:text-primary"
          aria-label="GitHub repository"
        >
          <svg class="h-5 w-5 shrink-0" viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/>
          </svg>
          <span class="text-xs">GitHub</span>
        </a>
        <a
          href="https://bloodontheclocktower.com/pages/community-created-content-policy"
          target="_blank"
          rel="noopener noreferrer"
          class="mt-1 block px-1 text-[10px] leading-tight text-tertiary transition-colors hover:text-secondary"
        >
          Unofficial fan project. Not affiliated with The Pandemonium Institute.
        </a>
      </div>
    </aside>
  </div>
{/if}
