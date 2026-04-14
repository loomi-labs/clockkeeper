<script lang="ts">
  import { page } from "$app/state";
  import type {
    Character,
    Game,
    BagSubstitution,
  } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { formatReminder } from "~/lib/format";
  import {
    teamCardColors,
    teamNameColors,
    teamDataAttr,
    iconSuffix,
  } from "~/lib/team-styles";

  let {
    game,
    scriptCharacters = [],
    deadRoleIds,
    activeRound,
    completedActions,
    gameNotes,
    roundNotes,
    ontoggle,
    ondeath,
    onundodeath,
    ongamenote,
    onroundnote,
    alignments,
    bagSubstitutions,
    playerNames,
    bluffs,
    onalignment,
  }: {
    game: Game;
    scriptCharacters?: Character[];
    deadRoleIds?: Set<string>;
    activeRound?: number;
    completedActions?: Set<string>;
    gameNotes?: Map<string, string>;
    roundNotes?: Map<string, string>;
    alignments?: Map<string, string>;
    bagSubstitutions?: BagSubstitution[];
    playerNames?: Map<string, string>;
    bluffs?: Character[];
    ontoggle?: (id: string, done: boolean) => void;
    ondeath?: (roleId: string) => void;
    onundodeath?: (roleId: string) => void;
    ongamenote?: (id: string, note: string) => void;
    onroundnote?: (id: string, note: string) => void;
    onalignment?: (id: string, alignment: string) => void;
  } = $props();

  import { onDestroy } from "svelte";
  import { usePan, type GestureCustomEvent } from "svelte-gestures";

  const SWIPE_THRESHOLD = 80;
  const HORIZONTAL_LOCK_THRESHOLD = 8;
  const NON_INTERACTIVE_SPECIALS = new Set(["dusk", "dawn"]);
  const SNAP_BACK_TRANSITION = "transform 250ms cubic-bezier(0.2, 0, 0, 1)";

  // Direct DOM manipulation during swipes avoids Svelte re-renders that can
  // interfere with active pointer/touch events on mobile.
  let activeDrag: {
    id: string;
    startX: number;
    startY: number;
    dx: number;
    el: HTMLElement;
    overlayEl: HTMLElement | null;
    lockedHorizontal: boolean;
    pointerId: number;
    removeTouchHandler: (() => void) | null;
  } | null = null;

  onDestroy(() => {
    activeDrag?.removeTouchHandler?.();
    activeDrag = null;
  });

  const gestureCache = new Map<string, object>();

  function panProps(entryId: string, onLeftSwipe?: () => void) {
    if (!ontoggle && !onLeftSwipe) return {};

    let cached = gestureCache.get(entryId);
    if (cached) return cached;

    const isCharEntry = !(entryId in SPECIAL_ENTRIES);

    cached = usePan(
      () => {},
      () => ({ delay: 0, touchAction: "pan-y" as const }),
      {
        onpandown: (e: GestureCustomEvent) => {
          // Don't start gesture on interactive elements — Svelte 5
          // delegates pointerdown to the document root, so the
          // stopPropagation pattern doesn't prevent the library's
          // direct DOM listener from firing first.
          const target = e.detail.event.target as HTMLElement;
          if (target.closest("button, a, input, textarea, [data-overflow-menu]")) return;

          const node = e.detail.attachmentNode;
          const wrapper = node.parentElement;
          const pointerId = e.detail.event.pointerId;
          const startX = e.detail.event.clientX;
          const startY = e.detail.event.clientY;

          // Capture immediately to prevent the library's pointerleave
          // handler from killing the gesture. With touch-action: pan-y,
          // the browser releases implicit capture, so we need explicit.
          node.setPointerCapture(pointerId);

          // Touchmove handler decides direction and controls scroll.
          // While undecided: preventDefault keeps the browser from
          // starting a scroll (which would fire pointercancel).
          // Horizontal: keep preventing, swipe takes over.
          // Vertical: release capture → library ends gesture → browser scrolls.
          const handleTouchMove = (te: TouchEvent) => {
            if (!activeDrag) return;
            const touch = te.touches[0];
            if (!touch) return;
            const dx = Math.abs(touch.clientX - startX);
            const dy = Math.abs(touch.clientY - startY);

            if (activeDrag.lockedHorizontal) {
              te.preventDefault();
              return;
            }

            if (Math.max(dx, dy) < HORIZONTAL_LOCK_THRESHOLD) {
              // Not enough movement to decide — prevent default to keep
              // the browser from making a premature scroll decision
              te.preventDefault();
              return;
            }

            if (dx > dy * 1.5) {
              activeDrag.lockedHorizontal = true;
              te.preventDefault();
            } else {
              // Vertical intent — release capture, let browser scroll.
              // Library fires lostpointercapture → onpanup → cleanup.
              node.releasePointerCapture(pointerId);
            }
          };
          node.addEventListener("touchmove", handleTouchMove, {
            passive: false,
          });

          activeDrag = {
            id: entryId,
            startX,
            startY,
            dx: 0,
            el: node,
            overlayEl: wrapper?.querySelector("[data-swipe-overlay]") ?? null,
            lockedHorizontal: false,
            pointerId,
            removeTouchHandler: () =>
              node.removeEventListener("touchmove", handleTouchMove),
          };
        },
        onpanmove: (e: GestureCustomEvent) => {
          if (!activeDrag || activeDrag.id !== entryId) return;

          // Direction locking for pointer (mouse) events — touchmove
          // handler covers touch devices, but mouse has no touch events.
          if (!activeDrag.lockedHorizontal) {
            const adx = Math.abs(e.detail.event.clientX - activeDrag.startX);
            const ady = Math.abs(e.detail.event.clientY - activeDrag.startY);
            if (Math.max(adx, ady) < HORIZONTAL_LOCK_THRESHOLD) return;
            if (adx > ady * 1.5) {
              activeDrag.lockedHorizontal = true;
            } else {
              // Vertical intent — abort gesture, release capture like the touch path
              activeDrag.el.releasePointerCapture(e.detail.event.pointerId);
              activeDrag.removeTouchHandler?.();
              activeDrag = null;
              return;
            }
          }

          let dx = e.detail.event.clientX - activeDrag.startX;
          if (!ontoggle) dx = Math.min(0, dx);
          if (!isCharEntry || (!ondeath && !onundodeath))
            dx = Math.max(0, dx);

          activeDrag.dx = dx;

          // Direct DOM updates — no Svelte re-render
          activeDrag.el.style.transform = `translate3d(${dx}px, 0, 0)`;
          activeDrag.el.style.transition = "none";
          if (activeDrag.overlayEl) {
            const dir = dx > 0 ? "right" : dx < 0 ? "left" : null;
            activeDrag.overlayEl.style.display = dir ? "" : "none";
            for (const child of activeDrag.overlayEl.children) {
              const el = child as HTMLElement;
              el.classList.toggle(
                "hidden",
                el.dataset.dir !== dir,
              );
            }
          }
        },
        onpanup: () => {
          if (!activeDrag || activeDrag.id !== entryId) return;
          const { dx, el, overlayEl, removeTouchHandler } = activeDrag;

          // Clean up touchmove handler
          removeTouchHandler?.();

          if (dx > SWIPE_THRESHOLD) {
            const isDone = completedActions?.has(entryId) ?? false;
            ontoggle?.(entryId, !isDone);
          } else if (dx < -SWIPE_THRESHOLD && isCharEntry) {
            const isDead = deadRoleIds?.has(entryId) ?? false;
            if (isDead) {
              onundodeath?.(entryId);
            } else {
              ondeath?.(entryId);
            }
          }

          // Animate snap-back
          el.style.transition = SNAP_BACK_TRANSITION;
          el.style.transform = "translate3d(0, 0, 0)";
          if (overlayEl) {
            overlayEl.style.display = "none";
          }

          activeDrag = null;
        },
      },
    );
    gestureCache.set(entryId, cached);
    return cached;
  }

  interface NightEntry {
    id: string;
    name: string;
    reminder: string;
    team?: number;
    edition?: string;
    isSpecial: boolean;
    inPlay: boolean;
    isDead: boolean;
  }

  const SPECIAL_ENTRIES: Record<
    string,
    {
      name: string;
      reminder: string;
      position: { first: number; other: number };
      minPlayers?: number;
    }
  > = {
    dusk: {
      name: "Dusk",
      reminder: "Night begins. All players close their eyes.",
      position: { first: 0, other: 0 },
    },
    minioninfo: {
      name: "Minion Info",
      reminder:
        "Show the *THIS IS THE DEMON* token. Point to the Demon. Show the *THESE ARE YOUR MINIONS* token. Point to the other Minions.",
      position: { first: 20, other: -1 },
      minPlayers: 7,
    },
    demoninfo: {
      name: "Demon Info",
      reminder:
        "Show the *THESE ARE YOUR MINIONS* token. Point to all Minions. Show the *THESE CHARACTERS ARE NOT IN PLAY* token. Show 3 not-in-play good character tokens.",
      position: { first: 25, other: -1 },
      minPlayers: 7,
    },
    dawn: {
      name: "Dawn",
      reminder: "Night ends. All players open their eyes.",
      position: { first: 999, other: 999 },
    },
  };

  // NightOrder applies opacity separately, so Traveller doesn't need opacity-60 in unselected.
  const unselectedColors: Record<number, string> = {
    [Team.TOWNSFOLK]:
      "border-blue-100 bg-blue-50/50 dark:border-blue-800/50 dark:bg-blue-950/20",
    [Team.OUTSIDER]:
      "border-cyan-100 bg-cyan-50/50 dark:border-cyan-800/50 dark:bg-cyan-950/20",
    [Team.MINION]:
      "border-orange-100 bg-orange-50/50 dark:border-orange-800/50 dark:bg-orange-950/20",
    [Team.DEMON]:
      "border-red-100 bg-red-50/50 dark:border-red-800/50 dark:bg-red-950/20",
    [Team.TRAVELLER]: "card-traveller",
    [Team.FABLED]:
      "border-yellow-200 bg-yellow-50/50 dark:border-yellow-700/50 dark:bg-yellow-950/20",
    [Team.LORIC]:
      "border-green-100 bg-green-50/50 dark:border-green-800/50 dark:bg-green-950/20",
  };
  const allSelectedChars = $derived([
    ...(game.selectedCharacters ?? []),
    ...(game.selectedTravellerCharacters ?? []),
    ...(game.extraCharacterDetails ?? []),
  ]);
  const selectedIdSet = $derived(new Set(allSelectedChars.map((c) => c.id)));
  const allScriptChars = $derived.by(() => {
    const seen = new Set<string>();
    const result: Character[] = [];
    for (const c of [
      ...scriptCharacters,
      ...(game.selectedTravellerCharacters ?? []),
      ...(game.extraCharacterDetails ?? []),
    ]) {
      if (!seen.has(c.id)) {
        seen.add(c.id);
        result.push(c);
      }
    }
    return result;
  });

  let showAll = $state(false);

  function buildNightOrder(night: "first" | "other"): NightEntry[] {
    const posField = night === "first" ? "firstNight" : "otherNight";
    const reminderField =
      night === "first" ? "firstNightReminder" : "otherNightReminder";
    const source = showAll ? allScriptChars : allSelectedChars;
    const charEntries: (NightEntry & { pos: number })[] = source
      .filter((c) => c[reminderField])
      .map((c) => ({
        id: c.id,
        name: c.name,
        reminder: c[reminderField],
        team: c.team,
        edition: c.edition,
        isSpecial: false,
        inPlay: selectedIdSet.has(c.id),
        isDead: deadRoleIds?.has(c.id) ?? false,
        pos: c[posField] || 500,
      }));
    // Add bag substitution characters (e.g., Drunk's townsfolk token) to the night order.
    if (bagSubstitutions) {
      const existingIds = new Set(charEntries.map((e) => e.id));
      for (const bs of bagSubstitutions) {
        if (!bs.characterId || existingIds.has(bs.characterId)) continue;
        const subChar = scriptCharacters.find((c) => c.id === bs.characterId);
        if (!subChar || !subChar[reminderField]) continue;
        charEntries.push({
          id: bs.characterId,
          name: `${subChar.name} (${bs.causedByName})`,
          reminder: subChar[reminderField],
          team: subChar.team,
          edition: subChar.edition,
          isSpecial: false,
          inPlay: true,
          isDead: deadRoleIds?.has(bs.characterId) ?? false,
          pos: subChar[posField] || 500,
        });
      }
    }
    const specialEntries: (NightEntry & { pos: number })[] = [];
    for (const [id, entry] of Object.entries(SPECIAL_ENTRIES)) {
      const pos =
        night === "first" ? entry.position.first : entry.position.other;
      if (pos < 0) continue;
      if (entry.minPlayers && game.playerCount < entry.minPlayers) continue;
      specialEntries.push({
        id,
        name: entry.name,
        reminder: entry.reminder,
        isSpecial: true,
        inPlay: true,
        isDead: false,
        pos,
      });
    }
    const all = [...charEntries, ...specialEntries];
    all.sort((a, b) => a.pos - b.pos);
    return all;
  }

  const firstNightOrder = $derived(buildNightOrder("first"));
  const otherNightOrder = $derived(buildNightOrder("other"));
  let manualNight = $state<"first" | "other">("first");
  const activeNight = $derived(
    activeRound !== undefined
      ? activeRound === 1
        ? "first"
        : "other"
      : manualNight,
  );
  const activeOrder = $derived(
    activeNight === "first" ? firstNightOrder : otherNightOrder,
  );
  const specialIcons: Record<string, string> = {
    dusk: "/night-dusk.webp",
    dawn: "/night-dawn.webp",
    minioninfo: "/night-minioninfo.webp",
    demoninfo: "/night-demoninfo.webp",
  };
  function handlePrint() {
    window.print();
  }

  let editingNoteId = $state<string | null>(null);
  let overflowMenu = $state<{
    entryId: string;
    entryName: string;
    entryTeam: number;
    entryIsDead: boolean;
    top: number;
    right: number;
  } | null>(null);

  function openOverflowMenu(
    entry: NightEntry,
    button: HTMLElement,
  ) {
    if (overflowMenu?.entryId === entry.id) {
      overflowMenu = null;
      return;
    }
    const rect = button.getBoundingClientRect();
    overflowMenu = {
      entryId: entry.id,
      entryName: entry.name,
      entryTeam: entry.team ?? 0,
      entryIsDead: entry.isDead,
      top: rect.bottom + 4,
      right: window.innerWidth - rect.right,
    };
  }

  function closeOverflowMenu() {
    overflowMenu = null;
  }

  function handleWindowClick(e: MouseEvent) {
    if (!overflowMenu) return;
    const target = e.target as HTMLElement;
    if (!target.closest("[data-overflow-menu]")) {
      overflowMenu = null;
    }
  }

  // Get the effective team for styling, accounting for alignment overrides.
  function effectiveTeam(entryId: string, originalTeam: number): number {
    const align = alignments?.get(entryId);
    if (!align) return originalTeam;
    if (align === "good") return Team.TOWNSFOLK;
    if (align === "evil") return Team.MINION;
    return originalTeam;
  }

  function effectiveIconSuffix(entryId: string, originalTeam: number): string {
    const align = alignments?.get(entryId);
    if (align === "good") return "_g";
    if (align === "evil") return "_e";
    return iconSuffix(originalTeam);
  }

  // Determine the default alignment for a team (what it is without override).
  function defaultAlignmentForTeam(
    team: number,
  ): "good" | "evil" | undefined {
    if (team === Team.TOWNSFOLK || team === Team.OUTSIDER) return "good";
    if (team === Team.MINION || team === Team.DEMON) return "evil";
    return undefined; // Travellers, Fabled, Loric
  }

  // Cycle alignment: clicking advances to the next state.
  // For characters with a default: default → opposite → default (toggle opposite)
  // For travellers (no default): undefined → good → evil → undefined
  function cycleAlignment(entryId: string, team: number) {
    const current = alignments?.get(entryId);
    const defaultAlign = defaultAlignmentForTeam(team);

    let next: string;
    if (defaultAlign === undefined) {
      // Traveller/Fabled/Loric: undefined → good → evil → undefined
      if (!current) next = "good";
      else if (current === "good") next = "evil";
      else next = ""; // back to undefined
    } else {
      // Has default: no override → opposite → clear
      if (!current) next = defaultAlign === "good" ? "evil" : "good";
      else next = ""; // clear override, back to default
    }
    onalignment?.(entryId, next);
  }

  // Get display label/color for current effective alignment.
  function alignmentDisplay(
    entryId: string,
    team: number,
  ): { label: string; colorClass: string } {
    const override = alignments?.get(entryId);
    const defaultAlign = defaultAlignmentForTeam(team);
    const effective = override || defaultAlign;

    if (effective === "good") {
      return {
        label: "G",
        colorClass: override
          ? "text-blue-500"
          : "text-muted",
      };
    }
    if (effective === "evil") {
      return {
        label: "E",
        colorClass: override
          ? "text-red-500"
          : "text-muted",
      };
    }
    // Undefined alignment (traveller with no override)
    return { label: "?", colorClass: "text-muted" };
  }
