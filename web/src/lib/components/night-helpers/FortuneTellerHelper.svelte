<script lang="ts">
  import {
    computeFortuneTeller,
    findDemonPlayers,
    type HelperPlayer,
  } from "~/lib/night-helpers/helpers";
  import type { NightHelperContext } from "~/lib/night-helpers/registry";
  import PlayerPickerPopover from "./PlayerPickerPopover.svelte";

  let { entryId, ctx }: { entryId: string; ctx: NightHelperContext } = $props();

  // Positional pick slots; ftPicks[0] -> slot 0, ftPicks[1] -> slot 1.
  // Empty slots are stored as "" so clearing one slot never shifts the other.
  const slot0 = $derived(ctx.ftPicks[0] || undefined);
  const slot1 = $derived(ctx.ftPicks[1] || undefined);

  const result = $derived(
    computeFortuneTeller(
      ctx.ftPicks.filter((id) => !!id),
      ctx.players,
      ctx.redHerringPlayerId,
    ),
  );

  const playerId = $derived(ctx.playerIdForEntry(entryId));
  const status = $derived(playerId ? ctx.statuses.get(playerId) : undefined);
  const impaired = $derived(!!status && (status.poisoned || status.drunk));

  // The demon seat(s) — detected exactly the way `computeFortuneTeller` decides
  // a pick gives YES, so the display and the answer cannot diverge.
  const demonPlayers = $derived(findDemonPlayers(ctx.players));

  // Popover state for the slot currently being picked.
  let picker = $state<{
    slot: 0 | 1;
    anchor: { top: number; left: number };
  } | null>(null);

  // Popover state for the Set Red Herring picker.
  let rhPicker = $state<{ anchor: { top: number; left: number } } | null>(null);

  // All players are available for the open slot (the Fortune Teller may choose
  // dead players and even themselves); only the other slot's pick is excluded.
  const pickerPlayers = $derived([...ctx.players.values()]);
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

  const redHerringName = $derived(playerName(ctx.redHerringPlayerId));
  const demonNames = $derived(
    demonPlayers.map((p) => p.name || p.characterName).join(", "),
  );

  // Advisory alignment badge for the Red Herring picker: good-aligned seats read
  // as a match (green), evil as a non-match (muted). Everyone stays pickable.
  function annotateAlignment(p: HelperPlayer) {
    if (p.alignment === "good") return { label: "good", tone: "ok" as const };
    if (p.alignment === "evil")
      return { label: "evil", tone: "muted" as const };
    return undefined;
  }

  function openPicker(slot: 0 | 1, e: MouseEvent) {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    picker = { slot, anchor: { top: rect.bottom + 4, left: rect.left } };
  }

  function openRhPicker(e: MouseEvent) {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    rhPicker = { anchor: { top: rect.bottom + 4, left: rect.left } };
  }

  function setRedHerring(id: string) {
    ctx.onattachtoken?.("fortuneteller", "Red Herring", id);
    rhPicker = null;
  }

  function clearRedHerring() {
    ctx.onattachtoken?.("fortuneteller", "Red Herring", undefined);
  }

  function setSlot(slot: 0 | 1, id: string) {
    const next = [slot0 ?? "", slot1 ?? ""];
    next[slot] = id;
    ctx.onftpick(next);
    picker = null;
  }

  function clearSlot(slot: 0 | 1) {
    const next = [slot0 ?? "", slot1 ?? ""];
    next[slot] = "";
    ctx.onftpick(next);
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

<!-- Red Herring + Demon info row -->
<div class="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px]">
  {#if ctx.onattachtoken}
    {#if redHerringName}
      <div class="flex items-center gap-1">
        <button
          type="button"
          onclick={openRhPicker}
          class="rounded border border-border px-2 py-1 text-xs font-medium text-primary transition-colors hover:bg-hover"
          title="Change Red Herring"
        >
          Red Herring: {redHerringName}
        </button>
        <button
          type="button"
          onclick={clearRedHerring}
          class="flex h-5 w-5 items-center justify-center rounded-full text-muted transition-colors hover:bg-hover hover:text-primary"
          aria-label="Clear Red Herring"
          title="Clear Red Herring"
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
      </div>
    {:else}
      <button
        type="button"
        onclick={openRhPicker}
        class="rounded border border-border px-2 py-1 text-xs text-muted transition-colors hover:bg-hover"
      >
        Set Red Herring
      </button>
    {/if}
  {:else if redHerringName}
    <span class="text-secondary"
      >Red Herring:
      <span class="font-medium text-primary">{redHerringName}</span></span
    >
  {/if}

  {#if demonNames}
    <span class="text-secondary"
      >Demon: <span class="font-medium text-primary">{demonNames}</span></span
    >
  {/if}
</div>

{#if ctx.redHerringPlayerId === undefined && !ctx.onattachtoken}
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

{#if rhPicker}
  <PlayerPickerPopover
    title="Set Red Herring"
    players={pickerPlayers}
    anchor={rhPicker.anchor}
    annotate={annotateAlignment}
    onpick={setRedHerring}
    onclose={() => (rhPicker = null)}
  />
{/if}
