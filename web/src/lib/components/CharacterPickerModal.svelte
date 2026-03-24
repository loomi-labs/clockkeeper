<script lang="ts">
  import type { Character } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import {
    teamCardColorsSelected,
    teamNameColors,
    teamCheckColors,
    teamDataAttr,
    iconSuffix,
  } from "~/lib/team-styles";

  let {
    title,
    characters,
    selectedIds,
    excludeIds,
    excludeTeams,
    team,
    onselect,
    ondeselect,
    onclose,
  }: {
    title: string;
    characters: Character[];
    selectedIds: Set<string>;
    excludeIds?: Set<string>;
    excludeTeams?: Team[];
    team?: Team;
    onselect: (char: Character) => void;
    ondeselect: (charId: string) => void;
    onclose: () => void;
  } = $props();

  let searchQuery = $state("");
  let editionFilter = $state("");
  let teamFilter = $state(team ?? Team.UNSPECIFIED);

  const editions = [
    { id: "", label: "All", active: "bg-indigo-500 text-white" },
    { id: "tb", label: "Trouble Brewing", active: "bg-rose-700 text-white" },
    { id: "bmr", label: "Bad Moon Rising", active: "bg-orange-700 text-white" },
    { id: "snv", label: "Sects & Violets", active: "bg-violet-700 text-white" },
  ];

  const teams = [
    { id: Team.UNSPECIFIED, label: "All", active: "bg-indigo-500 text-white" },
    {
      id: Team.TOWNSFOLK,
      label: "Townsfolk",
      active: "bg-blue-600 text-white",
    },
    { id: Team.OUTSIDER, label: "Outsiders", active: "bg-cyan-600 text-white" },
    { id: Team.MINION, label: "Minions", active: "bg-orange-600 text-white" },
    { id: Team.DEMON, label: "Demons", active: "bg-red-600 text-white" },
    {
      id: Team.TRAVELLER,
      label: "Travellers",
      active: "bg-gradient-to-r from-blue-600 to-red-600 text-white",
    },
    { id: Team.FABLED, label: "Fabled", active: "bg-yellow-600 text-white" },
    { id: Team.LORIC, label: "Lorics", active: "bg-green-600 text-white" },
  ];

  const teamCardColors: Record<number, string> = {
    [Team.TOWNSFOLK]:
      "border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-950/40",
    [Team.OUTSIDER]:
      "border-cyan-200 bg-cyan-50 dark:border-cyan-800 dark:bg-cyan-950/40",
    [Team.MINION]:
      "border-orange-200 bg-orange-50 dark:border-orange-800 dark:bg-orange-950/40",
    [Team.DEMON]:
      "border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950/40",
    [Team.TRAVELLER]: "card-traveller",
    [Team.FABLED]:
      "border-yellow-200 bg-yellow-50 dark:border-yellow-700 dark:bg-yellow-950/40",
    [Team.LORIC]:
      "border-green-200 bg-green-50 dark:border-green-800 dark:bg-green-950/40",
  };

  const excludeTeamSet = $derived(new Set(excludeTeams ?? []));

  const filteredCharacters = $derived.by(() => {
    let chars = characters.filter((c) => {
      if (excludeTeamSet.has(c.team)) return false;
      if (excludeIds?.has(c.id)) return false;
      return true;
    });
    if (editionFilter) {
      chars = chars.filter((c) => c.edition === editionFilter);
    }
    if (teamFilter !== Team.UNSPECIFIED) {
      chars = chars.filter((c) => c.team === teamFilter);
    }
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      chars = chars.filter(
        (c) =>
          c.name.toLowerCase().includes(q) ||
          c.ability.toLowerCase().includes(q),
      );
    }
    return chars;
  });

  let hoveredChar = $state<Character | null>(null);
  let hoverTimer: ReturnType<typeof setTimeout> | null = null;
  let popupPos = $state({ x: 0, y: 0 });

  function onCharHover(e: MouseEvent, char: Character) {
    clearHoverTimer();
    const target = e.currentTarget as HTMLElement;
    hoverTimer = setTimeout(() => {
      const rect = target.getBoundingClientRect();
      popupPos = { x: rect.left, y: rect.bottom + 4 };
      hoveredChar = char;
    }, 1000);
  }

  function clearHoverTimer() {
    if (hoverTimer) {
      clearTimeout(hoverTimer);
      hoverTimer = null;
    }
    hoveredChar = null;
  }
