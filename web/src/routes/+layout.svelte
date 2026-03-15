<script lang="ts">
	import type { Snippet } from 'svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { isAuthenticated, initAuth, getToken, logout } from '~/lib/auth';
	import '~/app.css';

	let { children }: { children: Snippet } = $props();
	let authenticated = $state(false);

	isAuthenticated.subscribe((v) => (authenticated = v));

	onMount(() => {
		initAuth();
		if (!getToken() && !page.url.pathname.startsWith('/login')) {
			goto('/login');
		}
	});
</script>

{#if page.url.pathname.startsWith('/login')}
	{@render children()}
{:else if authenticated}
	<div class="min-h-dvh bg-gray-950 text-gray-100">
		<nav class="border-b border-gray-800 bg-gray-900">
			<div class="mx-auto flex max-w-screen-xl items-center justify-between px-4 py-3">
				<a href="/" class="text-xl font-bold text-indigo-400">Clock Keeper</a>
				<button
					onclick={logout}
					class="rounded-lg px-3 py-1.5 text-sm text-gray-400 transition-colors hover:bg-gray-800 hover:text-gray-200"
				>
					Logout
				</button>
			</div>
		</nav>
		<main class="mx-auto max-w-screen-xl p-4">
			{@render children()}
		</main>
	</div>
{/if}
