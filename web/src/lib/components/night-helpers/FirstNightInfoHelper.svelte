<script lang="ts">
  import { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import type { NightHelperContext } from "~/lib/night-helpers/registry";
  import { firstNightInfoCard, noOutsidersCard } from "~/lib/info-cards";
  import { iconSuffix } from "~/lib/team-styles";
  import PlayerPickerPopover from "./PlayerPickerPopover.svelte";

  let { entryId, ctx }: { entryId: string; ctx: NightHelperContext } = $props();

  // Per-helper config. The team-token reminder text equals the team's singular
  // label (roles.json reminders: "Townsfolk"/"Outsider"/"Minion" + "Wrong").
  interface Config {
    team: Team;
    teamLabel: string;
  }
  const CONFIG: Record<string, Config> = {
    washerwoman: { team: Team.TOWNSFOLK, teamLabel: "Townsfolk" },
    librarian: { team: Team.OUTSIDER, teamLabel: "Outsider" },
    investigator: { team: Team.MINION, teamLabel: "Minion" },
  };
  const WRONG_TEXT = "Wrong";

  const cfg = $derived(CONFIG[entryId]);
  const isLibrarian = $derived(entryId === "librarian");

  const helperPlayerId = $derived(ctx.playerIdForEntry(entryId));

  // Render nothing unless the seat is in play and the page wired the new
  // first-night context fields (graceful when unwired).
  const ready = $derived(
    !!cfg &&
      !!helperPlayerId &&
      !!ctx.displayedCharacterOf &&
      !!ctx.infoPicks &&
      !!ctx.oninfopick,
  );

  const picks = $derived(ctx.infoPicks?.get(entryId) ?? {});
  const rightId = $derived(picks.rightId);
  const wrongId = $derived(picks.wrongId);

  // Librarian's "no Outsiders in play" mode (replaces the pickers).
  let noOutsiders = $state(false);

  const status = $derived(
    helperPlayerId ? ctx.statuses.get(helperPlayerId) : undefined,
  );
  const impaired = $derived(!!status && (status.poisoned || status.drunk));

  const shownChar = $derived(
    rightId ? ctx.displayedCharacterOf?.(rightId) : undefined,
  );

  // Right candidates: seats (alive or dead) whose DISPLAYED team matches the
  // required team, excluding the helper's own seat.
  const rightCandidates = $derived(
    [...ctx.players.values()].filter((p) => {
      if (p.id === helperPlayerId) return false;
      const dc = ctx.displayedCharacterOf?.(p.id);
      return dc?.team === cfg?.team;
    }),
  );
  // Wrong candidates: any other seat except the right pick and the helper's own.
  const wrongCandidates = $derived(
    [...ctx.players.values()].filter(
      (p) => p.id !== helperPlayerId && p.id !== rightId,
    ),
  );

  let picker = $state<{
    slot: "right" | "wrong";
    anchor: { top: number; left: number };
  } | null>(null);

  function playerName(id: string | undefined): string | undefined {
    if (!id) return undefined;
    const p = ctx.players.get(id);
    return p ? p.name || p.characterName : undefined;
  }

  function openPicker(slot: "right" | "wrong", e: MouseEvent) {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    picker = { slot, anchor: { top: rect.bottom + 4, left: rect.left } };
  }

  function setSlot(slot: "right" | "wrong", id: string) {
    const cur = ctx.infoPicks?.get(entryId) ?? {};
    const next = { ...cur };
    if (slot === "right") {
      next.rightId = id;
      // A previously-chosen decoy cannot also be the revealed player.
      if (next.wrongId === id) next.wrongId = undefined;
    } else {
      next.wrongId = id;
    }
    ctx.oninfopick?.(entryId, next);
    picker = null;
  }

  function clearSlot(slot: "right" | "wrong") {
    const cur = ctx.infoPicks?.get(entryId) ?? {};
    const next = { ...cur };
    if (slot === "right") next.rightId = undefined;
    else next.wrongId = undefined;
    ctx.oninfopick?.(entryId, next);
  }

  function attachTokens() {
    if (!cfg || !rightId || !wrongId) return;
    ctx.onattachreminder?.(entryId, cfg.teamLabel, rightId);
    ctx.onattachreminder?.(entryId, WRONG_TEXT, wrongId);
  }

  function showCard() {
    if (noOutsiders) {
      ctx.onshowcard?.(noOutsidersCard());
      return;
    }
    if (!shownChar || !rightId || !wrongId) return;
    ctx.onshowcard?.(
      firstNightInfoCard(shownChar, [
        playerName(rightId) ?? shownChar.name,
        playerName(wrongId) ?? "",
      ]),
    );
  }
</script>

{#if ready}
  {#if isLibrarian && noOutsiders}
    <div class="flex flex-wrap items-center gap-2">
      <span class="text-xs font-medium text-primary">No Outsiders in play</span>
      <button
        type="button"
        onclick={() => (noOutsiders = false)}
        class="rounded border border-border px-2 py-1 text-xs text-muted transition-colors hover:bg-hover"
      >
        Pick players instead
      </button>
      <button
        type="button"
        onclick={showCard}
        class="rounded border border-purple-400 px-2 py-1 text-xs font-medium text-purple-600 transition-colors hover:bg-purple-50 dark:text-purple-300 dark:hover:bg-purple-950/40"
      >
        Show card
      </button>
    </div>
  {:else}
    <div class="flex flex-wrap items-center gap-2">
      <!-- Right (revealed) player -->
      <div class="flex items-center gap-1">
        <button
          type="button"
          onclick={(e) => openPicker("right", e)}
          class="rounded border border-border px-2 py-1 text-xs transition-colors hover:bg-hover {rightId
            ? 'font-medium text-primary'
            : 'text-muted'}"
        >
          {rightId ? `Shows: ${playerName(rightId)}` : "Shows: pick player"}
        </button>
        {#if rightId}
          <button
            type="button"
            onclick={() => clearSlot("right")}
            class="flex h-5 w-5 items-center justify-center rounded-full text-muted transition-colors hover:bg-hover hover:text-primary"
            aria-label="Clear revealed player"
            title="Clear revealed player"
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

      <!-- Wrong (decoy) player -->
      <div class="flex items-center gap-1">
        <button
          type="button"
          onclick={(e) => openPicker("wrong", e)}
          class="rounded border border-border px-2 py-1 text-xs transition-colors hover:bg-hover {wrongId
            ? 'font-medium text-primary'
            : 'text-muted'}"
        >
          {wrongId ? `Wrong: ${playerName(wrongId)}` : "Wrong: pick player"}
        </button>
        {#if wrongId}
          <button
            type="button"
            onclick={() => clearSlot("wrong")}
            class="flex h-5 w-5 items-center justify-center rounded-full text-muted transition-colors hover:bg-hover hover:text-primary"
            aria-label="Clear decoy player"
            title="Clear decoy player"
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

      {#if isLibrarian}
        <button
          type="button"
          onclick={() => (noOutsiders = true)}
          class="rounded border border-border px-2 py-1 text-xs text-muted transition-colors hover:bg-hover"
        >
          No Outsiders
        </button>
      {/if}
    </div>

    <!-- Revealed character preview -->
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
    {/if}

    {#if rightId && wrongId}
      <div class="mt-2 flex flex-wrap gap-2">
        <button
          type="button"
          onclick={attachTokens}
          class="rounded border border-border px-2 py-1 text-xs font-medium text-secondary transition-colors hover:bg-hover hover:text-primary"
        >
          Attach tokens
        </button>
        <button
          type="button"
          onclick={showCard}
          class="rounded border border-purple-400 px-2 py-1 text-xs font-medium text-purple-600 transition-colors hover:bg-purple-50 dark:text-purple-300 dark:hover:bg-purple-950/40"
        >
          Show card
        </button>
      </div>
    {/if}
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
      title={picker.slot === "right"
        ? `${cfg.teamLabel} player`
        : "Wrong player"}
      players={picker.slot === "right" ? rightCandidates : wrongCandidates}
      anchor={picker.anchor}
      onpick={(id) => setSlot(picker!.slot, id)}
      onclose={() => (picker = null)}
    />
  {/if}
{/if}
