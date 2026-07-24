<script lang="ts">
  import { computeEmpath } from "~/lib/night-helpers/helpers";
  import type { HelperPlayer } from "~/lib/night-helpers/helpers";
  import { aliveNeighbours } from "~/lib/night-helpers/seating";
  import type { NightHelperContext } from "~/lib/night-helpers/registry";
  import { iconSuffix } from "~/lib/team-styles";

  let { entryId, ctx }: { entryId: string; ctx: NightHelperContext } = $props();

  const playerId = $derived(ctx.playerIdForEntry(entryId));
  const result = $derived(
    playerId ? computeEmpath(ctx.order, ctx.players, playerId) : undefined,
  );

  const deadSet = $derived(
    new Set([...ctx.players.values()].filter((p) => p.isDead).map((p) => p.id)),
  );

  // The two alive neighbours the reading counts (deduped: with two players
  // alive the single neighbour appears once).
  const neighbours = $derived.by(() => {
    if (!playerId || !result) return [];
    const { cw, ccw } = aliveNeighbours(ctx.order, playerId, deadSet);
    const ids = new Set<string>();
    if (cw) ids.add(cw);
    if (ccw) ids.add(ccw);
    return [...ids]
      .map((id) => ctx.players.get(id))
      .filter((p): p is HelperPlayer => !!p);
  });

  const status = $derived(playerId ? ctx.statuses.get(playerId) : undefined);
  const impaired = $derived(!!status && (status.poisoned || status.drunk));
</script>

{#if result}
  <div class="flex flex-wrap items-baseline gap-x-1.5 gap-y-0.5">
    <span class="text-secondary">Empath sees:</span>
    <span class="text-base font-bold text-primary">{result.count}</span>
    {#if result.unknown}
      <span
        class="text-amber-600 dark:text-amber-300"
        title="a neighbour has no known alignment">?</span
      >
    {/if}
  </div>
  {#if neighbours.length}
    <div class="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1">
      {#each neighbours as p (p.id)}
        <span class="flex items-center gap-1 text-[11px] text-muted">
          <img
            src="/characters/{p.edition}/{p.characterId}{iconSuffix(
              p.team,
            )}.webp"
            alt=""
            draggable="false"
            class="h-4 w-4 rounded-full"
            onerror={(e: Event) =>
              ((e.target as HTMLImageElement).style.display = "none")}
          />
          {p.name || p.characterName}
        </span>
      {/each}
    </div>
  {/if}
  {#if impaired}
    <div
      class="mt-1 text-[11px] font-medium {status?.poisoned
        ? 'text-purple-600 dark:text-purple-300'
        : 'text-amber-600 dark:text-amber-300'}"
    >
      {status?.poisoned && status?.drunk
        ? "Poisoned/Drunk"
        : status?.poisoned
          ? "Poisoned"
          : "Drunk"} — give any number
    </div>
  {/if}
{/if}
