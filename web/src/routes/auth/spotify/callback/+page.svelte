<script lang="ts">
  import { onMount } from "svelte";
  import { goto } from "$app/navigation";
  import { page } from "$app/state";
  import { client } from "~/lib/api";
  import {
    applySpotifyStatus,
    takeSpotifyReturnTo,
    validateSpotifyOAuthState,
  } from "~/lib/spotify.svelte";
  import { getErrorMessage } from "~/lib/errors";
  import { initTheme } from "~/lib/theme";

  let error = $state("");

  onMount(async () => {
    initTheme();

    const denied = page.url.searchParams.get("error");
    if (denied) {
      error =
        denied === "access_denied"
          ? "Spotify access was declined. You can connect again from the music panel."
          : `Spotify returned an error: ${denied}`;
      return;
    }

    const state = page.url.searchParams.get("state");
    if (!validateSpotifyOAuthState(state)) {
      error = "Invalid OAuth state. Please start the Spotify connection again.";
      return;
    }

    const code = page.url.searchParams.get("code");
    if (!code) {
      error = "No authorization code received from Spotify.";
      return;
    }

    const redirectUri = `${window.location.origin}/auth/spotify/callback`;

    try {
      const resp = await client.connectSpotify({ code, redirectUri });
      applySpotifyStatus(resp.status);
      goto(takeSpotifyReturnTo(), { replaceState: true });
    } catch (err) {
      error = getErrorMessage(err, "Connecting Spotify failed");
    }
  });
</script>

<div class="flex min-h-screen items-center justify-center">
  <div class="card-slate w-full max-w-sm rounded-xl bg-surface p-8 shadow-lg">
    {#if error}
      <h1 class="mb-4 text-center text-xl font-bold text-error-text">
        Spotify Connection Failed
      </h1>
      <p class="mb-4 text-center text-sm text-secondary">{error}</p>
      <a
        href="/"
        class="block w-full rounded-lg border border-border px-4 py-2 text-center text-sm font-medium text-secondary transition-colors hover:bg-hover hover:text-primary"
      >
        Back to Clock Keeper
      </a>
    {:else}
      <p class="text-center text-secondary">Connecting Spotify...</p>
    {/if}
  </div>
</div>
