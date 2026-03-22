<script lang="ts">
	import type { Character } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import { Team } from '~/lib/gen/clockkeeper/v1/clockkeeper_pb';
	import { formatReminder } from '~/lib/format';
	import { teamLabels, teamHeaderColors, teamCardColors, teamDataAttr, iconSuffix } from '~/lib/team-styles';

	let {
		characters
	}: {
		characters: Character[];
	} = $props();

	const teamOrder = [Team.TOWNSFOLK, Team.OUTSIDER, Team.MINION, Team.DEMON, Team.TRAVELLER, Team.FABLED, Team.LORIC] as const;

	const groupedByTeam = $derived.by(() => {
		const grouped: Record<number, Character[]> = {};
		for (const char of characters) {
			if (!grouped[char.team]) grouped[char.team] = [];
			grouped[char.team].push(char);
		}
		return grouped;
	});
</script>

<div class="space-y-6">
	{#each teamOrder as team}
		{@const chars = groupedByTeam[team]}
		{#if chars && chars.length > 0}
			<section>
				<h3 class="mb-3 text-sm font-semibold uppercase tracking-wide {teamHeaderColors[team] ?? 'text-secondary'}">
					{teamLabels[team] ?? ''} ({chars.length})
				</h3>
				<div class="space-y-2">
					{#each chars as char (char.id)}
						<a href="/almanac/{char.id}" class="card-slate block rounded-lg border p-4 transition-shadow hover:shadow-md {teamCardColors[char.team] ?? 'border-border bg-surface-alt'}" data-team={teamDataAttr[char.team] ?? ''} >
							<div class="flex gap-4">
								<img
									src="/characters/{char.edition}/{char.id}{iconSuffix(char.team)}.webp"
									alt=""
									class="h-24 w-24 shrink-0 rounded-full"
									onerror={(e: Event) => ((e.target as HTMLImageElement).style.display = 'none')}
								/>
								<div class="min-w-0 flex-1">
									<div class="flex items-center gap-2">
										<span class="font-medium text-primary">{char.name}</span>
										{#if char.setup}
											<span class="rounded bg-yellow-100 px-1.5 py-0.5 text-xs text-yellow-700 dark:bg-yellow-500/20 dark:text-yellow-300">setup</span>
										{/if}
									</div>
									<p class="mt-1 text-sm text-secondary">{@html formatReminder(char.ability)}</p>

									<!-- Night info -->
									{#if char.firstNightReminder || char.otherNightReminder}
										<div class="mt-2 space-y-1 border-t border-border/50 pt-2">
											{#if char.firstNightReminder}
												<div class="flex items-start gap-1.5 text-xs">
													<svg class="mt-0.5 h-3 w-3 shrink-0 text-indigo-400" fill="currentColor" viewBox="0 0 20 20"><path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z" /></svg>
													<span class="text-muted"><span class="font-medium text-secondary">First night:</span> {@html formatReminder(char.firstNightReminder)}</span>
												</div>
											{/if}
											{#if char.otherNightReminder}
												<div class="flex items-start gap-1.5 text-xs">
													<svg class="mt-0.5 h-3 w-3 shrink-0 text-amber-400" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4 8a4 4 0 11-8 0 4 4 0 018 0zm-.464 4.95l.707.707a1 1 0 001.414-1.414l-.707-.707a1 1 0 00-1.414 1.414zm2.12-10.607a1 1 0 010 1.414l-.706.707a1 1 0 11-1.414-1.414l.707-.707a1 1 0 011.414 0zM17 11a1 1 0 100-2h-1a1 1 0 100 2h1zm-7 4a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM5.05 6.464A1 1 0 106.465 5.05l-.708-.707a1 1 0 00-1.414 1.414l.707.707zm1.414 8.486l-.707.707a1 1 0 01-1.414-1.414l.707-.707a1 1 0 011.414 1.414zM4 11a1 1 0 100-2H3a1 1 0 000 2h1z" clip-rule="evenodd" /></svg>
													<span class="text-muted"><span class="font-medium text-secondary">Other nights:</span> {@html formatReminder(char.otherNightReminder)}</span>
												</div>
											{/if}
										</div>
									{/if}

									<!-- Reminders -->
									{#if char.reminders.length > 0 || char.remindersGlobal.length > 0}
										<div class="mt-2 flex flex-wrap gap-1">
											{#each char.reminders as reminder}
												<span class="rounded bg-element px-1.5 py-0.5 text-xs text-muted">{reminder}</span>
											{/each}
											{#each char.remindersGlobal as reminder}
												<span class="rounded bg-element px-1.5 py-0.5 text-xs italic text-muted">{reminder} (global)</span>
											{/each}
										</div>
									{/if}
								</div>
							</div>
						</a>
					{/each}
				</div>
			</section>
		{/if}
	{/each}

	{#if characters.length === 0}
		<p class="py-8 text-center text-sm text-muted">No characters in this script.</p>
	{/if}
</div>
