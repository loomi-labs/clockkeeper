<script lang="ts">
  import { untrack } from "svelte";
  import { client } from "~/lib/api";
  import { getErrorMessage } from "~/lib/errors";
  import { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import type {
    SetupStep,
    Character,
  } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { iconSuffix } from "~/lib/team-styles";

  let {
    gameId,
    selectedIds = [],
    characterById = new Map(),
    onstartgame,
    canStartGame = false,
  }: {
    gameId: bigint;
    selectedIds?: string[];
    characterById?: Map<string, Character>;
    onstartgame?: () => void;
    canStartGame?: boolean;
  } = $props();

  let steps = $state<SetupStep[]>([]);
  let completed = $state<Set<string>>(new Set());
  let loading = $state(true);
  let error = $state("");
  let bottomBarOpen = $state(false);
  let fetchVersion = 0;

  // Focus mode state
  let focusOpen = $state(false);
  let focusIndex = $state(0);

  const progress = $derived(
    steps.length > 0 ? Math.round((completed.size / steps.length) * 100) : 0,
  );
  const allDone = $derived(completed.size === steps.length && steps.length > 0);
  const hasSteps = $derived(steps.length > 0);
  const focusStep = $derived(steps[focusIndex]);

  async function fetchSteps() {
    const thisVersion = ++fetchVersion;
    loading = true;
    error = "";
    try {
      const resp = await client.getSetupChecklist({ gameId });
      if (thisVersion !== fetchVersion) return;
      const nextSteps = resp.steps;
      const validIds = new Set(nextSteps.map((s) => s.id));
      completed = new Set([...completed].filter((id) => validIds.has(id)));
      steps = nextSteps;
    } catch (err) {
      if (thisVersion !== fetchVersion) return;
      error = getErrorMessage(err, "Failed to load checklist");
    } finally {
      if (thisVersion === fetchVersion) loading = false;
    }
  }

  $effect(() => {
    void gameId;
    void selectedIds;
    untrack(() => fetchSteps());
  });

  function toggleStep(id: string) {
    const updated = new Set(completed);
    if (updated.has(id)) {
      updated.delete(id);
    } else {
      updated.add(id);
    }
    completed = updated;
  }

  function openFocus(index?: number) {
    focusIndex = index ?? 0;
    focusOpen = true;
  }

  function closeFocus() {
    focusOpen = false;
  }

  function handleFocusKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") closeFocus();
    if (e.key === "ArrowRight" && focusIndex < steps.length - 1) focusIndex++;
    if (e.key === "ArrowLeft" && focusIndex > 0) focusIndex--;
  }

  function focusCheck() {
    if (!focusStep) return;
    const wasCompleted = completed.has(focusStep.id);
    toggleStep(focusStep.id);
    // Auto-advance to next step on check (not uncheck)
    if (!wasCompleted && focusIndex < steps.length - 1) {
      setTimeout(() => {
        focusIndex++;
      }, 300);
    }
  }

  // Resolve token image URL for a character ID.
  function tokenUrl(charId: string, edition: string): string {
    const char = characterById.get(charId);
    if (char) {
      const suffix = iconSuffix(char.team);
      return `/characters/${char.edition}/${char.id}${suffix}.webp`;
    }
    // Fallback with edition from step data
    if (edition) {
      return `/characters/${edition}/${charId}_g.webp`;
    }
    return "";
  }
</script>

