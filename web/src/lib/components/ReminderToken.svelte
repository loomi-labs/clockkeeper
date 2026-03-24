<script lang="ts">
  import { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";

  let {
    characterId,
    characterName,
    text,
    edition,
    team,
  }: {
    characterId: string;
    characterName: string;
    text: string;
    edition: string;
    team: Team;
  } = $props();

  const iconSuffix = $derived(
    team === Team.TOWNSFOLK || team === Team.OUTSIDER
      ? "_g"
      : team === Team.MINION || team === Team.DEMON
        ? "_e"
        : "",
  );
  const iconUrl = $derived(
    `/characters/${edition}/${characterId}${iconSuffix}.webp`,
  );

  const tokenColors: Record<number, string> = {
    [Team.TOWNSFOLK]:
      "border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-950/40",
    [Team.OUTSIDER]:
      "border-cyan-200 bg-cyan-50 dark:border-cyan-800 dark:bg-cyan-950/40",
    [Team.MINION]:
      "border-orange-200 bg-orange-50 dark:border-orange-800 dark:bg-orange-950/40",
    [Team.DEMON]:
      "border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950/40",
    [Team.TRAVELLER]: "card-traveller",
    [Team.FABLED]:
      "border-yellow-300 bg-yellow-50 dark:border-yellow-700 dark:bg-yellow-950/40",
    [Team.LORIC]:
      "border-green-200 bg-green-50 dark:border-green-800 dark:bg-green-950/40",
  };

  const teamDataAttr: Record<number, string> = {
    [Team.TOWNSFOLK]: "townsfolk",
    [Team.OUTSIDER]: "outsider",
    [Team.MINION]: "minion",
    [Team.DEMON]: "demon",
    [Team.TRAVELLER]: "traveller",
    [Team.FABLED]: "fabled",
    [Team.LORIC]: "loric",
  };

  const colorClass = $derived(
    tokenColors[team] ?? "border-border bg-surface-alt",
  );

  let imgError = $state(false);
</script>

<div
  class="card-slate flex h-24 w-24 flex-col items-center justify-center rounded-full border-2 p-1 {colorClass}"
  data-team={teamDataAttr[team] ?? ""}
>
  {#if !imgError && edition}
    <img
      src={iconUrl}
      alt={characterName}
      class="h-14 w-14 shrink-0 rounded-full"
      onerror={() => (imgError = true)}
    />
  {:else}
    <div
      class="flex h-14 w-14 shrink-0 items-center justify-center rounded-full bg-element text-xs text-secondary"
    >
      {characterName.charAt(0)}
    </div>
  {/if}
  <span class="text-center text-[10px] leading-tight text-secondary"
    >{text}</span
  >
</div>
