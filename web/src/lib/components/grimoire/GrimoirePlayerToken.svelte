<script lang="ts">
  import { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { teamCardColors, goodColors, evilColors, teamDataAttr } from "~/lib/team-styles";
  import {
    useComposedGesture,
    panComposition,
    type GestureCustomEvent,
  } from "svelte-gestures";
  import type { GrimoirePlayer } from "./types";

  let {
    player,
    zoom,
    roundLabel = "Round",
    showNotes = false,
    highlightAttach = false,
    onmove,
    onrename,
    ontoggledeath,
    ongamenote,
    onroundnote,
    onalignment,
  }: {
    player: GrimoirePlayer;
    zoom: number;
    roundLabel?: string;
    showNotes?: boolean;
    highlightAttach?: boolean;
    onmove?: (x: number, y: number) => void;
    onrename?: (name: string) => void;
    ontoggledeath?: () => void;
    ongamenote?: (note: string) => void;
    onroundnote?: (note: string) => void;
    onalignment?: (alignment: string) => void;
  } = $props();

  // Effective alignment: override or default from team
  // Travellers/Fabled/Loric have no default — alignment must be explicitly set
  const defaultAlignment = $derived<"good" | "evil" | undefined>(
    player.team === Team.TOWNSFOLK || player.team === Team.OUTSIDER
      ? "good"
      : player.team === Team.MINION || player.team === Team.DEMON
        ? "evil"
        : undefined,
  );
  const effectiveAlignment = $derived(player.alignment ?? defaultAlignment);
  const hasAlignmentOverride = $derived(player.alignment !== undefined);

  const iconSuffix = $derived(
    effectiveAlignment === "good"
      ? "_g"
      : effectiveAlignment === "evil"
        ? "_e"
        : "",
  );
  const iconUrl = $derived(
    `/characters/${player.edition}/${player.characterId}${iconSuffix}.webp`,
  );

  const colorClass = $derived(
    hasAlignmentOverride
      ? player.alignment === "good"
        ? goodColors
        : evilColors
      : (teamCardColors[player.team] ?? "border-border bg-surface-alt"),
  );
  const hasNotes = $derived(!!player.gameNote || !!player.roundNote);

  let dragging = $state(false);
  let didDrag = $state(false);
  let offsetX = $state(0);
  let offsetY = $state(0);
  let imgError = $state(false);
  let localName: string | null = $state(null);
  const displayName = $derived(localName ?? player.name);

  let menuOpen = $state(false);
  let activeNoteType = $state<"game" | "round" | null>(null);
  let tokenEl: HTMLDivElement;

  function handleWindowPointerDown(e: PointerEvent) {
    if (!menuOpen && !activeNoteType) return;
    if (tokenEl && tokenEl.contains(e.target as Node)) return;
    menuOpen = false;
    activeNoteType = null;
  }

  const DRAG_THRESHOLD = 3;
  let dragStartClientX = 0;
  let dragStartClientY = 0;

  const gesture = useComposedGesture(
    (register) => {
      register(panComposition, { delay: 0, touchAction: "none" });
      return (activeEvents: PointerEvent[]) => {};
    },
    {
      oncomposedGesturedown: (e: GestureCustomEvent) => {
        e.detail.event.stopPropagation();
        if (menuOpen || activeNoteType) return;
        dragging = true;
        didDrag = false;
        dragStartClientX = e.detail.event.clientX;
        dragStartClientY = e.detail.event.clientY;
        offsetX = 0;
        offsetY = 0;
      },
      oncomposedGesturemove: (e: GestureCustomEvent) => {
        if (!dragging) return;
        offsetX = (e.detail.event.clientX - dragStartClientX) / zoom;
        offsetY = (e.detail.event.clientY - dragStartClientY) / zoom;
        if (
          Math.abs(offsetX) > DRAG_THRESHOLD ||
          Math.abs(offsetY) > DRAG_THRESHOLD
        ) {
          if (!didDrag) {
            e.detail.attachmentNode.setPointerCapture(
              e.detail.event.pointerId,
            );
          }
          didDrag = true;
        }
      },
      oncomposedGestureup: (e: GestureCustomEvent) => {
        if (!dragging) return;
        dragging = false;
        if (didDrag) {
          if (offsetX !== 0 || offsetY !== 0) {
            onmove?.(player.x + offsetX, player.y + offsetY);
          }
        } else {
          menuOpen = !menuOpen;
          activeNoteType = null;
        }
        offsetX = 0;
        offsetY = 0;
      },
    } as Record<string, (e: GestureCustomEvent) => void>,
  );

  function handleMenuAction(action: () => void) {
    action();
    menuOpen = false;
  }

  function openNote(type: "game" | "round") {
    activeNoteType = type;
    menuOpen = false;
  }
</script>

<svelte:window onpointerdown={handleWindowPointerDown} />

<div
  class="absolute select-none"
  style="left: {player.x + offsetX}px; top: {player.y +
    offsetY}px; transform: translate(-50%, -50%); z-index: {dragging
    ? 50
    : menuOpen || activeNoteType
      ? 40
      : 1};"
  bind:this={tokenEl}
  {...gesture}
  role="button"
  tabindex="0"
  onkeydown={(e: KeyboardEvent) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      menuOpen = true;
    }
  }}
