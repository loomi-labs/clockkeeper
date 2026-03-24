<script lang="ts">
  import { onMount } from "svelte";
  import type { GrimoirePlayer, GrimoireReminder } from "./types";
  import GrimoirePlayerToken from "./GrimoirePlayerToken.svelte";
  import GrimoireReminderToken from "./GrimoireReminderToken.svelte";

  let {
    players,
    reminders,
    roundLabel = "Round",
    onplayermove,
    onremindermove,
    onplayerrename,
    onplayertoggledeath,
    onplayergamenote,
    onplayerroundnote,
    onplayeralignment,
  }: {
    players: GrimoirePlayer[];
    reminders: GrimoireReminder[];
    roundLabel?: string;
    onplayermove?: (id: string, x: number, y: number) => void;
    onremindermove?: (id: string, x: number, y: number) => void;
    onplayerrename?: (id: string, name: string) => void;
    onplayertoggledeath?: (id: string) => void;
    onplayergamenote?: (id: string, note: string) => void;
    onplayerroundnote?: (id: string, note: string) => void;
    onplayeralignment?: (id: string, alignment: string) => void;
  } = $props();

  let panX = $state(0);
  let panY = $state(0);
  let zoom = $state(1);
  let showNotes = $state(false);

  let isPanning = $state(false);
  let panStartX = $state(0);
  let panStartY = $state(0);
  let panOriginX = $state(0);
  let panOriginY = $state(0);

  // Pinch zoom tracking
  let pointers = new Map<number, PointerEvent>();
  let lastPinchDist = 0;

  let canvasEl: HTMLDivElement;

  onMount(() => {
    if (!canvasEl) return;
    const rect = canvasEl.getBoundingClientRect();
    panX = rect.width / 2;
    panY = rect.height / 2;
  });

  function handlePointerDown(e: PointerEvent) {
    pointers.set(e.pointerId, e);

    if (pointers.size === 2) {
      // Start pinch — cancel any pan
      isPanning = false;
      const pts = [...pointers.values()];
      lastPinchDist = Math.hypot(
        pts[0].clientX - pts[1].clientX,
        pts[0].clientY - pts[1].clientY,
      );
      return;
    }

    if (pointers.size === 1) {
      isPanning = true;
      panStartX = e.clientX;
      panStartY = e.clientY;
      panOriginX = panX;
      panOriginY = panY;
      canvasEl.setPointerCapture(e.pointerId);
    }
  }

  function handlePointerMove(e: PointerEvent) {
    pointers.set(e.pointerId, e);

    if (pointers.size === 2) {
      const pts = [...pointers.values()];
      const dist = Math.hypot(
        pts[0].clientX - pts[1].clientX,
        pts[0].clientY - pts[1].clientY,
      );
      if (lastPinchDist > 0) {
        const scale = dist / lastPinchDist;
        const newZoom = Math.max(0.3, Math.min(3.0, zoom * scale));

        // Zoom toward pinch center
        const rect = canvasEl.getBoundingClientRect();
        const centerX = (pts[0].clientX + pts[1].clientX) / 2 - rect.left;
        const centerY = (pts[0].clientY + pts[1].clientY) / 2 - rect.top;
        panX = centerX - (centerX - panX) * (newZoom / zoom);
        panY = centerY - (centerY - panY) * (newZoom / zoom);
        zoom = newZoom;
      }
      lastPinchDist = dist;
      return;
    }

    if (!isPanning) return;
    panX = panOriginX + (e.clientX - panStartX);
    panY = panOriginY + (e.clientY - panStartY);
  }

  function handlePointerUp(e: PointerEvent) {
    pointers.delete(e.pointerId);
    if (pointers.size < 2) {
      lastPinchDist = 0;
    }
    if (pointers.size === 0) {
      isPanning = false;
    }
  }

  function handleWheel(e: WheelEvent) {
    e.preventDefault();
    const delta = e.deltaY > 0 ? 0.9 : 1.1;
    const newZoom = Math.max(0.3, Math.min(3.0, zoom * delta));

    const rect = canvasEl.getBoundingClientRect();
    const cursorX = e.clientX - rect.left;
    const cursorY = e.clientY - rect.top;
    panX = cursorX - (cursorX - panX) * (newZoom / zoom);
    panY = cursorY - (cursorY - panY) * (newZoom / zoom);
    zoom = newZoom;
  }
</script>

<div
  class="relative h-full w-full overflow-hidden bg-page touch-none"
  bind:this={canvasEl}
  onpointerdown={handlePointerDown}
  onpointermove={handlePointerMove}
  onpointerup={handlePointerUp}
  onpointercancel={handlePointerUp}
  onwheel={handleWheel}
  role="application"
  aria-label="Grimoire canvas"
>
  <!-- Notes toggle -->
  <button
    class="absolute right-3 top-3 z-10 flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors {showNotes
      ? 'border-amber-400 bg-amber-50 text-amber-700 dark:border-amber-600 dark:bg-amber-950/40 dark:text-amber-300'
      : 'border-border bg-surface text-secondary hover:bg-hover hover:text-medium'}"
    onpointerdown={(e) => e.stopPropagation()}
    onclick={() => (showNotes = !showNotes)}
  >
    <svg
      class="h-3.5 w-3.5"
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
    Notes
  </button>
  <div
    class="absolute"
    style="transform: translate({panX}px, {panY}px) scale({zoom}); transform-origin: 0 0;"
  >
    {#each reminders as reminder (reminder.id)}
      <GrimoireReminderToken
        {reminder}
        {zoom}
        onmove={(x, y) => onremindermove?.(reminder.id, x, y)}
      />
    {/each}
    {#each players as player (player.id)}
      <GrimoirePlayerToken
        {player}
        {zoom}
        {roundLabel}
        {showNotes}
        onmove={(x, y) => onplayermove?.(player.id, x, y)}
        onrename={(name) => onplayerrename?.(player.id, name)}
        ontoggledeath={() => onplayertoggledeath?.(player.id)}
        ongamenote={(note) => onplayergamenote?.(player.id, note)}
        onroundnote={(note) => onplayerroundnote?.(player.id, note)}
        onalignment={(alignment) => onplayeralignment?.(player.id, alignment)}
      />
    {/each}
  </div>
</div>
