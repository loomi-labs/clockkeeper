<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { client } from '~/lib/api';
	import { getErrorMessage } from '~/lib/errors';
	import type { SetupStep } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';

	let steps = $state<SetupStep[]>([]);
	let completed = $state<Set<string>>(new Set());
	let loading = $state(true);
	let error = $state('');

	const progress = $derived(
		steps.length > 0 ? Math.round((completed.size / steps.length) * 100) : 0
	);

	onMount(async () => {
		try {
			const id = BigInt(page.params.id);
			const resp = await client.getSetupChecklist({ gameId: id });
			steps = resp.steps;
		} catch (err) {
			error = getErrorMessage(err, 'Failed to load checklist');
		} finally {
			loading = false;
		}
	});

	function toggleStep(id: string) {
		const next = new Set(completed);
		if (next.has(id)) {
			next.delete(id);
		} else {
			next.add(id);
		}
		completed = next;
	}
</script>

{#if loading}
	<p class="text-gray-400">Loading...</p>
{:else if error}
	<div class="rounded-lg bg-red-900/50 px-4 py-2 text-sm text-red-300">{error}</div>
{:else}
	<div class="mx-auto max-w-2xl space-y-6">
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-3">
				<a href="/games/{page.params.id}" aria-label="Back to game" class="text-gray-400 transition-colors hover:text-gray-200">
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
					</svg>
				</a>
				<h1 class="text-2xl font-bold">Setup Checklist</h1>
			</div>
			<span class="text-sm text-gray-400">{completed.size}/{steps.length} complete</span>
		</div>

		<!-- Progress bar -->
		<div class="h-2 overflow-hidden rounded-full bg-gray-800">
			<div
				class="h-full bg-indigo-500 transition-all duration-300"
				style="width: {progress}%"
			></div>
		</div>

		<!-- Steps -->
		<div class="space-y-3">
			{#each steps as step (step.id)}
				{@const done = completed.has(step.id)}
				<button
					onclick={() => toggleStep(step.id)}
					class="w-full rounded-lg border p-4 text-left transition-colors {done
						? 'border-green-500/30 bg-green-950/20'
						: 'border-gray-700 bg-gray-900 hover:border-gray-600'}"
				>
					<div class="flex items-start gap-3">
						<div
							class="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded border transition-colors {done
								? 'border-green-500 bg-green-500'
								: 'border-gray-600'}"
						>
							{#if done}
								<svg class="h-3 w-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
								</svg>
							{/if}
						</div>
						<div class="min-w-0 flex-1">
							<div class="flex items-center gap-2">
								<span class="font-medium {done ? 'text-gray-500 line-through' : 'text-white'}">
									{step.title}
								</span>
								{#if step.requiresAction}
									<span class="rounded bg-indigo-500/20 px-1.5 py-0.5 text-xs text-indigo-300">
										action
									</span>
								{:else}
									<span class="rounded bg-gray-700 px-1.5 py-0.5 text-xs text-gray-400">
										info
									</span>
								{/if}
							</div>
							<p class="mt-1 whitespace-pre-line text-sm {done ? 'text-gray-600' : 'text-gray-400'}">
								{step.description}
							</p>
						</div>
					</div>
				</button>
			{/each}
		</div>

		{#if completed.size === steps.length && steps.length > 0}
			<div class="rounded-lg border border-green-500/30 bg-green-950/20 p-4 text-center">
				<p class="font-medium text-green-300">Setup complete! Ready to begin the first night.</p>
			</div>
		{/if}
	</div>
{/if}
