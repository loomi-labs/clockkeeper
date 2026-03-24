<script lang="ts">
  import { untrack } from "svelte";
  import { page } from "$app/state";
  import { goto } from "$app/navigation";
  import { client } from "~/lib/api";
  import { invalidateSidebar } from "~/lib/sidebar-data.svelte";
  import { getErrorMessage } from "~/lib/errors";
  import type {
    Game,
    Character,
    Script,
    Phase,
  } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import {
    Team,
    GameState,
    PhaseType,
    TravellerAlignment,
  } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { teamLabels } from "~/lib/team-styles";
  import CharacterCard from "~/lib/components/CharacterCard.svelte";
  import CharacterPickerModal from "~/lib/components/CharacterPickerModal.svelte";
  import ConfirmDialog from "~/lib/components/ConfirmDialog.svelte";
  import DeathTracker from "~/lib/components/DeathTracker.svelte";
  import DistributionBar from "~/lib/components/DistributionBar.svelte";
  import GrimoireCanvas from "~/lib/components/grimoire/GrimoireCanvas.svelte";
  import { circleLayout } from "~/lib/components/grimoire/layout";
  import type {
    GrimoirePlayer,
    GrimoireReminder,
  } from "~/lib/components/grimoire/types";
  import NightOrder from "~/lib/components/NightOrder.svelte";
  import PhaseHeader from "~/lib/components/PhaseHeader.svelte";
  import ReminderToken from "~/lib/components/ReminderToken.svelte";
  import SetupSidebar from "~/lib/components/SetupSidebar.svelte";
  import TeamSection from "~/lib/components/TeamSection.svelte";

  // --- Tab definitions (setup only) ---
  type GameTab = "setup" | "nightorder" | "grimoire";

  const setupTabs: { id: GameTab; label: string }[] = [
    { id: "setup", label: "Setup" },
    { id: "nightorder", label: "Night Order" },
    { id: "grimoire", label: "Grimoire" },
  ];

  const validTabs = new Set<GameTab>(["setup", "nightorder", "grimoire"]);
  const initialTab = page.url.searchParams.get("tab") as GameTab | null;
  let activeTab = $state<GameTab>(
    initialTab && validTabs.has(initialTab) ? initialTab : "setup",
  );

  function setTab(tab: GameTab) {
    activeTab = tab;
    const url = new URL(window.location.href);
    url.searchParams.set("tab", tab);
    goto(url.toString(), { replaceState: true, noScroll: true });
  }

  let game = $state<Game | undefined>();
  let script = $state<Script | undefined>();
  let loading = $state(true);
  let error = $state("");
  let randomizing = $state(false);

  // Fullscreen mode
  let isFullscreen = $state(false);
  function toggleFullscreen() {
    if (!document.fullscreenElement) {
      document.documentElement.requestFullscreen();
    } else {
      document.exitFullscreen();
    }
  }
  function onFullscreenChange() {
    isFullscreen = !!document.fullscreenElement;
  }

  // Confirm dialog state.
  let confirmDialog = $state<{
    title: string;
    message: string;
    confirmLabel: string;
    cancelLabel: string;
    onconfirm: () => void;
    oncancel: () => void;
  } | null>(null);

  // Picker state.
  let showCharacterPicker = $state(false);
  let pickerTeam = $state<Team | undefined>();
  let bluffPickerOpen = $state(false);
  let allCharacters = $state<Character[]>([]);

  const teamOrder = [
    Team.TOWNSFOLK,
    Team.OUTSIDER,
    Team.MINION,
    Team.DEMON,
  ] as const;

  // Characters grouped by team — includes both script and extra characters.
  const charactersByTeam = $derived.by(() => {
    const grouped: Record<number, Character[]> = {};
    const skip = new Set([Team.TRAVELLER, Team.FABLED, Team.LORIC]);
    for (const char of script?.characters ?? []) {
      if (skip.has(char.team)) continue;
      if (!grouped[char.team]) grouped[char.team] = [];
      grouped[char.team].push(char);
    }
    for (const char of game?.extraCharacterDetails ?? []) {
      if (skip.has(char.team)) continue;
      if (!grouped[char.team]) grouped[char.team] = [];
      grouped[char.team].push(char);
    }
    return grouped;
  });

  // Selected = script roles + extra characters (both show as "selected" in the grid).
  const selectedRoleIdSet = $derived(
    new Set([
      ...(game?.selectedRoleIds ?? []),
      ...(game?.extraCharacterIds ?? []),
    ]),
  );

  // Track which IDs belong to the script vs extra (for toggle behavior).
  const scriptCharIdSet = $derived(
    new Set(script?.characters?.map((c) => c.id) ?? []),
  );
  const extraCharIdSet = $derived(new Set(game?.extraCharacterIds ?? []));

  const selectedTravellerIdSet = $derived(
    new Set(game?.selectedTravellerIds ?? []),
  );

  // Bag substitutions keyed by caused_by_id (e.g., "drunk" → { characterId, characterName }).
  const bagSubByRole = $derived.by(() => {
    const map = new Map<
      string,
      { characterId: string; characterName: string }
    >();
    for (const bs of game?.bagSubstitutions ?? []) {
      map.set(bs.causedById, {
        characterId: bs.characterId,
        characterName: bs.characterName,
      });
    }
    return map;
  });

  const fabledCharacters = $derived(
    (game?.extraCharacterDetails ?? []).filter((c) => c.team === Team.FABLED),
  );
  const loricCharacters = $derived(
    (game?.extraCharacterDetails ?? []).filter((c) => c.team === Team.LORIC),
  );

  const optionalTeams = $derived([
    {
      team: Team.TRAVELLER,
      label: "Travellers",
      singular: "Traveller",
      chars: game?.selectedTravellerCharacters ?? [],
      remove: removeTraveller,
    },
    {
      team: Team.FABLED,
      label: "Fabled",
      singular: "Fabled",
      chars: fabledCharacters,
      remove: removeExtraChar,
    },
    {
      team: Team.LORIC,
      label: "Lorics",
      singular: "Loric",
      chars: loricCharacters,
      remove: removeExtraChar,
    },
  ]);
  const emptyOptionals = $derived(
    optionalTeams.filter((o) => o.chars.length === 0),
  );

  // Combined selectedIds for the character picker modal.
  const pickerSelectedIds = $derived(
    new Set([
      ...(game?.selectedRoleIds ?? []),
      ...(game?.extraCharacterIds ?? []),
      ...(script?.characterIds ?? []),
      ...(game?.selectedTravellerIds ?? []),
    ]),
  );

  const currentDist = $derived.by(() => {
    if (!game) return { townsfolk: 0, outsiders: 0, minions: 0, demons: 0 };
    const d = { townsfolk: 0, outsiders: 0, minions: 0, demons: 0 };
    // Count from all characters (script + extra) that are selected.
    for (const [, chars] of Object.entries(charactersByTeam)) {
      for (const c of chars) {
        if (!selectedRoleIdSet.has(c.id)) continue;
        if (c.team === Team.TOWNSFOLK) d.townsfolk++;
        else if (c.team === Team.OUTSIDER) d.outsiders++;
        else if (c.team === Team.MINION) d.minions++;
        else if (c.team === Team.DEMON) d.demons++;
      }
    }
    return d;
  });

  const characterById = $derived.by(() => {
    const map = new Map<string, Character>();
    for (const char of script?.characters ?? []) {
      map.set(char.id, char);
    }
    for (const char of game?.selectedTravellerCharacters ?? []) {
      map.set(char.id, char);
    }
    for (const char of game?.extraCharacterDetails ?? []) {
      map.set(char.id, char);
    }
    return map;
  });

  // --- Game state derived values ---
  const isSetup = $derived(game?.state === GameState.SETUP);
  const isInProgress = $derived(game?.state === GameState.IN_PROGRESS);
  const isCompleted = $derived(game?.state === GameState.COMPLETED);
  const canStartGame = $derived(
    isSetup && (game?.selectedRoleIds?.length ?? 0) > 0,
  );

  // --- Round-based navigation (in-progress) ---
  // Phases are grouped by round: each round has a Night + Day pair.
  interface Round {
    night?: Phase;
    day?: Phase;
    roundNumber: number;
  }

  const rounds = $derived.by((): Round[] => {
    const phases = game?.playState?.phases ?? [];
    const roundMap = new Map<number, Round>();
    for (const p of phases) {
      const entry = roundMap.get(p.roundNumber) ?? {
        roundNumber: p.roundNumber,
      };
      if (p.type === PhaseType.NIGHT) entry.night = p;
      else entry.day = p;
      roundMap.set(p.roundNumber, entry);
    }
    return [...roundMap.values()].sort((a, b) => a.roundNumber - b.roundNumber);
  });

  let viewingRoundIndex = $state(0);
  let prevRoundCount = $state(0);

  // Jump to latest round when new rounds are created.
  $effect(() => {
    const count = rounds.length;
    if (count !== prevRoundCount) {
      prevRoundCount = count;
      viewingRoundIndex = Math.max(0, count - 1);
    }
  });

  const viewingRound = $derived(rounds[viewingRoundIndex]);
  const nightPhase = $derived(viewingRound?.night);
  const dayPhase = $derived(viewingRound?.day);
  const isViewingCurrent = $derived(viewingRoundIndex === rounds.length - 1);

  // Dead characters per phase type.
  const nightDeadRoleIds = $derived(
    new Set((nightPhase?.deaths ?? []).map((d) => d.roleId)),
  );
  const dayDeadRoleIds = $derived(
    new Set((dayPhase?.deaths ?? []).map((d) => d.roleId)),
  );

  // deadRoleIds for the completed game view (all deaths from the last round's day phase).
  const deadRoleIds = $derived(dayDeadRoleIds);

  // Character alignments per phase.
  const nightAlignments = $derived(
    new Map<string, string>(
      Object.entries(nightPhase?.characterAlignments ?? {}),
    ),
  );
  const dayAlignments = $derived(
    new Map<string, string>(
      Object.entries(dayPhase?.characterAlignments ?? {}),
    ),
  );

  // Deaths new in this round (compared to previous round's day phase).
  const newDeathsThisRound = $derived.by(() => {
    // Combine deaths from both night and day of current round.
    const nightDeaths = nightPhase?.deaths ?? [];
    const dayDeaths = dayPhase?.deaths ?? [];
    const allCurrentDeaths = [...nightDeaths, ...dayDeaths];
    // Deduplicate by roleId (keep the one from whichever phase has it).
    const deathMap = new Map(allCurrentDeaths.map((d) => [d.roleId, d]));
    const currentDeaths = [...deathMap.values()];

    if (viewingRoundIndex <= 0) return currentDeaths;
    const prevRound = rounds[viewingRoundIndex - 1];
    const prevDeadRoleIds = new Set(
      (prevRound?.day?.deaths ?? []).map((d) => d.roleId),
    );
    return currentDeaths.filter((d) => !prevDeadRoleIds.has(d.roleId));
  });

  const totalRoundsPlayed = $derived(rounds.length);

  // --- Load game ---
  async function loadGame(gameId: bigint) {
    loading = true;
    error = "";
    game = undefined;
    script = undefined;
    try {
      const resp = await client.getGame({ id: gameId });
      game = resp.game;
      if (game) {
        const scriptResp = await client.getScript({ id: game.scriptId });
        script = scriptResp.script;
      }
    } catch (err) {
      error = getErrorMessage(err, "Failed to load game");
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    const id = page.params.id;
    untrack(() => {
      if (!id) {
        error = "Invalid game ID";
        loading = false;
        return;
      }
      let gameId: bigint;
      try {
        gameId = BigInt(id);
      } catch {
        error = "Invalid game ID";
        loading = false;
        return;
      }
      grimoireInitialized = false;
      loadGame(gameId);
    });
  });

  // --- Setup actions ---
  async function randomize() {
    if (!game) return;
    randomizing = true;
    error = "";
    try {
      const resp = await client.randomizeRoles({ gameId: game.id });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to randomize roles");
    } finally {
      randomizing = false;
    }
  }

  async function toggleRole(id: string) {
    if (!game || !isSetup) return;
    error = "";

    // If it's an extra character, toggle via the extra characters API.
    if (extraCharIdSet.has(id)) {
      const newIds = (game.extraCharacterIds ?? []).filter((eid) => eid !== id);
      try {
        const resp = await client.updateGameExtraCharacters({
          gameId: game.id,
          extraCharacterIds: newIds,
        });
        game = resp.game;
      } catch (err) {
        error = getErrorMessage(err, "Failed to update roles");
      }
      return;
    }

    // Otherwise toggle via the normal roles API.
    const newIds = selectedRoleIdSet.has(id)
      ? game.selectedRoleIds.filter((rid) => rid !== id)
      : [...game.selectedRoleIds, id];
    try {
      const resp = await client.updateGameRoles({
        gameId: game.id,
        selectedRoleIds: newIds,
      });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to update roles");
    }
  }

  async function openCharacterPicker(forTeam?: Team) {
    error = "";
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
    showCharacterPicker = true;
  }

  async function addExtraChar(char: Character) {
    if (!game) return;
    error = "";
    const newIds = [...(game.extraCharacterIds ?? []), char.id];
    try {
      const resp = await client.updateGameExtraCharacters({
        gameId: game.id,
        extraCharacterIds: newIds,
      });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to add character");
    }
  }

  async function removeExtraChar(charId: string) {
    if (!game) return;
    error = "";
    const newIds = (game.extraCharacterIds ?? []).filter(
      (eid) => eid !== charId,
    );
    try {
      const resp = await client.updateGameExtraCharacters({
        gameId: game.id,
        extraCharacterIds: newIds,
      });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to remove character");
    }
  }

  function handlePickerSelect(char: Character) {
    if (char.team === Team.TRAVELLER) {
      addTraveller(char);
    } else if (scriptCharIdSet.has(char.id)) {
      toggleRole(char.id);
    } else {
      addExtraChar(char);
    }
  }

  function handlePickerDeselect(charId: string) {
    if (selectedTravellerIdSet.has(charId)) {
      removeTraveller(charId);
    } else if (scriptCharIdSet.has(charId)) {
      toggleRole(charId);
    } else {
      removeExtraChar(charId);
    }
  }

  async function addTraveller(char: Character) {
    if (!game) return;
    error = "";
    const newIds = [...game.selectedTravellerIds, char.id];
    try {
      const resp = await client.updateGameTravellers({
        gameId: game.id,
        selectedTravellerIds: newIds,
      });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to add traveller");
    }
  }

  async function removeTraveller(charId: string) {
    if (!game) return;
    error = "";
    const newIds = game.selectedTravellerIds.filter((tid) => tid !== charId);
    try {
      const resp = await client.updateGameTravellers({
        gameId: game.id,
        selectedTravellerIds: newIds,
      });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to remove traveller");
    }
  }

  // --- Traveller alignment ---
  async function updateTravellerAlignment(
    roleId: string,
    alignment: TravellerAlignment,
  ) {
    if (!game) return;
    error = "";
    try {
      const resp = await client.updateTravellerAlignment({
        gameId: game.id,
        roleId,
        alignment,
      });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to update traveller alignment");
    }
  }

  function rerollBluffs() {
    if (!game || !script) return;
    const selectedIds = new Set([
      ...(game.selectedRoleIds ?? []),
      ...(game.extraCharacterIds ?? []),
    ]);
    const goodChars = (script.characters ?? []).filter(
      (c) =>
        !selectedIds.has(c.id) &&
        (c.team === Team.TOWNSFOLK || c.team === Team.OUTSIDER),
    );
    // Shuffle and pick 3
    for (let i = goodChars.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [goodChars[i], goodChars[j]] = [goodChars[j], goodChars[i]];
    }
    const bluffIds = goodChars.slice(0, 3).map((c) => c.id);
    updateDemonBluffs(bluffIds);
  }

  function openBluffPicker() {
    bluffPickerOpen = true;
  }

  function handleBluffSelect(char: Character) {
    if (!game) return;
    const currentBluffs = [...(game.selectedBluffIds ?? [])];
    if (!currentBluffs.includes(char.id)) {
      currentBluffs.push(char.id);
      updateDemonBluffs(currentBluffs);
    }
  }

  async function updateDemonBluffs(bluffIds: string[]) {
    if (!game) return;
    error = "";
    try {
      const resp = await client.updateDemonBluffs({
        gameId: game.id,
        bluffIds,
      });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to update demon bluffs");
    }
  }

  // --- Bag substitution management ---
  let bagSubPickerForRole = $state<string | null>(null);

  function openBagSubPicker(causedById: string) {
    bagSubPickerForRole = causedById;
  }

  async function setBagSubCharacter(causedById: string, char: Character) {
    if (!game) return;
    error = "";
    const updated = (game.bagSubstitutions ?? []).map((bs) => {
      if (bs.causedById === causedById) {
        return { ...bs, characterId: char.id, characterName: char.name };
      }
      return bs;
    });
    try {
      const resp = await client.updateBagSubstitutions({
        gameId: game.id,
        bagSubstitutions: updated,
      });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to update bag substitution");
    }
    bagSubPickerForRole = null;
  }

  // --- Game lifecycle actions ---
  async function startGame() {
    if (!game) return;
    error = "";
    try {
      const resp = await client.startGame({ gameId: game.id });
      game = resp.game;
      invalidateSidebar();
    } catch (err) {
      error = getErrorMessage(err, "Failed to start game");
    }
  }

  async function duplicateGame() {
    if (!game) return;
    error = "";
    try {
      const resp = await client.duplicateGame({ gameId: game.id });
      if (resp.game) {
        invalidateSidebar();
        goto(`/games/${resp.game.id}`);
      }
    } catch (err) {
      error = getErrorMessage(err, "Failed to duplicate game");
    }
  }

  function deleteGame() {
    if (!game) return;
    confirmDialog = {
      title: "Delete Game",
      message:
        "Are you sure you want to delete this game? This cannot be undone.",
      confirmLabel: "Delete",
      cancelLabel: "Cancel",
      onconfirm: async () => {
        confirmDialog = null;
        if (!game) return;
        try {
          await client.deleteGame({ id: game.id });
          invalidateSidebar();
          goto("/");
        } catch (err) {
          error = getErrorMessage(err, "Failed to delete game");
        }
      },
      oncancel: () => {
        confirmDialog = null;
      },
    };
  }

  async function advancePhase() {
    if (!game) return;
    error = "";
    try {
      const resp = await client.advancePhase({ gameId: game.id });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to advance phase");
    }
  }

  function endGame() {
    if (!game) return;
    confirmDialog = {
      title: "End Game",
      message: "Are you sure you want to end this game? This cannot be undone.",
      confirmLabel: "End Game",
      cancelLabel: "Cancel",
      onconfirm: async () => {
        confirmDialog = null;
        if (!game) return;
        error = "";
        try {
          const resp = await client.endGame({ gameId: game.id });
          game = resp.game;
          invalidateSidebar();
        } catch (err) {
          error = getErrorMessage(err, "Failed to end game");
        }
      },
      oncancel: () => {
        confirmDialog = null;
      },
    };
  }

  // --- Night action tracking ---
  const completedActions = $derived(
    new Set(nightPhase?.completedActions ?? []),
  );

  async function toggleNightAction(actionId: string, done: boolean) {
    if (!game || !nightPhase) return;
    error = "";
    try {
      const resp = await client.toggleNightAction({
        gameId: game.id,
        actionId,
        done,
        phaseId: nightPhase.id,
      });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to toggle night action");
    }
  }

  // --- Death tracking ---
  // Deaths from the night sheet go to the night phase; deaths from the grimoire go to the day phase.
  async function doRecordDeath(
    roleId: string,
    phaseId: bigint,
    propagate: boolean,
  ) {
    if (!game) return;
    error = "";
    try {
      const resp = await client.recordDeath({
        gameId: game.id,
        roleId,
        phaseId,
        propagate,
      });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to record death");
    }
  }

  function recordDeathOnNight(roleId: string) {
    if (!game || !nightPhase) return;
    if (isViewingCurrent) {
      doRecordDeath(roleId, nightPhase.id, true);
      return;
    }
    const charName = characterById.get(roleId)?.name ?? roleId;
    confirmDialog = {
      title: `Mark ${charName} as dead`,
      message: `Apply to later phases as well?`,
      confirmLabel: "All later phases",
      cancelLabel: "This phase only",
      onconfirm: () => {
        confirmDialog = null;
        doRecordDeath(roleId, nightPhase!.id, true);
      },
      oncancel: () => {
        confirmDialog = null;
        doRecordDeath(roleId, nightPhase!.id, false);
      },
    };
  }

  function recordDeathOnDay(roleId: string) {
    if (!game || !dayPhase) return;
    if (isViewingCurrent) {
      doRecordDeath(roleId, dayPhase.id, true);
      return;
    }
    const charName = characterById.get(roleId)?.name ?? roleId;
    confirmDialog = {
      title: `Mark ${charName} as dead`,
      message: `Apply to later phases as well?`,
      confirmLabel: "All later phases",
      cancelLabel: "This phase only",
      onconfirm: () => {
        confirmDialog = null;
        doRecordDeath(roleId, dayPhase!.id, true);
      },
      oncancel: () => {
        confirmDialog = null;
        doRecordDeath(roleId, dayPhase!.id, false);
      },
    };
  }

  async function removeDeath(deathId: bigint, propagate = false) {
    if (!game) return;
    error = "";
    try {
      const resp = await client.removeDeath({
        gameId: game.id,
        deathId,
        propagate,
      });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to remove death");
    }
  }

  function undoDeathByRoleOnNight(roleId: string) {
    const phaseDeath = (nightPhase?.deaths ?? []).find(
      (d) => d.roleId === roleId,
    );
    if (!phaseDeath) return;
    if (isViewingCurrent) {
      removeDeath(phaseDeath.id, true);
      return;
    }
    const charName = characterById.get(roleId)?.name ?? roleId;
    confirmDialog = {
      title: `Revive ${charName}`,
      message: `Also revive in all later phases?`,
      confirmLabel: "All later phases",
      cancelLabel: "This phase only",
      onconfirm: () => {
        confirmDialog = null;
        removeDeath(phaseDeath.id, true);
      },
      oncancel: () => {
        confirmDialog = null;
        removeDeath(phaseDeath.id, false);
      },
    };
  }

  function undoDeathByRoleOnDay(roleId: string) {
    const phaseDeath = (dayPhase?.deaths ?? []).find(
      (d) => d.roleId === roleId,
    );
    if (!phaseDeath) return;
    if (isViewingCurrent) {
      removeDeath(phaseDeath.id, true);
      return;
    }
    const charName = characterById.get(roleId)?.name ?? roleId;
    confirmDialog = {
      title: `Revive ${charName}`,
      message: `Also revive in all later phases?`,
      confirmLabel: "All later phases",
      cancelLabel: "This phase only",
      onconfirm: () => {
        confirmDialog = null;
        removeDeath(phaseDeath.id, true);
      },
      oncancel: () => {
        confirmDialog = null;
        removeDeath(phaseDeath.id, false);
      },
    };
  }

  async function useGhostVote(deathId: bigint) {
    if (!game) return;
    error = "";
    try {
      const resp = await client.useGhostVote({ gameId: game.id, deathId });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to use ghost vote");
    }
  }

  // --- Editable game name ---
  let editingName = $state(false);
  let nameInput = $state("");

  async function updateGameName() {
    if (!game || !nameInput.trim() || nameInput === game.name) {
      editingName = false;
      return;
    }
    error = "";
    try {
      const resp = await client.updateGameName({
        gameId: game.id,
        name: nameInput.trim(),
      });
      game = resp.game;
      invalidateSidebar();
    } catch (err) {
      error = getErrorMessage(err, "Failed to update game name");
    }
    editingName = false;
  }

  // --- State badge ---
  const stateBadge = $derived.by(() => {
    if (!game) return { label: "", class: "" };
    switch (game.state) {
      case GameState.IN_PROGRESS:
        return {
          label: "In Progress",
          class:
            "bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-300",
        };
      case GameState.COMPLETED:
        return { label: "Completed", class: "bg-element text-muted" };
      default:
        return { label: "", class: "" };
    }
  });

  // --- In-progress view toggle ---
  type InProgressView = "nightsheet" | "grimoire";
  let inProgressView = $state<InProgressView>("nightsheet");

  // --- Grimoire state (persisted per game) ---
  let grimoirePositions = $state(new Map<string, { x: number; y: number }>());
  let grimoireNames = $state(new Map<string, string>());
  let reminderPositions = $state(new Map<string, { x: number; y: number }>());
  let grimoireGameNotes = $state(new Map<string, string>());
  let grimoireRoundNotes = $state(new Map<string, string>());
  let grimoireInitialized = $state(false);

  // Initialize grimoire from persisted server state, then fill gaps with defaults
  $effect(() => {
    const chars = [
      ...(game?.selectedCharacters ?? []),
      ...(game?.selectedTravellerCharacters ?? []),
    ];
    if (chars.length === 0) return;
    if (grimoireInitialized) {
      // After initial load, only add positions for NEW characters (e.g., traveller added mid-game)
      let needsInit = false;
      for (const c of chars) {
        if (!grimoirePositions.has(c.id)) {
          needsInit = true;
          break;
        }
      }
      if (!needsInit) return;
      const positions = circleLayout(chars.length, 0, 0, 300);
      const newPositions = new Map(grimoirePositions);
      for (let i = 0; i < chars.length; i++) {
        if (!newPositions.has(chars[i].id)) {
          newPositions.set(chars[i].id, positions[i]);
        }
      }
      grimoirePositions = newPositions;
      return;
    }

    // First load: populate from server state
    const serverPositions = game?.grimoirePositions ?? {};
    const serverNames = game?.grimoirePlayerNames ?? {};
    const newPositions = new Map<string, { x: number; y: number }>();
    const newReminderPositions = new Map<string, { x: number; y: number }>();
    const newNames = new Map<string, string>();

    // Load all persisted positions, separating player vs reminder
    for (const [id, pos] of Object.entries(serverPositions)) {
      if (id.startsWith("reminder-")) {
        newReminderPositions.set(id, { x: pos.x, y: pos.y });
      } else {
        newPositions.set(id, { x: pos.x, y: pos.y });
      }
    }

    // Load persisted player names
    for (const [id, name] of Object.entries(serverNames)) {
      newNames.set(id, name);
    }

    // Fill gaps for characters without persisted positions (circleLayout)
    const defaultPositions = circleLayout(chars.length, 0, 0, 300);
    for (let i = 0; i < chars.length; i++) {
      if (!newPositions.has(chars[i].id)) {
        newPositions.set(chars[i].id, defaultPositions[i]);
      }
    }

    // Fill gaps for reminders without persisted positions (horizontal line at bottom)
    const tokens = game?.reminderTokens ?? [];
    if (tokens.length > 0) {
      const reminderY = 380;
      const totalWidth = tokens.length * 80;
      const startX = -totalWidth / 2 + 40;
      for (let i = 0; i < tokens.length; i++) {
        const rid = `reminder-${i}`;
        if (!newReminderPositions.has(rid)) {
          newReminderPositions.set(rid, { x: startX + i * 80, y: reminderY });
        }
      }
    }

    // Load persisted notes
    const serverGameNotes = game?.grimoireGameNotes ?? {};
    const serverRoundNotes = game?.grimoireRoundNotes ?? {};

    grimoirePositions = newPositions;
    reminderPositions = newReminderPositions;
    grimoireNames = newNames;
    grimoireGameNotes = new Map(Object.entries(serverGameNotes));
    grimoireRoundNotes = new Map(Object.entries(serverRoundNotes));
    grimoireInitialized = true;
  });

  // Current round notes (extract notes for the viewed round from the composite-key map)
  const currentRoundNotes = $derived.by(() => {
    const round = viewingRound?.roundNumber ?? 1;
    const prefix = `${round}:`;
    const notes = new Map<string, string>();
    for (const [key, val] of grimoireRoundNotes) {
      if (key.startsWith(prefix)) {
        notes.set(key.slice(prefix.length), val);
      }
    }
    return notes;
  });

  // Derive grimoire players from game data + local state (grimoire uses day phase)
  const grimoirePlayers = $derived.by((): GrimoirePlayer[] => {
    if (!game) return [];
    const chars = [
      ...(game.selectedCharacters ?? []),
      ...(game.selectedTravellerCharacters ?? []),
    ];
    const phaseDeaths = dayPhase?.deaths ?? [];
    const deathByRole = new Map(
      phaseDeaths.map((d: { roleId: string; ghostVote: boolean }) => [
        d.roleId,
        d,
      ]),
    );
    return chars.map((c) => {
      const pos = grimoirePositions.get(c.id) ?? { x: 0, y: 0 };
      const death = deathByRole.get(c.id);
      return {
        id: c.id,
        name: grimoireNames.get(c.id) ?? c.name,
        characterId: c.id,
        characterName: c.name,
        team: c.team,
        edition: c.edition,
        x: pos.x,
        y: pos.y,
        isDead: dayDeadRoleIds.has(c.id),
        ghostVoteUsed: death?.ghostVote ?? false,
        gameNote: grimoireGameNotes.get(c.id) ?? "",
        roundNote: currentRoundNotes.get(c.id) ?? "",
        alignment: dayAlignments.get(c.id) as "good" | "evil" | undefined,
      };
    });
  });

  // Derive grimoire reminders from game data + local state
  const grimoireReminders = $derived.by((): GrimoireReminder[] => {
    if (!game) return [];
    return (game.reminderTokens ?? []).map((token, i) => {
      const rid = `reminder-${i}`;
      const char = characterById.get(token.characterId);
      const pos = reminderPositions.get(rid) ?? { x: 0, y: 0 };
      return {
        id: rid,
        characterId: token.characterId,
        characterName: token.characterName,
        text: token.text,
        team: char?.team ?? Team.UNSPECIFIED,
        edition: char?.edition ?? "",
        x: pos.x,
        y: pos.y,
        alignment: dayAlignments.get(token.characterId) as
          | "good"
          | "evil"
          | undefined,
      };
    });
  });

  // Debounced save to server
  let grimoireSaveTimeout: ReturnType<typeof setTimeout> | undefined;
  function saveGrimoireState() {
    clearTimeout(grimoireSaveTimeout);
    grimoireSaveTimeout = setTimeout(async () => {
      if (!game) return;
      const allPositions: Record<string, { x: number; y: number }> = {};
      for (const [id, pos] of grimoirePositions) allPositions[id] = pos;
      for (const [id, pos] of reminderPositions) allPositions[id] = pos;
      try {
        await client.updateGrimoireState({
          gameId: game.id,
          positions: allPositions,
          playerNames: Object.fromEntries(grimoireNames),
          gameNotes: Object.fromEntries(grimoireGameNotes),
          roundNotes: Object.fromEntries(grimoireRoundNotes),
        });
      } catch (err) {
        console.error("Failed to save grimoire state", err);
      }
    }, 500);
  }

  // Grimoire event handlers
  function handleGrimoirePlayerMove(id: string, x: number, y: number) {
    grimoirePositions = new Map(grimoirePositions.set(id, { x, y }));
    saveGrimoireState();
  }
  function handleGrimoireReminderMove(id: string, x: number, y: number) {
    reminderPositions = new Map(reminderPositions.set(id, { x, y }));
    saveGrimoireState();
  }
  function handleGrimoirePlayerRename(id: string, name: string) {
    grimoireNames = new Map(grimoireNames.set(id, name));
    saveGrimoireState();
  }
  function handleGrimoirePlayerToggleDeath(id: string) {
    if (dayDeadRoleIds.has(id)) {
      undoDeathByRoleOnDay(id);
    } else {
      recordDeathOnDay(id);
    }
  }
  function handleGrimoireGameNote(id: string, note: string) {
    if (note) grimoireGameNotes.set(id, note);
    else grimoireGameNotes.delete(id);
    grimoireGameNotes = new Map(grimoireGameNotes);
    saveGrimoireState();
  }
  function handleGrimoireRoundNote(id: string, note: string) {
    const round = viewingRound?.roundNumber ?? 1;
    const key = `${round}:${id}`;
    if (note) grimoireRoundNotes.set(key, note);
    else grimoireRoundNotes.delete(key);
    grimoireRoundNotes = new Map(grimoireRoundNotes);
    saveGrimoireState();
  }

  // --- Character alignment ---
  async function updateCharacterAlignmentOnPhase(
    roleId: string,
    alignment: string,
    phaseId: bigint,
  ) {
    if (!game) return;
    error = "";
    try {
      const resp = await client.updateCharacterAlignment({
        gameId: game.id,
        phaseId,
        roleId,
        alignment,
        propagate: true,
      });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to update alignment");
    }
  }

  function handleGrimoireAlignment(id: string, alignment: string) {
    if (!dayPhase) return;
    updateCharacterAlignmentOnPhase(id, alignment, dayPhase.id);
  }

  function handleNightSheetAlignment(id: string, alignment: string) {
    if (!nightPhase) return;
    updateCharacterAlignmentOnPhase(id, alignment, nightPhase.id);
  }
