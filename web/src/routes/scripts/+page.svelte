<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { client } from '~/lib/api';
	import { getErrorMessage } from '~/lib/errors';
	import { editionStyle } from '~/lib/editions';
	import type { Script, Edition } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';

	let scripts = $state<Script[]>([]);
	let editions = $state<Edition[]>([]);
	let loading = $state(true);
	let error = $state('');
	let showImport = $state(false);
	let importJson = $state('');
	let importError = $state('');

	const systemScripts = $derived(scripts.filter((s) => s.isSystem));
	const userScripts = $derived(scripts.filter((s) => !s.isSystem));

	let charToEdition = $state(new Map<string, string>());

	function getScriptEditions(script: Script): string[] {
		const editionSet = new Set<string>();
		for (const cid of script.characterIds) {
			const ed = charToEdition.get(cid);
			if (ed) editionSet.add(ed);
		}
		return editions.map((e) => e.id).filter((e) => editionSet.has(e));
	}

	function scriptCardStyle(eds: string[]): { classes: string; inlineStyle: string } {
		if (eds.length === 0) {
			return { classes: 'border-gray-700 bg-gray-900', inlineStyle: '' };
		}
		if (eds.length === 1) {
			const s = editionStyle(eds[0]);
			return { classes: `${s.border} ${s.bg}`, inlineStyle: '' };
		}
		// Multi-edition gradient
		const colors = eds.map((e) => editionStyle(e).bgRaw);
		const stops = colors.map((c, i) => `${c} ${(i / (colors.length - 1)) * 100}%`).join(', ');
		return {
			classes: `${editionStyle(eds[0]).border}`,
			inlineStyle: `background: linear-gradient(to right, ${stops})`
		};
	}

	onMount(async () => {
		try {
			const [scriptsResp, editionsResp] = await Promise.all([
				client.listScripts({}),
				client.listEditions({})
			]);
			scripts = scriptsResp.scripts;
			editions = editionsResp.editions;

			// Build character → edition map
			const map = new Map<string, string>();
			for (const edition of editions) {
				for (const cid of edition.characterIds) {
					map.set(cid, edition.id);
				}
			}
			charToEdition = map;
		} catch (err) {
			error = getErrorMessage(err, 'Failed to load');
		} finally {
			loading = false;
		}
	});

	async function createFromEdition(editionId: string) {
		try {
			const resp = await client.createScriptFromEdition({ editionId, name: '' });
			if (resp.script) {
				goto(`/scripts/${resp.script.id}`);
			}
		} catch (err) {
			error = getErrorMessage(err, 'Failed to create script');
		}
	}

	async function createCustom() {
		try {
			const resp = await client.createScript({ name: 'New Script', characterIds: [] });
			if (resp.script) {
				goto(`/scripts/${resp.script.id}`);
			}
		} catch (err) {
			error = getErrorMessage(err, 'Failed to create script');
		}
	}

	async function deleteScript(id: bigint) {
		try {
			await client.deleteScript({ id });
			scripts = scripts.filter((s) => s.id !== id);
		} catch (err) {
			error = getErrorMessage(err, 'Failed to delete script');
		}
	}

	async function importScript() {
		importError = '';
		try {
			const resp = await client.importScript({ json: importJson });
			if (resp.script) {
				goto(`/scripts/${resp.script.id}`);
			}
		} catch (err) {
			importError = getErrorMessage(err, 'Failed to import');
		}
	}
</script>

<div class="space-y-8">
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold">Scripts</h1>
		<div class="flex gap-2">
			<button
				onclick={() => (showImport = !showImport)}
				class="rounded-lg border border-gray-700 px-3 py-1.5 text-sm text-gray-300 transition-colors hover:bg-gray-800"
			>
				Import JSON
			</button>
			<button
				onclick={createCustom}
				class="rounded-lg bg-indigo-500 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-indigo-400"
			>
				New Script
			</button>
		</div>
	</div>

	{#if error}
		<div class="rounded-lg bg-red-900/50 px-4 py-2 text-sm text-red-300">{error}</div>
	{/if}

	{#if showImport}
		<div class="rounded-lg border border-gray-700 bg-gray-900 p-4">
			<h2 class="mb-2 text-sm font-medium text-gray-300">Import Script JSON</h2>
			<textarea
				bind:value={importJson}
				placeholder='Paste official script JSON (e.g. [&#123;"id":"_meta","name":"My Script"&#125;,"washerwoman","librarian",...])'
				class="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white placeholder-gray-500 focus:border-indigo-400 focus:outline-none"
				rows="4"
			></textarea>
			{#if importError}
				<p class="mt-1 text-sm text-red-400">{importError}</p>
			{/if}
			<button
				onclick={importScript}
				disabled={!importJson.trim()}
				class="mt-2 rounded-lg bg-indigo-500 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-indigo-400 disabled:opacity-50"
			>
				Import
			</button>
		</div>
	{/if}

	{#if loading}
		<p class="text-gray-400">Loading...</p>
	{:else}
		<!-- Base editions (system scripts) -->
		{#if systemScripts.length > 0}
			<section>
				<h2 class="mb-3 text-lg font-semibold text-gray-300">Base Editions</h2>
				<div class="grid gap-3 sm:grid-cols-3">
					{#each systemScripts as sysScript (sysScript.id)}
						{@const style = editionStyle(sysScript.edition)}
						<a
							href="/scripts/{sysScript.id}"
							class="flex flex-col items-center rounded-lg border {style.border} {style.bg} p-5 transition-all hover:scale-[1.02] hover:brightness-110"
						>
							<img
								src="/editions/{sysScript.edition}.png"
								alt={sysScript.name}
								class="h-16 object-contain"
							/>
							<p class="mt-3 text-sm text-gray-400">{sysScript.characterIds.length} characters</p>
						</a>
					{/each}
				</div>
			</section>
		{/if}

		<!-- User scripts -->
		<section>
			<h2 class="mb-3 text-lg font-semibold text-gray-300">Your Scripts</h2>
			{#if userScripts.length === 0}
				<p class="text-gray-500">No saved scripts yet. Create one from an edition or start from scratch.</p>
			{:else}
				<div class="space-y-2">
					{#each userScripts as script (script.id)}
						{@const eds = getScriptEditions(script)}
						{@const cardStyle = scriptCardStyle(eds)}
						<div
							class="flex items-center justify-between rounded-lg border {cardStyle.classes} px-4 py-3 transition-colors"
							style={cardStyle.inlineStyle}
						>
							<a href="/scripts/{script.id}" class="min-w-0 flex-1">
								<span class="font-medium text-white">{script.name}</span>
								<p class="text-sm text-gray-400">{script.characterIds.length} characters</p>
							</a>
							<div class="ml-2 flex items-center gap-2">
								{#if eds.length > 0}
									<div class="flex items-center gap-1">
										{#each eds as ed}
											<img
												src="/editions/{ed}.png"
												alt={ed}
												class="h-7 object-contain opacity-80"
											/>
										{/each}
									</div>
								{/if}
								<button
									onclick={() => deleteScript(script.id)}
									aria-label="Delete {script.name}"
									class="rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-800/50 hover:text-red-400"
								>
									<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
									</svg>
								</button>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</section>
	{/if}
</div>
