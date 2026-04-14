<script lang="ts">
  import { goto } from "$app/navigation";
  import { onMount } from "svelte";
  import { rawClient } from "~/lib/api";
  import {
    auth,
    setToken,
    setAnonymous,
    getDiscordOAuthURL,
  } from "~/lib/auth.svelte";
  import ThemeSwitcher from "~/lib/components/ThemeSwitcher.svelte";
  import { getErrorMessage } from "~/lib/errors";
  import { initTheme } from "~/lib/theme";

  let error = $state("");
  let loading = $state(false);

  onMount(async () => {
    initTheme();

    // Fetch auth config if not already loaded.
    if (!auth.discordClientId) {
      try {
        const config = await rawClient.getAuthConfig({});
        auth.discordAvailable = !!config.discordClientId;
        auth.discordClientId = config.discordClientId;
      } catch {
        // Continue without Discord.
      }
    }
  });

  async function handleContinueAnonymous() {
    error = "";
    loading = true;

    try {
      const resp = await rawClient.createAnonymousSession({});
      setToken(resp.token);
      setAnonymous(true);
      goto("/");
    } catch (err) {
      error = getErrorMessage(err, "Failed to create session");
    } finally {
      loading = false;
    }
  }
</script>

<div class="relative flex min-h-screen items-center justify-center">
  <div class="absolute right-4 top-4">
    <ThemeSwitcher />
  </div>
  <div class="card-slate w-full max-w-sm rounded-xl bg-surface p-8 shadow-lg">
    <h1 class="mb-2 text-center text-2xl font-bold text-indigo-600">
      Clock Keeper
    </h1>
    <p class="mb-6 text-center text-sm text-secondary">
      Digital companion for Blood on the Clocktower
    </p>

    {#if error}
      <div class="mb-4 rounded-lg bg-error-bg px-4 py-2 text-sm text-error-text">
        {error}
      </div>
    {/if}

    <div class="space-y-3">
      {#if auth.discordAvailable}
        <a
          href={getDiscordOAuthURL()}
          class="flex w-full items-center justify-center gap-2 rounded-lg bg-[#5865F2] px-4 py-2.5 font-medium text-white transition-colors hover:bg-[#4752C4]"
        >
          <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
            <path
              d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028 14.09 14.09 0 0 0 1.226-1.994.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03z"
            />
          </svg>
          Login with Discord
        </a>

        <div class="relative">
          <div class="absolute inset-0 flex items-center">
            <div class="w-full border-t border-border"></div>
          </div>
          <div class="relative flex justify-center text-sm">
            <span class="bg-surface px-2 text-secondary">or</span>
          </div>
        </div>
      {/if}

      <button
        onclick={handleContinueAnonymous}
        disabled={loading}
        class="w-full rounded-lg border border-border px-4 py-2.5 text-sm font-medium text-secondary transition-colors hover:bg-hover hover:text-primary disabled:opacity-50"
      >
        {loading ? "Creating session..." : "Continue without account"}
      </button>
    </div>
  </div>
</div>
