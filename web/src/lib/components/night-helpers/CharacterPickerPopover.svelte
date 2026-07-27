<script lang="ts">
  import type { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { iconSuffix } from "~/lib/team-styles";

  /** A character reduced to what the picker needs (matches `scriptCharacters`). */
  interface PickableCharacter {
    id: string;
    name: string;
    team: Team;
    edition: string;
  }

  let {
    title,
    characters,
    anchor,
    onpick,
    onclose,
  }: {
    title: string;
    characters: ReadonlyArray<PickableCharacter>;
    anchor: { top: number; left?: number; right?: number };
    onpick: (c: PickableCharacter) => void;
    onclose: () => void;
  } = $props();

  const VIEWPORT_MARGIN = 8;

  let menuEl = $state<HTMLDivElement | null>(null);
  // Resolved top edge, clamped/flipped once the popover has a measured height.
  // Null until the effect runs; the style falls back to the raw anchor top.
  let resolvedTop = $state<number | null>(null);

  // Clamp to the viewport: if opening downward would overflow the bottom edge,
  // flip upward so the popover grows above the anchor point instead.
  $effect(() => {
    if (!menuEl) return;
    const height = menuEl.offsetHeight;
    const vh = window.innerHeight;
    if (anchor.top + height > vh - VIEWPORT_MARGIN) {
      resolvedTop = Math.max(VIEWPORT_MARGIN, anchor.top - height);
    } else {
      resolvedTop = anchor.top;
    }
  });

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      e.stopPropagation();
      onclose();
    }
  }

  // Outside pointerdown closes. Clicks inside the popover stop propagation
  // before reaching the window (see the container handler below), so this only
  // fires for genuine outside interactions.
  $effect(() => {
    function onWindowPointerDown(e: PointerEvent) {
      const target = e.target as HTMLElement;
      if (!target.closest("[data-character-picker]")) onclose();
    }
    window.addEventListener("pointerdown", onWindowPointerDown);
    return () => window.removeEventListener("pointerdown", onWindowPointerDown);
  });

  const positionStyle = $derived(
    [
      `top: ${resolvedTop ?? anchor.top}px`,
      anchor.left != null ? `left: ${anchor.left}px` : "",
      anchor.right != null ? `right: ${anchor.right}px` : "",
    ]
      .filter(Boolean)
      .join("; "),
  );

  // Portal the popover to <body> — same rationale as `PlayerPickerPopover`:
  // rendered inline its `position: fixed` would resolve against a transformed
  // ancestor (a swiped night row keeps a translate3d transform) and get clipped
  // by the row's `overflow-hidden` wrapper.
  function portalToBody(node: HTMLElement) {
    document.body.appendChild(node);
    return () => node.remove();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  bind:this={menuEl}
  {@attach portalToBody}
  class="fixed z-50 max-h-[min(60vh,20rem)] w-56 overflow-y-auto rounded-lg border border-border bg-surface py-1 shadow-lg"
  style={positionStyle}
  data-character-picker
  onpointerdown={(e: PointerEvent) => e.stopPropagation()}
  role="menu"
  tabindex="-1"
  aria-label={title}
>
  <div
    class="px-3 py-1 text-[10px] font-semibold uppercase tracking-wide text-muted"
  >
    {title}
  </div>
  {#if characters.length === 0}
    <div class="px-3 py-2 text-sm text-muted">No characters available</div>
  {:else}
    {#each characters as c (c.id)}
      <button
        type="button"
        role="menuitem"
        onclick={() => onpick(c)}
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
        <span
          class="block min-w-0 flex-1 truncate text-sm font-medium text-primary"
        >
          {c.name}
        </span>
      </button>
    {/each}
  {/if}
</div>
