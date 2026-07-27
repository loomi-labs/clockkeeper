<script lang="ts">
  import { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import type { NightHelperContext } from "~/lib/night-helpers/registry";
  import { characterTokenCard } from "~/lib/info-cards";
  import { iconSuffix } from "~/lib/team-styles";
  import PlayerPickerPopover from "./PlayerPickerPopover.svelte";

  let { entryId, ctx }: { entryId: string; ctx: NightHelperContext } = $props();

  const playerId = $derived(ctx.playerIdForEntry(entryId));

  // The Ravenkeeper only wakes the night it dies. Render nothing unless the
  // page wired `diedTonight` and this seat is in it (graceful when unwired).
  const woke = $derived(!!playerId && !!ctx.diedTonight?.has(playerId));

  const status = $derived(playerId ? ctx.statuses.get(playerId) : undefined);
  const impaired = $derived(!!status && (status.poisoned || status.drunk));

  // Ephemeral pick — the learned player is not persisted as a grimoire token.
  let pickId = $state<string | undefined>(undefined);

  // Every seat except the Ravenkeeper's own is a legal target.
  const pickerPlayers = $derived([...ctx.players.values()]);
  const pickerExclude = $derived(
    playerId ? new Set([playerId]) : new Set<string>(),
  );

  // The learned character is the picked seat's DISPLAYED character (bag-sub
  // aware — a Drunk shown as the Empath is learned as the Empath, which is what
  // the grimoire token shows), falling back to the seat's own displayed fields.
  const shownChar = $derived.by(() => {
    if (!pickId) return undefined;
    const derived = ctx.displayedCharacterOf?.(pickId);
    if (derived) return derived;
    const p = ctx.players.get(pickId);
    return p
      ? {
          id: p.characterId,
          name: p.characterName,
          edition: p.edition,
          team: p.team,
        }
      : undefined;
  });

  function playerName(id: string | undefined): string | undefined {
    if (!id) return undefined;
    const p = ctx.players.get(id);
    return p ? p.name || p.characterName : undefined;
  }

  let picker = $state<{ anchor: { top: number; left: number } } | null>(null);

  function openPicker(e: MouseEvent) {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    picker = { anchor: { top: rect.bottom + 4, left: rect.left } };
  }

  function pick(id: string) {
    pickId = id;
    picker = null;
  }

  function clear() {
    pickId = undefined;
  }

  // The bare character-token card: the icon IS the card (empty title/body,
  // neutral accent), matching the ad-hoc "Character token" display shape but
  // with the learned character baked in so it needs no show-time pick.
  function showCard() {
    if (!shownChar) return;
    ctx.onshowcard?.(
      characterTokenCard(
        {
          id: shownChar.id,
          name: shownChar.name,
          edition: shownChar.edition,
          team: shownChar.team ?? Team.UNSPECIFIED,
        },
        "ravenkeeper",
      ),
    );
  }
</script>

{#if woke}
  <div class="flex items-center gap-1">
    <button
      type="button"
      onclick={openPicker}
      class="rounded border border-border px-2 py-1 text-xs transition-colors hover:bg-hover {pickId
        ? 'font-medium text-primary'
        : 'text-muted'}"
    >
      {pickId ? `Learns: ${playerName(pickId)}` : "Learns: pick player"}
    </button>
    {#if pickId}
      <button
        type="button"
        onclick={clear}
        class="flex h-5 w-5 items-center justify-center rounded-full text-muted transition-colors hover:bg-hover hover:text-primary"
        aria-label="Clear learned player"
        title="Clear learned player"
      >
        <svg
          class="h-3 w-3"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2.5"
          ><path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M6 18L18 6M6 6l12 12"
          /></svg
        >
      </button>
    {/if}
  </div>

  <!-- Learned character preview -->
  {#if shownChar}
    <div class="mt-1.5 flex items-center gap-2">
      <img
        src="/characters/{shownChar.edition}/{shownChar.id}{iconSuffix(
          shownChar.team,
        )}.webp"
        alt=""
        class="h-7 w-7 rounded-full"
        onerror={(e: Event) =>
          ((e.target as HTMLImageElement).style.display = "none")}
      />
      <span class="text-xs font-medium text-primary">{shownChar.name}</span>
    </div>

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
          : "Drunk"} — this info can be wrong; give any info
    </div>
  {/if}

  {#if picker}
    <PlayerPickerPopover
      title="Ravenkeeper learns"
      players={pickerPlayers}
      excludeIds={pickerExclude}
      anchor={picker.anchor}
      onpick={pick}
      onclose={() => (picker = null)}
    />
  {/if}
{/if}
