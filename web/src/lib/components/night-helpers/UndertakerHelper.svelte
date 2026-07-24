<script lang="ts">
  import type { NightHelperContext } from "~/lib/night-helpers/registry";
  import { iconSuffix } from "~/lib/team-styles";

  let { entryId, ctx }: { entryId: string; ctx: NightHelperContext } = $props();

  const executed = $derived(ctx.executedToday);

  const playerId = $derived(ctx.playerIdForEntry(entryId));
  const status = $derived(playerId ? ctx.statuses.get(playerId) : undefined);
  const impaired = $derived(!!status && (status.poisoned || status.drunk));
</script>

{#if !executed}
  <p class="text-muted">
    No execution today &mdash; the Undertaker does not wake.
  </p>
{:else}
  {@const p = executed.player}
  <div class="flex items-center gap-2">
    <img
      src="/characters/{p.edition}/{p.characterId}{iconSuffix(p.team)}.webp"
      alt=""
      draggable="false"
      class="h-8 w-8 shrink-0 rounded-full"
      onerror={(e: Event) =>
        ((e.target as HTMLImageElement).style.display = "none")}
    />
    <div class="min-w-0">
      <div class="text-secondary">
        Undertaker sees:
        <span class="font-bold text-primary">{p.characterName}</span>
      </div>
      {#if p.name}
        <div class="text-[11px] text-muted">{p.name}</div>
      {/if}
    </div>
  </div>
  {#if executed.heuristic}
    <p class="mt-1 text-[11px] text-muted">
      (died today &mdash; cause unrecorded)
    </p>
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
          : "Drunk"} — show any character
    </div>
  {/if}
{/if}
