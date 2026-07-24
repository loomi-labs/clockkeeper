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
    class="ml-1 rounded px-1.5 py-0.5 text-[10px] font-medium bg-purple-100 text-purple-700 dark:bg-purple-500/20 dark:text-purple-300"
    title={sourceTitle}
  >
    Poisoned
  </span>
{/if}
{#if status.drunk}
  <span
    class="ml-1 rounded px-1.5 py-0.5 text-[10px] font-medium bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-300"
    title={sourceTitle}
  >
    Drunk
  </span>
{/if}