>
  <div
    class="card-slate flex h-32 w-32 flex-col items-center justify-center rounded-full p-1 transition-shadow {colorClass} {highlightAttach ? 'ring-3 ring-primary/40' : ''}"
    class:token-bezel={!player.isDead}
    class:token-bezel-dead={player.isDead}
    class:token-bezel-drag={dragging}
    class:grayscale={player.isDead}
    class:opacity-75={player.isDead}
    data-team={teamDataAttr[player.team] ?? ""}
  >
    {#if player.isDead}
      <div class="absolute inset-0 rounded-full bg-black/20"></div>
    {/if}
    {#if !imgError && player.characterId}
      <img
        src={iconUrl}
        alt={player.characterName}
        class="h-20 w-20 shrink-0 drop-shadow-sm"
        onerror={() => (imgError = true)}
        draggable="false"
      />
    {:else}
      <div
        class="flex h-20 w-20 shrink-0 items-center justify-center rounded-full bg-element text-xl text-secondary"
      >
        ?
      </div>
    {/if}
    <span
      class="relative z-10 -mt-1 max-w-[100px] truncate text-center text-[10px] font-bold uppercase tracking-wide leading-tight text-primary drop-shadow-sm"
    >
      {player.characterName}
    </span>
    {#if player.isDead}
      <div
        class="absolute -right-1 -top-1 flex h-7 w-7 items-center justify-center rounded-full bg-surface text-xs"
        title={player.ghostVoteUsed
          ? "Ghost vote used"
          : "Ghost vote available"}
      >
        {#if player.ghostVoteUsed}
          <svg
            class="h-4 w-4 text-muted"
            viewBox="0 0 24 24"
            fill="currentColor"
          >
            <path
              d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.42 0-8-3.58-8-8 0-1.85.63-3.55 1.69-4.9L16.9 18.31C15.55 19.37 13.85 20 12 20zm6.31-3.1L7.1 5.69C8.45 4.63 10.15 4 12 4c4.42 0 8 3.58 8 8 0 1.85-.63 3.55-1.69 4.9z"
            />
          </svg>
        {:else}
          <svg
            class="h-4 w-4 text-secondary"
            viewBox="0 0 24 24"
            fill="currentColor"
          >
            <path
              d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"
            />
          </svg>
        {/if}
      </div>
    {/if}
    {#if hasNotes && !showNotes && !activeNoteType}
      <div
        class="absolute -left-1 -top-1 flex h-6 w-6 items-center justify-center rounded-full bg-amber-100 dark:bg-amber-900/50"
        title="Has notes"
      >
        <svg
          class="h-3.5 w-3.5 text-amber-600 dark:text-amber-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
          />
        </svg>
      </div>
    {/if}
  </div>
  <input
    class="mt-1 w-24 bg-transparent text-center text-xs font-medium text-primary outline-none focus:border-b focus:border-primary"
    value={displayName}
    oninput={(e: Event) => {
      localName = (e.currentTarget as HTMLInputElement).value;
    }}
    onblur={() => {
      if (localName !== null && localName !== player.name)
        onrename?.(localName);
      localName = null;
    }}
    onkeydown={(e: KeyboardEvent) => {
      if (e.key === "Enter") (e.currentTarget as HTMLInputElement).blur();
    }}
    onpointerdown={(e: PointerEvent) => e.stopPropagation()}
  />
  <!-- Read-only notes display (when canvas toggle is on) -->
  {#if showNotes && !activeNoteType && hasNotes}
    <div class="w-28 space-y-0.5 text-center">
      {#if player.roundNote}
        <p
          class="truncate text-[9px] italic text-amber-600 dark:text-amber-400"
          title={player.roundNote}
        >
          {player.roundNote}
        </p>
      {/if}
      {#if player.gameNote}
        <p
          class="truncate text-[9px] italic text-indigo-600 dark:text-indigo-400"
          title={player.gameNote}
        >
          {player.gameNote}
        </p>
      {/if}
    </div>
  {/if}
  <!-- Inline note input (opened from context menu) -->
  {#if activeNoteType === "round"}
    <input
      type="text"
      class="mt-1 w-28 rounded border border-amber-400 bg-surface px-1.5 py-0.5 text-center text-[10px] italic text-amber-600 outline-none dark:border-amber-600 dark:text-amber-400"
      value={player.roundNote}
      placeholder="{roundLabel} note..."
      oninput={(e) =>
        onroundnote?.((e.currentTarget as HTMLInputElement).value)}
      onblur={() => (activeNoteType = null)}
      onkeydown={(e) => {
        if (e.key === "Enter" || e.key === "Escape")
          (e.currentTarget as HTMLInputElement).blur();
      }}
      onpointerdown={(e) => e.stopPropagation()}
      autofocus
    />
  {:else if activeNoteType === "game"}
    <input
      type="text"
      class="mt-1 w-28 rounded border border-indigo-400 bg-surface px-1.5 py-0.5 text-center text-[10px] italic text-indigo-600 outline-none dark:border-indigo-600 dark:text-indigo-400"
      value={player.gameNote}
      placeholder="Game note..."
      oninput={(e) => ongamenote?.((e.currentTarget as HTMLInputElement).value)}
      onblur={() => (activeNoteType = null)}
      onkeydown={(e) => {
        if (e.key === "Enter" || e.key === "Escape")
          (e.currentTarget as HTMLInputElement).blur();
      }}
      onpointerdown={(e) => e.stopPropagation()}
      autofocus
    />
  {/if}

  <!-- Context menu -->
  {#if menuOpen}
    <div
      class="absolute left-1/2 top-full z-50 mt-1 min-w-[160px] -translate-x-1/2 rounded-lg border border-border bg-surface py-1 shadow-lg"
      onpointerdown={(e: PointerEvent) => e.stopPropagation()}
    >
      <button
        class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-hover"
        onclick={() => handleMenuAction(() => ontoggledeath?.())}
      >
        {#if player.isDead}
          <svg
            class="h-4 w-4 text-green-500"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"
            />
          </svg>
          <span>Revive</span>
        {:else}
          <svg
            class="h-4 w-4 text-red-500"
            viewBox="0 0 24 24"
            fill="currentColor"
          >
            <path
              d="M12 2C7.58 2 4 5.58 4 10c0 3.88 3.18 7.54 5.43 9.21.40.30.87.54 1.35.68.49.15.99.11 1.22.11s.73.04 1.22-.11c.48-.14.95-.38 1.35-.68C15.82 18.54 21 14.88 21 11a9 9 0 0 0-9-9zm-3 10a1.5 1.5 0 1 1 0-3 1.5 1.5 0 0 1 0 3zm6 0a1.5 1.5 0 1 1 0-3 1.5 1.5 0 0 1 0 3z"
            />
          </svg>
          <span>Kill</span>
        {/if}
      </button>
      {#if onalignment}
        <div class="flex items-center gap-1 border-t border-border px-3 py-2">
          <span class="mr-auto text-xs text-secondary">Alignment</span>
          <button
            class="rounded px-2 py-0.5 text-xs font-medium transition-colors {effectiveAlignment ===
            'good'
              ? 'bg-blue-100 text-blue-700 dark:bg-blue-500/20 dark:text-blue-300'
              : 'text-muted hover:bg-hover hover:text-medium'}"
            onclick={() =>
              handleMenuAction(() =>
                onalignment?.(
                  effectiveAlignment === "good" && hasAlignmentOverride
                    ? ""
                    : "good",
                ),
              )}>Good</button
          >
          <button
            class="rounded px-2 py-0.5 text-xs font-medium transition-colors {effectiveAlignment ===
            'evil'
              ? 'bg-red-100 text-red-700 dark:bg-red-500/20 dark:text-red-300'
              : 'text-muted hover:bg-hover hover:text-medium'}"
            onclick={() =>
              handleMenuAction(() =>
                onalignment?.(
                  effectiveAlignment === "evil" && hasAlignmentOverride
                    ? ""
                    : "evil",
                ),
              )}>Evil</button
          >
        </div>
      {/if}
      <button
        class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-hover"
        onclick={() => openNote("round")}
      >
        <svg
          class="h-4 w-4 text-amber-500"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <circle cx="12" cy="12" r="10" /><path d="M12 6v6l4 2" />
        </svg>
        <span>{roundLabel} Note</span>
      </button>
      <button
        class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-hover"
        onclick={() => openNote("game")}
      >
        <svg
          class="h-4 w-4 text-indigo-500"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
          />
        </svg>
        <span>Game Note</span>
      </button>
    </div>
  {/if}
</div>
