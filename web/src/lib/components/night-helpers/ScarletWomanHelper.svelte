<script lang="ts">
  import { scarletWomanPromotionAlert } from "~/lib/night-helpers/helpers";
  import type { NightHelperContext } from "~/lib/night-helpers/registry";

  let { entryId, ctx }: { entryId: string; ctx: NightHelperContext } = $props();
  // `entryId` is unused: the alert is a whole-board condition, not a seat pick.
  void entryId;

  // Show only when the Demon has died with 5+ players alive AND the page wired
  // the promotion prompt (graceful when unwired).
  const alert = $derived(
    scarletWomanPromotionAlert(ctx.players) && !!ctx.onstarpass,
  );
</script>

{#if alert}
  <div
    class="flex flex-wrap items-center gap-2 rounded border border-amber-400 bg-amber-50 px-2 py-1.5 text-xs text-amber-700 dark:bg-amber-950/40 dark:text-amber-300"
  >
    <span class="font-medium">
      The Demon is dead with 5+ players alive — the Scarlet Woman becomes the
      Demon.
    </span>
    <button
      type="button"
      onclick={() => ctx.onstarpass?.()}
      class="rounded border border-amber-500 px-2 py-1 font-medium text-amber-700 transition-colors hover:bg-amber-100 dark:text-amber-200 dark:hover:bg-amber-900/40"
    >
      Promote…
    </button>
  </div>
{/if}
