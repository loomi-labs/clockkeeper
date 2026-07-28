<script lang="ts">
  // A player's phone, reached by scanning the Storyteller's join QR code.
  //
  // Public route: the root layout renders it bare and never mints a session, so
  // everything here goes through the unauthenticated player bag. One screen at a
  // time, driven by `derivePlayerView` plus this page's own picker flags.
  import { onMount } from "svelte";
  import { page } from "$app/state";
  import ConfirmDialog from "~/lib/components/ConfirmDialog.svelte";
  import RoleCard from "~/lib/components/tokenbag/RoleCard.svelte";
  import { TokenBagPhase } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { initTheme } from "~/lib/theme";
  import { NO_ID, derivePlayerView, neighborOptions } from "~/lib/tokenbag";
  import { createPlayerBag } from "~/lib/tokenbag.svelte";
  import { loadCredential } from "~/lib/tokenbag-credentials";
  import { refinePlayerView } from "~/lib/tokenbag-views";

  const code = page.params.code ?? "";
  const bag = createPlayerBag(code);

  let nameInput = $state("");
  let joining = $state(false);

  let leftPick = $state(NO_ID);
  let rightPick = $state(NO_ID);
  let savingNeighbors = $state(false);
  /** The picker has had its turn on this device (submitted or skipped). */
  let neighborsSettled = $state(false);
  /** The player reopened the picker to change an earlier answer. */
  let editingNeighbors = $state(false);

  let confirmShow = $state(false);
  let showingToken = $state(false);

  onMount(() => {
    initTheme();
  });

  $effect(() => {
    bag.start();
    return () => bag.stop();
  });

  const baseView = $derived(
    derivePlayerView({
      phase: bag.state.phase,
      players: bag.state.players,
      selfId: bag.state.selfId,
      hasCredential: bag.state.hasCredential,
      dismissed: bag.state.dismissed,
      streamStatus: bag.state.status,
    }),
  );
  const view = $derived(
    refinePlayerView(baseView, {
      settled: neighborsSettled,
      editing: editingNeighbors,
      // `gone` is the factory's verdict on an unknown code; a stopped stream
      // covers the other fatal errors (a rejected credential, say). Either way
      // this device will never hear from the bag again.
      streamDead: bag.state.gone || bag.state.status === "stopped",
    }),
  );

  const self = $derived(
    bag.state.players.find((player) => player.id === bag.state.selfId) ?? null,
  );
  /** The stream knows the name once registered; before that, the credential does. */
  const selfName = $derived(self?.name ?? loadCredential(code)?.name ?? "");
  const options = $derived(
    neighborOptions(bag.state.players, bag.state.selfId),
  );
  function nameOf(id: string): string {
    if (id === NO_ID) return "";
    return bag.state.players.find((player) => player.id === id)?.name ?? "";
  }

  /** Plain local: prefilling must not re-run on every snapshot. */
  let picksPrefilled = false;
  $effect(() => {
    if (baseView.kind !== "neighbor_pick") {
      // Registration reopened, or the bag was reset — the picker starts over.
      picksPrefilled = false;
      neighborsSettled = false;
      editingNeighbors = false;
      return;
    }
    if (picksPrefilled) return;
    picksPrefilled = true;
    leftPick = self?.leftId ?? NO_ID;
    rightPick = self?.rightId ?? NO_ID;
  });

  /**
   * The very first reveal shows the character without asking. Normally the
   * stream carries it; if it does not (the stream opened after the reveal and
   * lost the race), fetch it rather than sit on an empty screen.
   */
  let autoFetched = false;
  $effect(() => {
    if (view.kind !== "revealed_shown") {
      autoFetched = false;
      return;
    }
    if (bag.state.selfToken || autoFetched) return;
    autoFetched = true;
    void bag.fetchMyToken();
  });

  async function join(event: SubmitEvent) {
    event.preventDefault();
    const name = nameInput.trim();
    if (name === "" || joining) return;
    joining = true;
    const ok = await bag.register(name);
    joining = false;
    if (ok) nameInput = "";
  }

  /**
   * Submit is always available (bar the in-flight guard): one side, neither side
   * and both sides are all legal answers. `SetTokenBagNeighbors` reads id 0 as
   * "no neighbor on that side", so submitting with a picker back on "Not sure
   * yet" is how a player *clears* an earlier answer — a predicate demanding both
   * sides would make a wrong pick unremovable.
   */
  async function submitNeighbors() {
    if (savingNeighbors) return;
    savingNeighbors = true;
    const ok = await bag.setNeighbors(leftPick, rightPick);
    savingNeighbors = false;
    if (!ok) return;
    neighborsSettled = true;
    editingNeighbors = false;
  }

  function skipNeighbors() {
    neighborsSettled = true;
    editingNeighbors = false;
  }

  async function revealAgain() {
    confirmShow = false;
    showingToken = true;
    const character = await bag.fetchMyToken();
    showingToken = false;
    // Only un-hide once the character is actually in hand, so a failed fetch
    // does not drop the player onto a blank "shown" screen.
    if (character) bag.dismissToken(false);
  }
