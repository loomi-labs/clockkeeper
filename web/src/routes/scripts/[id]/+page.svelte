<script lang="ts">
  import { untrack } from "svelte";
  import { goto } from "$app/navigation";
  import { page } from "$app/state";
  import { client } from "~/lib/api";
  import { getErrorMessage } from "~/lib/errors";
  import type {
    Script,
    Character,
  } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import TeamSection from "~/lib/components/TeamSection.svelte";
  import CharacterPickerModal from "~/lib/components/CharacterPickerModal.svelte";

  let script = $state<Script | undefined>();
  let name = $state("");
  let editingName = $state(false);
  let loading = $state(true);
  let error = $state("");
  let showAddCharacter = $state(false);
  let pickerTeam = $state<Team | undefined>();
  let allCharacters = $state.raw<Character[]>([]);
  let lastSavedName = "";
  let lastSavedIds: string[] = [];

  const teamOrder = [
    Team.TOWNSFOLK,
    Team.OUTSIDER,
    Team.MINION,
    Team.DEMON,
  ] as const;
  const optionalTeamOrder = [Team.TRAVELLER, Team.FABLED, Team.LORIC] as const;

  const optionalTeamLabels: Record<number, string> = {
    [Team.TRAVELLER]: "Travellers",
    [Team.FABLED]: "Fabled",
    [Team.LORIC]: "Lorics",
  };

  const charactersByTeam = $derived.by(() => {
    if (!script?.characters) return {};
    const grouped: Record<number, Character[]> = {};
    for (const char of script.characters) {
      if (!grouped[char.team]) grouped[char.team] = [];
      grouped[char.team].push(char);
    }
    return grouped;
  });

  const scriptIdSet = $derived(new Set(script?.characterIds ?? []));

  // Auto-save with debounce.
  let saveTimer: ReturnType<typeof setTimeout> | undefined;
  const characterIds = $derived(script?.characterIds ?? []);

  $effect(() => {
    const _name = name;
    const _ids = characterIds;

    if (script?.isSystem) return;
    if (
      _name === lastSavedName &&
      JSON.stringify(_ids) === JSON.stringify(lastSavedIds)
    )
      return;

    clearTimeout(saveTimer);
    saveTimer = setTimeout(() => {
      untrack(() => autoSave(_name, _ids));
    }, 800);

    return () => clearTimeout(saveTimer);
  });

  async function autoSave(currentName: string, currentIds: string[]) {
    if (!script) return;
    try {
      await client.updateScript({
        id: script.id,
        name: currentName,
        characterIds: currentIds,
      });
      lastSavedName = currentName;
      lastSavedIds = currentIds;
    } catch (err) {
      error = getErrorMessage(err, "Failed to save");
    }
  }

  async function loadScript(id: bigint) {
    loading = true;
    error = "";
    try {
      const resp = await client.getScript({ id });
      script = resp.script;
      name = script?.name ?? "";
      lastSavedName = name;
      lastSavedIds = script?.characterIds ?? [];
    } catch (err) {
      error = getErrorMessage(err, "Failed to load script");
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    const id = page.params.id;
    untrack(() => {
      if (!id) return;
      loadScript(BigInt(id));
    });
  });

  async function openAddCharacter(forTeam?: Team) {
    if (allCharacters.length === 0) {
      try {
        const resp = await client.listCharacters({});
        allCharacters = resp.characters;
      } catch (err) {
        error = getErrorMessage(err, "Failed to load characters");
        return;
      }
    }
    pickerTeam = forTeam;
    showAddCharacter = true;
  }

  const optionalCharsByTeam = $derived.by(() => {
    const grouped: Record<number, Character[]> = {};
    for (const t of optionalTeamOrder) {
      const chars = charactersByTeam[t];
      if (chars && chars.length > 0) grouped[t] = chars;
    }
    return grouped;
  });

  const emptyOptionals = $derived(
    optionalTeamOrder.filter((t) => !optionalCharsByTeam[t]),
  );

  function removeCharacter(charId: string) {
    if (!script) return;
    script = {
      ...script,
      characterIds: script.characterIds.filter((id) => id !== charId),
      characters: script.characters.filter((c) => c.id !== charId),
    } as Script;
  }

  function addCharacter(char: Character) {
    if (!script) return;
    script = {
      ...script,
      characterIds: [...script.characterIds, char.id],
      characters: [...script.characters, char],
    } as Script;
  }

  async function createFromEdition(editionId: string) {
    try {
      const resp = await client.createScriptFromEdition({
        editionId,
        name: "",
      });
      if (resp.script) {
        goto(`/scripts/${resp.script.id}`);
      }
    } catch (err) {
      error = getErrorMessage(err, "Failed to duplicate script");
    }
  }

  async function deleteScript() {
    if (!script) return;
    try {
      await client.deleteScript({ id: script.id });
      goto("/scripts");
    } catch (err) {
      error = getErrorMessage(err, "Failed to delete script");
    }
  }
