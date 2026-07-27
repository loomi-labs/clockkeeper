<script lang="ts">
  import { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import type { NightHelperContext } from "~/lib/night-helpers/registry";
  import type { CharacterRef, HelperPlayer } from "~/lib/night-helpers/helpers";
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

  // Ephemeral shown-character override. The revealed character is normally the
  // "right" seat's DISPLAYED character, but the ST may override it (e.g. to show
  // a category-appropriate character that differs from the seat's token). Reset
  // whenever the picked "right" seat changes, so a stale override never leaks.
  let overrideChar = $state<CharacterRef | null>(null);
  $effect(() => {
    rightId;
    overrideChar = null;
  });

  const status = $derived(
    helperPlayerId ? ctx.statuses.get(helperPlayerId) : undefined,
  );
  const impaired = $derived(!!status && (status.poisoned || status.drunk));

  // The shown character is the manual override when set, else whatever the
  // picked "right" seat DISPLAYS — bag-sub aware and NOT team-filtered. A
  // drunk/poisoned info character can legitimately be shown any character, so we
  // never restrict the DERIVED pick by team (the override picker does, below).
  const shownChar = $derived(
    overrideChar ?? (rightId ? ctx.displayedCharacterOf?.(rightId) : undefined),
  );

  // Override candidates: the script's characters of the helper's category
  // (Washerwoman → Townsfolk, Librarian → Outsider, Investigator → Minion).
  const overrideCandidates = $derived(
    (ctx.scriptCharacters ?? []).filter((c) => cfg && c.team === cfg.team),
  );

  // Both slots list ALL seats (alive or dead), excluding only the helper's own
  // seat and the other slot's current pick. No team filtering — see above.
  const rightCandidates = $derived(
    [...ctx.players.values()].filter(
      (p) => p.id !== helperPlayerId && p.id !== wrongId,
    ),
  );
  const wrongCandidates = $derived(
    [...ctx.players.values()].filter(
      (p) => p.id !== helperPlayerId && p.id !== rightId,
    ),
  );

  // "Shows" slot annotation: green badge with the required team's label on
  // seats whose DISPLAYED team matches, muted "not a {team}" otherwise. Advisory
  // only — every seat stays pickable.
  function annotateRight(p: HelperPlayer) {
    if (!cfg) return undefined;
    const dc = ctx.displayedCharacterOf?.(p.id);
    return dc?.team === cfg.team
      ? { label: cfg.teamLabel, tone: "ok" as const }
      : { label: `not a ${cfg.teamLabel}`, tone: "muted" as const };
  }

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

  function showCard() {
    if (noOutsiders) {
      ctx.onshowcard?.(noOutsidersCard());
      return;
    }
    if (!shownChar || !rightId || !wrongId) return;
    ctx.onshowcard?.(firstNightInfoCard(shownChar));
  }

  // ── Shown-character override picker ──
  // A small popover listing the category's script characters. Portalled to
  // <body> (same rationale as PlayerPickerPopover): rendered inline its
  // `position: fixed` would resolve against a transformed/clipped night row.
  const CHAR_VIEWPORT_MARGIN = 8;
  let charPicker = $state<{ top: number; left: number } | null>(null);
  let charMenuEl = $state<HTMLDivElement | null>(null);
  let charResolvedTop = $state<number | null>(null);

  function openCharPicker(e: MouseEvent) {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    charPicker = { top: rect.bottom + 4, left: rect.left };
  }

  function pickOverride(c: CharacterRef) {
    overrideChar = { id: c.id, name: c.name, team: c.team, edition: c.edition };
    charPicker = null;
  }

  // Clamp/flip the popover so it stays within the viewport.
  $effect(() => {
    if (!charMenuEl || !charPicker) return;
    const height = charMenuEl.offsetHeight;
    const vh = window.innerHeight;
    if (charPicker.top + height > vh - CHAR_VIEWPORT_MARGIN) {
      charResolvedTop = Math.max(CHAR_VIEWPORT_MARGIN, charPicker.top - height);
    } else {
      charResolvedTop = charPicker.top;
    }
  });

  // Outside pointerdown closes the override picker.
  $effect(() => {
    if (!charPicker) return;
    function onWindowPointerDown(e: PointerEvent) {
      const target = e.target as HTMLElement;
      if (!target.closest("[data-char-picker]")) charPicker = null;
    }
    window.addEventListener("pointerdown", onWindowPointerDown);
    return () => window.removeEventListener("pointerdown", onWindowPointerDown);
  });

  function portalToBody(node: HTMLElement) {
    document.body.appendChild(node);
    return () => node.remove();
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
        {#if overrideChar}
          <span class="text-[11px] italic text-secondary">(override)</span>
        {/if}
        {#if ctx.scriptCharacters}
          <button
            type="button"
            onclick={openCharPicker}
            class="rounded border border-border px-1.5 py-0.5 text-[11px] text-muted transition-colors hover:bg-hover"
          >
            Change
          </button>
        {/if}
        {#if overrideChar}
          <button
            type="button"
            onclick={() => (overrideChar = null)}
            class="flex h-5 w-5 items-center justify-center rounded-full text-muted transition-colors hover:bg-hover hover:text-primary"
            aria-label="Reset to derived character"
            title="Reset to derived character"
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
    {/if}

    {#if rightId && wrongId}
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
        ? `Shows the ${cfg.teamLabel}`
        : "Wrong player"}
      players={picker.slot === "right" ? rightCandidates : wrongCandidates}
      anchor={picker.anchor}
      annotate={picker.slot === "right" ? annotateRight : undefined}
      onpick={(id) => setSlot(picker!.slot, id)}
      onclose={() => (picker = null)}
    />
  {/if}

  {#if charPicker}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      bind:this={charMenuEl}
      {@attach portalToBody}
      class="fixed z-50 max-h-[min(60vh,20rem)] w-56 overflow-y-auto rounded-lg border border-border bg-surface py-1 shadow-lg"
      style="top: {charResolvedTop ??
        charPicker.top}px; left: {charPicker.left}px"
      data-char-picker
      onpointerdown={(e: PointerEvent) => e.stopPropagation()}
      role="menu"
      tabindex="-1"
      aria-label="Shows the {cfg.teamLabel}"
    >
      <div
        class="px-3 py-1 text-[10px] font-semibold uppercase tracking-wide text-muted"
      >
        Shows the {cfg.teamLabel}
      </div>
      {#if overrideCandidates.length === 0}
        <div class="px-3 py-2 text-sm text-muted">No characters available</div>
      {:else}
        {#each overrideCandidates as c (c.id)}
          <button
            type="button"
            role="menuitem"
            onclick={() => pickOverride(c)}
            class="flex w-full items-center gap-2 px-3 py-1.5 text-left transition-colors hover:bg-hover"
          >
            <img
              src="/characters/{c.edition}/{c.id}{iconSuffix(c.team)}.webp"
              alt=""
              draggable="false"
              class="h-8 w-8 shrink-0 rounded-full"
              onerror={(e: Event) =>
                ((e.target as HTMLImageElement).style.display = "none")}
            />
            <span class="block truncate text-sm font-medium text-primary">
              {c.name}
            </span>
          </button>
        {/each}
      {/if}
    </div>
  {/if}
{/if}
