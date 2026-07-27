<script lang="ts">
  import type { NightHelperContext } from "~/lib/night-helpers/registry";
  import { characterTokenCard } from "~/lib/info-cards";
  import { iconSuffix } from "~/lib/team-styles";
  import CharacterPickerPopover from "./CharacterPickerPopover.svelte";

  let { entryId, ctx }: { entryId: string; ctx: NightHelperContext } = $props();

  const executed = $derived(ctx.executedToday);

  const playerId = $derived(ctx.playerIdForEntry(entryId));
  const status = $derived(playerId ? ctx.statuses.get(playerId) : undefined);
  const impaired = $derived(!!status && (status.poisoned || status.drunk));

  // The executed seat's TRUE character — what the Undertaker learns when its
  // info is true. Grimoire truth, so a bag-sub facade is ignored (a Drunk is
  // learned as the Drunk, not as the Townsfolk they believe they are) while a
  // promotion counts (a star-passed Baron is learned as the Imp).
  const computedChar = $derived(executed?.player.trueCharacter);

  // The facade the grimoire shows for that seat, when it differs from the truth
  // (bag substitution) — surfaced so the ST isn't surprised by the mismatch.
  const facadeName = $derived(
    executed && executed.player.characterId !== executed.player.trueCharacter.id
      ? executed.player.characterName
      : undefined,
  );

  // Ephemeral show-card override: the ST sometimes must show a DIFFERENT
  // character than the computed one (Undertaker drunk/poisoned, or an event
  // altered the info). Only affects what the card shows — the computed
  // "Undertaker sees: X" display above stays untouched.
  let overrideId = $state<string | undefined>(undefined);

  const overrideChar = $derived(
    overrideId
      ? ctx.scriptCharacters?.find((c) => c.id === overrideId)
      : undefined,
  );

  // A new execution means a new computed target — drop a stale override.
  const computedKey = $derived(executed?.player.id);
  $effect(() => {
    computedKey;
    overrideId = undefined;
  });

  const effectiveChar = $derived(overrideChar ?? computedChar);

  let picker = $state<{ top: number; left: number } | null>(null);

  function openPicker(e: MouseEvent) {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    picker = { top: rect.bottom + 4, left: rect.left };
  }

  // The bare character-token card for the character actually being shown — the
  // icon IS the card, so the Storyteller can show the Undertaker's info across
  // the table instead of describing it.
  function showCard() {
    const char = effectiveChar;
    if (!char) return;
    ctx.onshowcard?.(characterTokenCard(char, "undertaker"));
  }
</script>

{#if !executed}
  <p class="text-muted">
    No execution today &mdash; the Undertaker does not wake.
  </p>
{:else}
  {@const p = executed.player}
  {@const t = p.trueCharacter}
  <div class="flex items-center gap-2">
    <img
      src="/characters/{t.edition}/{t.id}{iconSuffix(t.team)}.webp"
      alt=""
      draggable="false"
      class="h-8 w-8 shrink-0 rounded-full"
      onerror={(e: Event) =>
        ((e.target as HTMLImageElement).style.display = "none")}
    />
    <div class="min-w-0">
      <div class="text-secondary">
        Undertaker sees:
        <span class="font-bold text-primary">{t.name}</span>
      </div>
      {#if p.name}
        <div class="text-[11px] text-muted">{p.name}</div>
      {/if}
    </div>
  </div>
  {#if facadeName}
    <p class="mt-1 text-[11px] text-muted">
      (seat shows {facadeName} &mdash; the Undertaker learns the real character)
    </p>
  {/if}
  {#if executed.heuristic}
    <p class="mt-1 text-[11px] text-muted">
      (died today &mdash; cause unrecorded)
    </p>
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
        aria-label="Show the computed character instead"
        title="Show the computed character instead"
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
      <button
        type="button"
        onclick={showCard}
        class="rounded border border-purple-400 px-2 py-1 text-xs font-medium text-purple-600 transition-colors hover:bg-purple-50 dark:text-purple-300 dark:hover:bg-purple-950/40"
      >
        Show card
      </button>
      {#if ctx.scriptCharacters}
        <button
          type="button"
          onclick={openPicker}
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
          : "Drunk"} — show any character
    </div>
  {/if}

  {#if picker && ctx.scriptCharacters}
    <CharacterPickerPopover
      title="Show a different character"
      characters={ctx.scriptCharacters}
      anchor={picker}
      onpick={(c) => {
        overrideId = c.id;
        picker = null;
      }}
      onclose={() => (picker = null)}
    />
  {/if}
{/if}
