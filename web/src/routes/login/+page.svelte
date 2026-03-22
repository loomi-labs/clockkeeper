<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { client } from '~/lib/api';
	import { setToken } from '~/lib/auth.svelte';
	import ThemeSwitcher from '~/lib/components/ThemeSwitcher.svelte';
	import { getErrorMessage } from '~/lib/errors';
	import { initTheme } from '~/lib/theme';

	let username = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	onMount(() => {
		initTheme();
	});

	async function handleLogin(e: Event) {
		e.preventDefault();
		error = '';
		loading = true;

		try {
			const resp = await client.login({ username, password });
			setToken(resp.token);
			goto('/');
		} catch (err) {
			error = getErrorMessage(err, 'Login failed');
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
		<h1 class="mb-6 text-center text-2xl font-bold text-indigo-600">Clock Keeper</h1>
		<form onsubmit={handleLogin} class="space-y-4">
			{#if error}
				<div class="rounded-lg bg-error-bg px-4 py-2 text-sm text-error-text">{error}</div>
			{/if}
			<div>
				<label for="username" class="mb-1 block text-sm text-label">Username</label>
				<input
					id="username"
					type="text"
					bind:value={username}
					class="w-full rounded-lg border border-border-strong bg-surface-alt px-3 py-2 text-primary placeholder-muted focus:border-indigo-400 focus:outline-none"
					required
				/>
			</div>
			<div>
				<label for="password" class="mb-1 block text-sm text-label">Password</label>
				<input
					id="password"
					type="password"
					bind:value={password}
					class="w-full rounded-lg border border-border-strong bg-surface-alt px-3 py-2 text-primary placeholder-muted focus:border-indigo-400 focus:outline-none"
					required
				/>
			</div>
			<button
				type="submit"
				disabled={loading}
				class="btn-primary w-full rounded-lg bg-indigo-500 px-4 py-2 font-medium text-white transition-colors hover:bg-indigo-400 disabled:opacity-50"
			>
				{loading ? 'Signing in...' : 'Sign In'}
			</button>
		</form>
	</div>
</div>
