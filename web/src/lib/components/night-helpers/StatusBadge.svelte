<script lang="ts">
  import type { PlayerStatus } from "~/lib/night-helpers/status";

  let { status }: { status: PlayerStatus } = $props();

  // Human-readable source list for the tooltip, e.g. "via Poisoner, Widow".
  const sourceTitle = $derived(
    status.sources.length ? `via ${status.sources.join(", ")}` : undefined,
  );
</script>

{#if status.poisoned}
  <span
    class="ml-1 inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] font-semibold bg-purple-200 text-purple-800 ring-1 ring-purple-400/60 dark:bg-purple-500/30 dark:text-purple-200 dark:ring-purple-400/40"
    title={sourceTitle}
  >
    <span
      class="h-1.5 w-1.5 shrink-0 rounded-full bg-purple-600 dark:bg-purple-300"
    ></span>
    Poisoned
  </span>
{/if}
{#if status.drunk}
  <span
    class="ml-1 inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] font-semibold bg-amber-200 text-amber-800 ring-1 ring-amber-400/60 dark:bg-amber-500/30 dark:text-amber-200 dark:ring-amber-400/40"
    title={sourceTitle}
  >
    <span
      class="h-1.5 w-1.5 shrink-0 rounded-full bg-amber-600 dark:bg-amber-300"
    ></span>
    Drunk
  </span>
{/if}
