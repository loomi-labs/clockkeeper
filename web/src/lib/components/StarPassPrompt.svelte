<script lang="ts">
  import type { HelperPlayer } from "~/lib/night-helpers/helpers";
  import { iconSuffix } from "~/lib/team-styles";

  let {
    minions,
    onpick,
    onskip,
  }: {
    // Alive, in-play Minions by their REAL role (id = role id).
    minions: HelperPlayer[];
    onpick: (minionRoleId: string) => void;
    onskip: () => void;
  } = $props();

  // The Scarlet Woman is the canonical star-pass recipient — flag it when present
  // and alive so the ST's eye lands on the recommended choice.
  const RECOMMENDED_ID = "scarletwoman";

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") onskip();
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="fixed inset-0 z-50 flex items-center justify-center"
  onkeydown={handleKeydown}
>
  <!-- Backdrop -->
  <button
    type="button"
    tabindex="-1"
    class="absolute inset-0 bg-black/40"
    onclick={onskip}
    aria-label="Close"
  ></button>

  <!-- Dialog -->
  <div
    role="dialog"
    aria-modal="true"
    class="relative z-10 w-full max-w-sm rounded-xl border border-border bg-surface p-6 shadow-xl"
  >
    <h3 class="text-lg font-semibold text-primary">Who becomes the new Imp?</h3>
    <p class="mt-1 text-sm text-secondary">
      The Imp killed itself — a Minion becomes the Imp (Star Pass).
    </p>

    <div class="mt-4 space-y-1 max-h-72 overflow-y-auto">
      {#if minions.length === 0}
        <p class="py-4 text-center text-sm text-muted">
          No living Minions to promote.
        </p>
      {:else}
        {#each minions as m (m.id)}
          {@const recommended = m.id === RECOMMENDED_ID}
          <button
            type="button"
            onclick={() => onpick(m.id)}
            class="flex w-full items-center gap-3 rounded-lg border px-3 py-2 text-left transition-colors hover:bg-hover {recommended
              ? 'border-indigo-400 bg-indigo-50/50 dark:border-indigo-600 dark:bg-indigo-950/30'
              : 'border-border'}"
          >
            <img
              src="/characters/{m.edition}/{m.characterId}{iconSuffix(
                m.team,
              )}.webp"
              alt=""
              draggable="false"
              class="h-9 w-9 shrink-0 rounded-full"
              onerror={(e: Event) =>
                ((e.target as HTMLImageElement).style.display = "none")}
            />
            <span class="min-w-0 flex-1">
              <span class="block truncate text-sm font-medium text-primary">
                {m.name || m.characterName}
              </span>
              <span class="block truncate text-xs text-muted">
                {m.characterName}
              </span>
            </span>
            {#if recommended}
              <span
                class="shrink-0 rounded-full bg-indigo-100 px-2 py-0.5 text-[10px] font-medium text-indigo-700 dark:bg-indigo-500/20 dark:text-indigo-300"
              >
                Recommended
              </span>
            {/if}
          </button>
        {/each}
      {/if}
    </div>

    <div class="mt-5 flex justify-end">
      <button
        type="button"
        onclick={onskip}
        class="rounded-lg border border-border px-4 py-2 text-sm font-medium text-secondary transition-colors hover:bg-hover hover:text-medium"
      >
        Skip
      </button>
    </div>
  </div>
</div>
