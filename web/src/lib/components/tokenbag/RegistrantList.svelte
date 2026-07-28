<script lang="ts">
  // The Storyteller's view of who has joined the token bag.
  //
  // Presentational: it renders the live registrant list and emits a remove
  // intent. Which phases allow removal is a backend rule, mirrored here only to
  // decide whether to draw the button (see `removable` below).
  import ConfirmDialog from "../ConfirmDialog.svelte";
  import { TokenBagPhase } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { NO_ID, normalizeName, type BagPlayer } from "~/lib/tokenbag";

  let {
    players,
    phase,
    assignedNames,
    onremove,
  }: {
    players: readonly BagPlayer[];
    phase: TokenBagPhase;
    /**
     * Grimoire seat names. When given, each row says whether this registrant
     * has a role yet — the precondition for revealing.
     */
    assignedNames?: ReadonlySet<string>;
    onremove?: (id: string) => void;
  } = $props();

  /**
   * Removal is only possible while registration is open or closed: after the
   * reveal the server refuses it (a player already saw their character, so the
   * only way back is a full reset).
   */
  const removable = $derived(
    onremove !== undefined &&
      (phase === TokenBagPhase.OPEN || phase === TokenBagPhase.CLOSED),
  );

  // Neighbor picks only mean something once registration is closed — that is the
  // phase the players are asked to make them in.
  const showNeighbors = $derived(phase === TokenBagPhase.CLOSED);

  const nameById = $derived(
    new Map(players.map((player) => [player.id, player.name])),
  );
  const taken = $derived(
    assignedNames === undefined
      ? null
      : new Set([...assignedNames].map(normalizeName)),
  );

  let pending = $state<BagPlayer | null>(null);

  function neighborOf(id: string): string {
    if (id === NO_ID) return "—";
    return nameById.get(id) ?? "—";
  }

  function confirmRemove() {
    const target = pending;
    pending = null;
    if (target) onremove?.(target.id);
  }
</script>

{#if players.length === 0}
  <p class="text-sm text-muted">Nobody has joined yet.</p>
{:else}
  <ul class="divide-y divide-border">
    {#each players as player, i (player.id)}
      <li class="flex items-center gap-2 py-1.5">
        <span class="w-5 shrink-0 text-center text-xs text-muted">{i + 1}</span>
        <span class="min-w-0 flex-1 truncate text-sm text-primary"
          >{player.name}</span
        >

        {#if player.viaSharedDevice}
          <!-- Added on the shared tablet rather than from their own phone. -->
          <svg
            class="h-3.5 w-3.5 shrink-0 text-muted"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="2"
            role="img"
            aria-label="Added on the shared device"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M9.75 17h4.5m-6.75 4h9a1.5 1.5 0 001.5-1.5v-15A1.5 1.5 0 0016.5 3h-9A1.5 1.5 0 006 4.5v15A1.5 1.5 0 007.5 21z"
            />
          </svg>
        {/if}

        {#if showNeighbors}
          {#if player.leftId === NO_ID && player.rightId === NO_ID}
            <span class="shrink-0 text-[11px] text-muted">no picks</span>
          {:else}
            <span class="shrink-0 text-[11px] text-secondary">
              ⇐ {neighborOf(player.leftId)} · {neighborOf(player.rightId)} ⇒
            </span>
          {/if}
        {/if}

        {#if taken}
          {#if taken.has(normalizeName(player.name))}
            <span
              class="shrink-0 rounded-full bg-element px-2 py-0.5 text-[11px] font-medium text-green-600 dark:text-green-400"
            >
              assigned ✓
            </span>
          {:else}
            <span
              class="shrink-0 rounded-full bg-element px-2 py-0.5 text-[11px] font-medium text-amber-600 dark:text-amber-400"
            >
              unassigned
            </span>
          {/if}
        {/if}

        {#if removable}
          <button
            type="button"
            onclick={() => (pending = player)}
            class="shrink-0 rounded-full p-1 text-muted transition-colors hover:bg-hover hover:text-red-500"
            aria-label="Remove {player.name}"
            title="Remove {player.name}"
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
                d="M6 18L18 6M6 6l12 12"
              /></svg
            >
          </button>
        {/if}
      </li>
    {/each}
  </ul>
{/if}

{#if pending}
  <ConfirmDialog
    title="Remove {pending.name}?"
    message="They lose their place in the bag. They can join again while registration is open."
    confirmLabel="Remove"
    oncancel={() => (pending = null)}
    onconfirm={confirmRemove}
  />
{/if}
