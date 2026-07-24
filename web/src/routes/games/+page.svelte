<script lang="ts">
  import { onMount } from "svelte";
  import { goto } from "$app/navigation";
  import { page } from "$app/state";
  import { client } from "~/lib/api";
  import { invalidateSidebar } from "~/lib/sidebar-data.svelte";
  import { getErrorMessage } from "~/lib/errors";
  import { editionStyle } from "~/lib/editions";
  import type {
    Script,
    Edition,
    RoleDistribution,
    GameSummary,
  } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { GameState } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import DistributionBar from "~/lib/components/DistributionBar.svelte";
  import GamesList from "~/lib/components/GamesList.svelte";
  import PlayerPresetsModal from "~/lib/components/PlayerPresetsModal.svelte";

  // --- Game list state ---
  let games = $state<GameSummary[]>([]);

  const runningGames = $derived(
    [...games]
      .filter(
        (g) => g.state === GameState.SETUP || g.state === GameState.IN_PROGRESS,
      )
      .sort((a, b) => {
        const order: Record<number, number> = {
          [GameState.IN_PROGRESS]: 0,
          [GameState.SETUP]: 1,
        };
        return (order[a.state] ?? 2) - (order[b.state] ?? 2);
      }),
  );

  // --- New game wizard state ---
  let scripts = $state<Script[]>([]);
  let editions = $state<Edition[]>([]);
  let selectedScriptId = $state<bigint | undefined>();
  let playerCount = $state(8);
  let travellerCount = $state(0);
  let loading = $state(true);
  let creating = $state(false);
  let error = $state("");
  let currentDist = $state<RoleDistribution | undefined>();

  // Optional preset player names (assigned to seats later, in game setup).
  let presetNames = $state<string[]>([]);
  let showPresets = $state(false);

  async function reloadPresets() {
    try {
      const resp = await client.getPlayerPresets({});
      presetNames = [...resp.names];
    } catch {
      // silently ignore — names are optional
    }
  }

  const selectedScript = $derived(
    scripts.find((s) => s.id === selectedScriptId),
  );
  const totalPeople = $derived(playerCount + travellerCount);
  const showTotalWarning = $derived(totalPeople > 20);

  let distributionRequest = 0;

  $effect(() => {
    const requestId = ++distributionRequest;
    const pc = playerCount;
    client
      .getDistribution({ playerCount: pc })
      .then((resp) => {
        if (requestId !== distributionRequest) return;
        currentDist = resp.distribution;
      })
      .catch(() => {
        if (requestId !== distributionRequest) return;
        currentDist = undefined;
      });
  });

  onMount(async () => {
    try {
      const [scriptsResp, editionsResp] = await Promise.all([
        client.listScripts({}),
        client.listEditions({}),
      ]);
      scripts = scriptsResp.scripts;
      editions = editionsResp.editions;

      const scriptParam = page.url.searchParams.get("script");
      if (scriptParam) {
        try {
          const parsedId = BigInt(scriptParam);
          if (scripts.some((s) => s.id === parsedId)) {
            selectedScriptId = parsedId;
          }
        } catch {
          // Ignore invalid script parameter
        }
      }
    } catch (err) {
      error = getErrorMessage(err, "Failed to load");
    } finally {
      loading = false;
    }

    // Load games list independently
    client
      .listGames({})
      .then((resp) => {
        games = resp.games;
      })
      .catch((err) => {
        console.error("Failed to load games", err);
      });

    // Load optional player-name presets independently.
    reloadPresets();
  });

  function selectEdition(editionId: string) {
    const existing = scripts.find((s) => s.edition === editionId && s.isSystem);
    if (existing) {
      selectedScriptId = existing.id;
    }
  }

  async function createGame() {
    if (!selectedScriptId || creating) return;
    creating = true;
    error = "";
    try {
      const resp = await client.createGame({
        scriptId: selectedScriptId,
        playerCount,
        travellerCount,
      });
      if (!resp.game) {
        error = "Failed to create game";
        creating = false;
        return;
      }
      invalidateSidebar();
      await goto(`/games/${resp.game.id}`);
    } catch (err) {
      error = getErrorMessage(err, "Failed to create game");
      creating = false;
    }
  }
