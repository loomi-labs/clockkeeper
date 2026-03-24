<script lang="ts">
  let {
    title,
    message,
    confirmLabel = "Confirm",
    cancelLabel = "Cancel",
    onconfirm,
    oncancel,
  }: {
    title: string;
    message: string;
    confirmLabel?: string;
    cancelLabel?: string;
    onconfirm: () => void;
    oncancel: () => void;
  } = $props();

  let cancelBtn: HTMLButtonElement | undefined = $state();

  $effect(() => {
    cancelBtn?.focus();
  });

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") oncancel();
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="fixed inset-0 z-50 flex items-center justify-center"
  onkeydown={handleKeydown}
>
  <!-- Backdrop -->
  <button
    type="button"
    tabindex="-1"
    class="absolute inset-0 bg-black/40"
    onclick={oncancel}
    aria-label="Close"
  ></button>

  <!-- Dialog -->
  <div
    role="dialog"
    aria-modal="true"
    class="relative z-10 w-full max-w-sm rounded-xl border border-border bg-surface p-6 shadow-xl"
  >
    <h3 class="text-lg font-semibold text-primary">{title}</h3>
    <p class="mt-2 text-sm text-secondary">{message}</p>
    <div class="mt-5 flex gap-3 justify-end">
      <button
        type="button"
        bind:this={cancelBtn}
        onclick={oncancel}
        class="rounded-lg border border-border px-4 py-2 text-sm font-medium text-secondary transition-colors hover:bg-hover hover:text-medium"
      >
        {cancelLabel}
      </button>
      <button
        type="button"
        onclick={onconfirm}
        class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-500"
      >
        {confirmLabel}
      </button>
    </div>
  </div>
</div>
