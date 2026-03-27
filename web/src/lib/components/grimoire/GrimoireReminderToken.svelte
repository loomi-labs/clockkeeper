<script lang="ts">
  import { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import {
    teamCardColors,
    goodColors,
    evilColors,
    teamDataAttr,
  } from "~/lib/team-styles";
  import { usePan, type GestureCustomEvent } from "svelte-gestures";
  import type { GrimoireReminder } from "./types";

  let {
    reminder,
    zoom,
    onmove,
    ondragmove,
  }: {
    reminder: GrimoireReminder;
    zoom: number;
    onmove?: (x: number, y: number) => void;
    ondragmove?: (x: number, y: number) => void;
  } = $props();

  // Effective alignment for icon/color (alignment override takes priority)
  const effectiveAlignment = $derived<"good" | "evil" | undefined>(
    reminder.alignment ??
      (reminder.team === Team.TOWNSFOLK || reminder.team === Team.OUTSIDER
        ? "good"
        : reminder.team === Team.MINION || reminder.team === Team.DEMON
          ? "evil"
          : undefined),
  );

  const iconSuffix = $derived(
    effectiveAlignment === "good"
      ? "_g"
      : effectiveAlignment === "evil"
        ? "_e"
        : "",
  );
  const iconUrl = $derived(
    `/characters/${reminder.edition}/${reminder.characterId}${iconSuffix}.webp`,
  );

  const colorClass = $derived(
    reminder.alignment
      ? reminder.alignment === "good"
        ? goodColors
        : evilColors
      : (teamCardColors[reminder.team] ?? "border-border bg-surface-alt"),
  );

  const isAttached = $derived(!!reminder.attachedTo);

  let dragging = $state(false);
  let offsetX = $state(0);
  let offsetY = $state(0);
  let imgError = $state(false);
  let dragStartClientX = 0;
  let dragStartClientY = 0;

  const gesture = usePan(
    () => {},
    () => ({ delay: 0, touchAction: "none" }),
    {
      onpandown: (e: GestureCustomEvent) => {
        e.detail.event.stopPropagation();
        dragging = true;
        dragStartClientX = e.detail.event.clientX;
        dragStartClientY = e.detail.event.clientY;
        offsetX = 0;
        offsetY = 0;
        e.detail.attachmentNode.setPointerCapture(e.detail.event.pointerId);
      },
      onpanmove: (e: GestureCustomEvent) => {
        if (!dragging) return;
        offsetX = (e.detail.event.clientX - dragStartClientX) / zoom;
        offsetY = (e.detail.event.clientY - dragStartClientY) / zoom;
        ondragmove?.(reminder.x + offsetX, reminder.y + offsetY);
      },
      onpanup: (e: GestureCustomEvent) => {
        if (!dragging) return;
        dragging = false;
        if (offsetX !== 0 || offsetY !== 0) {
          onmove?.(reminder.x + offsetX, reminder.y + offsetY);
        }
        offsetX = 0;
        offsetY = 0;
      },
    },
  );
</script>

<div
  class="absolute select-none"
  style="left: {reminder.x + offsetX}px; top: {reminder.y +
    offsetY}px; transform: translate(-50%, -50%); z-index: {dragging ? 50 : 0};"
  {...gesture}
  role="button"
  tabindex="0"
>
  <div
    class="card-slate token-bezel-sm flex h-16 w-16 flex-col items-center justify-center rounded-full p-0.5 {colorClass} {isAttached && !dragging ? 'ring-2 ring-primary/20' : ''}"
    data-team={teamDataAttr[reminder.team] ?? ""}
  >
    {#if !imgError && reminder.edition}
      <img
        src={iconUrl}
        alt={reminder.characterName}
        class="h-10 w-10 shrink-0 drop-shadow-sm"
        onerror={() => (imgError = true)}
        draggable="false"
      />
    {:else}
      <div
        class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-element text-sm text-secondary"
      >
        {reminder.characterName.charAt(0)}
      </div>
    {/if}
  </div>
  <div
    class="mt-0.5 max-w-16 text-center text-[10px] leading-tight text-secondary"
  >
    {reminder.text}
  </div>
</div>
