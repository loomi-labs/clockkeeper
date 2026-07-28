<script lang="ts">
  import type { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { iconSuffix } from "~/lib/team-styles";
  import type { SeatRegistration } from "~/lib/tokenbag-names";
  import SourceIcon from "./tokenbag/SourceIcon.svelte";

  // The seat list of the setup-tab Players panel: one row per role in play, each
  // with the name assigned to it. Names come from the preset list (or the Token
  // Bag registrants, whichever owns them) plus free-text entry.
  //
  // Bare on purpose — the card shell, the header and the action bar belong to
  // `tokenbag/PlayersPanel.svelte`, which mounts this. Pure name-map logic lives
  // in `~/lib/player-names.ts`; this component only renders and emits intent
  // (onassign / onunassign / ontogglelock).
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
    lockedIds,
    onassign,
    onunassign,
    ontogglelock,
    renameLockedIds,
    renameLockedHint = "This name cannot be edited here",
    seatMeta,
    showNeighborCaptions = false,
  }: {
    players: PanelPlayer[];
    presetNames: string[];
    // Seats whose name survives "Assign in order" / "Randomize".
    lockedIds: ReadonlySet<string>;
    onassign: (playerId: string, name: string) => void;
    onunassign: (playerId: string) => void;
    ontogglelock: (playerId: string) => void;
    // Seats whose name is owned elsewhere: the rename pencil is shown disabled
    // with `renameLockedHint` as its tooltip.
    renameLockedIds?: ReadonlySet<string>;
    renameLockedHint?: string;
    // What the Token Bag knows about the registrant sitting in each seat: where
    // they joined from, and who they picked as neighbors.
    seatMeta?: ReadonlyMap<string, SeatRegistration>;
    // Neighbor picks are only meaningful once registration is closed, so the
    // caption line is drawn on request rather than whenever picks exist.
    showNeighborCaptions?: boolean;
  } = $props();

  let openPlayerId = $state<string | null>(null);
  let freeText = $state("");

  // Inline rename of an already-assigned seat (pencil -> text input).
  let editingPlayerId = $state<string | null>(null);
  let editText = $state("");

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

  function isRenameLocked(id: string): boolean {
    return renameLockedIds?.has(id) === true;
  }

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

  function startEdit(id: string, name: string) {
    editingPlayerId = id;
    editText = name;
  }
  function cancelEdit() {
    editingPlayerId = null;
    editText = "";
  }
  // Commit an inline rename: trimmed text re-assigns, empty text unassigns.
  // Guarded on id so the input's blur handler is a no-op after Escape cancels.
  function commitEdit(id: string) {
    if (editingPlayerId !== id) return;
    const trimmed = editText.trim();
    if (trimmed) onassign(id, trimmed);
    else onunassign(id);
    editingPlayerId = null;
    editText = "";
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

{#if players.length === 0}
  <p class="text-sm text-muted">Select or randomize roles first.</p>
{:else}
  <div class="divide-y divide-border">
    {#each players as p, i (p.id)}
      {@const meta = seatMeta?.get(p.id)}
      <div class="py-1.5">
        <div class="flex items-center gap-3">
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
              {#if editingPlayerId === p.id}
                <!-- svelte-ignore a11y_autofocus -->
                <input
                  type="text"
                  bind:value={editText}
                  autofocus
                  onblur={() => commitEdit(p.id)}
                  onkeydown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      commitEdit(p.id);
                    } else if (e.key === "Escape") {
                      e.preventDefault();
                      cancelEdit();
                    }
                  }}
                  aria-label="Rename {p.name}"
                  class="w-32 rounded-full border border-border bg-transparent px-2.5 py-0.5 text-xs text-primary outline-none focus:border-indigo-500"
                />
              {:else}
                <span
                  class="inline-flex items-center gap-1 rounded-full border border-border bg-element pl-2.5 pr-1 py-0.5 text-xs font-medium text-primary"
                >
                  {p.name}
                  {#if meta}
                    <SourceIcon viaShared={meta.viaShared} />
                  {/if}
                  {#if lockedIds.has(p.id)}
                    <button
                      onclick={() => ontogglelock(p.id)}
                      aria-pressed="true"
                      class="rounded-full p-0.5 text-amber-500 transition-colors hover:bg-hover"
                      aria-label="Unlock name (randomize may replace it)"
                      title="Unlock name (randomize may replace it)"
                    >
                      <svg
                        class="h-3 w-3"
                        viewBox="0 0 24 24"
                        fill="currentColor"
                        ><path
                          fill-rule="evenodd"
                          clip-rule="evenodd"
                          d="M12 1.5a5.25 5.25 0 00-5.25 5.25v3a3 3 0 00-3 3v6.75a3 3 0 003 3h10.5a3 3 0 003-3v-6.75a3 3 0 00-3-3v-3c0-2.9-2.35-5.25-5.25-5.25zm3.75 8.25v-3a3.75 3.75 0 10-7.5 0v3h7.5z"
                        /></svg
                      >
                    </button>
                  {:else}
                    <button
                      onclick={() => ontogglelock(p.id)}
                      aria-pressed="false"
                      class="rounded-full p-0.5 text-muted transition-colors hover:bg-hover hover:text-amber-500"
                      aria-label="Lock name (randomize keeps it)"
                      title="Lock name (randomize keeps it)"
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
                          d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
                        /></svg
                      >
                    </button>
                  {/if}
                  <button
                    onclick={() => startEdit(p.id, p.name ?? "")}
                    disabled={isRenameLocked(p.id)}
                    class="rounded-full p-0.5 text-muted transition-colors hover:bg-hover hover:text-indigo-500 disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-muted"
                    aria-label={isRenameLocked(p.id)
                      ? renameLockedHint
                      : `Rename ${p.name}`}
                    title={isRenameLocked(p.id)
                      ? renameLockedHint
                      : `Rename ${p.name}`}
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
                        d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                      /></svg
                    >
                  </button>
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
              {/if}
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

        {#if showNeighborCaptions && meta && (meta.leftName || meta.rightName)}
          <!-- Who this player says they are sitting between. -->
          <p class="pl-20 text-[11px] text-secondary">
            ⇐ {meta.leftName ?? "—"} · {meta.rightName ?? "—"} ⇒
          </p>
        {/if}
      </div>
    {/each}
  </div>
{/if}
