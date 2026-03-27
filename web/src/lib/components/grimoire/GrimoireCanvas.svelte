<script lang="ts">
  import { onMount } from "svelte";
  import type { GrimoirePlayer, GrimoireReminder } from "./types";
  import GrimoirePlayerToken from "./GrimoirePlayerToken.svelte";
  import GrimoireReminderToken from "./GrimoireReminderToken.svelte";
  import {
    ATTACH_THRESHOLD,
    DETACH_THRESHOLD,
    angleFromPosition,
    distanceBetween,
  } from "./layout";
  import {
    useComposedGesture,
    pinchComposition,
    type GestureCustomEvent,
  } from "svelte-gestures";

  let {
    players,
    reminders,
    roundLabel = "Round",
    onplayermove,
    onremindermove,
    onreminderattach,
    onreminderdetach,
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
    onreminderattach?: (
      reminderId: string,
      playerId: string,
      angle: number,
    ) => void;
    onreminderdetach?: (reminderId: string) => void;
    onplayerrename?: (id: string, name: string) => void;
    onplayertoggledeath?: (id: string) => void;
    onplayergamenote?: (id: string, note: string) => void;
    onplayerroundnote?: (id: string, note: string) => void;
    onplayeralignment?: (id: string, alignment: string) => void;
  } = $props();

  let panX = $state(0);
  let panY = $state(0);
  let zoom = $state(1);
  let showNotes = $state(true);

  let canvasEl: HTMLDivElement;

  // Track which player is being highlighted as an attach target during reminder drag
  let attachPreviewPlayerId = $state<string | null>(null);

  // Pan tracking
  let isPanning = false;
  let panStartClientX = 0;
  let panStartClientY = 0;
  let panOriginX = 0;
  let panOriginY = 0;

  // Pinch tracking — zoomAtPinchStart converts cumulative scale to absolute zoom
  let zoomAtPinchStart = 1;

  interface PinchDetail {
    scale: number;
    center: { x: number; y: number };
    pointerType: string;
  }

  const canvasGesture = useComposedGesture(
    (register) => {
      const pinchFns = register(pinchComposition, { touchAction: "none" });
      return (activeEvents: PointerEvent[], event: PointerEvent) => {
        // Forward to pinch detection (only dispatches when 2 pointers)
        pinchFns.onMove?.(activeEvents, event);
      };
    },
    {
      oncomposedGesturedown: (e: GestureCustomEvent) => {
        if (e.detail.pointersCount === 1) {
          isPanning = true;
          panStartClientX = e.detail.event.clientX;
          panStartClientY = e.detail.event.clientY;
          panOriginX = panX;
          panOriginY = panY;
        } else if (e.detail.pointersCount === 2) {
          isPanning = false;
          zoomAtPinchStart = zoom;
        }
      },
      oncomposedGesturemove: (e: GestureCustomEvent) => {
        if (e.detail.pointersCount !== 1) return;
        if (!isPanning) {
          isPanning = true;
          panStartClientX = e.detail.event.clientX;
          panStartClientY = e.detail.event.clientY;
          panOriginX = panX;
          panOriginY = panY;
          return;
        }
        panX = panOriginX + (e.detail.event.clientX - panStartClientX);
        panY = panOriginY + (e.detail.event.clientY - panStartClientY);
      },
      oncomposedGestureup: (e: GestureCustomEvent) => {
        if (e.detail.pointersCount === 0) {
          isPanning = false;
        }
      },
    } as Record<string, (e: GestureCustomEvent) => void>,
  );

  onMount(() => {
    if (!canvasEl) return;
    const rect = canvasEl.getBoundingClientRect();
    panX = rect.width / 2;
    panY = rect.height / 2;

    // Listen for pinch custom events dispatched by svelte-gestures
    const handlePinch = ((e: CustomEvent<PinchDetail>) => {
      const { scale, center } = e.detail;
      const newZoom = Math.max(0.3, Math.min(3.0, zoomAtPinchStart * scale));

      panX = center.x - (center.x - panX) * (newZoom / zoom);
      panY = center.y - (center.y - panY) * (newZoom / zoom);
      zoom = newZoom;
    }) as EventListener;
    canvasEl.addEventListener("pinch", handlePinch);

    return () => {
      canvasEl.removeEventListener("pinch", handlePinch);
    };
  });

  function findNearestPlayer(
    x: number,
    y: number,
    threshold: number,
  ): GrimoirePlayer | null {
    let nearest: GrimoirePlayer | null = null;
    let nearestDist = threshold;
    for (const p of players) {
      const dist = distanceBetween(x, y, p.x, p.y);
      if (dist < nearestDist) {
        nearest = p;
        nearestDist = dist;
      }
    }
    return nearest;
  }

  function handleReminderMoveEnd(
    reminderId: string,
    x: number,
    y: number,
  ) {
    const reminder = reminders.find((r) => r.id === reminderId);
    const wasAttached = reminder?.attachedTo;

    if (wasAttached) {
      const player = players.find((p) => p.id === wasAttached);
      const nearPlayer = findNearestPlayer(x, y, ATTACH_THRESHOLD);

      if (nearPlayer && nearPlayer.id !== wasAttached) {
        // Dropped onto a different player — reattach there
        const angle = angleFromPosition(x, y, nearPlayer.x, nearPlayer.y);
        onreminderattach?.(reminderId, nearPlayer.id, angle);
      } else if (player) {
        const dist = distanceBetween(x, y, player.x, player.y);
        if (dist > DETACH_THRESHOLD) {
          onreminderdetach?.(reminderId);
          onremindermove?.(reminderId, x, y);
        } else {
          // Reposition along orbit — compute new angle
          const angle = angleFromPosition(x, y, player.x, player.y);
          onreminderattach?.(reminderId, wasAttached, angle);
        }
      } else {
        onreminderdetach?.(reminderId);
        onremindermove?.(reminderId, x, y);
      }
    } else {
      // Check if dropped near a player to attach
      const nearPlayer = findNearestPlayer(x, y, ATTACH_THRESHOLD);
      if (nearPlayer) {
        const angle = angleFromPosition(x, y, nearPlayer.x, nearPlayer.y);
        onreminderattach?.(reminderId, nearPlayer.id, angle);
      } else {
        onremindermove?.(reminderId, x, y);
      }
    }
    attachPreviewPlayerId = null;
  }

  function handleReminderDragMove(x: number, y: number) {
    const nearPlayer = findNearestPlayer(x, y, ATTACH_THRESHOLD);
    attachPreviewPlayerId = nearPlayer?.id ?? null;
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
  class="relative h-full w-full overflow-hidden bg-page"
  bind:this={canvasEl}
  {...canvasGesture}
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
        onmove={(x: number, y: number) => handleReminderMoveEnd(reminder.id, x, y)}
        ondragmove={(x: number, y: number) => handleReminderDragMove(x, y)}
      />
    {/each}
    {#each players as player (player.id)}
      <GrimoirePlayerToken
        {player}
        {zoom}
        {roundLabel}
        {showNotes}
        highlightAttach={attachPreviewPlayerId === player.id}
        onmove={(x: number, y: number) => onplayermove?.(player.id, x, y)}
        onrename={(name: string) => onplayerrename?.(player.id, name)}
        ontoggledeath={() => onplayertoggledeath?.(player.id)}
        ongamenote={(note: string) => onplayergamenote?.(player.id, note)}
        onroundnote={(note: string) => onplayerroundnote?.(player.id, note)}
        onalignment={(alignment: string) =>
          onplayeralignment?.(player.id, alignment)}
      />
    {/each}
  </div>
</div>
