<script lang="ts">
  import type { RoleDistribution } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";

  let {
    current,
    expected,
    travellers = 0,
    bagExtras = [],
  }: {
    current: {
      townsfolk: number;
      outsiders: number;
      minions: number;
      demons: number;
    };
    expected?: RoleDistribution | undefined;
    travellers?: number;
    bagExtras?: {
      causedByName: string;
      characterName: string;
      picked: boolean;
    }[];
  } = $props();

  const baseTeams = [
    {
      key: "townsfolk" as const,
      label: "Townsfolk",
      team: "townsfolk",
      color: "bg-blue-600",
    },
    {
      key: "outsiders" as const,
      label: "Outsiders",
      team: "outsider",
      color: "bg-cyan-600",
    },
    {
      key: "minions" as const,
      label: "Minions",
      team: "minion",
      color: "bg-orange-600",
    },
    {
      key: "demons" as const,
      label: "Demons",
      team: "demon",
      color: "bg-red-600",
    },
  ];

  const total = $derived(
    current.townsfolk +
      current.outsiders +
      current.minions +
      current.demons +
      travellers,
  );
</script>

<div class="space-y-2">
  {#if total > 0}
    <div class="flex gap-0.5 h-3 overflow-hidden rounded-full bg-element">
      {#each baseTeams as team}
        {@const count = current[team.key]}
        {#if count > 0}
          <div
            class="card-slate {team.color} transition-all duration-300"
            data-team={team.team}
            style="width: {(count / total) * 100}%"
          ></div>
        {/if}
      {/each}
      {#if travellers > 0}
        <div
          class="card-slate bg-gradient-to-r from-blue-600 to-red-600 transition-all duration-300"
          data-team="traveller"
          style="width: {(travellers / total) * 100}%"
        ></div>
      {/if}
    </div>
  {/if}
  <div class="flex flex-wrap gap-4 text-sm">
    {#each baseTeams as team}
      {@const count = current[team.key]}
      {@const exp = expected ? expected[team.key] : undefined}
      <div class="flex items-center gap-1.5">
        <div
          class="card-slate h-2.5 w-2.5 rounded-full {team.color} overflow-hidden"
          data-team={team.team}
        ></div>
        <span class="text-secondary">{team.label}</span>
        <span
          class={exp !== undefined && count !== exp
            ? "font-medium text-red-600"
            : "text-medium"}
        >
          {count}{exp !== undefined ? `/${exp}` : ""}
        </span>
      </div>
    {/each}
    {#if travellers > 0}
      <div class="flex items-center gap-1.5">
        <div
          class="card-slate h-2.5 w-2.5 rounded-full bg-gradient-to-r from-blue-600 to-red-600 overflow-hidden"
          data-team="traveller"
        ></div>
        <span class="text-secondary">Travellers</span>
        <span class="text-medium">{travellers}</span>
      </div>
    {/if}
    <div class="flex items-center gap-1.5 ml-auto">
      <span class="text-muted">Total</span>
      <span class="font-medium text-medium">{total}</span>
    </div>
  </div>
  {#if bagExtras.length > 0}
    <div class="flex flex-wrap gap-3 text-xs">
      {#each bagExtras as extra}
        <div
          class="flex items-center gap-1.5 rounded-full border px-2.5 py-1 {extra.picked
            ? 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-300'
            : 'border-yellow-300 bg-yellow-50 text-yellow-700 dark:border-yellow-700 dark:bg-yellow-950/30 dark:text-yellow-300'}"
        >
          <span class="font-medium">+1 Townsfolk</span>
          <span class="opacity-70"
            >for bag ({extra.causedByName}{extra.picked
              ? ` → ${extra.characterName}`
              : ""})</span
          >
        </div>
      {/each}
    </div>
  {/if}
</div>