</script>

{#if loading}
  <p class="text-secondary">Loading...</p>
{:else if error && !script}
  <div class="rounded-lg bg-error-bg px-4 py-2 text-sm text-error-text">
    {error}
  </div>
{:else if script}
  <div class="space-y-6">
    <!-- Top bar -->
    <div class="flex items-center justify-between gap-4">
      <div class="flex min-w-0 flex-1 items-center gap-3">
        <a
          href="/scripts"
          aria-label="Back to scripts"
          class="text-secondary transition-colors hover:text-medium"
        >
          <svg
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M15 19l-7-7 7-7"
            />
          </svg>
        </a>
        {#if script.isSystem}
          <h2 class="min-w-0 flex-1 font-[Goudy_Stout] text-lg text-primary">
            {script.name}
          </h2>
        {:else if editingName}
          <input
            type="text"
            bind:value={name}
            onblur={() => (editingName = false)}
            onkeydown={(e) => {
              if (e.key === "Enter" || e.key === "Escape") editingName = false;
            }}
            class="min-w-0 flex-1 border-b-2 border-indigo-500 bg-transparent font-[Goudy_Stout] text-lg text-primary outline-none"
            autofocus
          />
        {:else}
          <button
            onclick={() => (editingName = true)}
            class="group flex min-w-0 flex-1 items-center gap-1.5 text-left font-[Goudy_Stout] text-lg text-primary transition-colors hover:text-indigo-500"
            title="Click to edit name"
          >
            <span class="truncate">{name || "Untitled Script"}</span>
            <svg
              class="h-4 w-4 shrink-0 text-muted"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0115.75 21H5.25A2.25 2.25 0 013 18.75V8.25A2.25 2.25 0 015.25 6H10"
              />
            </svg>
          </button>
        {/if}
      </div>
      <div class="flex items-center gap-2">
        <a
          href="/games/new?script={script.id}"
          class="btn-secondary rounded-lg border border-border px-3 py-2 text-sm text-medium transition-colors hover:bg-hover"
        >
          Start Game
        </a>
        {#if script.isSystem}
          <button
            onclick={() => createFromEdition(script!.edition)}
            class="btn-secondary rounded-lg border border-border px-3 py-2 text-sm text-medium transition-colors hover:bg-hover"
          >
            Duplicate
          </button>
        {:else}
          <button
            onclick={deleteScript}
            aria-label="Delete script"
            class="rounded p-2 text-muted transition-colors hover:bg-hover hover:text-red-500"
          >
            <svg
              class="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
              />
            </svg>
          </button>
        {/if}
      </div>
    </div>

    {#if error}
      <div class="rounded-lg bg-error-bg px-4 py-2 text-sm text-error-text">
        {error}
      </div>
    {/if}

    <!-- Character list grouped by team -->
    <div class="space-y-6">
      {#each teamOrder as team}
        {@const chars = charactersByTeam[team] ?? []}
        {#if chars.length > 0 || !script.isSystem}
          <TeamSection
            {team}
            characters={chars}
            removable={!script.isSystem}
            onremove={removeCharacter}
            onadd={script.isSystem ? undefined : () => openAddCharacter(team)}
          />
        {/if}
      {/each}
    </div>

    <!-- Optional teams with characters -->
    {#each optionalTeamOrder as team}
      {@const chars = optionalCharsByTeam[team]}
      {#if chars}
        <TeamSection
          {team}
          characters={chars}
          removable={!script.isSystem}
          onremove={removeCharacter}
          onadd={script.isSystem ? undefined : () => openAddCharacter(team)}
        />
      {/if}
    {/each}

    <!-- Compact row for empty optional teams -->
    {#if !script.isSystem && emptyOptionals.length > 0}
      <div
        class="grid gap-2"
        style="grid-template-columns: repeat({emptyOptionals.length}, 1fr)"
      >
        {#each emptyOptionals as team}
          <TeamSection
            {team}
            characters={[]}
            compact
            onadd={() => openAddCharacter(team)}
            addLabel={optionalTeamLabels[team]}
          />
        {/each}
      </div>
    {/if}

    {#if script.characterIds.length === 0}
      <div
        class="card-slate rounded-lg border border-dashed border-border-strong p-8 text-center"
      >
        <p class="text-secondary">
          No characters yet. Use the add buttons above to get started.
        </p>
      </div>
    {/if}
  </div>

  {#if showAddCharacter}
    <CharacterPickerModal
      title="Add Characters"
      characters={allCharacters}
      selectedIds={scriptIdSet}
      team={pickerTeam}
      onselect={addCharacter}
      ondeselect={removeCharacter}
      onclose={() => (showAddCharacter = false)}
    />
  {/if}
{/if}
