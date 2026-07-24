<script lang="ts">
  import { NIGHT_HELPERS } from "~/lib/night-helpers/registry";
  import type { NightHelperContext } from "~/lib/night-helpers/registry";

  let { entryId, ctx }: { entryId: string; ctx: NightHelperContext } = $props();

  const def = $derived(NIGHT_HELPERS[entryId]);
  // Gate: only render when a helper exists AND it applies to the current night.
  const active = $derived(!!def && def.nights.includes(ctx.night));
</script>

{#if active && def}
  {@const Helper = def.component}
  <div
    class="mt-1.5 rounded-md border border-border/60 bg-black/[0.03] px-2.5 py-1.5 text-xs dark:bg-white/[0.04]"
  >
    <Helper {entryId} {ctx} />
  </div>
{/if}
