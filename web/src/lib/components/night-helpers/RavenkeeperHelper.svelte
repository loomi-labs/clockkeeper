<script lang="ts">
  import type { NightHelperContext } from "~/lib/night-helpers/registry";
  import { characterTokenCard } from "~/lib/info-cards";
  import { iconSuffix } from "~/lib/team-styles";
  import PlayerPickerPopover from "./PlayerPickerPopover.svelte";
  import CharacterPickerPopover from "./CharacterPickerPopover.svelte";

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

  // The learned character is the picked seat's TRUE character (grimoire truth):
  // a bag-sub facade is ignored — a Drunk is learned as the Drunk, not as the
  // Townsfolk they believe they are — while a promotion counts (a star-passed
  // Baron is learned as the Imp).
  const shownChar = $derived(
    pickId ? ctx.players.get(pickId)?.trueCharacter : undefined,
  );

  // The facade the grimoire shows for the picked seat, when it differs from the
  // truth (bag substitution) — surfaced so the ST isn't surprised by it.
  const facadeName = $derived.by(() => {
    const p = pickId ? ctx.players.get(pickId) : undefined;
    if (!p || !shownChar) return undefined;
    return p.characterId !== shownChar.id ? p.characterName : undefined;
  });

  // Ephemeral show-card override: the ST sometimes must show a DIFFERENT
  // character than the learned one (Ravenkeeper drunk/poisoned, or an event
  // altered the info). Only affects what the card shows — the "learned
  // character" preview above stays untouched.
  let overrideId = $state<string | undefined>(undefined);

  const overrideChar = $derived(
    overrideId
      ? ctx.scriptCharacters?.find((c) => c.id === overrideId)
      : undefined,
  );

  // A different picked seat means a new computed target — drop a stale override.
  $effect(() => {
    pickId;
    overrideId = undefined;
  });

  // What "Show card" actually shows: the override when set, else the learned
  // character. With an override active the card is showable even without a pick.
  const effectiveChar = $derived(overrideChar ?? shownChar);

  function playerName(id: string | undefined): string | undefined {
    if (!id) return undefined;
    const p = ctx.players.get(id);
    // Seat label for the ST: the player's name, else the seat's real character.
    return p ? p.name || p.trueCharacter.name : undefined;
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

  let charPicker = $state<{ top: number; left: number } | null>(null);

  function openCharPicker(e: MouseEvent) {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    charPicker = { top: rect.bottom + 4, left: rect.left };
  }

  // The bare character-token card: the icon IS the card (empty title/body,
  // neutral accent), matching the ad-hoc "Character token" display shape but
  // with the shown character baked in so it needs no show-time pick.
  function showCard() {
    const char = effectiveChar;
    if (!char) return;
    ctx.onshowcard?.(characterTokenCard(char, "ravenkeeper"));
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
    {#if facadeName}
      <p class="mt-1 text-[11px] text-muted">
        (seat shows {facadeName} &mdash; the Ravenkeeper learns the real character)
      </p>
    {/if}
  {/if}

  <!-- Active override: what the card will actually show. -->
  {#if overrideChar}
    <div
      class="mt-1.5 flex flex-wrap items-center gap-2 rounded border border-amber-400 bg-amber-50 px-2 py-1.5 text-xs text-amber-700 dark:bg-amber-950/40 dark:text-amber-300"
    >
      <span class="font-medium">Showing instead:</span>
      <img
        src="/characters/{overrideChar.edition}/{overrideChar.id}{iconSuffix(
          overrideChar.team,
        )}.webp"
        alt=""
        draggable="false"
        class="h-6 w-6 shrink-0 rounded-full"
        onerror={(e: Event) =>
          ((e.target as HTMLImageElement).style.display = "none")}
      />
      <span class="font-semibold">{overrideChar.name}</span>
      <button
        type="button"
        onclick={() => (overrideId = undefined)}
        class="flex h-5 w-5 items-center justify-center rounded-full transition-colors hover:bg-amber-100 dark:hover:bg-amber-900/40"
        aria-label="Show the learned character instead"
        title="Show the learned character instead"
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
  {/if}

  {#if ctx.onshowcard}
    <div class="mt-2 flex flex-wrap gap-2">
      {#if effectiveChar}
        <button
          type="button"
          onclick={showCard}
          class="rounded border border-purple-400 px-2 py-1 text-xs font-medium text-purple-600 transition-colors hover:bg-purple-50 dark:text-purple-300 dark:hover:bg-purple-950/40"
        >
          Show card
        </button>
      {/if}
      {#if ctx.scriptCharacters}
        <button
          type="button"
          onclick={openCharPicker}
          class="rounded border border-border px-2 py-1 text-xs text-muted transition-colors hover:bg-hover"
          title="Show a different character"
        >
          Show different&hellip;
        </button>
      {/if}
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

  {#if charPicker && ctx.scriptCharacters}
    <CharacterPickerPopover
      title="Show a different character"
      characters={ctx.scriptCharacters}
      anchor={charPicker}
      onpick={(c) => {
        overrideId = c.id;
        charPicker = null;
      }}
      onclose={() => (charPicker = null)}
    />
  {/if}
{/if}