{#snippet compactStepList()}
  {#if loading}
    <p class="p-4 text-sm text-secondary">Loading...</p>
  {:else if error}
    <div
      class="m-4 rounded-lg bg-error-bg border border-error-border px-4 py-2 text-sm text-error-text"
    >
      {error}
    </div>
  {:else if !hasSteps}
    <div class="p-4">
      <p class="text-sm text-muted">Select roles to see setup steps.</p>
    </div>
  {:else}
    <div class="divide-y divide-border">
      {#each steps as step, i (step.id)}
        {@const done = completed.has(step.id)}
        <button
          onclick={() => toggleStep(step.id)}
          class="w-full text-left transition-colors hover:bg-hover {done
            ? 'bg-success-bg/50'
            : ''}"
        >
          <div class="flex items-center gap-3 px-4 py-2.5">
            <div
              class="flex h-5 w-5 shrink-0 items-center justify-center rounded border transition-colors {done
                ? 'border-green-500 bg-green-500'
                : 'border-border-strong'}"
            >
              {#if done}
                <svg
                  class="h-3 w-3 text-white"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="3"
                    d="M5 13l4 4L19 7"
                  />
                </svg>
              {/if}
            </div>
            <span
              class="min-w-0 flex-1 truncate text-sm {done
                ? 'text-muted line-through'
                : 'text-primary'}"
            >
              {step.title}
            </span>
            <span class="shrink-0 text-xs text-muted">{i + 1}</span>
          </div>
        </button>
      {/each}
    </div>

    {#if allDone}
      <div class="border-t border-border p-4 space-y-3">
        <div
          class="rounded-lg border border-success-border bg-success-bg px-4 py-3 text-center"
        >
          <p class="text-sm font-medium text-success-text">Setup complete!</p>
        </div>
        {#if onstartgame && canStartGame}
          <button
            onclick={onstartgame}
            class="w-full rounded-lg bg-green-600 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-green-500"
          >
            Start Game
          </button>
        {/if}
      </div>
    {/if}
  {/if}
{/snippet}

<!-- Wide screens: right sidebar in the gutter -->
<div
  class="card-slate fixed top-[57px] right-0 bottom-0 hidden w-72 flex-col border-l border-border bg-surface 2xl:flex"
>
  <div class="flex items-center gap-3 border-b border-border px-4 py-3">
    <h2 class="text-lg font-semibold text-primary">Setup</h2>
    {#if hasSteps}
      <span class="text-sm text-secondary">{completed.size}/{steps.length}</span
      >
      <div class="flex-1"></div>
      <button
        onclick={() => openFocus()}
        class="rounded-md border border-border px-2 py-1 text-xs font-medium text-secondary transition-colors hover:bg-hover hover:text-primary"
        title="Step-by-step focus mode"
      >
        Focus
      </button>
    {/if}
  </div>

  {#if hasSteps}
    <div class="h-1.5 bg-element">
      <div
        class="h-full bg-emerald-500 transition-all duration-300"
        style="width: {progress}%"
      ></div>
    </div>
  {/if}

  <div class="flex-1 overflow-y-auto">
    {@render compactStepList()}
  </div>
</div>

<!-- Narrow screens: bottom bar -->
<div
  class="card-slate fixed bottom-0 left-0 right-0 z-20 border-t border-border bg-surface shadow-[0_-2px_8px_rgba(0,0,0,0.1)] 2xl:hidden"
>
  <div class="flex w-full items-center justify-between px-4 py-3">
    <button
      onclick={() => (bottomBarOpen = !bottomBarOpen)}
      class="flex flex-1 items-center gap-3"
    >
      <span class="text-sm font-semibold text-primary">Setup</span>
      {#if hasSteps}
        <span class="text-sm text-secondary"
          >{completed.size}/{steps.length}</span
        >
        <div class="h-1.5 w-24 overflow-hidden rounded-full bg-element">
          <div
            class="h-full bg-emerald-500 transition-all duration-300"
            style="width: {progress}%"
          ></div>
        </div>
      {:else}
        <span class="text-xs text-muted">No roles selected</span>
      {/if}
    </button>
    <div class="flex items-center gap-2">
      {#if hasSteps}
        <button
          onclick={() => openFocus()}
          class="rounded-md border border-border px-2 py-1 text-xs font-medium text-secondary transition-colors hover:bg-hover hover:text-primary"
        >
          Focus
        </button>
      {/if}
      <button
        onclick={() => (bottomBarOpen = !bottomBarOpen)}
        class="p-1"
        aria-label={bottomBarOpen ? "Collapse" : "Expand"}
      >
        <svg
          class="h-4 w-4 text-secondary transition-transform {bottomBarOpen
            ? 'rotate-180'
            : ''}"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M5 15l7-7 7 7"
          />
        </svg>
      </button>
    </div>
  </div>

  {#if bottomBarOpen}
    <div class="max-h-[50vh] overflow-y-auto border-t border-border">
      {@render compactStepList()}
    </div>
  {/if}
</div>

<!-- Focus mode modal -->
{#if focusOpen && focusStep}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
    onclick={closeFocus}
    onkeydown={handleFocusKeydown}
  >
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="mx-4 w-full max-w-lg rounded-xl border border-border bg-surface p-6 shadow-2xl"
      onclick={(e) => e.stopPropagation()}
    >
      <!-- Header -->
      <div class="flex items-center justify-between">
        <span class="text-sm font-medium text-muted"
          >Step {focusIndex + 1} of {steps.length}</span
        >
        <button
          onclick={closeFocus}
          class="rounded-lg p-1 text-muted transition-colors hover:bg-hover hover:text-primary"
          aria-label="Close"
        >
          <svg
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="2"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      </div>

      <!-- Progress bar -->
      <div class="mt-3 h-1.5 overflow-hidden rounded-full bg-element">
        <div
          class="h-full bg-emerald-500 transition-all duration-300"
          style="width: {((focusIndex + 1) / steps.length) * 100}%"
        ></div>
      </div>

      <!-- Step content — entire area is clickable to toggle check -->
      <button
        class="mt-6 w-full text-left"
        onclick={focusCheck}
      >
        <div class="flex items-start gap-3">
          <div
            class="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded border-2 transition-colors {completed.has(
              focusStep.id,
            )
              ? 'border-green-500 bg-green-500'
              : 'border-border-strong'}"
          >
            {#if completed.has(focusStep.id)}
              <svg
                class="h-4 w-4 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="3"
                  d="M5 13l4 4L19 7"
                />
              </svg>
            {/if}
          </div>
          <div class="min-w-0 flex-1">
            <h3
              class="text-lg font-semibold {completed.has(focusStep.id)
                ? 'text-muted line-through'
                : 'text-primary'}"
            >
              {focusStep.title}
            </h3>
          </div>
        </div>

        {#if focusStep.description}
          <p
            class="mt-4 whitespace-pre-line text-sm leading-relaxed text-secondary"
          >
            {focusStep.description}
          </p>
        {/if}

        <!-- Token images -->
        {#if focusStep.characterIds.length > 0}
          <div class="mt-4 flex flex-wrap gap-2">
            {#each focusStep.characterIds as charId, i}
              {@const url = tokenUrl(charId, focusStep.editions[i] ?? "")}
              {#if url}
                <img
                  src={url}
                  alt={characterById.get(charId)?.name ?? charId}
                  class="h-12 w-12 rounded-full"
                  title={characterById.get(charId)?.name ?? charId}
                  onerror={(e: Event) =>
                    ((e.target as HTMLImageElement).style.display = "none")}
                />
              {/if}
            {/each}
          </div>
        {/if}
      </button>

      <!-- Navigation -->
      <div class="mt-8 flex items-center justify-between">
        <button
          onclick={() => focusIndex--}
          disabled={focusIndex <= 0}
          class="flex items-center gap-1 rounded-lg px-3 py-2 text-sm font-medium text-secondary transition-colors hover:bg-hover hover:text-primary disabled:opacity-30"
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
              d="M15 19l-7-7 7-7"
            />
          </svg>
          Previous
        </button>
        {#if focusIndex >= steps.length - 1 && onstartgame && canStartGame}
          <button
            onclick={() => {
              closeFocus();
              onstartgame?.();
            }}
            class="rounded-lg bg-green-600 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-green-500"
          >
            Start Game
          </button>
        {:else}
          <button
            onclick={() => focusIndex++}
            disabled={focusIndex >= steps.length - 1}
            class="flex items-center gap-1 rounded-lg px-3 py-2 text-sm font-medium text-secondary transition-colors hover:bg-hover hover:text-primary disabled:opacity-30"
          >
            Next
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
                d="M9 5l7 7-7 7"
              />
            </svg>
          </button>
        {/if}
      </div>
    </div>
  </div>
{/if}
