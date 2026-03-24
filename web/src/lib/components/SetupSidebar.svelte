<script lang="ts">
  import { untrack } from "svelte";
  import { client } from "~/lib/api";
  import { getErrorMessage } from "~/lib/errors";
  import type { SetupStep } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";

  let {
    gameId,
    selectedIds = [],
  }: {
    gameId: bigint;
    selectedIds?: string[];
  } = $props();

  let steps = $state<SetupStep[]>([]);
  let completed = $state<Set<string>>(new Set());
  let loading = $state(true);
  let error = $state("");
  let bottomBarOpen = $state(false);
  let fetchVersion = 0;

  const progress = $derived(
    steps.length > 0 ? Math.round((completed.size / steps.length) * 100) : 0,
  );
  const allDone = $derived(completed.size === steps.length && steps.length > 0);
  const hasSteps = $derived(steps.length > 0);

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
</script>

{#snippet stepList()}
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
          <div class="flex items-start gap-3 px-4 py-3">
            <div
              class="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded border transition-colors {done
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
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span
                  class="text-sm {done
                    ? 'text-muted line-through'
                    : 'font-medium text-primary'}"
                >
                  {step.title}
                </span>
                {#if step.requiresAction}
                  <span
                    class="rounded bg-info-bg px-1.5 py-0.5 text-[10px] text-info-text"
                    >action</span
                  >
                {/if}
              </div>
              {#if step.description}
                <p
                  class="mt-1.5 whitespace-pre-line text-xs leading-relaxed {done
                    ? 'text-muted'
                    : 'text-secondary'}"
                >
                  {step.description}
                </p>
              {/if}
            </div>
            <span class="mt-0.5 text-xs text-muted">{i + 1}</span>
          </div>
        </button>
      {/each}
    </div>

    {#if allDone}
      <div class="border-t border-border p-4">
        <div
          class="rounded-lg border border-success-border bg-success-bg px-4 py-3 text-center"
        >
          <p class="text-sm font-medium text-success-text">Setup complete!</p>
        </div>
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
    {@render stepList()}
  </div>
</div>

<!-- Narrow screens: bottom bar -->
<div
  class="card-slate fixed bottom-0 left-0 right-0 border-t border-border bg-surface shadow-[0_-2px_8px_rgba(0,0,0,0.1)] 2xl:hidden"
>
  <button
    onclick={() => (bottomBarOpen = !bottomBarOpen)}
    class="flex w-full items-center justify-between px-4 py-3"
  >
    <div class="flex items-center gap-3">
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
    </div>
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

  {#if bottomBarOpen}
    <div class="max-h-[50vh] overflow-y-auto border-t border-border">
      {@render stepList()}
    </div>
  {/if}
</div>
