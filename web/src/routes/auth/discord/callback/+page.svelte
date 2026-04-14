<script lang="ts">
  import { onMount } from "svelte";
  import { goto } from "$app/navigation";
  import { page } from "$app/state";
  import { client } from "~/lib/api";
  import { setToken, setAnonymous } from "~/lib/auth.svelte";
  import { getErrorMessage } from "~/lib/errors";
  import { initTheme } from "~/lib/theme";

  let error = $state("");

  onMount(async () => {
    initTheme();

    const code = page.url.searchParams.get("code");
    if (!code) {
      error = "No authorization code received from Discord.";
      return;
    }

    const redirectUri = `${window.location.origin}/auth/discord/callback`;

    try {
      const resp = await client.loginWithDiscord({ code, redirectUri });
      setToken(resp.token);
      setAnonymous(false);
      goto("/");
    } catch (err) {
      error = getErrorMessage(err, "Discord login failed");
    }
  });
</script>

<div class="flex min-h-screen items-center justify-center">
  <div class="card-slate w-full max-w-sm rounded-xl bg-surface p-8 shadow-lg">
    {#if error}
      <h1 class="mb-4 text-center text-xl font-bold text-error-text">
        Login Failed
      </h1>
      <p class="mb-4 text-center text-sm text-secondary">{error}</p>
      <a
        href="/login"
        class="block w-full rounded-lg border border-border px-4 py-2 text-center text-sm font-medium text-secondary transition-colors hover:bg-hover hover:text-primary"
      >
        Back to Login
      </a>
    {:else}
      <p class="text-center text-secondary">Signing in with Discord...</p>
    {/if}
  </div>
</div>
