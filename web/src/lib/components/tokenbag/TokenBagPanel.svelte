<script lang="ts">
  // The Storyteller's Digital Token Bag control panel, mounted in the setup tab.
  //
  // It owns one `createStorytellerBag` for the game: the phase buttons, the two
  // QR codes and the live registrant list. Everything the game page needs from it
  // travels through callbacks — the registered names (which replace the preset
  // names in the assignment UI), the grimoire arrange request, and a pre-reveal
  // hook so a pending debounced grimoire save lands before the server reads the
  // seat names.
  import { onMount, untrack } from "svelte";
  import ConfirmDialog from "../ConfirmDialog.svelte";
  import QrCode from "../QrCode.svelte";
  import RegistrantList from "./RegistrantList.svelte";
  import { TokenBagPhase } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import {
    NO_ID,
    deviceUrl,
    joinUrl,
    unassignedRegistrants,
  } from "~/lib/tokenbag";
  import { createStorytellerBag } from "~/lib/tokenbag.svelte";
  import { seatingNames } from "~/lib/tokenbag-arrange";
  import type { BagRegistrants } from "~/lib/tokenbag-names";

  let {
    gameId,
    playerCount,
    assignedNames,
    onregistrants,
    onarrange,
    onbeforereveal,
  }: {
    gameId: bigint;
    /** What the game is set up for — shown next to the registrant count. */
    playerCount: number;
    /** Grimoire seat names, for the assigned / unassigned badges and Reveal. */
    assignedNames: ReadonlySet<string>;
    /**
     * Fires whenever the live registrant list or the phase changes (including on
     * mount). The report is stamped with the game it belongs to so the page can
     * discard it once it no longer applies.
     */
    onregistrants: (registrants: BagRegistrants) => void;
    /** Arranges the grimoire; resolves to an error message, or null on success. */
    onarrange: (orderedNames: string[]) => Promise<string | null>;
    /**
     * Flushes pending page state to the server before the reveal reads it.
     * REJECTS when the state did not make it, and the reveal is abandoned.
     */
    onbeforereveal: () => Promise<void>;
  } = $props();

  // One bag per mounted panel, tied to the game the panel was mounted for. The
  // game id is a route-derived constant here, so it is read once on purpose —
  // the same non-reactive capture the /join and /device pages make of their code.
  const bagGameId = untrack(() => gameId);
  const bag = createStorytellerBag(bagGameId);

  /** Set on the client only: prerendering has no origin to encode into a QR. */
  let origin = $state("");
  let viewportWidth = $state(0);

  let busy = $state(false);
  let confirming = $state<"reveal" | "reset" | null>(null);
  let qrModal = $state<{ label: string; url: string } | null>(null);
  let copied = $state(false);

  let arranging = $state(false);
  let arrangeConflicts = $state<string[]>([]);
  let arrangeError = $state<string | null>(null);
  let arranged = $state(false);
  /** Why the reveal did not happen. Set when the pre-reveal flush failed. */
  let revealError = $state<string | null>(null);

  onMount(() => {
    origin = window.location.origin;
    void bag.load();
  });

  const phase = $derived(bag.state.phase);
  const players = $derived(bag.state.players);
  const playerNames = $derived(players.map((player) => player.name));

  /**
   * The watch stream is what makes the registrant list live. It is only worth
   * dialling once the codes are known and the bag actually exists — and the
   * condition is deliberately a boolean, so moving between OPEN / CLOSED /
   * REVEALED does not tear the stream down and redial it.
   */
  const shouldWatch = $derived(
    bag.state.joinCode !== "" &&
      phase !== TokenBagPhase.UNSPECIFIED &&
      phase !== TokenBagPhase.INACTIVE,
  );
  /** Whether a stream is meant to be up right now (for the "lost it" strip). */
  let watching = $state(false);

  $effect(() => {
    if (!shouldWatch) return;
    // Read inside the effect: a reset rotates the join code, and the next stream
    // has to carry the new one.
    const code = bag.state.joinCode;
    bag.start(code);
    watching = true;
    return () => {
      bag.stop();
      watching = false;
    };
  });

  // The page mirrors the registrant names into its assignment UI. The phase rides
  // along because it decides whether seat renames are still dangerous, and the
  // game id because this panel is not mounted for the whole session.
  $effect(() => {
    onregistrants({
      names: playerNames,
      prereveal: phase !== TokenBagPhase.REVEALED,
      gameId: String(bagGameId),
    });
  });

  // A stream that stopped while it was supposed to be up hit a fatal error; the
  // list on screen is frozen from here on, so say so and offer a redial.
  const streamLost = $derived(watching && bag.state.status === "stopped");

  const joinLink = $derived(
    origin && bag.state.joinCode ? joinUrl(origin, bag.state.joinCode) : "",
  );
  const deviceLink = $derived(
    origin && bag.state.sharedCode
      ? deviceUrl(origin, bag.state.sharedCode)
      : "",
  );

  const unassigned = $derived(
    unassignedRegistrants(playerNames, assignedNames),
  );
  const canReveal = $derived(
    players.length > 0 && unassigned.length === 0 && !busy,
  );
  const pickedCount = $derived(
    players.filter(
      (player) => player.leftId !== NO_ID || player.rightId !== NO_ID,
    ).length,
  );

  /** The enlarged QR, clamped so it fits a phone and never dwarfs a desktop. */
  const modalQrSize = $derived(
    Math.min(Math.max(Math.round(viewportWidth * 0.9), 240), 480),
  );

  /** Runs a bag action, keeping the buttons quiet while it is in flight. */
  async function run(action: () => Promise<boolean>) {
    if (busy) return;
    busy = true;
    clearArrangeFeedback();
    try {
      await action();
    } finally {
      busy = false;
    }
  }

  function clearArrangeFeedback() {
    arrangeConflicts = [];
    arrangeError = null;
    arranged = false;
    revealError = null;
  }

  function openQr(label: string, url: string) {
    qrModal = { label, url };
    copied = false;
  }

  /**
   * Escape closes the QR modal. On window rather than on the overlay: the
   * overlay is not focusable, so a keydown only ever reached it if the focus
   * happened to be inside it — Escape did nothing right after opening the modal
   * from the card button. Bound only while the modal is open.
   */
  function onModalKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") qrModal = null;
  }

  async function copyLink(url: string) {
    try {
      await navigator.clipboard.writeText(url);
      copied = true;
      return;
    } catch {
      // Clipboard access can be refused outright (insecure origin, no
      // permission). Fall through to the selection-based copy.
    }
    try {
      const scratch = document.createElement("textarea");
      scratch.value = url;
      scratch.setAttribute("readonly", "");
      scratch.style.position = "fixed";
      scratch.style.opacity = "0";
      document.body.appendChild(scratch);
      scratch.select();
      copied = document.execCommand("copy");
      document.body.removeChild(scratch);
    } catch {
      copied = false;
    }
  }

  async function doArrange() {
    if (arranging) return;
    arranging = true;
    clearArrangeFeedback();
    try {
      const seating = await bag.seating();
      // A null response means the RPC failed; bag.state.error explains it.
      if (!seating) return;
      if (!seating.complete) {
        arrangeConflicts = [...seating.conflicts];
        return;
      }
      const ordered = seatingNames(seating.orderedRegistrationIds, players);
      const failure = await onarrange(ordered);
      if (failure) arrangeError = failure;
      else arranged = true;
    } finally {
      arranging = false;
    }
  }

  async function doReveal() {
    confirming = null;
    await run(async () => {
      // Seat names are read server-side, so a debounced save still in flight
      // would make the reveal act on stale assignments. If that save cannot be
      // landed, ABORT: revealing on assignments the server never saw deals real
      // players the wrong characters, and a reveal cannot be taken back.
      try {
        await onbeforereveal();
      } catch {
        revealError =
          "Could not save the role assignments — nothing was revealed. Check your connection and try again.";
        return false;
      }
      return await bag.reveal();
    });
  }

  async function doReset() {
    confirming = null;
    await run(bag.reset);
  }