</script>

<svelte:head>
  <title>Join game — Clock Keeper</title>
</svelte:head>

{#snippet errorBox()}
  {#if bag.state.error}
    <p
      class="rounded-lg border border-error-border bg-error-bg px-3 py-2 text-sm text-error-text"
    >
      {bag.state.error}
    </p>
  {/if}
{/snippet}

{#snippet spinner()}
  <div
    class="h-8 w-8 animate-spin rounded-full border-2 border-border border-t-indigo-500"
  ></div>
{/snippet}

{#snippet neighborPicker(
  id: string,
  label: string,
  value: string,
  onpick: (next: string) => void,
)}
  <div class="space-y-1">
    <label for={id} class="text-sm font-medium text-secondary">{label}</label>
    <select
      {id}
      value={value === NO_ID ? "" : value}
      onchange={(event) =>
        onpick((event.currentTarget as HTMLSelectElement).value || NO_ID)}
      class="w-full rounded-lg border border-border bg-surface px-3 py-2.5 text-base text-primary"
    >
      <option value="">Not sure yet</option>
      {#each options as option (option.id)}
        <option value={option.id}>{option.name}</option>
      {/each}
    </select>
  </div>
{/snippet}

<div class="min-h-dvh bg-surface-alt text-primary">
  {#if bag.state.status === "reconnecting"}
    <div
      class="sticky top-0 z-40 bg-yellow-100 px-4 py-1 text-center text-xs font-medium text-yellow-800 dark:bg-yellow-500/20 dark:text-yellow-200"
    >
      Reconnecting…
    </div>
  {/if}

  <div class="mx-auto flex min-h-dvh w-full max-w-md flex-col gap-6 p-4 pt-8">
    <header class="text-center">
      <h1 class="text-xl font-bold text-primary">
        {bag.state.gameName || "Join game"}
      </h1>
      {#if bag.state.gameName}
        <p class="mt-0.5 text-xs text-secondary">Blood on the Clocktower</p>
      {/if}
    </header>

    <main
      class="card-slate flex flex-1 flex-col rounded-xl bg-surface p-5 shadow-sm"
    >
      {#if view.kind === "loading"}
        <div class="flex flex-1 flex-col items-center justify-center gap-3">
          {@render spinner()}
          <p class="text-sm text-secondary">Connecting…</p>
        </div>
      {:else if view.kind === "enter_name"}
        <form class="space-y-4" onsubmit={join}>
          <div class="space-y-1">
            <label for="player-name" class="text-sm font-medium text-secondary">
              Your name
            </label>
            <input
              id="player-name"
              type="text"
              maxlength="50"
              autocomplete="name"
              bind:value={nameInput}
              placeholder="How the Storyteller knows you"
              class="w-full rounded-lg border border-border bg-surface px-3 py-2.5 text-base text-primary placeholder:text-muted"
            />
          </div>
          {@render errorBox()}
          <button
            type="submit"
            disabled={joining || nameInput.trim() === ""}
            class="w-full rounded-lg bg-indigo-600 px-4 py-3 text-base font-medium text-white transition-colors hover:bg-indigo-500 disabled:opacity-50"
          >
            {joining ? "Joining…" : "Join"}
          </button>
        </form>
      {:else if view.kind === "waiting_open"}
        <div class="flex flex-1 flex-col items-center justify-center gap-3">
          <p class="text-lg font-semibold text-primary">
            You're in{selfName ? `, ${selfName}` : ""}
          </p>
          <p class="text-sm text-secondary">
            {bag.state.players.length}
            {bag.state.players.length === 1 ? "player" : "players"} joined
          </p>
          <p class="text-xs text-muted">Waiting for the Storyteller…</p>
          {@render errorBox()}
        </div>
      {:else if view.kind === "neighbor_pick"}
        <div class="space-y-4">
          <div>
            <h2 class="text-base font-semibold text-primary">
              Who's next to you?
            </h2>
            <p class="mt-1 text-sm text-secondary">
              Optional — helps the Storyteller arrange the grimoire.
            </p>
          </div>
          {#if options.length === 0}
            <p class="text-sm text-muted">Nobody else has joined.</p>
          {:else}
            {@render neighborPicker(
              "left-neighbor",
              "On your left",
              leftPick,
              (next) => (leftPick = next),
            )}
            {@render neighborPicker(
              "right-neighbor",
              "On your right",
              rightPick,
              (next) => (rightPick = next),
            )}
          {/if}
          {@render errorBox()}
          <div class="flex gap-3">
            <button
              type="button"
              onclick={skipNeighbors}
              class="flex-1 rounded-lg border border-border px-4 py-3 text-base font-medium text-secondary transition-colors hover:bg-hover hover:text-medium"
            >
              Skip
            </button>
            <button
              type="button"
              onclick={submitNeighbors}
              disabled={savingNeighbors}
              class="flex-1 rounded-lg bg-indigo-600 px-4 py-3 text-base font-medium text-white transition-colors hover:bg-indigo-500 disabled:opacity-50"
            >
              {savingNeighbors ? "Saving…" : "Submit"}
            </button>
          </div>
        </div>
      {:else if view.kind === "waiting_reveal"}
        <div class="flex flex-1 flex-col items-center justify-center gap-4">
          {@render spinner()}
          <p class="text-sm text-secondary">Waiting for the reveal…</p>
          {#if self && (self.leftId !== NO_ID || self.rightId !== NO_ID)}
            <p class="text-center text-xs text-muted">
              Left: {nameOf(self.leftId) || "—"} · Right: {nameOf(
                self.rightId,
              ) || "—"}
            </p>
          {/if}
          <button
            type="button"
            onclick={() => (editingNeighbors = true)}
            class="rounded-lg border border-border px-4 py-2 text-sm font-medium text-secondary transition-colors hover:bg-hover hover:text-medium"
          >
            {self && (self.leftId !== NO_ID || self.rightId !== NO_ID)
              ? "Edit neighbors"
              : "Add neighbors"}
          </button>
        </div>
      {:else if view.kind === "revealed_shown"}
        <div class="flex flex-1 flex-col items-center justify-between gap-6">
          {#if bag.state.selfToken}
            <RoleCard character={bag.state.selfToken} />
          {:else}
            <div class="flex flex-1 flex-col items-center justify-center gap-3">
              {@render spinner()}
              <p class="text-sm text-secondary">Fetching your character…</p>
            </div>
          {/if}
          {@render errorBox()}
          <button
            type="button"
            onclick={() => bag.dismissToken(true)}
            class="w-full rounded-lg border border-border px-4 py-3 text-base font-medium text-secondary transition-colors hover:bg-hover hover:text-medium"
          >
            Hide my role
          </button>
        </div>
      {:else if view.kind === "revealed_hidden"}
        <div class="flex flex-1 flex-col items-center justify-center gap-4">
          <p class="text-lg font-semibold text-primary">Your role is hidden</p>
          <p class="text-center text-sm text-secondary">
            Nobody can see it until you show it again.
          </p>
          {@render errorBox()}
          <button
            type="button"
            onclick={() => (confirmShow = true)}
            disabled={showingToken}
            class="rounded-lg bg-indigo-600 px-4 py-3 text-base font-medium text-white transition-colors hover:bg-indigo-500 disabled:opacity-50"
          >
            {showingToken ? "Loading…" : "Show my role"}
          </button>
        </div>
      {:else if view.kind === "removed"}
        <div class="flex flex-1 flex-col items-center justify-center gap-4">
          <p class="text-center text-base text-primary">
            The Storyteller removed you from the game.
          </p>
          {#if bag.state.phase === TokenBagPhase.OPEN}
            <button
              type="button"
              onclick={() => bag.forget()}
              class="rounded-lg bg-indigo-600 px-4 py-3 text-base font-medium text-white transition-colors hover:bg-indigo-500"
            >
              Join again
            </button>
          {/if}
        </div>
      {:else}
        <div class="flex flex-1 flex-col items-center justify-center gap-2">
          <p class="text-center text-base text-primary">
            This game's token bag is closed.
          </p>
          <p class="text-center text-xs text-muted">
            Ask the Storyteller for a fresh link.
          </p>
        </div>
      {/if}
    </main>
  </div>
</div>

{#if confirmShow}
  <ConfirmDialog
    title="Show your role?"
    message="Make sure nobody else can see your screen."
    confirmLabel="Show it"
    oncancel={() => (confirmShow = false)}
    onconfirm={revealAgain}
  />
{/if}
