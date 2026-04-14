<script lang="ts">
  import { client } from "~/lib/api";
  import { getErrorMessage } from "~/lib/errors";

  let {
    onclose,
    onassign,
  }: {
    onclose: () => void;
    onassign?: (names: string[]) => void;
  } = $props();

  let names = $state<string[]>([]);
  let loading = $state(true);
  let saving = $state(false);
  let error = $state("");
  let newName = $state("");

  async function load() {
    loading = true;
    error = "";
    try {
      const resp = await client.getPlayerPresets({});
      names = [...resp.names];
    } catch (err) {
      error = getErrorMessage(err, "Failed to load player presets");
    } finally {
      loading = false;
    }
  }

  let saveChain = Promise.resolve();

  async function save() {
    saveChain = saveChain.then(async () => {
      saving = true;
      error = "";
      try {
        const resp = await client.updatePlayerPresets({ names });
        names = [...resp.names];
      } catch (err) {
        error = getErrorMessage(err, "Failed to save player presets");
      } finally {
        saving = false;
      }
    });
    return saveChain;
  }

  function addName() {
    const trimmed = newName.trim();
    if (!trimmed || names.includes(trimmed)) return;
    names = [...names, trimmed];
    newName = "";
    save();
  }

  function removeName(index: number) {
    names = names.filter((_, i) => i !== index);
    save();
  }

  function moveName(from: number, to: number) {
    if (to < 0 || to >= names.length) return;
    const updated = [...names];
    const [item] = updated.splice(from, 1);
    updated.splice(to, 0, item);
    names = updated;
    save();
  }

  function handleAssign() {
    onassign?.(names);
    onclose();
  }

  load();
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
  onclick={onclose}
  onkeydown={(e) => e.key === "Escape" && onclose()}
>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="mx-4 w-full max-w-md rounded-xl border border-border bg-surface p-6 shadow-2xl"
    onclick={(e) => e.stopPropagation()}
  >
    <div class="flex items-center justify-between">
      <h2 class="text-lg font-semibold text-primary">Player Names</h2>
      <button
        onclick={onclose}
        class="rounded-lg p-1 text-muted transition-colors hover:bg-hover hover:text-primary"
        aria-label="Close"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    {#if error}
      <div class="mt-3 rounded-lg bg-error-bg border border-error-border px-3 py-2 text-sm text-error-text">
        {error}
      </div>
    {/if}

    {#if loading}
      <p class="mt-4 text-sm text-secondary">Loading...</p>
    {:else}
      <div class="mt-4 space-y-1 max-h-60 overflow-y-auto">
        {#each names as name, i (i)}
          <div class="flex items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-hover group">
            <span class="w-5 text-xs text-muted text-center">{i + 1}</span>
            <span class="flex-1 text-sm text-primary">{name}</span>
            <button
              onclick={() => moveName(i, i - 1)}
              disabled={i === 0}
              class="rounded p-0.5 text-muted opacity-100 transition-opacity hover:text-primary focus-visible:opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:focus-visible:opacity-100 disabled:opacity-40"
              aria-label="Move up"
            >
              <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M5 15l7-7 7 7" /></svg>
            </button>
            <button
              onclick={() => moveName(i, i + 1)}
              disabled={i === names.length - 1}
              class="rounded p-0.5 text-muted opacity-100 transition-opacity hover:text-primary focus-visible:opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:focus-visible:opacity-100 disabled:opacity-40"
              aria-label="Move down"
            >
              <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" /></svg>
            </button>
            <button
              onclick={() => removeName(i)}
              class="rounded p-0.5 text-muted opacity-100 transition-opacity hover:text-red-500 focus-visible:opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:focus-visible:opacity-100"
              aria-label="Remove {name}"
            >
              <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
          </div>
        {/each}
        {#if names.length === 0}
          <p class="py-4 text-center text-sm text-muted">No player names saved yet.</p>
        {/if}
      </div>

      <form
        class="mt-3 flex gap-2"
        onsubmit={(e) => { e.preventDefault(); addName(); }}
      >
        <input
          type="text"
          bind:value={newName}
          placeholder="Add player name..."
          class="flex-1 rounded-lg border border-border bg-transparent px-3 py-2 text-sm text-primary outline-none focus:border-indigo-500"
        />
        <button
          type="submit"
          disabled={!newName.trim() || saving}
          class="rounded-lg bg-indigo-500 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-400 disabled:opacity-50"
        >
          Add
        </button>
      </form>

      {#if onassign && names.length > 0}
        <button
          onclick={handleAssign}
          class="mt-4 w-full rounded-lg border border-indigo-500 px-4 py-2 text-sm font-medium text-indigo-500 transition-colors hover:bg-indigo-500 hover:text-white"
        >
          Assign to Players in Order
        </button>
      {/if}
    {/if}
  </div>
</div>
