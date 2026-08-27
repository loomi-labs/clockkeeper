<script lang="ts">
  // A player's phone, reached by scanning the Storyteller's join QR code.
  //
  // Public route: the root layout renders it bare and never mints a session, so
  // everything here goes through the unauthenticated player bag. One screen at a
  // time, driven by `derivePlayerView`.
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
  /** True from a successful save until the next pick — the "Saved ✓" line. */
  let neighborsSaved = $state(false);

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
      selfId: bag.state.selfId,
      hasCredential: bag.state.hasCredential,
      dismissed: bag.state.dismissed,
      gameStarted: bag.state.gameStarted,
      streamStatus: bag.state.status,
    }),
  );
  const view = $derived(
    refinePlayerView(baseView, {
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
  /**
   * Registrations nobody has claimed from their own phone yet: added by the
   * Storyteller, typed on the shared device, or backfilled from the grimoire's
   * seat names. Tapping one joins under that exact name, which the server turns
   * into a claim of that registration — same response, same credential, same
   * waiting screen as a fresh join.
   */
  const unclaimed = $derived(
    bag.state.players.filter((player) => player.viaSharedDevice),
  );
  /**
   * The selects follow the server. Re-synced on every snapshot, not just the
   * first: a pick the bag invalidated — the Storyteller removed that player, so
   * the server cleared the reference — has to disappear here too, and a second
   * device on the same registration has to stay in step.
   *
   * Skipped while a save is in flight, where the local value is the newer of the
   * two. The save flipping back to idle re-runs this, so the answer the server
   * actually stored is what ends up on screen either way.
   */
  $effect(() => {
    const left = self?.leftId ?? NO_ID;
    const right = self?.rightId ?? NO_ID;
    if (savingNeighbors) return;
    leftPick = left;
    rightPick = right;
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

  /**
   * The one way into the bag, shared by the free-text form and the claim chips.
   * A chip is not a special case: the name it sends is an existing unclaimed
   * registration, and `JoinTokenBag` answers a claim exactly as it answers a new
   * registration — so a failed claim (someone else got there first) surfaces
   * through the same inline error as a taken name.
   */
  async function submitName(name: string) {
    if (name === "" || joining) return;
    joining = true;
    const ok = await bag.register(name);
    joining = false;
    if (ok) nameInput = "";
  }

  async function join(event: SubmitEvent) {
    event.preventDefault();
    await submitName(nameInput.trim());
  }

  /** Plain local: a queued re-save is not something the screen renders. */
  let resaveNeighbors = false;

  /**
   * Saves the moment a select changes — there is no Submit button. One side,
   * neither side and both sides are all legal answers: `SetTokenBagNeighbors`
   * reads id 0 as "no neighbor on that side", so picking "Not sure yet" back is
   * how a player *clears* an earlier answer.
   *
   * Single-flight with a re-run, so touching the second select while the first
   * is still in flight is not lost. A failed save sets nothing: the sync effect
   * puts the selects back to whatever the bag last reported, and `errorBox` says
   * why.
   */
  async function saveNeighbors(left: string, right: string) {
    leftPick = left;
    rightPick = right;
    neighborsSaved = false;
    if (savingNeighbors) {
      resaveNeighbors = true;
      return;
    }
    savingNeighbors = true;
    let ok = false;
    do {
      resaveNeighbors = false;
      ok = await bag.setNeighbors(leftPick, rightPick);
    } while (ok && resaveNeighbors);
    savingNeighbors = false;
    neighborsSaved = ok;
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
          {#if unclaimed.length > 0}
            <div class="space-y-1.5">
              <p class="text-xs text-muted">
                Already on the list? Tap your name
              </p>
              <div class="flex flex-wrap gap-2">
                {#each unclaimed as player (player.id)}
                  <button
                    type="button"
                    onclick={() => submitName(player.name)}
                    disabled={joining}
                    class="rounded-full border border-border bg-element px-4 py-2 text-sm font-medium text-primary transition-colors hover:border-indigo-400 hover:text-indigo-500 disabled:opacity-50"
                  >
                    {player.name}
                  </button>
                {/each}
              </div>
            </div>
            <div class="flex items-center gap-3">
              <span class="h-px flex-1 bg-border"></span>
              <span class="text-xs text-muted">or</span>
              <span class="h-px flex-1 bg-border"></span>
            </div>
          {/if}
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
      {:else if view.kind === "in_bag"}
        <!--
          One screen for the whole wait. The picker saves on select, so there is
          nothing to submit and no reason to leave it — which is what lets it
          open the moment a player joins, while the rest of the table is still
          registering and the options list is still growing.
        -->
        <div class="flex flex-1 flex-col gap-5">
          <div class="text-center">
            <p class="text-lg font-semibold text-primary">
              You're in{selfName ? `, ${selfName}` : ""}
            </p>
            {#if view.registrationOpen}
              <p class="mt-0.5 text-sm text-secondary">
                {bag.state.players.length}
                {bag.state.players.length === 1 ? "player" : "players"} joined
              </p>
            {/if}
          </div>

          <div class="space-y-3">
            <div>
              <h2 class="text-base font-semibold text-primary">
                Who's next to you?
              </h2>
              <p class="mt-1 text-sm text-secondary">
                Optional — helps the Storyteller arrange the grimoire. Saved as
                you pick.
              </p>
            </div>
            {#if options.length === 0}
              <p class="text-sm text-muted">Nobody else has joined yet.</p>
            {:else}
              {@render neighborPicker(
                "left-neighbor",
                "On your left",
                leftPick,
                (next) => saveNeighbors(next, rightPick),
              )}
              {@render neighborPicker(
                "right-neighbor",
                "On your right",
                rightPick,
                (next) => saveNeighbors(leftPick, next),
              )}
              <!-- Fixed height: the line must not shift the selects as it changes. -->
              <p class="h-4 text-xs">
                {#if savingNeighbors}
                  <span class="text-muted">Saving…</span>
                {:else if neighborsSaved}
                  <span class="text-green-600 dark:text-green-400">Saved ✓</span
                  >
                {/if}
              </p>
            {/if}
            {@render errorBox()}
          </div>

          <div class="mt-auto flex flex-col items-center gap-2 pt-2">
            {#if !view.registrationOpen}
              {@render spinner()}
            {/if}
            <p class="text-xs text-muted">
              {view.registrationOpen
                ? "Waiting for the Storyteller…"
                : "Waiting for the reveal…"}
            </p>
          </div>
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
      {:else if view.kind === "game_started"}
        <div class="flex flex-1 flex-col items-center justify-center gap-3">
          <p class="text-lg font-semibold text-primary">The game has started</p>
          <p class="text-center text-sm text-secondary">
            Your role stays with you — good luck!
          </p>
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
