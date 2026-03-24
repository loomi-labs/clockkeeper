<script lang="ts">
  import {
    theme,
    setTheme,
    applyPreview,
    revertPreview,
    type Theme,
  } from "~/lib/theme";

  let current = $state<Theme>("light");
  theme.subscribe((v) => (current = v));

  let open = $state(false);

  const themes: { id: Theme; label: string; icon: string }[] = [
    { id: "light", label: "Light", icon: "sun" },
    { id: "dark", label: "Dark", icon: "moon" },
    { id: "tavern", label: "Tavern", icon: "goblet" },
    { id: "crypt", label: "Crypt", icon: "potion" },
  ];

  function select(id: Theme, event: MouseEvent) {
    setTheme(id, event);
    open = false;
  }

  function handleDropdownLeave() {
    revertPreview();
  }

  function handleClickOutside(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (!target.closest(".theme-switcher")) {
      open = false;
    }
  }

  $effect(() => {
    if (open) {
      document.addEventListener("click", handleClickOutside, true);
      return () =>
        document.removeEventListener("click", handleClickOutside, true);
    }
  });
</script>

{#snippet sunIcon(size: string)}
  <svg
    class={size}
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    stroke-width="1.5"
  >
    <path
      stroke-linecap="round"
      stroke-linejoin="round"
      d="M12 3v2.25m6.364.386l-1.591 1.591M21 12h-2.25m-.386 6.364l-1.591-1.591M12 18.75V21m-4.773-4.227l-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0z"
    />
  </svg>
{/snippet}

{#snippet moonIcon(size: string)}
  <svg
    class={size}
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    stroke-width="1.5"
  >
    <path
      stroke-linecap="round"
      stroke-linejoin="round"
      d="M21.752 15.002A9.718 9.718 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z"
    />
  </svg>
{/snippet}

{#snippet gobletIcon(size: string)}
  <!-- Wine goblet / chalice -->
  <svg
    class={size}
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    stroke-width="1.5"
  >
    <path
      stroke-linecap="round"
      stroke-linejoin="round"
      d="M8 3h8v5a4 4 0 01-8 0V3z"
    />
    <line x1="12" y1="12" x2="12" y2="18" stroke-linecap="round" />
    <path stroke-linecap="round" stroke-linejoin="round" d="M8 21h8" />
    <path stroke-linecap="round" stroke-linejoin="round" d="M9 18h6" />
  </svg>
{/snippet}

{#snippet potionIcon(size: string)}
  <!-- Potion bottle / vial -->
  <svg
    class={size}
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    stroke-width="1.5"
  >
    <path
      stroke-linecap="round"
      stroke-linejoin="round"
      d="M10 2h4v4l3 5v7a2 2 0 01-2 2H9a2 2 0 01-2-2v-7l3-5V2z"
    />
    <line x1="9" y1="2" x2="15" y2="2" stroke-linecap="round" />
    <path
      stroke-linecap="round"
      stroke-linejoin="round"
      d="M7 15h10"
      opacity="0.5"
    />
  </svg>
{/snippet}

{#snippet themeIcon(icon: string, size: string)}
  {#if icon === "sun"}
    {@render sunIcon(size)}
  {:else if icon === "moon"}
    {@render moonIcon(size)}
  {:else if icon === "goblet"}
    {@render gobletIcon(size)}
  {:else}
    {@render potionIcon(size)}
  {/if}
{/snippet}

<div class="theme-switcher relative">
  <button
    onclick={() => (open = !open)}
    title="Change theme"
    aria-label="Change theme"
    class="rounded-lg p-2 text-secondary transition-colors hover:bg-hover hover:text-primary"
  >
    {@render themeIcon(
      themes.find((t) => t.id === current)?.icon ?? "sun",
      "h-5 w-5",
    )}
  </button>

  {#if open}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="absolute right-0 top-full z-50 mt-1 w-48 rounded-lg border border-border bg-surface py-1 shadow-lg"
      onmouseleave={handleDropdownLeave}
    >
      {#each themes as t}
        <button
          onclick={(e) => select(t.id, e)}
          onmouseenter={() => applyPreview(t.id)}
          class="flex w-full items-center gap-3 px-3 py-2 text-sm transition-colors
						{current === t.id
            ? 'bg-hover text-primary font-medium'
            : 'text-secondary hover:bg-hover hover:text-primary'}"
        >
          {@render themeIcon(t.icon, "h-4 w-4")}
          {t.label}
        </button>
      {/each}
    </div>
  {/if}
</div>
