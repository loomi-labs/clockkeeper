<script lang="ts">
  import {
    chefPairs,
    chefRangeCauses,
    computeChef,
    RECLUSE_ID,
    SPY_ID,
    type ChefPair,
    type HelperPlayer,
  } from "~/lib/night-helpers/helpers";
  import type { NightHelperContext } from "~/lib/night-helpers/registry";

  let { entryId, ctx }: { entryId: string; ctx: NightHelperContext } = $props();

  const result = $derived(computeChef(ctx.order, ctx.players));
  const pairs = $derived(chefPairs(ctx.order, ctx.players));

  const playerLabel = (p: HelperPlayer) => p.name || p.characterName;

  /** Suffix for an ambiguous pair, naming which registration is assumed. */
  function ambiguousSuffix(pair: ChefPair): string {
    const isAmbiguous = (p: HelperPlayer) =>
      p.id === RECLUSE_ID || p.id === SPY_ID;
    const recluse =
      (isAmbiguous(pair.a) && pair.a.id === RECLUSE_ID) ||
      (isAmbiguous(pair.b) && pair.b.id === RECLUSE_ID);
    const spy =
      (isAmbiguous(pair.a) && pair.a.id === SPY_ID) ||
      (isAmbiguous(pair.b) && pair.b.id === SPY_ID);
    if (recluse && spy) return " (if Recluse & Spy register evil)";
    if (spy) return " (if Spy registers evil)";
    return " (if Recluse registers evil)";
  }

  // Only report a Recluse/Spy that actually widens the range.
  const causes = $derived(
    result.max > result.min
      ? chefRangeCauses(ctx.order, ctx.players)
      : { recluse: false, spy: false },
  );

  const playerId = $derived(ctx.playerIdForEntry(entryId));
  const status = $derived(playerId ? ctx.statuses.get(playerId) : undefined);
  const impaired = $derived(!!status && (status.poisoned || status.drunk));
</script>

<div class="flex flex-wrap items-baseline gap-x-1.5 gap-y-0.5">
  <span class="text-secondary">Chef sees:</span>
  <span class="text-base font-bold text-primary">
    {result.min === result.max ? result.min : `${result.min}–${result.max}`}
  </span>
  <span class="text-secondary"
    >{result.min === result.max && result.max === 1 ? "pair" : "pairs"}</span
  >
  {#if result.unknown}
    <span
      class="text-amber-600 dark:text-amber-300"
      title="a player has no known alignment">?</span
    >
  {/if}
</div>
{#if pairs.length > 0}
  <ul class="mt-0.5 space-y-0.5 text-[11px] text-muted">
    {#each pairs as pair (pair.a.id + "+" + pair.b.id)}
      <li>
        {playerLabel(pair.a)} + {playerLabel(pair.b)}{pair.ambiguous
          ? ambiguousSuffix(pair)
          : ""}
      </li>
    {/each}
  </ul>
{/if}
{#if causes.recluse || causes.spy}
  <div class="mt-0.5 text-[11px] text-muted">
    {#if causes.recluse}<span>Recluse may register as evil</span>{/if}
    {#if causes.recluse && causes.spy}<span> &middot; </span>{/if}
    {#if causes.spy}<span>Spy may register as good</span>{/if}
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