</script>

<svelte:document onfullscreenchange={onFullscreenChange} />

{#if loading}
  <p class="text-secondary">Loading...</p>
{:else if error && !game}
  <div
    class="rounded-lg bg-error-bg border border-error-border px-4 py-2 text-sm text-error-text"
  >
    {error}
  </div>
{:else if game}
  <div class="space-y-6 {isFullscreen ? 'pb-0' : 'pb-16 2xl:pb-0'}">
    <!-- Header -->
    {#if !isFullscreen}
      <div class="no-print flex items-center justify-between">
        <div>
          <div class="flex items-center gap-3">
            {#if editingName}
              <input
                type="text"
                bind:value={nameInput}
                onblur={updateGameName}
                onkeydown={(e) => {
                  if (e.key === "Enter") updateGameName();
                  if (e.key === "Escape") editingName = false;
                }}
                class="text-2xl font-bold text-primary bg-transparent border-b-2 border-indigo-500 outline-none w-full max-w-md"
                autofocus
              />
            {:else}
              <button
                onclick={() => {
                  nameInput = game?.name ?? "";
                  editingName = true;
                }}
                class="flex items-center gap-2 text-2xl font-bold text-primary hover:text-indigo-500 transition-colors text-left"
                title="Click to edit name"
              >
                {game.name || "Untitled Game"}
                <svg
                  class="h-5 w-5 shrink-0 text-muted"
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
            {#if stateBadge.label}
              <span
                class="rounded-full px-2.5 py-0.5 text-xs font-medium {stateBadge.class}"
                >{stateBadge.label}</span
              >
            {/if}
          </div>
          <p class="mt-1 text-secondary">
            {game.playerCount} players{#if game.travellerCount > 0}
              + {game.travellerCount}
              {game.travellerCount === 1 ? "traveller" : "travellers"}
              = {game.playerCount + game.travellerCount} total{/if}
          </p>
        </div>
        <div class="flex items-center gap-2">
          {#if isSetup}
            <button
              onclick={deleteGame}
              class="rounded-lg border border-border px-3 py-2.5 text-sm font-medium text-muted transition-colors hover:border-red-300 hover:bg-red-50 hover:text-red-600 dark:hover:border-red-800 dark:hover:bg-red-950/30 dark:hover:text-red-400"
              title="Delete game"
            >
              <svg
                class="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                />
              </svg>
            </button>
          {/if}
          <button
            onclick={duplicateGame}
            class="rounded-lg border border-border px-3 py-2.5 text-sm font-medium text-secondary transition-colors hover:bg-hover hover:text-primary"
            title="Duplicate game"
          >
            <svg
              class="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
              />
            </svg>
          </button>
          {#if canStartGame}
            <button
              onclick={startGame}
              class="rounded-lg bg-green-600 px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-green-500"
            >
              Start Game
            </button>
          {/if}
        </div>
      </div>
    {/if}

    <!-- Completed game banner -->
    {#if isCompleted}
      <div class="rounded-lg border border-border bg-surface p-6 text-center">
        <svg
          class="mx-auto mb-3 h-12 w-12 text-muted"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.5"
            d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
        <h2 class="text-xl font-bold text-primary">Game Complete</h2>
        <p class="mt-1 text-sm text-secondary">
          {totalRoundsPlayed}
          {totalRoundsPlayed === 1 ? "round" : "rounds"} played
          {#if (game.playState?.allDeaths ?? []).length > 0}
            &middot; {game.playState?.allDeaths.length}
            {game.playState?.allDeaths.length === 1 ? "death" : "deaths"}
          {/if}
        </p>

        <!-- Round history -->
        {#if rounds.length > 0}
          <div class="mt-4 flex flex-wrap items-center justify-center gap-1">
            {#each rounds as round (round.roundNumber)}
              <span
                class="rounded px-2 py-0.5 text-xs font-medium bg-element text-secondary"
              >
                Night {round.roundNumber}
              </span>
            {/each}
          </div>
        {/if}

        <!-- Deaths summary (read-only) -->
        {#if (game.playState?.allDeaths ?? []).length > 0}
          <div class="mt-6 max-w-lg mx-auto text-left">
            <DeathTracker
              {game}
              onrecord={() => {}}
              onremove={() => {}}
              onuseghostvote={() => {}}
              readonly
            />
          </div>
        {/if}

        <!-- Setup info (read-only) -->
        <div class="mt-6 max-w-lg mx-auto text-left">
          <h3
            class="mb-2 text-sm font-semibold uppercase tracking-wide text-secondary"
          >
            Roles in Play
          </h3>
          <div class="flex flex-wrap gap-2">
            {#each game.selectedCharacters as char (char.id)}
              {@const isDead = deadRoleIds.has(char.id)}
              <span
                class="inline-flex items-center gap-1.5 rounded-full border border-border px-2.5 py-1 text-xs font-medium {isDead
                  ? 'text-muted line-through'
                  : 'text-primary'}"
              >
                {char.name}
              </span>
            {/each}
          </div>
        </div>
      </div>
    {/if}

    <!-- Tab bar (setup only) -->
    {#if isSetup}
      <div class="no-print flex gap-1 rounded-lg bg-element p-1">
        {#each setupTabs as t}
          <button
            onclick={() => setTab(t.id)}
            class="rounded-md px-4 py-2 text-sm font-medium transition-colors {activeTab ===
            t.id
              ? 'bg-surface text-primary shadow-sm'
              : 'text-secondary hover:text-medium'}"
          >
            {t.label}
          </button>
        {/each}
      </div>
    {/if}

    {#if error}
      <div
        class="rounded-lg bg-error-bg border border-error-border px-4 py-2 text-sm text-error-text"
      >
        {error}
      </div>
    {/if}

    <!-- ===== IN-PROGRESS ===== -->
    {#if isInProgress && game.playState}
      <div class="space-y-6">
        <PhaseHeader
          {game}
          {viewingRoundIndex}
          {rounds}
          onadvance={advancePhase}
          onend={endGame}
          onnavigate={(i) => (viewingRoundIndex = i)}
        />

        <!-- View toggle: Night Sheet / Grimoire + Fullscreen -->
        <div class="no-print flex items-center gap-2">
          <div class="flex gap-1 rounded-lg bg-element p-1">
            <button
              onclick={() => (inProgressView = "nightsheet")}
              class="rounded-md px-4 py-2 text-sm font-medium transition-colors {inProgressView ===
              'nightsheet'
                ? 'bg-surface text-primary shadow-sm'
                : 'text-secondary hover:text-medium'}"
            >
              Night Sheet
            </button>
            <button
              onclick={() => (inProgressView = "grimoire")}
              class="rounded-md px-4 py-2 text-sm font-medium transition-colors {inProgressView ===
              'grimoire'
                ? 'bg-surface text-primary shadow-sm'
                : 'text-secondary hover:text-medium'}"
            >
              Grimoire
            </button>
          </div>
          <button
            onclick={toggleFullscreen}
            class="rounded-lg border border-border p-2 text-secondary transition-colors hover:bg-hover hover:text-medium"
            title={isFullscreen ? "Exit fullscreen" : "Fullscreen"}
          >
            {#if isFullscreen}
              <svg
                class="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M9 9V4.5M9 9H4.5M9 9L3.75 3.75M9 15v4.5M9 15H4.5M9 15l-5.25 5.25M15 9h4.5M15 9V4.5M15 9l5.25-5.25M15 15h4.5M15 15v4.5m0-4.5l5.25 5.25"
                />
              </svg>
            {:else}
              <svg
                class="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M3.75 3.75v4.5m0-4.5h4.5m-4.5 0L9 9M3.75 20.25v-4.5m0 4.5h4.5m-4.5 0L9 15M20.25 3.75h-4.5m4.5 0v4.5m0-4.5L15 9m5.25 11.25h-4.5m4.5 0v-4.5m0 4.5L15 15"
                />
              </svg>
            {/if}
          </button>
        </div>

        {#if inProgressView === "nightsheet"}
          <NightOrder
            {game}
            scriptCharacters={script?.characters ?? []}
            deadRoleIds={nightDeadRoleIds}
            activeRound={viewingRound?.roundNumber}
            {completedActions}
            gameNotes={grimoireGameNotes}
            roundNotes={currentRoundNotes}
            ontoggle={toggleNightAction}
            ondeath={recordDeathOnNight}
            onundodeath={undoDeathByRoleOnNight}
            ongamenote={handleGrimoireGameNote}
            onroundnote={handleGrimoireRoundNote}
            alignments={nightAlignments}
            bluffs={game.selectedBluffCharacters}
            onalignment={handleNightSheetAlignment}
          />

          <!-- Death tracker -->
          <DeathTracker
            {game}
            viewedPhaseDeaths={newDeathsThisRound}
            onrecord={recordDeathOnDay}
            onremove={removeDeath}
            onuseghostvote={useGhostVote}
            readonly={!isViewingCurrent}
          />
        {:else}
          <!-- Grimoire view -->
          <div
            class="-mx-4 {isFullscreen
              ? 'h-[calc(100dvh-100px)]'
              : 'h-[calc(100dvh-240px)]'} sm:mx-0 sm:rounded-lg sm:border sm:border-border overflow-hidden"
          >
            <GrimoireCanvas
              players={grimoirePlayers}
              reminders={grimoireReminders}
              roundLabel="Night {viewingRound?.roundNumber ?? 1}"
              onplayermove={handleGrimoirePlayerMove}
              onremindermove={handleGrimoireReminderMove}
              onplayerrename={handleGrimoirePlayerRename}
              onplayertoggledeath={handleGrimoirePlayerToggleDeath}
              onplayergamenote={handleGrimoireGameNote}
              onplayerroundnote={handleGrimoireRoundNote}
              onplayeralignment={handleGrimoireAlignment}
            />
          </div>
        {/if}
      </div>

      <!-- ===== SETUP TABS (setup state only) ===== -->
    {:else if isSetup}
      {#if activeTab === "setup"}
        <div class="space-y-6">
          <div class="flex items-center justify-end">
            <button
              onclick={randomize}
              disabled={randomizing}
              class="btn-primary rounded-lg bg-indigo-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-400 disabled:opacity-50"
            >
              {randomizing ? "Randomizing..." : "Randomize Roles"}
            </button>
          </div>

          <!-- Distribution -->
          <div class="rounded-lg border border-border bg-surface p-4">
            <DistributionBar
              current={currentDist}
              expected={game.distribution}
              travellers={game.selectedTravellerCharacters.length}
              bagExtras={(game.bagSubstitutions ?? []).map((bs) => ({
                causedByName: bs.causedByName,
                characterName: bs.characterName,
                picked: !!bs.characterId,
              }))}
            />
          </div>

          <!-- Characters — click to toggle selection (script + extra merged) -->
          {#if script}
            <div class="space-y-6">
              {#each teamOrder as team}
                {@const chars = charactersByTeam[team]}
                {#if chars && chars.length > 0}
                  <TeamSection
                    {team}
                    characters={chars}
                    selectedIds={selectedRoleIdSet}
                    onclick={toggleRole}
                    onadd={() => openCharacterPicker(team)}
                    bagSubstitutions={bagSubByRole}
                    onbagsubchange={openBagSubPicker}
                  />
                {/if}
                {#if team === Team.DEMON && game.playerCount >= 7}
                  <div
                    class="rounded-lg border border-dashed border-border bg-surface/50 p-4"
                  >
                    <div class="mb-2 flex items-center justify-between">
                      <h3 class="text-sm font-semibold text-secondary">
                        Demon Bluffs
                      </h3>
                      <button
                        onclick={rerollBluffs}
                        class="rounded px-2 py-1 text-xs text-secondary transition-colors hover:bg-hover hover:text-medium"
                      >
                        {(game.selectedBluffCharacters ?? []).length > 0
                          ? "Re-roll"
                          : "Generate"}
                      </button>
                    </div>
                    <div class="flex flex-wrap items-center gap-2">
                      {#each game.selectedBluffCharacters ?? [] as char (char.id)}
                        <button
                          class="flex items-center gap-1.5 rounded-full border border-border bg-surface px-2.5 py-1 transition-colors hover:border-red-300 hover:bg-red-50 dark:hover:border-red-700 dark:hover:bg-red-950/30"
                          title="Remove {char.name}"
                          onclick={() =>
                            updateDemonBluffs(
                              (game?.selectedBluffIds ?? []).filter(
                                (id) => id !== char.id,
                              ),
                            )}
                        >
                          <img
                            src="/characters/{char.edition}/{char.id}_g.webp"
                            alt={char.name}
                            class="h-6 w-6 rounded-full"
                            onerror={(e) =>
                              ((e.target as HTMLImageElement).style.display =
                                "none")}
                          />
                          <span class="text-xs font-medium text-primary"
                            >{char.name}</span
                          >
                          <svg
                            class="h-3 w-3 text-muted"
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
                      {/each}
                      {#if (game.selectedBluffCharacters ?? []).length < 3}
                        <button
                          onclick={() => openBluffPicker()}
                          class="flex h-8 items-center gap-1 rounded-full border border-dashed border-border px-2.5 text-xs text-secondary transition-colors hover:bg-hover hover:text-medium"
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
                              d="M12 4v16m8-8H4"
                            /></svg
                          >
                          Add
                        </button>
                      {/if}
                    </div>
                  </div>
                {/if}
              {/each}
            </div>
          {/if}

          <!-- Optional teams: Travellers, Fabled, Lorics -->
          {#each optionalTeams as opt}
            {#if opt.chars.length > 0}
              <TeamSection
                team={opt.team}
                characters={opt.chars}
                removable
                onremove={opt.remove}
                onadd={() => openCharacterPicker(opt.team)}
                addLabel="Add {opt.singular}"
                travellerAlignments={opt.team === Team.TRAVELLER
                  ? game.travellerAlignments
                  : undefined}
                onalignmentchange={opt.team === Team.TRAVELLER
                  ? updateTravellerAlignment
                  : undefined}
              />
            {/if}
          {/each}

          <!-- Compact row for empty teams -->
          {#if emptyOptionals.length > 0}
            <div
              class="grid gap-2"
              style="grid-template-columns: repeat({emptyOptionals.length}, 1fr)"
            >
              {#each emptyOptionals as opt}
                <TeamSection
                  team={opt.team}
                  characters={[]}
                  compact
                  onadd={() => openCharacterPicker(opt.team)}
                  addLabel={opt.label}
                />
              {/each}
            </div>
          {/if}

          <!-- Reminder tokens -->
          {#if game.reminderTokens.length > 0}
            <section>
              <h2 class="mb-3 text-lg font-semibold text-medium">
                Reminder Tokens
              </h2>
              <div class="flex flex-wrap gap-4">
                {#each game.reminderTokens as token}
                  {@const char = characterById.get(token.characterId)}
                  <ReminderToken
                    characterId={token.characterId}
                    characterName={token.characterName}
                    text={token.text}
                    edition={char?.edition ?? ""}
                    team={char?.team ?? Team.UNSPECIFIED}
                  />
                {/each}
              </div>
            </section>
          {/if}
        </div>
      {:else if activeTab === "nightorder"}
        <NightOrder
          {game}
          scriptCharacters={script?.characters ?? []}
          bluffs={game.selectedBluffCharacters}
        />
      {:else if activeTab === "grimoire"}
        <div
          class="-mx-4 h-[calc(100dvh-200px)] sm:mx-0 sm:rounded-lg sm:border sm:border-border overflow-hidden"
        >
          <GrimoireCanvas
            players={grimoirePlayers}
            reminders={grimoireReminders}
            onplayermove={handleGrimoirePlayerMove}
            onremindermove={handleGrimoireReminderMove}
            onplayerrename={handleGrimoirePlayerRename}
            onplayertoggledeath={handleGrimoirePlayerToggleDeath}
            onplayergamenote={handleGrimoireGameNote}
            onplayerroundnote={handleGrimoireRoundNote}
          />
        </div>
      {/if}
    {/if}
  </div>

  <!-- Character picker modal (setup only) -->
  {#if showCharacterPicker && isSetup}
    <CharacterPickerModal
      title={pickerTeam
        ? `Add ${teamLabels[pickerTeam] ?? "Character"}`
        : "Add Character"}
      characters={allCharacters}
      selectedIds={pickerSelectedIds}
      team={pickerTeam}
      onselect={handlePickerSelect}
      ondeselect={handlePickerDeselect}
      onclose={() => (showCharacterPicker = false)}
    />
  {/if}

  <!-- Bluff picker modal (setup only) -->
  {#if bluffPickerOpen && isSetup}
    <CharacterPickerModal
      title="Select Demon Bluff"
      characters={script?.characters ?? []}
      selectedIds={new Set(game.selectedBluffIds ?? [])}
      excludeIds={new Set([
        ...(game.selectedRoleIds ?? []),
        ...(game.extraCharacterIds ?? []),
      ])}
      excludeTeams={[
        Team.MINION,
        Team.DEMON,
        Team.TRAVELLER,
        Team.FABLED,
        Team.LORIC,
      ]}
      onselect={handleBluffSelect}
      ondeselect={(id) =>
        updateDemonBluffs(
          (game?.selectedBluffIds ?? []).filter((bid) => bid !== id),
        )}
      onclose={() => (bluffPickerOpen = false)}
    />
  {/if}

  <!-- Bag substitution picker modal (setup only) -->
  {#if bagSubPickerForRole && isSetup}
    <CharacterPickerModal
      title="Pick Townsfolk Token for Bag"
      characters={script?.characters ?? []}
      selectedIds={new Set()}
      excludeIds={selectedRoleIdSet}
      team={Team.TOWNSFOLK}
      excludeTeams={[
        Team.OUTSIDER,
        Team.MINION,
        Team.DEMON,
        Team.TRAVELLER,
        Team.FABLED,
        Team.LORIC,
      ]}
      onselect={(char) => setBagSubCharacter(bagSubPickerForRole!, char)}
      ondeselect={() => {}}
      onclose={() => (bagSubPickerForRole = null)}
    />
  {/if}

  <!-- Setup sidebar (setup tab + setup state only) -->
  {#if activeTab === "setup" && isSetup}
    <SetupSidebar
      gameId={game.id}
      selectedIds={[
        ...(game.selectedRoleIds ?? []),
        ...(game.extraCharacterIds ?? []),
      ]}
    />
  {/if}

  <!-- Confirm dialog -->
  {#if confirmDialog}
    <ConfirmDialog
      title={confirmDialog.title}
      message={confirmDialog.message}
      confirmLabel={confirmDialog.confirmLabel}
      cancelLabel={confirmDialog.cancelLabel}
      onconfirm={confirmDialog.onconfirm}
      oncancel={confirmDialog.oncancel}
    />
  {/if}
{/if}
