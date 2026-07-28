<script lang="ts">
  // One character, shown to the person it belongs to. Big enough to read at a
  // glance on a phone held under the table, with no controls of its own — the
  // page around it owns "hide", "done" and the countdown.
  import type { Character } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { iconSuffix, teamNameColors, teamSingulars } from "~/lib/team-styles";

  let { character }: { character: Character } = $props();

  const iconUrl = $derived(
    `/characters/${character.edition}/${character.id}${iconSuffix(character.team)}.webp`,
  );
</script>

<div class="flex flex-col items-center gap-4 text-center">
  <img
    src={iconUrl}
    alt=""
    class="h-40 w-40 shrink-0 rounded-full sm:h-48 sm:w-48"
    onerror={(e: Event) =>
      ((e.target as HTMLImageElement).style.display = "none")}
  />

  <div>
    <h2
      class="text-3xl font-bold {teamNameColors[character.team] ??
        'text-primary'}"
    >
      {character.name}
    </h2>
    {#if teamSingulars[character.team]}
      <p
        class="mt-1 text-xs font-medium uppercase tracking-wide text-secondary"
      >
        {teamSingulars[character.team]}
      </p>
    {/if}
  </div>

  <p class="max-w-md text-lg leading-relaxed text-primary">
    {character.ability}
  </p>
</div>