</script>

<div class="fixed inset-0 z-50 flex items-start justify-center pt-16">
  <!-- Backdrop -->
  <button
    class="absolute inset-0 bg-black/40"
    onclick={onclose}
    aria-label="Close"
  ></button>

  <!-- Modal -->
  <div
    class="card-slate relative mx-4 flex max-h-[80vh] w-full max-w-2xl flex-col rounded-xl border border-border bg-surface shadow-2xl"
  >
    <!-- Header -->
    <div
      class="flex items-center justify-between border-b border-border px-4 py-3"
    >
      <h2 class="text-lg font-semibold text-primary">{title}</h2>
      <button
        onclick={onclose}
        aria-label="Close"
        class="rounded p-1 text-secondary transition-colors hover:bg-hover hover:text-medium"
      >
        <svg
          class="h-5 w-5"
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

    <!-- Filters -->
    <div class="space-y-3 border-b border-border px-4 py-3">
      <input
        bind:value={searchQuery}
        placeholder="Search characters..."
        class="w-full rounded-lg border border-border bg-surface-alt px-3 py-2 text-sm text-primary placeholder-muted focus:border-indigo-400 focus:outline-none"
      />
      <div class="flex flex-wrap items-center gap-1.5">
        <span class="text-xs text-muted">Edition:</span>
        {#each editions as ed}
          <button
            onclick={() => (editionFilter = ed.id)}
            class="rounded-lg px-3 py-1 text-xs font-medium transition-colors {editionFilter ===
            ed.id
              ? ed.active
              : 'bg-element text-secondary hover:text-medium'}"
          >
            {ed.label}
          </button>
        {/each}
      </div>
      {#if !team}
        <div class="flex flex-wrap items-center gap-1.5">
          <span class="text-xs text-muted">Type:</span>
          {#each teams as t}
            <button
              onclick={() => (teamFilter = t.id)}
              class="rounded-lg px-3 py-1 text-xs font-medium transition-colors {teamFilter ===
              t.id
                ? t.active
                : 'bg-element text-secondary hover:text-medium'}"
            >
              {t.label}
            </button>
          {/each}
        </div>
      {/if}
    </div>

    <!-- Character list -->
    <div class="overflow-y-auto p-4">
      <div class="grid gap-2 sm:grid-cols-2">
        {#each filteredCharacters as char (char.id)}
          {@const added = selectedIds.has(char.id)}
          <button
            onclick={() => (added ? ondeselect(char.id) : onselect(char))}
            onmouseenter={(e) => onCharHover(e, char)}
            onmouseleave={clearHoverTimer}
            class="card-slate rounded-lg border p-2.5 text-left transition-colors {added
              ? (teamCardColorsSelected[char.team] ??
                  'border-border-strong bg-hover') + ' hover:brightness-90'
              : (teamCardColors[char.team] ?? 'border-border bg-hover') +
                ' hover:brightness-110'}"
            data-team={teamDataAttr[char.team] ?? ""}
          >
            <div class="flex items-center gap-2.5">
              <img
                src="/characters/{char.edition}/{char.id}{iconSuffix(
                  char.team,
                )}.webp"
                alt=""
                class="h-8 w-8 shrink-0 rounded-full"
                onerror={(e: Event) =>
                  ((e.target as HTMLImageElement).style.display = "none")}
              />
              <div class="min-w-0 flex-1">
                <span
                  class="text-sm font-medium {added
                    ? (teamNameColors[char.team] ?? 'text-primary')
                    : 'text-primary'}">{char.name}</span
                >
              </div>
              {#if added}
                <svg
                  class="h-4 w-4 shrink-0 {teamCheckColors[char.team] ??
                    'text-secondary'}"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 13l4 4L19 7"
                  />
                </svg>
              {/if}
            </div>
          </button>
        {/each}
      </div>
      {#if filteredCharacters.length === 0}
        <p class="py-4 text-center text-sm text-muted">
          No matching characters.
        </p>
      {/if}
    </div>
  </div>

  {#if hoveredChar?.ability}
    <div
      class="fixed z-[60] max-w-xs rounded border px-3 py-2 text-xs text-secondary shadow-lg {teamCardColors[
        hoveredChar.team
      ] ?? 'border-border bg-surface-alt'}"
      style="left: {popupPos.x}px; top: {popupPos.y}px;"
    >
      {hoveredChar.ability}
    </div>
  {/if}
</div>
