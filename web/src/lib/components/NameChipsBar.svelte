<script lang="ts">
  // Grimoire "Names" chip bar. Extracted from the two near-identical inline
  // blocks that lived in `games/[id]/+page.svelte` (in-progress grimoire and
  // setup grimoire). Tap a chip to enter assign mode, or drag a chip onto a
  // grimoire player token to assign it. Chips already assigned to a seat show
  // an "x" to unassign.
  let {
    presetNames,
    assignedNames,
    onpickname,
    ondragname,
    onunassignname,
    onmanagepresets,
    onclose,
    selectedName = null,
  }: {
    presetNames: string[];
    // Names currently assigned to some seat (preset names only — free-text
    // names typed directly into a token aren't tracked here).
    assignedNames: ReadonlySet<string>;
    // Tap a chip (assign mode toggle).
    onpickname: (name: string) => void;
    // Optional hook fired on drag start; the component always populates the
    // dataTransfer itself (see below) so drop targets keep working even when
    // this is omitted.
    ondragname?: (name: string, ev: DragEvent) => void;
    // Click the "x" on an assigned chip.
    onunassignname: (name: string) => void;
    // Omitted when the name list is not the Storyteller's to edit (e.g. it comes
    // from Token Bag registration) — the "Edit" chip then disappears.
    onmanagepresets?: () => void;
    onclose: () => void;
    // Currently selected chip (assign mode). Optional — when provided the chip
    // is highlighted and a hint is shown, matching the original behavior.
    selectedName?: string | null;
  } = $props();

  function handleDragStart(name: string, e: DragEvent) {
    // Preserve the exact dataTransfer contract the grimoire drop target reads
    // (GrimoirePlayerToken.ondrop -> getData("text/plain")).
    e.dataTransfer?.setData("text/plain", name);
    if (e.dataTransfer) e.dataTransfer.effectAllowed = "copy";
    ondragname?.(name, e);
  }
</script>

<div class="no-print rounded-lg border border-border bg-surface px-3 py-2">
  <div class="flex items-center gap-2 flex-wrap">
    <span class="text-xs font-medium text-muted shrink-0">Names:</span>
    {#if presetNames.length === 0}
      <span class="text-xs text-muted">No presets saved.</span>
    {:else}
      {#each presetNames as name (name)}
        {@const isUsed = assignedNames.has(name)}
        {@const isSelected = selectedName === name}
        {#if isUsed}
          <span
            class="inline-flex items-center gap-1 rounded-full border border-border pl-2.5 pr-1 py-0.5 text-xs font-medium text-muted"
          >
            <button
              class="line-through opacity-70 hover:opacity-100 transition-opacity cursor-grab"
              draggable={true}
              ondragstart={(e) => handleDragStart(name, e)}
              onclick={() => onpickname(name)}
            >
              {name}
            </button>
            <button
              onclick={() => onunassignname(name)}
              class="rounded-full p-0.5 text-muted transition-colors hover:bg-hover hover:text-red-500"
              aria-label="Unassign {name}"
              title="Unassign {name}"
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
            class="rounded-full border px-2.5 py-0.5 text-xs font-medium transition-colors {isSelected
              ? 'border-indigo-500 bg-indigo-500 text-white'
              : 'border-border text-primary hover:border-indigo-400 hover:text-indigo-500 cursor-grab'}"
            draggable={true}
            ondragstart={(e) => handleDragStart(name, e)}
            onclick={() => onpickname(name)}
          >
            {name}
          </button>
        {/if}
      {/each}
    {/if}
    {#if onmanagepresets}
      <button
        onclick={onmanagepresets}
        class="rounded-full border border-dashed border-border px-2 py-0.5 text-xs text-muted transition-colors hover:border-indigo-400 hover:text-indigo-500"
        title="Edit player presets"
      >
        <svg
          class="inline h-3 w-3"
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
        Edit
      </button>
    {/if}
    {#if selectedName}
      <span class="text-xs text-indigo-500 ml-auto"
        >Tap a player to assign "{selectedName}"</span
      >
    {/if}
    <button
      onclick={onclose}
      class="{selectedName
        ? ''
        : 'ml-auto'} rounded-full p-1 text-muted transition-colors hover:bg-hover hover:text-primary"
      aria-label="Close names bar"
      title="Close"
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
  </div>
</div>