</script>

<div class="mx-auto max-w-2xl space-y-8">
  <h1 class="text-2xl font-bold text-primary">Games</h1>

  {#if error}
    <div
      class="rounded-lg bg-error-bg border border-error-border px-4 py-2 text-sm text-error-text"
    >
      {error}
    </div>
  {/if}

  <!-- Running Games -->
  {#if !loading && runningGames.length > 0}
    <section class="space-y-3">
      <h2 class="text-lg font-semibold text-medium">Running Games</h2>
      <GamesList
        games={runningGames}
        ondeleted={(id) => (games = games.filter((g) => g.id !== id))}
      />
    </section>
  {/if}

  <!-- New Game Wizard -->
  <section class="space-y-6">
    <h2 class="text-lg font-semibold text-medium">New Game</h2>

    {#if loading}
      <p class="text-secondary">Loading...</p>
    {:else}
      <!-- Step 1: Pick script -->
      <div class="space-y-3">
        <h3 class="text-base font-medium text-secondary">
          1. Choose an Edition
        </h3>

        {#if editions.length > 0}
          <div class="grid gap-3 sm:grid-cols-3">
            {#each editions as edition}
              {@const style = editionStyle(edition.id)}
              {@const isSelected =
                selectedScript?.isSystem === true &&
                selectedScript?.edition === edition.id}
              <button
                onclick={() => selectEdition(edition.id)}
                class="flex flex-col items-center rounded-lg border p-5 transition-all hover:scale-[1.02] hover:brightness-110 {isSelected
                  ? `${style.activeBorder} ${style.activeBg}`
                  : `${style.border} ${style.bg}`}"
              >
                <img
                  src="/editions/{edition.id}.webp"
                  alt={edition.name}
                  class="h-16 object-contain"
                />
                <p class="mt-3 text-sm text-secondary">
                  {edition.characterIds.length} characters
                </p>
              </button>
            {/each}
          </div>
        {/if}

        {#if scripts.filter((s) => !s.isSystem).length > 0}
          <p class="text-sm text-muted">Or choose a saved script:</p>
          <div class="grid gap-2 sm:grid-cols-2">
            {#each scripts.filter((s) => !s.isSystem) as script (script.id)}
              <button
                onclick={() => (selectedScriptId = script.id)}
                class="card-slate rounded-lg border p-3 text-left transition-colors {selectedScriptId ===
                script.id
                  ? 'border-indigo-500 bg-indigo-500/10'
                  : 'border-border bg-surface hover:border-border-strong'}"
              >
                <span class="font-medium text-primary">{script.name}</span>
                <span class="ml-2 text-sm text-secondary"
                  >{script.characterIds.length} chars</span
                >
              </button>
            {/each}
            <a
              href="/scripts"
              class="flex items-center justify-center gap-2 rounded-lg border-2 border-dashed border-border p-3 text-sm font-medium text-muted transition-colors hover:border-indigo-400 hover:bg-hover hover:text-medium"
            >
              <svg
                class="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M12 4v16m8-8H4"
                />
              </svg>
              Create Script
            </a>
          </div>
        {:else}
          <a
            href="/scripts"
            class="flex items-center justify-center gap-2 rounded-lg border-2 border-dashed border-border p-4 text-sm font-medium text-muted transition-colors hover:border-indigo-400 hover:bg-hover hover:text-medium"
          >
            <svg
              class="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M12 4v16m8-8H4"
              />
            </svg>
            Create Custom Script
          </a>
        {/if}
      </div>

      {#if selectedScriptId}
        <!-- Step 2: Player count -->
        <div class="space-y-3">
          <h3 class="text-base font-medium text-secondary">
            2. How many players?
          </h3>
          <div class="flex flex-wrap gap-2">
            {#each Array.from({ length: 11 }, (_, i) => i + 5) as n}
              <button
                onclick={() => (playerCount = n)}
                class="h-10 w-10 rounded-lg text-sm font-medium transition-colors {playerCount ===
                n
                  ? 'bg-indigo-500 text-white'
                  : 'border border-border bg-surface text-medium hover:bg-hover'}"
              >
                {n}
              </button>
            {/each}
          </div>

          {#if currentDist}
            <div class="rounded-lg border border-border bg-surface p-4">
              <p class="mb-2 text-sm text-secondary">
                Expected distribution for {playerCount} players:
              </p>
              <DistributionBar
                current={{
                  townsfolk: currentDist.townsfolk,
                  outsiders: currentDist.outsiders,
                  minions: currentDist.minions,
                  demons: currentDist.demons,
                }}
                travellers={travellerCount}
              />
            </div>
          {/if}
        </div>

        <!-- Step 3: Travellers -->
        <div class="space-y-3">
          <h3 class="text-base font-medium text-secondary">3. Travellers</h3>
          <div class="flex items-center gap-3">
            <button
              onclick={() => (travellerCount = Math.max(0, travellerCount - 1))}
              disabled={travellerCount <= 0}
              class="flex h-10 w-10 items-center justify-center rounded-lg border border-border bg-surface text-lg font-medium text-medium transition-colors hover:bg-hover disabled:opacity-30"
            >
              -
            </button>
            <span class="w-8 text-center text-lg font-medium text-primary"
              >{travellerCount}</span
            >
            <button
              onclick={() => (travellerCount = travellerCount + 1)}
              disabled={totalPeople >= 25}
              class="flex h-10 w-10 items-center justify-center rounded-lg border border-border bg-surface text-lg font-medium text-medium transition-colors hover:bg-hover disabled:opacity-30"
            >
              +
            </button>
            <span class="text-sm text-secondary">
              Total: {totalPeople}
              {totalPeople === 1 ? "person" : "people"}
            </span>
          </div>

          {#if showTotalWarning}
            <div
              class="rounded-lg border border-warning-border bg-warning-bg px-4 py-2 text-sm text-warning-text"
            >
              The recommended maximum is 20 players. Games with more players may
              be harder to manage.
            </div>
          {/if}
        </div>

        <!-- Step 4: Players (optional) -->
        <div class="space-y-3">
          <h3 class="text-base font-medium text-secondary">
            4. Players <span class="text-sm font-normal text-muted"
              >(optional)</span
            >
          </h3>
          <p class="text-sm text-muted">
            Assign names to seats in game setup after choosing roles.
          </p>
          <div class="flex flex-wrap items-center gap-2">
            {#if presetNames.length === 0}
              <span class="text-sm text-muted">No saved names yet.</span>
            {:else}
              {#each presetNames as name (name)}
                <span
                  class="rounded-full border border-border bg-surface px-2.5 py-0.5 text-xs font-medium text-primary"
                >
                  {name}
                </span>
              {/each}
            {/if}
            <button
              onclick={() => (showPresets = true)}
              class="rounded-full border border-dashed border-border px-2.5 py-0.5 text-xs text-muted transition-colors hover:border-indigo-400 hover:text-indigo-500"
            >
              Edit names
            </button>
          </div>
        </div>

        <!-- Step 5: Create -->
        <div>
          <button
            onclick={createGame}
            disabled={creating || !selectedScriptId}
            class="btn-primary rounded-lg bg-indigo-500 px-6 py-2.5 font-medium text-white transition-colors hover:bg-indigo-400 disabled:opacity-50"
          >
            {creating ? "Creating..." : "Create Game"}
          </button>
        </div>
      {/if}
    {/if}
  </section>
</div>

{#if showPresets}
  <PlayerPresetsModal
    onclose={() => {
      showPresets = false;
      reloadPresets();
    }}
  />
{/if}
