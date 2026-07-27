<script lang="ts">
  import type { Snippet } from "svelte";

  // Kebab ("⋮") trigger + right-aligned dropdown for secondary header actions.
  // The `children` snippet receives a `close` callback so each menu item can
  // dismiss the panel before running its action.
  let {
    label = "More actions",
    children,
  }: {
    label?: string;
    children: Snippet<[() => void]>;
  } = $props();

  let open = $state(false);

  function close() {
    open = false;
  }

  // Close the open menu on outside click / Escape.
  $effect(() => {
    if (!open) return;
    const onDocClick = (e: MouseEvent) => {
      const target = e.target as HTMLElement | null;
      if (target && target.closest("[data-overflow-menu]")) return;
      close();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") close();
    };
    document.addEventListener("click", onDocClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("click", onDocClick);
      document.removeEventListener("keydown", onKey);
    };
  });
</script>

<div class="relative" data-overflow-menu>
  <button
    onclick={() => (open = !open)}
    aria-haspopup="menu"
    aria-expanded={open}
    aria-label={label}
    title={label}
    class="rounded-lg border border-border px-3 py-2.5 text-sm font-medium text-secondary transition-colors hover:bg-hover hover:text-primary"
  >
    <svg
      class="h-4 w-4"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="2"
      ><path
        stroke-linecap="round"
        stroke-linejoin="round"
        d="M12 5h.01M12 12h.01M12 19h.01"
      /></svg
    >
  </button>

  {#if open}
    <div
      class="absolute right-0 top-full z-20 mt-1 w-48 rounded-lg border border-border bg-surface p-1 shadow-lg"
      role="menu"
    >
      {@render children(close)}
    </div>
  {/if}
</div>
