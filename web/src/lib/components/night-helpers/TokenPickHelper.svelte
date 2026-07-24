<script lang="ts">
  import type { NightHelperContext } from "~/lib/night-helpers/registry";
  import PlayerPickerPopover from "./PlayerPickerPopover.svelte";

  let { entryId, ctx }: { entryId: string; ctx: NightHelperContext } = $props();

  // Per-helper config. Each helper attaches a single reminder token to one seat.
  // `excludeSelf` drops the acting character's own seat from the picker (the
  // Butler cannot pick themselves as Master; the Poisoner may poison anyone).
  interface Config {
    characterId: string;
    tokenText: string;
    label: string;
    excludeSelf: boolean;
  }
  const CONFIG: Record<string, Config> = {
    poisoner: {
      characterId: "poisoner",
      tokenText: "Poisoned",
      label: "Poisoned",
      excludeSelf: false,
    },
    butler: {
      characterId: "butler",
      tokenText: "Master",
      label: "Master",
      excludeSelf: true,
    },
    monk: {
      characterId: "monk",
      tokenText: "Safe",
      label: "Safe",
      excludeSelf: true,
    },
  };

  const cfg = $derived(CONFIG[entryId]);
  const playerId = $derived(ctx.playerIdForEntry(entryId));

  // Render nothing unless the seat is in play and the page wired the token
  // callbacks (graceful when unwired).
  const ready = $derived(
    !!cfg && !!playerId && !!ctx.tokenHolder && !!ctx.onattachtoken,
  );

  // Current pick is the grimoire token holder — derived, so it stays in sync
  // with manual token attachment in the grimoire.
  const holder = $derived(
    cfg && ctx.tokenHolder
      ? ctx.tokenHolder(cfg.characterId, cfg.tokenText)
      : undefined,
  );

  const status = $derived(playerId ? ctx.statuses.get(playerId) : undefined);
  const impaired = $derived(!!status && (status.poisoned || status.drunk));

  // All players are available for the pick, minus the acting seat when excluded.
  const pickerPlayers = $derived([...ctx.players.values()]);
  const pickerExclude = $derived(
    cfg?.excludeSelf && playerId ? new Set([playerId]) : new Set<string>(),
  );

  function playerName(id: string | undefined): string | undefined {
    if (!id) return undefined;
    const p = ctx.players.get(id);
    return p ? p.name || p.characterName : undefined;
  }

  let picker = $state<{ anchor: { top: number; left: number } } | null>(null);

  function openPicker(e: MouseEvent) {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    picker = { anchor: { top: rect.bottom + 4, left: rect.left } };
  }

  function pick(id: string) {
    if (cfg) ctx.onattachtoken?.(cfg.characterId, cfg.tokenText, id);
    picker = null;
  }

  function clear() {
    if (cfg) ctx.onattachtoken?.(cfg.characterId, cfg.tokenText, undefined);
  }
</script>

{#if ready}
  <div class="flex items-center gap-1">
    <button
      type="button"
      onclick={openPicker}
      class="rounded border border-border px-2 py-1 text-xs transition-colors hover:bg-hover {holder
        ? 'font-medium text-primary'
        : 'text-muted'}"
    >
      {holder
        ? `${cfg.label}: ${playerName(holder)}`
        : `${cfg.label}: pick player`}
    </button>
    {#if holder}
      <button
        type="button"
        onclick={clear}
        class="flex h-5 w-5 items-center justify-center rounded-full text-muted transition-colors hover:bg-hover hover:text-primary"
        aria-label="Clear {cfg.label}"
        title="Clear {cfg.label}"
      >
        <svg
          class="h-3 w-3"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2.5"
          ><path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M6 18L18 6M6 6l12 12"
          /></svg
        >
      </button>
    {/if}
  </div>

  {#if impaired}
    <div
      class="mt-1 text-[11px] font-medium {status?.poisoned
        ? 'text-purple-600 dark:text-purple-300'
        : 'text-amber-600 dark:text-amber-300'}"
    >
      {status?.poisoned && status?.drunk
        ? "Poisoned/Drunk"
        : status?.poisoned
          ? "Poisoned"
          : "Drunk"} — ability does nothing tonight
    </div>
  {/if}

  {#if picker}
    <PlayerPickerPopover
      title={cfg.label}
      players={pickerPlayers}
      excludeIds={pickerExclude}
      anchor={picker.anchor}
      onpick={pick}
      onclose={() => (picker = null)}
    />
  {/if}
{/if}
