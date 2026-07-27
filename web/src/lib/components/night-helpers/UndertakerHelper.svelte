<script lang="ts">
  import type { NightHelperContext } from "~/lib/night-helpers/registry";
  import { characterTokenCard } from "~/lib/info-cards";
  import { iconSuffix } from "~/lib/team-styles";

  let { entryId, ctx }: { entryId: string; ctx: NightHelperContext } = $props();

  const executed = $derived(ctx.executedToday);

  const playerId = $derived(ctx.playerIdForEntry(entryId));
  const status = $derived(playerId ? ctx.statuses.get(playerId) : undefined);
  const impaired = $derived(!!status && (status.poisoned || status.drunk));

  // The bare character-token card for the executed player's DISPLAYED character
  // — the icon IS the card, so the Storyteller can show the Undertaker's info
  // across the table instead of describing it.
  function showCard() {
    if (!executed) return;
    const p = executed.player;
    ctx.onshowcard?.(
      characterTokenCard(
        {
          id: p.characterId,
          name: p.characterName,
          edition: p.edition,
          team: p.team,
        },
        "undertaker",
      ),
    );
  }
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
  {#if ctx.onshowcard}
    <div class="mt-2 flex flex-wrap gap-2">
      <button
        type="button"
        onclick={showCard}
        class="rounded border border-purple-400 px-2 py-1 text-xs font-medium text-purple-600 transition-colors hover:bg-purple-50 dark:text-purple-300 dark:hover:bg-purple-950/40"
      >
        Show card
      </button>
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
          : "Drunk"} — show any character
    </div>
  {/if}
{/if}