</script>

<svelte:window
  bind:innerWidth={viewportWidth}
  onkeydown={qrModal ? onModalKeydown : undefined}
/>

{#snippet qrCard(label: string, url: string)}
  <button
    type="button"
    onclick={() => openQr(label, url)}
    class="flex-1 rounded-lg border border-border p-3 transition-colors hover:border-indigo-400"
    title="Enlarge {label} code"
  >
    <QrCode value={url} {label} size={160} />
  </button>
{/snippet}

<div class="rounded-lg border border-border bg-surface p-4">
  <div class="mb-3 flex items-center justify-between gap-2">
    <h3 class="text-sm font-semibold text-secondary">Digital Token Bag</h3>
    {#if phase !== TokenBagPhase.INACTIVE && players.length > 0}
      <span class="text-xs text-muted">
        {players.length} registered · game is set for {playerCount} players
      </span>
    {/if}
  </div>

  {#if bag.state.status === "reconnecting"}
    <p
      class="mb-3 rounded-lg bg-yellow-100 px-3 py-1.5 text-xs font-medium text-yellow-800 dark:bg-yellow-500/20 dark:text-yellow-200"
    >
      Reconnecting…
    </p>
  {:else if streamLost}
    <div
      class="mb-3 flex items-center gap-2 rounded-lg bg-yellow-100 px-3 py-1.5 text-xs font-medium text-yellow-800 dark:bg-yellow-500/20 dark:text-yellow-200"
    >
      <span class="min-w-0 flex-1">The registrant list is no longer live.</span>
      <button
        type="button"
        onclick={() => bag.start(bag.state.joinCode)}
        class="shrink-0 underline transition-opacity hover:opacity-70"
      >
        Reconnect
      </button>
    </div>
  {/if}

  {#if bag.state.error}
    <p
      class="mb-3 rounded-lg border border-error-border bg-error-bg px-3 py-2 text-xs text-error-text"
    >
      {bag.state.error}
    </p>
  {/if}

  {#if phase === TokenBagPhase.UNSPECIFIED}
    <p class="text-sm text-muted">Loading…</p>
  {:else if phase === TokenBagPhase.INACTIVE}
    <p class="text-sm text-secondary">
      Players scan a QR code, register their name, and receive their character
      digitally after you assign roles.
    </p>
    <button
      type="button"
      onclick={() => run(bag.open)}
      disabled={busy}
      class="mt-3 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-500 disabled:opacity-50"
    >
      Open registration
    </button>
  {:else if phase === TokenBagPhase.OPEN}
    <div class="space-y-4">
      <div class="flex flex-col gap-3 sm:flex-row">
        {#if joinLink}
          {@render qrCard("Join game", joinLink)}
        {/if}
        {#if deviceLink}
          {@render qrCard("Shared Device", deviceLink)}
        {/if}
      </div>
      <p class="text-xs text-muted">
        Players scan the join code on their own phone. The shared device code is
        for a tablet you pass around the table for anyone without one.
      </p>

      <RegistrantList
        {players}
        {phase}
        onremove={(id) => run(() => bag.remove(id))}
      />

      <button
        type="button"
        onclick={() => run(bag.close)}
        disabled={busy}
        class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-500 disabled:opacity-50"
      >
        Close registration
      </button>
    </div>
  {:else if phase === TokenBagPhase.CLOSED}
    <div class="space-y-4">
      <p class="text-sm text-secondary">
        Registration closed. Assign every registered player to a role, then
        reveal.
      </p>

      <RegistrantList
        {players}
        {phase}
        {assignedNames}
        onremove={(id) => run(() => bag.remove(id))}
      />

      {#if unassigned.length > 0}
        <p class="text-xs text-amber-600 dark:text-amber-400">
          Not assigned to a role yet: {unassigned.join(", ")}
        </p>
      {/if}

      {#if arrangeConflicts.length > 0}
        <div
          class="max-h-40 overflow-y-auto rounded-lg border border-border bg-element px-3 py-2"
        >
          <p class="text-xs font-medium text-secondary">
            The neighbor picks don't form one ring yet:
          </p>
          <ul class="mt-1 space-y-0.5">
            <!-- Unkeyed: the server's conflict strings are free text and may repeat. -->
            {#each arrangeConflicts as conflict}
              <li class="text-xs text-muted">{conflict}</li>
            {/each}
          </ul>
        </div>
      {/if}
      {#if arrangeError}
        <p class="text-xs text-amber-600 dark:text-amber-400">{arrangeError}</p>
      {/if}
      {#if revealError}
        <p
          class="rounded-lg border border-error-border bg-error-bg px-3 py-2 text-xs text-error-text"
        >
          {revealError}
        </p>
      {/if}
      {#if arranged}
        <p class="text-xs text-green-600 dark:text-green-400">
          Grimoire arranged ✓
        </p>
      {/if}

      <div class="flex flex-wrap gap-2">
        <button
          type="button"
          onclick={() => run(bag.open)}
          disabled={busy}
          class="rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-secondary transition-colors hover:border-indigo-400 hover:text-indigo-500 disabled:opacity-40"
        >
          Re-open registration
        </button>
        <button
          type="button"
          onclick={doArrange}
          disabled={arranging || pickedCount === 0}
          title={pickedCount === 0
            ? "No player has picked their neighbors yet"
            : "Seat the grimoire in the order the players picked"}
          class="rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-secondary transition-colors hover:border-indigo-400 hover:text-indigo-500 disabled:opacity-40 disabled:hover:border-border disabled:hover:text-secondary"
        >
          {arranging ? "Arranging…" : "Arrange grimoire from neighbor picks"}
        </button>
        <button
          type="button"
          onclick={() => (confirming = "reveal")}
          disabled={!canReveal}
          title={players.length === 0
            ? "Nobody has joined the bag"
            : unassigned.length > 0
              ? `Assign these players to a role first: ${unassigned.join(", ")}`
              : "Show every player their character"}
          class="rounded-lg bg-indigo-600 px-4 py-1.5 text-xs font-medium text-white transition-colors hover:bg-indigo-500 disabled:opacity-50 disabled:hover:bg-indigo-600"
        >
          Reveal tokens
        </button>
      </div>
    </div>
  {:else}
    <div class="space-y-4">
      <p class="text-sm text-secondary">
        Tokens revealed — players can see their roles.
      </p>

      {#if deviceLink}
        <div class="flex">
          {@render qrCard("Shared Device", deviceLink)}
        </div>
        <p class="text-xs text-muted">
          Players without a phone come back to the shared device to see their
          character again.
        </p>
      {/if}

      <RegistrantList {players} {phase} {assignedNames} />

      <button
        type="button"
        onclick={() => (confirming = "reset")}
        disabled={busy}
        class="rounded-lg border border-red-300 px-3 py-1.5 text-xs font-medium text-red-500 transition-colors hover:bg-red-500 hover:text-white disabled:opacity-40 dark:border-red-700"
      >
        Reset token bag
      </button>
    </div>
  {/if}
</div>

{#if qrModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center">
    <button
      type="button"
      tabindex="-1"
      class="absolute inset-0 bg-black/70"
      onclick={() => (qrModal = null)}
      aria-label="Close"
    ></button>
    <div
      role="dialog"
      aria-modal="true"
      aria-label={qrModal.label}
      class="relative z-10 flex max-h-[95dvh] w-full max-w-lg flex-col items-center gap-4 overflow-y-auto rounded-xl border border-border bg-surface p-5 shadow-xl"
    >
      <QrCode value={qrModal.url} label={qrModal.label} size={modalQrSize} />
      <p
        class="w-full break-all rounded-lg bg-element px-3 py-2 text-center text-xs text-secondary select-all"
      >
        {qrModal.url}
      </p>
      <div class="flex gap-3">
        <button
          type="button"
          onclick={() => qrModal && copyLink(qrModal.url)}
          class="rounded-lg border border-border px-4 py-2 text-sm font-medium text-secondary transition-colors hover:bg-hover hover:text-medium"
        >
          {copied ? "Copied ✓" : "Copy link"}
        </button>
        <button
          type="button"
          onclick={() => (qrModal = null)}
          class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-500"
        >
          Done
        </button>
      </div>
    </div>
  </div>
{/if}

{#if confirming === "reveal"}
  <ConfirmDialog
    title="Reveal tokens?"
    message="Every player will see their role. This cannot be undone without a reset."
    confirmLabel="Reveal tokens"
    oncancel={() => (confirming = null)}
    onconfirm={doReveal}
  />
{/if}

{#if confirming === "reset"}
  <ConfirmDialog
    title="Reset token bag?"
    message="This removes all registrations and invalidates the QR codes. Players have to scan a new code and register again."
    confirmLabel="Reset token bag"
    oncancel={() => (confirming = null)}
    onconfirm={doReset}
  />
{/if}
