<script lang="ts">
  import type { HelperPlayer } from "~/lib/night-helpers/helpers";
  import { iconSuffix } from "~/lib/team-styles";

  let {
    title,
    players,
    excludeIds,
    anchor,
    onpick,
    onclose,
    annotate,
  }: {
    title: string;
    players: HelperPlayer[];
    excludeIds?: ReadonlySet<string>;
    anchor: { top: number; left?: number; right?: number };
    onpick: (id: string) => void;
    onclose: () => void;
    /**
     * Optional per-row badge. Returning `undefined` renders no badge for that
     * row. `tone: "ok"` reads as a match (green), `"muted"` as a non-match
     * (gray). Purely advisory — every row stays pickable (a misled info
     * character can be shown anything).
     */
    annotate?: (
      p: HelperPlayer,
    ) => { label: string; tone: "ok" | "muted" } | undefined;
  } = $props();

  const VIEWPORT_MARGIN = 8;

  let menuEl = $state<HTMLDivElement | null>(null);
  // Resolved top edge, clamped/flipped once the popover has a measured height.
  // Null until the effect runs; the style falls back to the raw anchor top.
  let resolvedTop = $state<number | null>(null);

  const visiblePlayers = $derived(
    players.filter((p) => !excludeIds?.has(p.id)),
  );

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
      if (!target.closest("[data-overflow-menu]")) onclose();
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

  // Portal the popover to <body>. Rendered inline it would resolve its
  // `position: fixed` against a transformed ancestor (e.g. a swiped night row
  // whose pan element keeps a translate3d transform) and get clipped by the
  // row's `overflow-hidden` wrapper. Reparenting to <body> makes it immune to
  // its render location. Svelte 5 attaches delegated event listeners to both
  // the mount container and `document`, so onclick/onpointerdown handlers keep
  // working after the node is moved outside the app root.
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
  data-overflow-menu
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
  {#if visiblePlayers.length === 0}
    <div class="px-3 py-2 text-sm text-muted">No players available</div>
  {:else}
    {#each visiblePlayers as p (p.id)}
      <button
        type="button"
        role="menuitem"
        onclick={() => onpick(p.id)}
        class="flex w-full items-center gap-2 px-3 py-1.5 text-left transition-colors hover:bg-hover {p.isDead
          ? 'opacity-60'
          : ''}"
      >
        <img
          src="/characters/{p.edition}/{p.characterId}{iconSuffix(p.team)}.webp"
          alt=""
          draggable="false"
          class="h-8 w-8 shrink-0 rounded-full {p.isDead ? 'grayscale' : ''}"
          onerror={(e: Event) =>
            ((e.target as HTMLImageElement).style.display = "none")}
        />
        <span class="min-w-0 flex-1">
          <span
            class="block truncate text-sm font-medium text-primary {p.isDead
              ? 'line-through'
              : ''}"
          >
            {p.name || p.characterName}
          </span>
          <span class="block truncate text-xs text-muted">
            {p.characterName}
          </span>
        </span>
        {#if annotate}
          {@const badge = annotate(p)}
          {#if badge}
            <span
              class="shrink-0 rounded-full px-1.5 py-0.5 text-[10px] font-medium {badge.tone ===
              'ok'
                ? 'bg-green-100 text-green-700 dark:bg-green-950/50 dark:text-green-300'
                : 'bg-hover text-muted'}"
            >
              {badge.label}
            </span>
          {/if}
        {/if}
        {#if p.isDead}
          <span class="shrink-0 text-xs text-red-500 dark:text-red-400"
            >&#128128;</span
          >
        {/if}
      </button>
    {/each}
  {/if}
</div>
