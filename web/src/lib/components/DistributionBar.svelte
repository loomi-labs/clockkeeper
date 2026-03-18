<script lang="ts">
	import type { RoleDistribution } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';

	let {
		current,
		expected,
		travellers = 0
	}: {
		current: { townsfolk: number; outsiders: number; minions: number; demons: number };
		expected?: RoleDistribution | undefined;
		travellers?: number;
	} = $props();

	const baseTeams = [
		{ key: 'townsfolk' as const, label: 'Townsfolk', color: 'bg-blue-500' },
		{ key: 'outsiders' as const, label: 'Outsiders', color: 'bg-cyan-500' },
		{ key: 'minions' as const, label: 'Minions', color: 'bg-orange-500' },
		{ key: 'demons' as const, label: 'Demons', color: 'bg-red-500' }
	];

	const total = $derived(current.townsfolk + current.outsiders + current.minions + current.demons + travellers);
</script>

<div class="space-y-2">
	{#if total > 0}
		<div class="flex h-3 overflow-hidden rounded-full bg-gray-800">
			{#each baseTeams as team}
				{@const count = current[team.key]}
				{#if count > 0}
					<div
						class="{team.color} transition-all duration-300"
						style="width: {(count / total) * 100}%"
					></div>
				{/if}
			{/each}
			{#if travellers > 0}
				<div
					class="bg-yellow-500 transition-all duration-300"
					style="width: {(travellers / total) * 100}%"
				></div>
			{/if}
		</div>
	{/if}
	<div class="flex flex-wrap gap-4 text-sm">
		{#each baseTeams as team}
			{@const count = current[team.key]}
			{@const exp = expected ? expected[team.key] : undefined}
			<div class="flex items-center gap-1.5">
				<div class="h-2.5 w-2.5 rounded-full {team.color}"></div>
				<span class="text-gray-400">{team.label}</span>
				<span
					class={exp !== undefined && count !== exp ? 'font-medium text-red-400' : 'text-gray-300'}
				>
					{count}{exp !== undefined ? `/${exp}` : ''}
				</span>
			</div>
		{/each}
		{#if travellers > 0}
			<div class="flex items-center gap-1.5">
				<div class="h-2.5 w-2.5 rounded-full bg-yellow-500"></div>
				<span class="text-gray-400">Travellers</span>
				<span class="text-gray-300">{travellers}</span>
			</div>
		{/if}
		<div class="flex items-center gap-1.5 ml-auto">
			<span class="text-gray-500">Total</span>
			<span class="font-medium text-gray-300">{total}</span>
		</div>
	</div>
</div>