</script>

<svelte:window onclick={handleWindowClick} />

<div class="space-y-4">
  <h2 class="print-title hidden text-xl font-bold">
    {activeNight === "first" ? "First Night" : "Other Nights"}
  </h2>
  <div class="no-print flex items-center justify-between">
    {#if activeRound === undefined}
      <div class="flex gap-1 rounded-lg bg-element p-1">
        <button
          onclick={() => (manualNight = "first")}
          class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors {activeNight ===
          'first'
            ? 'bg-surface text-primary shadow-sm'
            : 'text-secondary hover:text-medium'}">First Night</button
        >
        <button
          onclick={() => (manualNight = "other")}
          class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors {activeNight ===
          'other'
            ? 'bg-surface text-primary shadow-sm'
            : 'text-secondary hover:text-medium'}">Other Nights</button
        >
      </div>
    {:else}
      <div></div>
    {/if}
    <div class="flex items-center gap-3">
      <button
        onclick={() => (showAll = !showAll)}
        class="flex items-center gap-1.5 text-xs text-secondary"
      >
        <div
          class="flex h-5 w-5 shrink-0 items-center justify-center rounded border transition-colors {showAll
            ? 'border-green-500 bg-green-500'
            : 'border-border-strong'}"
        >
          {#if showAll}<svg
              class="h-3 w-3 text-white"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              ><path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="3"
                d="M5 13l4 4L19 7"
              /></svg
            >{/if}
        </div>
        Show all
      </button>
      {#if activeRound === undefined}
        <button
          onclick={handlePrint}
          class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-sm text-secondary transition-colors hover:bg-hover hover:text-medium"
        >
          <svg
            class="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            ><path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M17 17h2a2 2 0 002-2v-4a2 2 0 00-2-2H5a2 2 0 00-2 2v4a2 2 0 002 2h2m2 4h6a2 2 0 002-2v-4a2 2 0 00-2-2H9a2 2 0 00-2 2v4a2 2 0 002 2zm8-12V5a2 2 0 00-2-2H9a2 2 0 00-2 2v4h10z"
            /></svg
          >
          Print
        </button>
      {/if}
    </div>
  </div>

  <div class="space-y-1">
    {#if activeOrder.length === 0}
      <p class="py-8 text-center text-sm text-muted">
        No characters with night actions selected.
      </p>
    {:else}
      {#each activeOrder as entry, i (entry.id)}
        {@const isDone = completedActions?.has(entry.id) ?? false}
        {#if entry.isSpecial}
          {@const isInteractive = !NON_INTERACTIVE_SPECIALS.has(entry.id)}
          <div class="overflow-hidden rounded-lg" data-entry={entry.id}>
            <div
              {...isInteractive ? panProps(entry.id) : {}}
              class="relative flex items-center gap-2 bg-element/50 px-2 py-2 sm:gap-3 sm:px-3 sm:py-2.5 {isInteractive &&
              isDone
                ? 'opacity-50 border-l-4 border-l-green-500'
                : ''}"
            >
              <img
                src={specialIcons[entry.id]}
                alt=""
                class="h-12 w-12 shrink-0 object-contain sm:h-20 sm:w-20"
                onerror={(e: Event) =>
                  ((e.target as HTMLImageElement).style.display = "none")}
              />
              <div class="min-w-0 flex-1">
                <span
                  class="text-base font-bold text-primary {isInteractive &&
                  isDone
                    ? 'line-through'
                    : ''}">{entry.name}</span
                >
                <p class="text-sm text-muted">
                  {@html formatReminder(entry.reminder)}
                </p>
                {#if entry.id === "demoninfo" && bluffs && bluffs.length > 0}
                  <div class="mt-2 flex flex-wrap items-center gap-2">
                    <span class="text-xs font-semibold text-secondary"
                      >Bluffs:</span
                    >
                    {#each bluffs as bluff (bluff.id)}
                      <div
                        class="flex items-center gap-1.5 rounded-full border border-border bg-surface px-2 py-0.5"
                      >
                        <img
                          src="/characters/{bluff.edition}/{bluff.id}_g.webp"
                          alt=""
                          class="h-5 w-5 rounded-full"
                          onerror={(e: Event) =>
                            ((e.target as HTMLImageElement).style.display =
                              "none")}
                        />
                        <span class="text-xs font-medium text-primary"
                          >{bluff.name}</span
                        >
                      </div>
                    {/each}
                  </div>
                {/if}
              </div>
              {#if ontoggle && isInteractive}
                <button
                  onclick={() => {
                    const done = completedActions?.has(entry.id) ?? false;
                    ontoggle?.(entry.id, !done);
                  }}
                  class="no-print flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-2 transition-colors {isDone
                    ? 'border-green-500 bg-green-500 text-white'
                    : 'border-border-strong text-transparent hover:border-green-400'}"
                  title={isDone ? "Mark as not done" : "Mark as done"}
                  aria-label={isDone ? "Mark as not done" : "Mark as done"}
                  aria-pressed={isDone}
                >
                  <svg
                    class="h-3.5 w-3.5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    ><path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="3"
                      d="M5 13l4 4L19 7"
                    /></svg
                  >
                </button>
              {/if}
              <span
                class="w-6 shrink-0 text-center text-xs font-bold text-muted"
                >{i + 1}</span
              >
            </div>
          </div>
        {:else}
          {@const leftSwipeAction = ondeath
            ? entry.isDead
              ? () => onundodeath?.(entry.id)
              : () => ondeath?.(entry.id)
            : undefined}
          <div
            class="relative overflow-hidden rounded-lg"
            data-entry={entry.id}
          >
            <!-- Swipe overlays: always in DOM, toggled via direct DOM manipulation -->
            <div
              data-swipe-overlay
              style="display: none"
              class="pointer-events-none absolute inset-0 rounded-lg"
            >
              <!-- Right swipe: mark done -->
              <div
                data-dir="right"
                class="hidden absolute inset-0 flex items-center rounded-lg bg-green-500/20 pl-4"
              >
                <svg
                  class="h-6 w-6 text-green-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  ><path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="3"
                    d="M5 13l4 4L19 7"
                  /></svg
                >
              </div>
              <!-- Left swipe: kill (or revive) -->
              {#if entry.isDead}
                <div
                  data-dir="left"
                  class="hidden absolute inset-0 flex items-center justify-end rounded-lg bg-green-500/20 pr-4"
                >
                  <svg
                    class="h-6 w-6 text-green-500"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    ><path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"
                    /></svg
                  >
                </div>
              {:else}
                <div
                  data-dir="left"
                  class="hidden absolute inset-0 flex items-center justify-end rounded-lg bg-red-500/20 pr-4"
                >
                  <svg
                    class="h-6 w-6 text-red-500"
                    viewBox="0 0 24 24"
                    fill="currentColor"
                    ><path
                      d="M12 2C7.58 2 4 5.58 4 10c0 2.76 1.34 5.2 3.4 6.72V20a1 1 0 001 1h7.2a1 1 0 001-1v-3.28C18.66 15.2 20 12.76 20 10c0-4.42-3.58-8-8-8zm-1 15v-2h2v2h-2zm4-7a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm-5 0a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0z"
                    /></svg
                  >
                </div>
              {/if}
            </div>
            <div
              {...panProps(entry.id, leftSwipeAction)}
              class="card-slate relative flex items-center gap-2 border px-2 py-2 sm:gap-3 sm:px-3 sm:py-2.5 {isDone
                ? 'opacity-50 border-l-4 border-l-green-500'
                : ''} {entry.isDead
                ? (unselectedColors[effectiveTeam(entry.id, entry.team ?? 0)] ??
                    'border-border/50') + ' opacity-40 border-dashed'
                : entry.inPlay
                  ? (teamCardColors[effectiveTeam(entry.id, entry.team ?? 0)] ??
                    'border-border')
                  : (unselectedColors[
                      effectiveTeam(entry.id, entry.team ?? 0)
                    ] ?? 'border-border/50') + ' opacity-40 border-dashed'}"
              data-team={teamDataAttr[
                effectiveTeam(entry.id, entry.team ?? 0)
              ] ?? ""}
            >
              <img
                src="/characters/{entry.edition}/{entry.id}{effectiveIconSuffix(
                  entry.id,
                  entry.team ?? 0,
                )}.webp"
                alt=""
                class="h-12 w-12 shrink-0 rounded-full sm:h-20 sm:w-20 {entry.isDead
                  ? 'grayscale'
                  : ''}"
                onerror={(e: Event) =>
                  ((e.target as HTMLImageElement).style.display = "none")}
              />
              <div class="min-w-0 flex-1">
                <span
                  class="text-sm font-medium sm:text-base {isDone
                    ? 'line-through '
                    : ''}{entry.isDead
                    ? 'line-through text-muted'
                    : (teamNameColors[
                        effectiveTeam(entry.id, entry.team ?? 0)
                      ] ?? 'text-primary')}">{entry.name}{#if playerNames?.get(entry.id)}<span class="ml-1.5 text-xs font-normal text-muted">&mdash; {playerNames.get(entry.id)}</span>{/if}</span
                >
                {#if entry.isDead}<span
                    class="ml-2 text-xs text-red-500 dark:text-red-400"
                    >Dead</span
                  >{/if}
                {#if !entry.isSpecial && alignments?.has(entry.id)}
                  {@const align = alignments.get(entry.id)}
                  <span
                    class="ml-1 rounded px-1.5 py-0.5 text-[10px] font-medium {align ===
                    'good'
                      ? 'bg-blue-100 text-blue-700 dark:bg-blue-500/20 dark:text-blue-300'
                      : 'bg-red-100 text-red-700 dark:bg-red-500/20 dark:text-red-300'}"
                  >
                    {align === "good" ? "Good" : "Evil"}
                  </span>
                {/if}
                <p
                  class="text-xs sm:text-sm {entry.isDead
                    ? 'text-muted'
                    : 'text-secondary'}"
                >
                  {@html formatReminder(entry.reminder)}
                </p>
                {#if !entry.isSpecial && (ongamenote || onroundnote)}
                  {@const hasNotes = !!(
                    gameNotes?.get(entry.id) || roundNotes?.get(entry.id)
                  )}
                  {@const isEditing = editingNoteId === entry.id}
                  {#if isEditing}
                    <div class="no-print mt-1 space-y-1" data-note-panel>
                      <div class="flex items-center gap-1">
                        <svg
                          class="h-3 w-3 shrink-0 text-amber-500"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                          stroke-width="2"
                          ><circle cx="12" cy="12" r="10" /><path
                            d="M12 6v6l4 2"
                          /></svg
                        >
                        <input
                          type="text"
                          class="flex-1 rounded border border-amber-300 bg-transparent px-1.5 py-0.5 text-xs text-primary outline-none focus:border-amber-500 dark:border-amber-700 dark:focus:border-amber-500"
                          value={roundNotes?.get(entry.id) ?? ""}
                          placeholder="Round note..."
                          onblur={(e) => {
                            onroundnote?.(
                              entry.id,
                              (e.currentTarget as HTMLInputElement).value,
                            );
                            const panel = (
                              e.currentTarget as HTMLElement
                            ).closest("[data-note-panel]");
                            if (!panel?.contains(e.relatedTarget as Node)) {
                              editingNoteId = null;
                            }
                          }}
                          onkeydown={(e) => {
                            if (e.key === "Enter" || e.key === "Escape")
                              (e.currentTarget as HTMLInputElement).blur();
                          }}
                        />
                      </div>
                      <div class="flex items-center gap-1">
                        <svg
                          class="h-3 w-3 shrink-0 text-indigo-500"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                          stroke-width="2"
                          ><path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                          /></svg
                        >
                        <input
                          type="text"
                          class="flex-1 rounded border border-indigo-300 bg-transparent px-1.5 py-0.5 text-xs text-primary outline-none focus:border-indigo-500 dark:border-indigo-700 dark:focus:border-indigo-500"
                          value={gameNotes?.get(entry.id) ?? ""}
                          placeholder="Game note..."
                          onblur={(e) => {
                            ongamenote?.(
                              entry.id,
                              (e.currentTarget as HTMLInputElement).value,
                            );
                            const panel = (
                              e.currentTarget as HTMLElement
                            ).closest("[data-note-panel]");
                            if (!panel?.contains(e.relatedTarget as Node)) {
                              editingNoteId = null;
                            }
                          }}
                          onkeydown={(e) => {
                            if (e.key === "Enter" || e.key === "Escape")
                              (e.currentTarget as HTMLInputElement).blur();
                          }}
                        />
                      </div>
                    </div>
                  {:else if hasNotes}
                    <button
                      class="mt-1 space-y-0.5 text-left"
                      onclick={() => (editingNoteId = entry.id)}
                    >
                      {#if roundNotes?.get(entry.id)}
                        <p
                          class="flex items-start gap-1 text-xs italic text-amber-600 dark:text-amber-400"
                        >
                          <svg
                            class="mt-0.5 h-3 w-3 shrink-0"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                            stroke-width="2"
                            ><circle cx="12" cy="12" r="10" /><path
                              d="M12 6v6l4 2"
                            /></svg
                          >
                          {roundNotes.get(entry.id)}
                        </p>
                      {/if}
                      {#if gameNotes?.get(entry.id)}
                        <p
                          class="flex items-start gap-1 text-xs italic text-indigo-600 dark:text-indigo-400"
                        >
                          <svg
                            class="mt-0.5 h-3 w-3 shrink-0"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                            stroke-width="2"
                            ><path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                            /></svg
                          >
                          {gameNotes.get(entry.id)}
                        </p>
                      {/if}
                    </button>
                  {/if}
                {/if}
              </div>
              <!-- Desktop: inline action buttons -->
              <div
                class="no-print hidden shrink-0 items-center gap-1 sm:flex"
              >
                {#if ondeath && !entry.isDead}
                  <button
                    onclick={() => ondeath?.(entry.id)}
                    class="rounded p-1 text-muted transition-colors hover:bg-hover hover:text-red-500"
                    title="Kill"
                    aria-label="Kill {entry.name}"
                  >
                    <svg class="h-4 w-4" viewBox="0 0 24 24" fill="currentColor"
                      ><path
                        d="M12 2C7.58 2 4 5.58 4 10c0 2.76 1.34 5.2 3.4 6.72V20a1 1 0 001 1h7.2a1 1 0 001-1v-3.28C18.66 15.2 20 12.76 20 10c0-4.42-3.58-8-8-8zm-1 15v-2h2v2h-2zm4-7a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm-5 0a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0z"
                      /></svg
                    >
                  </button>
                {:else if onundodeath && entry.isDead}
                  <button
                    onclick={() => onundodeath?.(entry.id)}
                    class="rounded p-1 text-red-400 transition-colors hover:bg-hover hover:text-green-500"
                    title="Undo death"
                    aria-label="Undo death for {entry.name}"
                  >
                    <svg
                      class="h-4 w-4"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="1.5"
                      ><path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M12 2C7.58 2 4 5.58 4 10c0 2.76 1.34 5.2 3.4 6.72V20a1 1 0 001 1h7.2a1 1 0 001-1v-3.28C18.66 15.2 20 12.76 20 10c0-4.42-3.58-8-8-8zm-2 15v-1h4v1h-4zm0-3h1v2h2v-2h1v2h-4zm5.6-2.08l-.6.46V17h-6v-2.62l-.6-.46A5.94 5.94 0 016 10c0-3.31 2.69-6 6-6s6 2.69 6 6a5.94 5.94 0 01-2.4 3.92z"
                      /><line
                        x1="4"
                        y1="4"
                        x2="20"
                        y2="20"
                        stroke-width="2"
                      /></svg
                    >
                  </button>
                {/if}
                {#if ongamenote || onroundnote}
                  <button
                    onclick={() =>
                      (editingNoteId =
                        editingNoteId === entry.id ? null : entry.id)}
                    class="rounded p-1 transition-colors hover:bg-hover {editingNoteId ===
                    entry.id
                      ? 'text-amber-500'
                      : 'text-muted hover:text-amber-500'}"
                    title="Notes"
                    aria-label="Edit notes for {entry.name}"
                  >
                    <svg
                      class="h-4 w-4"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      stroke-width="2"
                      ><path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                      /></svg
                    >
                  </button>
                {/if}
                {#if onalignment}
                  {@const display = alignmentDisplay(entry.id, entry.team ?? 0)}
                  <button
                    onclick={() => cycleAlignment(entry.id, entry.team ?? 0)}
                    class="rounded p-1 text-xs font-bold transition-colors hover:bg-hover {display.colorClass}"
                    title="Change alignment"
                    aria-label="Cycle alignment for {entry.name}"
                    >{display.label}</button
                  >
                {/if}
                <a
                  href="/almanac/{entry.id}?from={encodeURIComponent(
                    page.url.pathname + page.url.search,
                  )}"
                  class="rounded p-1 text-muted transition-colors hover:bg-hover hover:text-medium"
                  title="Almanac"
                  aria-label="View {entry.name} in almanac"
                >
                  <svg
                    class="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    ><path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"
                    /></svg
                  >
                </a>
                <a
                  href="https://wiki.bloodontheclocktower.com/{entry.name.replace(
                    / /g,
                    '_',
                  )}"
                  target="_blank"
                  rel="noopener"
                  class="rounded p-1 text-muted transition-colors hover:bg-hover hover:text-medium"
                  title="Wiki"
                  aria-label="View {entry.name} on wiki"
                >
                  <svg
                    class="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    ><path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                    /></svg
                  >
                </a>
              </div>
              <!-- Mobile: overflow menu -->
              <div
                class="no-print shrink-0 sm:hidden"
                data-overflow-menu
              >
                <button
                  onclick={(e: MouseEvent) =>
                    openOverflowMenu(entry, e.currentTarget as HTMLElement)}
                  class="rounded p-1 text-muted transition-colors hover:bg-hover"
                  aria-label="Actions for {entry.name}"
                >
                  <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="5" r="1.5" fill="currentColor" />
                    <circle cx="12" cy="12" r="1.5" fill="currentColor" />
                    <circle cx="12" cy="19" r="1.5" fill="currentColor" />
                  </svg>
                </button>
              </div>
              {#if ontoggle}
                <button
                  onclick={() => {
                    const done = completedActions?.has(entry.id) ?? false;
                    ontoggle?.(entry.id, !done);
                  }}
                  class="no-print flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-2 transition-colors {isDone
                    ? 'border-green-500 bg-green-500 text-white'
                    : 'border-border-strong text-transparent hover:border-green-400'}"
                  title={isDone ? "Mark as not done" : "Mark as done"}
                  aria-label={isDone
                    ? "Mark {entry.name} as not done"
                    : "Mark {entry.name} as done"}
                  aria-pressed={isDone}
                >
                  <svg
                    class="h-3.5 w-3.5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    ><path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="3"
                      d="M5 13l4 4L19 7"
                    /></svg
                  >
                </button>
              {/if}
              <span
                class="w-6 shrink-0 text-center text-xs font-medium text-muted"
                >{i + 1}</span
              >
            </div>
          </div>
        {/if}
      {/each}
    {/if}
  </div>
</div>

<!-- Mobile overflow menu — rendered at root to escape overflow-hidden + transform containers -->
{#if overflowMenu}
  {@const m = overflowMenu}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="fixed z-50 min-w-[160px] rounded-lg border border-border bg-surface py-1 shadow-lg"
    style="top: {m.top}px; right: {m.right}px"
    data-overflow-menu
    onpointerdown={(e: PointerEvent) => e.stopPropagation()}
  >
    {#if ondeath && !m.entryIsDead}
      <button
        onclick={() => { ondeath?.(m.entryId); closeOverflowMenu(); }}
        class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-primary transition-colors hover:bg-hover"
      >
        <svg class="h-4 w-4 text-red-500" viewBox="0 0 24 24" fill="currentColor"
          ><path d="M12 2C7.58 2 4 5.58 4 10c0 2.76 1.34 5.2 3.4 6.72V20a1 1 0 001 1h7.2a1 1 0 001-1v-3.28C18.66 15.2 20 12.76 20 10c0-4.42-3.58-8-8-8zm-1 15v-2h2v2h-2zm4-7a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0zm-5 0a1.5 1.5 0 11-3 0 1.5 1.5 0 013 0z" /></svg
        >
        Kill
      </button>
    {:else if onundodeath && m.entryIsDead}
      <button
        onclick={() => { onundodeath?.(m.entryId); closeOverflowMenu(); }}
        class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-primary transition-colors hover:bg-hover"
      >
        <svg class="h-4 w-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor"
          ><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z" /></svg
        >
        Revive
      </button>
    {/if}
    {#if ongamenote || onroundnote}
      <button
        onclick={() => { editingNoteId = editingNoteId === m.entryId ? null : m.entryId; closeOverflowMenu(); }}
        class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-primary transition-colors hover:bg-hover"
      >
        <svg class="h-4 w-4 text-amber-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
          ><path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" /></svg
        >
        Notes
      </button>
    {/if}
    {#if onalignment}
      {@const display = alignmentDisplay(m.entryId, m.entryTeam)}
      <button
        onclick={() => { cycleAlignment(m.entryId, m.entryTeam); closeOverflowMenu(); }}
        class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-primary transition-colors hover:bg-hover"
      >
        <span class="flex h-4 w-4 items-center justify-center text-xs font-bold {display.colorClass}">{display.label}</span>
        Alignment
      </button>
    {/if}
    <a
      href="/almanac/{m.entryId}?from={encodeURIComponent(page.url.pathname + page.url.search)}"
      onclick={closeOverflowMenu}
      class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-primary transition-colors hover:bg-hover"
    >
      <svg class="h-4 w-4 text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor"
        ><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" /></svg
      >
      Almanac
    </a>
    <a
      href="https://wiki.bloodontheclocktower.com/{m.entryName.replace(/ /g, '_')}"
      target="_blank"
      rel="noopener"
      onclick={closeOverflowMenu}
      class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-primary transition-colors hover:bg-hover"
    >
      <svg class="h-4 w-4 text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor"
        ><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg
      >
      Wiki
    </a>
  </div>
{/if}
