<script lang="ts">
  import type {
    Game,
    Character,
    Death,
  } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { Team, PhaseType } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { teamLabels, iconSuffix } from "~/lib/team-styles";

  let {
    game,
    viewedPhaseDeaths,
    onrecord,
    onremove,
    onuseghostvote,
    readonly = false,
  }: {
    game: Game;
    viewedPhaseDeaths?: Death[];
    onrecord: (roleId: string) => void;
    onremove: (deathId: bigint) => void;
    onuseghostvote: (deathId: bigint) => void;
    readonly?: boolean;
  } = $props();

  let showPicker = $state(false);

  const allDeaths = $derived(game.playState?.allDeaths ?? []);
  const displayDeaths = $derived(viewedPhaseDeaths ?? allDeaths);
  const showPhaseSummary = $derived(viewedPhaseDeaths !== undefined);
  const phases = $derived(game.playState?.phases ?? []);

  // Build a lookup from character ID to Character details.
  const characterById = $derived.by(() => {
    const map = new Map<string, Character>();
    for (const char of game.selectedCharacters ?? []) {
      map.set(char.id, char);
    }
    for (const char of game.selectedTravellerCharacters ?? []) {
      map.set(char.id, char);
    }
    for (const char of game.extraCharacterDetails ?? []) {
      map.set(char.id, char);
    }
    return map;
  });

  // Build a lookup from phase ID to Phase for display.
  const phaseById = $derived.by(() => {
    const map = new Map<bigint, { type: PhaseType; roundNumber: number }>();
    for (const phase of phases) {
      map.set(phase.id, { type: phase.type, roundNumber: phase.roundNumber });
    }
    return map;
  });

  // Dead role IDs for filtering the picker.
  const deadRoleIds = $derived(new Set(allDeaths.map((d) => d.roleId)));

  // Alive characters grouped by team for the picker.
  const aliveByTeam = $derived.by(() => {
    const grouped: Record<number, Character[]> = {};
    const allChars = [
      ...(game.selectedCharacters ?? []),
      ...(game.selectedTravellerCharacters ?? []),
      ...(game.extraCharacterDetails ?? []),
    ];
    for (const char of allChars) {
      if (deadRoleIds.has(char.id)) continue;
      if (!grouped[char.team]) grouped[char.team] = [];
      grouped[char.team].push(char);
    }
    return grouped;
  });

  const teamOrder = [
    Team.TOWNSFOLK,
    Team.OUTSIDER,
    Team.MINION,
    Team.DEMON,
    Team.TRAVELLER,
  ] as const;

  // DeathTracker uses different color weight (600/400) than shared teamNameColors (700/300).
  const teamNameColors: Record<number, string> = {
    [Team.TOWNSFOLK]: "text-blue-600 dark:text-blue-400",
    [Team.OUTSIDER]: "text-cyan-600 dark:text-cyan-400",
    [Team.MINION]: "text-orange-600 dark:text-orange-400",
    [Team.DEMON]: "text-red-600 dark:text-red-400",
    [Team.TRAVELLER]: "text-blue-600 dark:text-blue-400",
  };

  function phaseLabel(phaseId: bigint): string {
    const phase = phaseById.get(phaseId);
    if (!phase) return "";
    return phase.type === PhaseType.NIGHT
      ? `Night ${phase.roundNumber}`
      : `Day ${phase.roundNumber}`;
  }

  function handleRecord(roleId: string) {
    showPicker = false;
    onrecord(roleId);
  }

  const hasAliveCharacters = $derived(
    teamOrder.some((t) => (aliveByTeam[t]?.length ?? 0) > 0),
  );
</script>

