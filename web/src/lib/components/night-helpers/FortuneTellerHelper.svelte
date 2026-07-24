<script lang="ts">
  import { computeFortuneTeller } from "~/lib/night-helpers/helpers";
  import type { NightHelperContext } from "~/lib/night-helpers/registry";
  import PlayerPickerPopover from "./PlayerPickerPopover.svelte";

  let { entryId, ctx }: { entryId: string; ctx: NightHelperContext } = $props();

  // Positional pick slots; ftPicks[0] -> slot 0, ftPicks[1] -> slot 1.
  const slot0 = $derived(ctx.ftPicks[0]);
  const slot1 = $derived(ctx.ftPicks[1]);

  const result = $derived(
    computeFortuneTeller(ctx.ftPicks, ctx.players, ctx.redHerringPlayerId),
  );

  const playerId = $derived(ctx.playerIdForEntry(entryId));
  const status = $derived(playerId ? ctx.statuses.get(playerId) : undefined);
  const impaired = $derived(!!status && (status.poisoned || status.drunk));

  // Popover state for the slot currently being picked.
  let picker = $state<{
    slot: 0 | 1;
    anchor: { top: number; left: number };
  } | null>(null);

  // Alive players available for the open slot, excluding the other slot's pick.
  const pickerPlayers = $derived(
    [...ctx.players.values()].filter((p) => !p.isDead),
  );
  const pickerExclude = $derived.by(() => {
    if (!picker) return new Set<string>();
    const other = picker.slot === 0 ? slot1 : slot0;
    return other ? new Set([other]) : new Set<string>();
  });

  function playerName(id: string | undefined): string | undefined {
    if (!id) return undefined;
    const p = ctx.players.get(id);
    return p ? p.name || p.characterName : undefined;
  }

  function openPicker(slot: 0 | 1, e: MouseEvent) {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    picker = { slot, anchor: { top: rect.bottom + 4, left: rect.left } };
  }

  function setSlot(slot: 0 | 1, id: string) {
    const next: (string | undefined)[] = [slot0, slot1];
    next[slot] = id;
    ctx.onftpick(next.filter((x): x is string => !!x));
    picker = null;
  }

  function clearSlot(slot: 0 | 1) {
    const next: (string | undefined)[] = [slot0, slot1];
    next[slot] = undefined;
    ctx.onftpick(next.filter((x): x is string => !!x));
  }
</script>

<div class="flex flex-wrap items-center gap-2">
  {#each [0, 1] as const as slot (slot)}
    {@const picked = slot === 0 ? slot0 : slot1}
    {@const label = playerName(picked)}
    <div class="flex items-center gap-1">
      <button
        type="button"
        onclick={(e) => openPicker(slot as 0 | 1, e)}
        class="rounded border border-border px-2 py-1 text-xs transition-colors hover:bg-hover {picked
          ? 'font-medium text-primary'
          : 'text-muted'}"
      >
        {label ?? `Pick player ${slot + 1}`}
      </button>
      {#if picked}
        <button
          type="button"
          onclick={() => clearSlot(slot as 0 | 1)}
          class="flex h-5 w-5 items-center justify-center rounded-full text-muted transition-colors hover:bg-hover hover:text-primary"
          aria-label="Clear pick"
          title="Clear pick"
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
  {/each}
</div>

{#if result}
  <div class="mt-1.5 flex items-baseline gap-2">
    {#if result.answer === "yes"}
      <span class="text-xl font-extrabold text-green-600 dark:text-green-400"
        >YES</span
      >
    {:else}
      <span class="text-xl font-bold text-muted">NO</span>
    {/if}
    {#if result.viaRedHerring}
      <span
        class="text-[11px] italic text-secondary"
        title="This YES comes from the Red Herring, not a real Demon."
      >
        (red herring)
      </span>
    {/if}
    {#if result.recluseMayYes}
      <span
        class="text-[11px] italic text-secondary"
        title="A picked Recluse may register as the Demon — you may answer YES."
      >
        (Recluse could register as YES)
      </span>
    {/if}
  </div>
{/if}

{#if ctx.redHerringPlayerId === undefined}
  <p class="mt-1 text-[11px] text-muted">
    No Red Herring assigned &mdash; attach the Fortune Teller's Red Herring
    token to a player in the Grimoire.
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
        : "Drunk"} — give any answer
  </div>
{/if}

{#if picker}
  <PlayerPickerPopover
    title="Fortune Teller pick {picker.slot + 1}"
    players={pickerPlayers}
    excludeIds={pickerExclude}
    anchor={picker.anchor}
    onpick={(id) => setSlot(picker!.slot, id)}
    onclose={() => (picker = null)}
  />
{/if}
