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
    onmove,
    onexecute,
    readonly = false,
  }: {
    game: Game;
    viewedPhaseDeaths?: Death[];
    onrecord: (roleId: string) => void;
    onremove: (deathId: bigint) => void;
    onuseghostvote: (deathId: bigint) => void;
    // Move a death to the sibling phase of the same round (Night N <-> Day N).
    onmove?: (death: Death) => void;
    /**
     * Execute by the town — a DAY death, recorded on the current or most recent
     * day. Omitted when no day exists yet (first night), which hides the
     * Execute button entirely.
     */
    onexecute?: (roleId: string) => void;
    readonly?: boolean;
  } = $props();

  // Which button's character dropdown is open (null = closed). Doubles as the
  // pending action: picking a character routes by this value.
  let openAction = $state<"kill" | "execute" | null>(null);

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

  // Deaths propagate forward into every later phase, so a displayed row may
  // just be a carried copy. The role's EARLIEST row across all phases (phases
  // are ordered chronologically by id) is the ORIGIN — labels and moves must
  // operate on it, not on a copy.
  function originDeath(death: Death): Death {
    for (const p of phases) {
      const row = (p.deaths ?? []).find((d) => d.roleId === death.roleId);
      if (row) return row;
    }
    return death;
  }

  // The sibling phase of a death's ORIGIN phase in the same round (a night kill
  // can move to that round's Day and vice versa). Returns its label, or
  // undefined when the round has no sibling phase yet (nothing to move to).
  function siblingPhaseLabel(death: Death): string | undefined {
    const cur = phaseById.get(originDeath(death).phaseId);
    if (!cur) return undefined;
    const siblingType =
      cur.type === PhaseType.NIGHT ? PhaseType.DAY : PhaseType.NIGHT;
    const exists = phases.some(
      (p) => p.roundNumber === cur.roundNumber && p.type === siblingType,
    );
    if (!exists) return undefined;
    return siblingType === PhaseType.NIGHT
      ? `Night ${cur.roundNumber}`
      : `Day ${cur.roundNumber}`;
  }

  // Toggle a button's dropdown. Clicking the other button while one is open
  // just moves the dropdown (and the pending action) to that button.
  function togglePicker(action: "kill" | "execute") {
    openAction = openAction === action ? null : action;
  }

  function handleRecord(roleId: string) {
    const action = openAction;
    openAction = null;
    if (action === "execute") {
      onexecute?.(roleId);
      return;
    }
    onrecord(roleId);
  }

  const hasAliveCharacters = $derived(
    teamOrder.some((t) => (aliveByTeam[t]?.length ?? 0) > 0),
  );

  // Close the open dropdown on outside click / Escape.
  $effect(() => {
    if (!openAction) return;
    const onDocClick = (e: MouseEvent) => {
      const target = e.target as HTMLElement | null;
      if (target && target.closest("[data-death-picker]")) return;
      openAction = null;
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") openAction = null;
    };
    document.addEventListener("click", onDocClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("click", onDocClick);
      document.removeEventListener("keydown", onKey);
    };
  });
</script>

<!-- Floating character list, rendered inside whichever action button is active
     so it anchors under that button. -->
{#snippet characterPicker(actionLabel: string)}
  <div
    class="absolute right-0 top-full z-20 mt-1 max-h-80 w-64 overflow-y-auto rounded-lg border border-border bg-surface p-3 shadow-lg"
    role="menu"
  >
    <span
      class="mb-2 block text-xs font-semibold uppercase tracking-wide text-muted"
    >
      {actionLabel}
    </span>
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
            <div class="grid gap-1">
              {#each chars as char (char.id)}
                <button
                  role="menuitem"
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
{/snippet}

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
      <!-- Two death causes, two buttons: a plain kill (night/day) and a town
           execution. Each opens the character list as a dropdown anchored under
           itself. Execute only exists once a day is available to record onto. -->
      <div class="flex items-center gap-2">
        <div class="relative" data-death-picker>
          <button
            onclick={() => togglePicker("kill")}
            disabled={!hasAliveCharacters}
            aria-haspopup="menu"
            aria-expanded={openAction === "kill"}
            class="rounded-lg bg-red-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-red-500 disabled:opacity-50"
          >
            Kill
          </button>
          {#if openAction === "kill"}
            {@render characterPicker("Kill")}
          {/if}
        </div>

        {#if onexecute}
          <div class="relative" data-death-picker>
            <button
              onclick={() => togglePicker("execute")}
              disabled={!hasAliveCharacters}
              aria-haspopup="menu"
              aria-expanded={openAction === "execute"}
              title="Execute (by the town, during the day)"
              class="rounded-lg bg-amber-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-amber-500 disabled:opacity-50"
            >
              Execute
            </button>
            {#if openAction === "execute"}
              {@render characterPicker("Execute")}
            {/if}
          </div>
        {/if}
      </div>
    {/if}
  </div>

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
              >{phaseLabel(originDeath(death).phaseId)}</span
            >
          </div>
          <!-- Ghost vote indicator -->
          <button
            onclick={() => onuseghostvote(death.id)}
            disabled={readonly || !death.ghostVote}
            class="shrink-0 rounded p-1 transition-colors {!death.ghostVote
              ? 'text-muted cursor-default'
              : readonly
                ? 'text-secondary cursor-default'
                : 'text-secondary hover:bg-hover hover:text-medium'}"
            title={!death.ghostVote ? "Ghost vote used" : "Use ghost vote"}
            aria-label={!death.ghostVote ? "Ghost vote used" : "Use ghost vote"}
          >
            <!-- Skull icon -->
            <svg
              class="h-5 w-5"
              viewBox="0 0 24 24"
              fill={!death.ghostVote ? "none" : "currentColor"}
              stroke="currentColor"
              stroke-width={!death.ghostVote ? "1.5" : "0"}
            >
              {#if !death.ghostVote}
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
          {#if !readonly && onmove}
            {@const moveLabel = siblingPhaseLabel(death)}
            {#if moveLabel}
              <button
                onclick={() => onmove?.(originDeath(death))}
                class="shrink-0 rounded border border-border px-1.5 py-0.5 text-xs font-medium text-secondary transition-colors hover:border-indigo-400 hover:text-indigo-500"
                title="Move death to {moveLabel}"
                aria-label="Move death to {moveLabel}"
              >
                &rarr; {moveLabel}
              </button>
            {/if}
          {/if}
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