<div class="rounded-lg border border-border bg-surface p-4">
  <div class="flex items-center justify-between mb-3">
    <h3 class="text-lg font-semibold text-primary">
      {showPhaseSummary ? "Deaths this phase" : "Deaths"}
      {#if displayDeaths.length > 0}
        <span class="ml-1 text-sm font-normal text-secondary"
          >({displayDeaths.length})</span
        >
      {/if}
    </h3>
    {#if !readonly}
      <button
        onclick={() => (showPicker = !showPicker)}
        disabled={!hasAliveCharacters}
        class="rounded-lg bg-red-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-red-500 disabled:opacity-50"
      >
        Kill
      </button>
    {/if}
  </div>

  <!-- Death picker -->
  {#if showPicker}
    <div class="mb-4 rounded-lg border border-border bg-element p-3">
      <div class="mb-2 flex items-center justify-between">
        <span class="text-sm font-medium text-secondary">Select character</span>
        <button
          onclick={() => (showPicker = false)}
          aria-label="Close picker"
          class="rounded p-1 text-muted transition-colors hover:bg-hover hover:text-medium"
        >
          <svg
            class="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      </div>
      <div class="space-y-3">
        {#each teamOrder as team}
          {@const chars = aliveByTeam[team]}
          {#if chars && chars.length > 0}
            <div>
              <span
                class="mb-1 block text-xs font-semibold uppercase tracking-wide {teamNameColors[
                  team
                ] ?? 'text-secondary'}"
              >
                {teamLabels[team] ?? ""}
              </span>
              <div class="grid gap-1 sm:grid-cols-2">
                {#each chars as char (char.id)}
                  <button
                    onclick={() => handleRecord(char.id)}
                    class="flex items-center gap-2 rounded-lg border border-border px-2 py-1.5 text-left transition-colors hover:bg-hover"
                  >
                    <img
                      src="/characters/{char.edition}/{char.id}{iconSuffix(
                        char.team,
                      )}.webp"
                      alt=""
                      class="h-8 w-8 shrink-0 rounded-full"
                      onerror={(e: Event) =>
                        ((e.target as HTMLImageElement).style.display = "none")}
                    />
                    <span class="text-sm font-medium text-primary"
                      >{char.name}</span
                    >
                  </button>
                {/each}
              </div>
            </div>
          {/if}
        {/each}
      </div>
    </div>
  {/if}

  <!-- Deaths list -->
  {#if displayDeaths.length === 0}
    <p class="py-4 text-center text-sm text-muted">
      {showPhaseSummary ? "No deaths this phase." : "No deaths yet."}
    </p>
  {:else}
    <div class="space-y-1">
      {#each displayDeaths as death (death.id)}
        {@const char = characterById.get(death.roleId)}
        <div class="flex items-center gap-3 rounded-lg bg-element/50 px-3 py-2">
          {#if char}
            <img
              src="/characters/{char.edition}/{char.id}{iconSuffix(
                char.team,
              )}.webp"
              alt=""
              class="h-10 w-10 shrink-0 rounded-full grayscale"
              onerror={(e: Event) =>
                ((e.target as HTMLImageElement).style.display = "none")}
            />
          {:else}
            <div
              class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-element text-sm text-secondary"
            >
              ?
            </div>
          {/if}
          <div class="min-w-0 flex-1">
            <span class="text-sm font-medium text-primary"
              >{char?.name ?? death.roleId}</span
            >
            <span class="ml-2 text-xs text-muted"
              >{phaseLabel(death.phaseId)}</span
            >
          </div>
          <!-- Ghost vote indicator -->
          <button
            onclick={() => onuseghostvote(death.id)}
            disabled={readonly || death.ghostVote}
            class="shrink-0 rounded p-1 transition-colors {death.ghostVote
              ? 'text-muted cursor-default'
              : readonly
                ? 'text-secondary cursor-default'
                : 'text-secondary hover:bg-hover hover:text-medium'}"
            title={death.ghostVote ? "Ghost vote used" : "Use ghost vote"}
            aria-label={death.ghostVote ? "Ghost vote used" : "Use ghost vote"}
          >
            <!-- Skull icon -->
            <svg
              class="h-5 w-5"
              viewBox="0 0 24 24"
              fill={death.ghostVote ? "none" : "currentColor"}
              stroke="currentColor"
              stroke-width={death.ghostVote ? "1.5" : "0"}
            >
              {#if death.ghostVote}
                <!-- Empty skull (vote used) with strikethrough -->
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M12 2C7.58 2 4 5.58 4 10c0 2.76 1.34 5.2 3.4 6.72V20a1 1 0 001 1h7.2a1 1 0 001-1v-3.28C18.66 15.2 20 12.76 20 10c0-4.42-3.58-8-8-8zm-2 17v-1h4v1h-4zm0-3h1v2h2v-2h1v2h-4zm5.6-2.08l-.6.46V17h-6v-2.62l-.6-.46A5.94 5.94 0 016 10c0-3.31 2.69-6 6-6s6 2.69 6 6a5.94 5.94 0 01-2.4 3.92z"
                />
                <line x1="4" y1="4" x2="20" y2="20" stroke-width="2" />
              {:else}
                <!-- Filled skull (vote available) -->
                <path
                  d="M12 2C7.58 2 4 5.58 4 10c0 2.76 1.34 5.2 3.4 6.72V20a1 1 0 001 1h7.2a1 1 0 001-1v-3.28C18.66 15.2 20 12.76 20 10c0-4.42-3.58-8-8-8zm-1 15v-2h2v2h-2zm4-7a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm-5 0a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0z"
                />
              {/if}
            </svg>
          </button>
          {#if !readonly}
            <button
              onclick={() => onremove(death.id)}
              class="shrink-0 rounded p-1 text-muted transition-colors hover:bg-hover hover:text-red-500"
              title="Undo death"
              aria-label="Undo death"
            >
              <svg
                class="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          {/if}
        </div>
      {/each}
    </div>
  {/if}

  {#if showPhaseSummary && allDeaths.length > 0}
    <p class="mt-3 text-center text-xs text-muted">
      {allDeaths.length} total {allDeaths.length === 1 ? "death" : "deaths"} across
      all phases
    </p>
  {/if}
</div>
