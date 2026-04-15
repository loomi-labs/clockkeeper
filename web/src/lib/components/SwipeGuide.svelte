<script lang="ts">
  let {
    step,
    ondismiss,
  }: {
    step: number;
    ondismiss: () => void;
  } = $props();

  let showConfirm = $state(false);

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      if (showConfirm) {
        showConfirm = false;
      } else {
        showConfirm = true;
      }
    }
  }

  const hints: { left: string; right: string }[] = [
    { left: "Swipe right to mark done", right: "" },
    { left: "Swipe right again to undo", right: "" },
    { left: "", right: "Swipe left to kill" },
    { left: "", right: "Swipe left again to undo" },
  ];

  const hint = $derived(hints[step] ?? hints[0]);
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Dark backdrop covering everything -->
<div class="fixed inset-0 z-40 bg-black/60"></div>

<!-- Hint labels below the highlighted card -->
<div class="relative z-50 mt-2 flex items-center justify-between px-1">
  <!-- Left hint (swipe right labels) -->
  <div class="flex items-center gap-1.5">
    {#if hint.left}
      <span class="text-sm font-medium text-green-400">{hint.left}</span>
      <svg
        class="h-5 w-5 text-green-400"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="2"
        ><path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M14 5l7 7m0 0l-7 7m7-7H3"
        /></svg
      >
    {/if}
  </div>

  <!-- Dismiss X -->
  <button
    type="button"
    onclick={() => (showConfirm = true)}
    class="flex h-8 w-8 items-center justify-center rounded-full border border-neutral-500 bg-neutral-800/80 text-neutral-300 transition-colors hover:border-neutral-400 hover:text-white"
    aria-label="Skip guide"
    title="Skip guide"
  >
    <svg
      class="h-4 w-4"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="2.5"
      ><path
        stroke-linecap="round"
        stroke-linejoin="round"
        d="M6 18L18 6M6 6l12 12"
      /></svg
    >
  </button>

  <!-- Right hint (swipe left labels) -->
  <div class="flex items-center gap-1.5">
    {#if hint.right}
      <svg
        class="h-5 w-5 text-red-400"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="2"
        ><path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M10 19l-7-7m0 0l7-7m-7 7h18"
        /></svg
      >
      <span class="text-sm font-medium text-red-400">{hint.right}</span>
    {/if}
  </div>
</div>

<!-- Confirmation dialog -->
{#if showConfirm}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center"
    onkeydown={(e) => {
      if (e.key === "Escape") showConfirm = false;
    }}
  >
    <div class="absolute inset-0 bg-black/40"></div>
    <div
      role="dialog"
      aria-modal="true"
      class="relative z-10 w-full max-w-xs rounded-xl border border-border bg-surface p-5 shadow-xl"
    >
      <h3 class="text-base font-semibold text-primary">Skip guide?</h3>
      <p class="mt-1.5 text-sm text-secondary">
        The guide won't be shown again.
      </p>
      <div class="mt-4 flex gap-3 justify-end">
        <button
          type="button"
          onclick={() => (showConfirm = false)}
          class="rounded-lg border border-border px-4 py-2 text-sm font-medium text-secondary transition-colors hover:bg-hover hover:text-medium"
        >
          Continue
        </button>
        <button
          type="button"
          onclick={ondismiss}
          class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-500"
        >
          Skip
        </button>
      </div>
    </div>
  </div>
{/if}
