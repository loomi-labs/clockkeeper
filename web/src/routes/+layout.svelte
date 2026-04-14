<script lang="ts">
  import type { Snippet } from "svelte";
  import { onMount } from "svelte";
  import { page } from "$app/state";
  import {
    auth,
    initAuth,
    getToken,
    setToken,
    setAnonymous,
    getDiscordOAuthURL,
    logout,
  } from "~/lib/auth.svelte";
  import { rawClient } from "~/lib/api";
  import { initTheme } from "~/lib/theme";
  import { sidebar, initSidebar } from "~/lib/sidebar.svelte";
  import ThemeSwitcher from "~/lib/components/ThemeSwitcher.svelte";
  import Sidebar from "~/lib/components/Sidebar.svelte";
  import "~/app.css";

  let { children }: { children: Snippet } = $props();
  let mobileMenuOpen = $state(false);
  let initialized = $state(false);

  onMount(async () => {
    initAuth();
    initTheme();
    initSidebar();

    // Fetch auth config from server.
    try {
      const config = await rawClient.getAuthConfig({});
      auth.discordAvailable = !!config.discordClientId;
      auth.discordClientId = config.discordClientId;
    } catch {
      // Server may be unavailable — continue with defaults.
    }

    const isAuthPath =
      page.url.pathname.startsWith("/login") ||
      page.url.pathname.startsWith("/auth/");

    if (!getToken() && !isAuthPath) {
      // Auto-create anonymous session.
      try {
        const resp = await rawClient.createAnonymousSession({});
        setToken(resp.token);
        setAnonymous(true);
      } catch {
        // Session creation failed — app will show errors on API calls,
        // but don't redirect to login. User can retry or login with Discord.
      }
    }

    initialized = true;
  });
</script>

{#if page.url.pathname.startsWith("/login") || page.url.pathname.startsWith("/auth/")}
  {@render children()}
{:else if initialized && auth.isAuthenticated}
  <div class="min-h-dvh text-primary">
    <nav class="nav-bar border-b border-border bg-surface">
      <div class="flex items-center justify-between px-4 py-3">
        <div class="flex items-center gap-4">
          <button
            onclick={() => (mobileMenuOpen = !mobileMenuOpen)}
            class="rounded-lg p-1.5 text-secondary transition-colors hover:bg-hover hover:text-primary md:hidden"
            aria-label="Toggle menu"
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
                d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5"
              />
            </svg>
          </button>
          <a
            href="/"
            class="flex items-center gap-2 text-xl font-bold text-indigo-600 dark:text-indigo-400"
          >
            <img src="/logo.webp" alt="Clock Keeper" class="h-8 w-8 rounded" />
            <span class="hidden sm:inline">Clock Keeper</span>
          </a>
        </div>
        <div class="flex items-center gap-2">
          <ThemeSwitcher />
          {#if auth.isAnonymous}
            {#if auth.discordAvailable}
              <a
                href={getDiscordOAuthURL()}
                class="rounded-lg bg-[#5865F2] px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-[#4752C4]"
              >
                Login with Discord
              </a>
            {/if}
          {:else}
            <button
              onclick={logout}
              class="rounded-lg px-3 py-1.5 text-sm text-secondary transition-colors hover:bg-hover hover:text-primary"
            >
              Logout
            </button>
          {/if}
        </div>
      </div>
    </nav>
    <Sidebar bind:mobileOpen={mobileMenuOpen} />
    <main
      class="transition-[margin-left] duration-200 {sidebar.expanded
        ? 'md:ml-48'
        : 'md:ml-14'}"
    >
      <div class="mx-auto max-w-screen-xl p-4">
        {@render children()}
      </div>
    </main>
  </div>
{/if}
