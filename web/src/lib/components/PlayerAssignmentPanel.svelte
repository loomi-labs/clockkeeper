<script lang="ts">
  import type { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { iconSuffix } from "~/lib/team-styles";

  // Setup-tab panel for assigning player names to seats. Seats are the roles
  // in play; names come from the global preset list plus free-text entry.
  // Pure name-map logic lives in `~/lib/player-names.ts`; this component only
  // renders and emits intent (onassign / onunassign / ...).
  interface PanelPlayer {
    id: string;
    characterName: string;
    edition: string;
    team: Team;
    name: string | undefined;
  }

  let {
    players,
    presetNames,
    onassign,
    onunassign,
    onassigninorder,
    onrandomize,
    onclearall,
    onmanagepresets,
  }: {
    players: PanelPlayer[];
    presetNames: string[];
    onassign: (playerId: string, name: string) => void;
    onunassign: (playerId: string) => void;
    onassigninorder: () => void;
    onrandomize?: () => void;
    onclearall: () => void;
    onmanagepresets: () => void;
  } = $props();

  let openPlayerId = $state<string | null>(null);
  let freeText = $state("");

  // Preset names already assigned to some seat (so the dropdown only offers
  // the ones still free).
  const assignedNames = $derived(
    new Set(
      players
        .map((p) => p.name)
        .filter((n): n is string => n !== undefined && n !== ""),
    ),
  );
  const unusedPresets = $derived(
    presetNames.filter((n) => !assignedNames.has(n)),
  );
  const anyAssigned = $derived(players.some((p) => p.name));

  function toggleDropdown(id: string) {
    openPlayerId = openPlayerId === id ? null : id;
    freeText = "";
  }
  function closeDropdown() {
    openPlayerId = null;
    freeText = "";
  }
  function pick(id: string, name: string) {
    onassign(id, name);
    closeDropdown();
  }
  function confirmFreeText(id: string) {
    const trimmed = freeText.trim();
    if (!trimmed) return;
    onassign(id, trimmed);
    closeDropdown();
  }

  // Close the open dropdown on outside click / Escape.
  $effect(() => {
    if (openPlayerId === null) return;
    const onDocClick = (e: MouseEvent) => {
      const target = e.target as HTMLElement | null;
      if (target && target.closest("[data-assign-menu]")) return;
      closeDropdown();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") closeDropdown();
    };
    document.addEventListener("click", onDocClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("click", onDocClick);
      document.removeEventListener("keydown", onKey);
    };
  });
</script>

<div class="rounded-lg border border-border bg-surface p-4">
  <div class="mb-3 flex items-center justify-between">
    <h3 class="text-sm font-semibold text-secondary">Players</h3>
  </div>

  {#if players.length === 0}
    <p class="text-sm text-muted">Select or randomize roles first.</p>
  {:else}
    <div class="divide-y divide-border">
      {#each players as p, i (p.id)}
        <div class="flex items-center gap-3 py-1.5">
          <span class="w-6 shrink-0 text-center text-xs text-muted"
            >{i + 1}</span
          >
          <img
            src="/characters/{p.edition}/{p.id}{iconSuffix(p.team)}.webp"
            alt=""
            class="h-8 w-8 shrink-0 rounded-full"
            onerror={(e: Event) =>
              ((e.target as HTMLImageElement).style.display = "none")}
          />
          <span class="min-w-0 flex-1 truncate text-sm text-primary"
            >{p.characterName}</span
          >

          <div class="relative shrink-0" data-assign-menu>
            {#if p.name}
              <span
                class="inline-flex items-center gap-1 rounded-full border border-border bg-element pl-2.5 pr-1 py-0.5 text-xs font-medium text-primary"
              >
                {p.name}
                <button
                  onclick={() => onunassign(p.id)}
                  class="rounded-full p-0.5 text-muted transition-colors hover:bg-hover hover:text-red-500"
                  aria-label="Unassign {p.name}"
                  title="Unassign {p.name}"
                >
                  <svg
                    class="h-3 w-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                    ><path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M6 18L18 6M6 6l12 12"
                    /></svg
                  >
                </button>
              </span>
            {:else}
              <button
                onclick={() => toggleDropdown(p.id)}
                aria-haspopup="menu"
                aria-expanded={openPlayerId === p.id}
                class="flex items-center gap-1 rounded-full border border-dashed border-border px-2.5 py-0.5 text-xs text-secondary transition-colors hover:border-indigo-400 hover:text-indigo-500"
              >
                <svg
                  class="h-3 w-3"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="2"
                  ><path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M12 4v16m8-8H4"
                  /></svg
                >
                name
              </button>
            {/if}

            {#if openPlayerId === p.id}
              <div
                class="absolute right-0 top-full z-20 mt-1 w-56 rounded-lg border border-border bg-surface p-2 shadow-lg"
                role="menu"
              >
                {#if unusedPresets.length > 0}
                  <div class="max-h-48 space-y-0.5 overflow-y-auto">
                    {#each unusedPresets as name (name)}
                      <button
                        role="menuitem"
                        onclick={() => pick(p.id, name)}
                        class="block w-full rounded-md px-2 py-1.5 text-left text-sm text-primary transition-colors hover:bg-hover"
                      >
                        {name}
                      </button>
                    {/each}
                  </div>
                {:else}
                  <p class="px-2 py-1 text-xs text-muted">
                    No unused preset names.
                  </p>
                {/if}
                <form
                  class="mt-2 flex gap-1 border-t border-border pt-2"
                  onsubmit={(e) => {
                    e.preventDefault();
                    confirmFreeText(p.id);
                  }}
                >
                  <!-- svelte-ignore a11y_autofocus -->
                  <input
                    type="text"
                    bind:value={freeText}
                    placeholder="Type a name..."
                    autofocus
                    class="min-w-0 flex-1 rounded-md border border-border bg-transparent px-2 py-1 text-sm text-primary outline-none focus:border-indigo-500"
                  />
                  <button
                    type="submit"
                    disabled={!freeText.trim()}
                    class="rounded-md bg-indigo-500 px-2 py-1 text-xs font-medium text-white transition-colors hover:bg-indigo-400 disabled:opacity-50"
                    aria-label="Confirm name"
                  >
                    <svg
                      class="h-3.5 w-3.5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      stroke-width="2"
                      ><path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M5 13l4 4L19 7"
                      /></svg
                    >
                  </button>
                </form>
              </div>
            {/if}
          </div>
        </div>
      {/each}
    </div>

    <div
      class="mt-3 flex flex-wrap items-center gap-2 border-t border-border pt-3"
    >
      <button
        onclick={onassigninorder}
        disabled={presetNames.length === 0}
        class="rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-secondary transition-colors hover:border-indigo-400 hover:text-indigo-500 disabled:opacity-40 disabled:hover:border-border disabled:hover:text-secondary"
      >
        Assign in order
      </button>
      {#if onrandomize}
        <button
          onclick={onrandomize}
          disabled={presetNames.length === 0}
          class="rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-secondary transition-colors hover:border-indigo-400 hover:text-indigo-500 disabled:opacity-40 disabled:hover:border-border disabled:hover:text-secondary"
        >
          Randomize
        </button>
      {/if}
      {#if anyAssigned}
        <button
          onclick={onclearall}
          class="rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-muted transition-colors hover:border-red-300 hover:text-red-500"
        >
          Clear all
        </button>
      {/if}
      <button
        onclick={onmanagepresets}
        class="ml-auto rounded-lg border border-dashed border-border px-3 py-1.5 text-xs text-muted transition-colors hover:border-indigo-400 hover:text-indigo-500"
      >
        Manage names
      </button>
    </div>
  {/if}
</div>
