<script lang="ts">
	import { goto } from '$app/navigation';
	import { client } from '~/lib/api';
	import { setToken } from '~/lib/auth';

	let username = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	async function handleLogin(e: Event) {
		e.preventDefault();
		error = '';
		loading = true;

		try {
			const resp = await client.login({ username, password });
			setToken(resp.token);
			goto('/');
		} catch (err: any) {
			error = err.message || 'Login failed';
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex min-h-screen items-center justify-center bg-gray-950">
	<div class="w-full max-w-sm rounded-xl bg-gray-900 p-8 shadow-lg">
		<h1 class="mb-6 text-center text-2xl font-bold text-indigo-400">Clock Keeper</h1>
		<form onsubmit={handleLogin} class="space-y-4">
			{#if error}
				<div class="rounded-lg bg-red-900/50 px-4 py-2 text-sm text-red-300">{error}</div>
			{/if}
			<div>
				<label for="username" class="mb-1 block text-sm text-gray-400">Username</label>
				<input
					id="username"
					type="text"
					bind:value={username}
					class="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-white focus:border-indigo-400 focus:outline-none"
					required
				/>
			</div>
			<div>
				<label for="password" class="mb-1 block text-sm text-gray-400">Password</label>
				<input
					id="password"
					type="password"
					bind:value={password}
					class="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-white focus:border-indigo-400 focus:outline-none"
					required
				/>
			</div>
			<button
				type="submit"
				disabled={loading}
				class="w-full rounded-lg bg-indigo-500 px-4 py-2 font-medium text-white transition-colors hover:bg-indigo-400 disabled:opacity-50"
			>
				{loading ? 'Signing in...' : 'Sign In'}
			</button>
		</form>
	</div>
</div>
