<script lang="ts">
  import {
    ACCENT_STYLES,
    type DisplayCard,
    type DisplayCharacter,
  } from "~/lib/info-cards";

  let {
    card,
    character = null,
    onclose,
  }: {
    card: DisplayCard;
    /** Show-time pick for `needsCharacterPick` cards. */
    character?: DisplayCharacter | null;
    onclose: () => void;
  } = $props();

  const style = $derived(ACCENT_STYLES[card.accent]);

  // Icons come either baked into the card (bluffs) or from the show-time pick.
  const icons = $derived<DisplayCharacter[]>([
    ...card.characters,
    ...(character ? [character] : []),
  ]);

  // When the card has no title/body the character token IS the card — render
  // the icon extra large (e.g. the ad-hoc "Character token" card).
  const iconOnly = $derived(!card.title && !card.body);

  // Close on Escape from anywhere (tap/click is handled on the overlay itself).
  $effect(() => {
    function onKeydown(e: KeyboardEvent) {
      if (e.key === "Escape") onclose();
    }
    window.addEventListener("keydown", onKeydown);
    return () => window.removeEventListener("keydown", onKeydown);
  });

  function hideBrokenIcon(e: Event) {
    (e.target as HTMLImageElement).style.display = "none";
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
  class="fixed inset-0 z-[60] flex items-center justify-center overflow-hidden select-none"
  style="background: {style.background}; color: {style.text};"
  onclick={onclose}
  role="dialog"
  aria-modal="true"
  aria-label={card.title}
  tabindex="-1"
>
  <!-- Decorative double border with corner ornaments -->
  <div
    class="pointer-events-none absolute inset-4 sm:inset-8"
    style="border: 2px solid {style.border}; outline: 2px solid {style.border}; outline-offset: 6px; opacity: 0.75;"
  >
    {#each ["-top-2 -left-2", "-top-2 -right-2", "-bottom-2 -left-2", "-bottom-2 -right-2"] as pos (pos)}
      <span
        class="absolute h-3 w-3 rotate-45 {pos}"
        style="background: {style.border};"
      ></span>
    {/each}
  </div>

  <!-- Card content -->
  <div
    class="relative z-10 flex max-h-full w-full max-w-3xl flex-col items-center gap-6 overflow-y-auto px-8 py-16 text-center sm:gap-8"
  >
    {#if card.title}
      <h1
        class="font-serif text-4xl font-bold tracking-wide uppercase sm:text-6xl"
        style="text-shadow: 0 2px 6px rgba(0,0,0,0.45);"
      >
        {card.title}
      </h1>
    {/if}

    {#if card.body}
      <p class="font-serif text-2xl leading-relaxed" style="opacity: 0.95;">
        {card.body}
      </p>
    {/if}

    {#if icons.length > 0}
      <div class="flex flex-wrap items-start justify-center gap-6 sm:gap-10">
        {#each icons as icon (icon.id)}
          <div
            class="flex flex-col items-center gap-2 {iconOnly
              ? 'w-56 sm:w-72'
              : 'w-32 sm:w-44'}"
          >
            <img
              src="/characters/{icon.edition}/{icon.id}{icon.iconSuffix}.webp"
              alt=""
              class="rounded-full object-contain {iconOnly
                ? 'h-48 w-48 sm:h-64 sm:w-64'
                : 'h-32 w-32 sm:h-44 sm:w-44'}"
              style="filter: drop-shadow(0 3px 8px rgba(0,0,0,0.5));"
              onerror={hideBrokenIcon}
            />
            <span
              class="font-serif font-semibold tracking-wide uppercase {iconOnly
                ? 'text-2xl sm:text-4xl'
                : 'text-xl sm:text-2xl'}"
              style="text-shadow: 0 1px 4px rgba(0,0,0,0.45);"
            >
              {icon.name}
            </span>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- Dismiss hint -->
  <span
    class="pointer-events-none absolute right-6 bottom-6 text-sm tracking-wide uppercase"
    style="opacity: 0.5;"
  >
    tap to close
  </span>
</div>
