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
    Death,
  } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import {
    Team,
    GameState,
    PhaseType,
    TravellerAlignment,
    DeathCause,
  } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
  import { teamLabels, teamSingulars } from "~/lib/team-styles";
  import CharacterCard from "~/lib/components/CharacterCard.svelte";
  import CharacterPickerModal from "~/lib/components/CharacterPickerModal.svelte";
  import ConfirmDialog from "~/lib/components/ConfirmDialog.svelte";
  import DeathTracker from "~/lib/components/DeathTracker.svelte";
  import DistributionBar from "~/lib/components/DistributionBar.svelte";
  import GrimoireCanvas from "~/lib/components/grimoire/GrimoireCanvas.svelte";
  import {
    circleLayout,
    orbitPosition,
  } from "~/lib/components/grimoire/layout";
  import type {
    GrimoirePlayer,
    GrimoireReminder,
  } from "~/lib/components/grimoire/types";
  import NightOrder from "~/lib/components/NightOrder.svelte";
  import PhaseHeader from "~/lib/components/PhaseHeader.svelte";
  import ReminderToken from "~/lib/components/ReminderToken.svelte";
  import SetupSidebar from "~/lib/components/SetupSidebar.svelte";
  import TeamSection from "~/lib/components/TeamSection.svelte";
  import CharacterPreviewPopup from "~/lib/components/CharacterPreviewPopup.svelte";
  import PlayerPresetsModal from "~/lib/components/PlayerPresetsModal.svelte";
  import WakeLockToggle from "~/lib/components/WakeLockToggle.svelte";
  import SpotifyPanel from "~/lib/components/SpotifyPanel.svelte";
  import { spotify, syncPhase } from "~/lib/spotify.svelte";
  import PlayerAssignmentPanel from "~/lib/components/PlayerAssignmentPanel.svelte";
  import NameChipsBar from "~/lib/components/NameChipsBar.svelte";
  import InfoCardPicker from "~/lib/components/InfoCardPicker.svelte";
  import InfoCardDisplay from "~/lib/components/InfoCardDisplay.svelte";
  import StarPassPrompt from "~/lib/components/StarPassPrompt.svelte";
  import {
    getStartGameWarnings as computeStartGameWarnings,
    bluffCharactersInPlay,
    bluffCharactersShownByBagSubs,
    inPlayCharacterIds,
  } from "~/lib/game-warnings";
  import {
    assignNameInMap,
    unassignName,
    assignInOrder,
    shuffled,
    renameAssignedName,
  } from "~/lib/player-names";
  import {
    stableReminderIds,
    canonicalizeReminderKeys,
  } from "~/lib/components/grimoire/reminder-ids";
  import {
    bagSubDropTarget,
    bagSubDropHint,
  } from "~/lib/components/grimoire/bagsub";
  import { derivePlayerStatuses } from "~/lib/night-helpers/status";
  import { seatingOrder } from "~/lib/night-helpers/seating";
  import { effectiveAlignment } from "~/lib/night-helpers/alignment";
  import {
    findExecutedToday,
    newDeathsTonight,
  } from "~/lib/night-helpers/helpers";
  import type { HelperPlayer } from "~/lib/night-helpers/helpers";
  import { buildPromotionsByRole } from "~/lib/promotions";
  import type { NightHelperContext } from "~/lib/night-helpers/registry";
  import {
    generateStandardCards,
    type DisplayCard,
    type DisplayCharacter,
  } from "~/lib/info-cards";

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
  function exitFullscreenIfActive() {
    if (document.fullscreenElement) document.exitFullscreen().catch(() => {});
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

  // Bag substitutions whose shown token is ALSO a role in play (the "two Chefs"
  // collision) — keyed by caused_by_id, used to amber-flag the affected row.
  const bagSubCollisions = $derived.by(() => {
    const set = new Set<string>();
    if (!game) return set;
    const inPlay = new Set([
      ...(game.selectedRoleIds ?? []),
      ...(game.extraCharacterIds ?? []),
      ...(game.selectedTravellerIds ?? []),
    ]);
    for (const bs of game.bagSubstitutions ?? []) {
      if (bs.characterId && inPlay.has(bs.characterId)) set.add(bs.causedById);
    }
    return set;
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

  // Role-promotion overlay (star pass / Scarlet Woman): original role id -> how
  // that seat now displays ("Imp (ex Baron)", team Demon, …). Never renames a
  // grimoire key — it only overrides how the promoted seat reads downstream.
  const promotionsByRole = $derived(
    buildPromotionsByRole(game?.rolePromotions ?? [], characterById),
  );

  // --- Game state derived values ---
  const isSetup = $derived(game?.state === GameState.SETUP);
  const isInProgress = $derived(game?.state === GameState.IN_PROGRESS);
  const isCompleted = $derived(game?.state === GameState.COMPLETED);
  const canStartGame = $derived(
    isSetup && (game?.selectedRoleIds?.length ?? 0) > 0,
  );

  // Ensure the browser leaves fullscreen whenever the game becomes completed
  // (the completed view has no PhaseHeader / exit-fullscreen control).
  $effect(() => {
    if (isCompleted) exitFullscreenIfActive();
  });

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

  // The single active step (the server marks exactly one phase is_active). The
  // day/night pair of a round advances step-wise, so this tells us whether the
  // current round is presently on its Night or its Day.
  const activePhase = $derived(
    (game?.playState?.phases ?? []).find((p) => p.isActive),
  );
  const activeIsDay = $derived(activePhase?.type === PhaseType.DAY);

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

  // If the added character equals a bag substitution's shown token, clear that
  // token so the shown character can't double up in play (a "two Chefs" state).
  // Setup only — the RPC is setup-gated; in progress the drag hint + start
  // warning cover it. Runs after the add so `game` already reflects the new role.
  async function clearCollidingBagSub(addedId: string) {
    if (!game || !isSetup) return;
    const subs = game.bagSubstitutions ?? [];
    if (!subs.some((bs) => bs.characterId === addedId)) return;
    const updated = subs.map((bs) =>
      bs.characterId === addedId
        ? { ...bs, characterId: "", characterName: "" }
        : bs,
    );
    try {
      const resp = await client.updateBagSubstitutions({
        gameId: game.id,
        bagSubstitutions: updated,
      });
      game = resp.game;
      const name = characterById.get(addedId)?.name ?? addedId;
      const causedByName =
        subs.find((bs) => bs.characterId === addedId)?.causedByName ?? "Drunk";
      showSetupHint(
        `${name} was the ${causedByName}'s shown token — pick a new token for the ${causedByName}.`,
      );
    } catch (err) {
      error = getErrorMessage(err, "Failed to update bag substitution");
    }
  }

  async function toggleRole(id: string) {
    if (!game || (!isSetup && !isInProgress)) return;
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
    const isAdding = !selectedRoleIdSet.has(id);
    const newIds = isAdding
      ? [...game.selectedRoleIds, id]
      : game.selectedRoleIds.filter((rid) => rid !== id);
    try {
      const resp = await client.updateGameRoles({
        gameId: game.id,
        selectedRoleIds: newIds,
      });
      game = resp.game;
      if (isAdding) await clearCollidingBagSub(id);
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
      await clearCollidingBagSub(char.id);
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
    // A bag substitution's shown token (the character the Drunk believes they
    // are) acts "in play" from the players' perspective — never re-roll it in.
    for (const bs of game.bagSubstitutions ?? []) {
      if (bs.characterId) selectedIds.add(bs.characterId);
    }
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
      // Guard against a future regression where the RPC drops playState — keep
      // the current one so the in-progress view doesn't blank out.
      if (game?.playState && resp.game && !resp.game.playState)
        resp.game.playState = game.playState;
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
  // Demon bluffs that are actually in play (advisory — see game-warnings.ts).
  const bluffsInPlay = $derived(game ? bluffCharactersInPlay(game) : []);
  const bluffsShownByBagSubs = $derived(
    game ? bluffCharactersShownByBagSubs(game) : [],
  );

  function getStartGameWarnings(): string[] {
    if (!game) return [];
    return computeStartGameWarnings(game, currentDist);
  }

  async function doStartGame() {
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

  function startGame() {
    const warnings = getStartGameWarnings();
    if (warnings.length > 0) {
      confirmDialog = {
        title: "Start Game with Warnings",
        message: warnings.join("\n"),
        confirmLabel: "Start Anyway",
        cancelLabel: "Go Back",
        onconfirm: () => {
          confirmDialog = null;
          doStartGame();
        },
        oncancel: () => {
          confirmDialog = null;
        },
      };
      return;
    }
    doStartGame();
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
          exitFullscreenIfActive();
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
    cause: DeathCause = DeathCause.UNSPECIFIED,
  ) {
    if (!game) return;
    error = "";
    try {
      const resp = await client.recordDeath({
        gameId: game.id,
        roleId,
        phaseId,
        propagate,
        cause,
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
    // Day-phase kills come from the grimoire / death tracker — an execution.
    if (isViewingCurrent) {
      doRecordDeath(roleId, dayPhase.id, true, DeathCause.EXECUTION);
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
        doRecordDeath(roleId, dayPhase!.id, true, DeathCause.EXECUTION);
      },
      oncancel: () => {
        confirmDialog = null;
        doRecordDeath(roleId, dayPhase!.id, false, DeathCause.EXECUTION);
      },
    };
  }

  // Resolves to whether the removal reached the server, so callers with
  // follow-up writes (moveDeath) can gate on it.
  async function removeDeath(
    deathId: bigint,
    propagate = false,
  ): Promise<boolean> {
    if (!game) return false;
    error = "";
    try {
      const resp = await client.removeDeath({
        gameId: game.id,
        deathId,
        propagate,
      });
      game = resp.game;
      return true;
    } catch (err) {
      error = getErrorMessage(err, "Failed to remove death");
      return false;
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

  // Move a death to the sibling phase of the SAME round (Night N <-> Day N):
  // remove it, then re-record on the sibling with a phase-appropriate cause
  // (execution on a day; DEMON preserved, else UNSPECIFIED, on a night).
  async function moveDeath(death: Death) {
    if (!game) return;
    const phase = (game.playState?.phases ?? []).find(
      (p) => p.id === death.phaseId,
    );
    if (!phase) return;
    const round = rounds.find((r) => r.roundNumber === phase.roundNumber);
    if (!round) return;
    const sibling = phase.type === PhaseType.NIGHT ? round.day : round.night;
    if (!sibling) return;
    const cause =
      sibling.type === PhaseType.DAY
        ? DeathCause.EXECUTION
        : death.cause === DeathCause.DEMON
          ? DeathCause.DEMON
          : DeathCause.UNSPECIFIED;
    // Only re-record once the removal reached the server — otherwise the
    // death would end up on BOTH sibling phases.
    const removed = await removeDeath(death.id, true);
    if (removed) await doRecordDeath(death.roleId, sibling.id, true, cause);
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
  let previewCharacter = $state<
    import("~/lib/gen/clockkeeper/v1/clockkeeper_pb").Character | null
  >(null);

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

  // Auto-switch the in-progress view when the active STEP transitions: the Day
  // step defaults to the grimoire (executions are recorded there via the death
  // toggle), the Night step to the night sheet. Tracks the previous step so it
  // fires only on a transition and never fights a manual toggle within a step.
  let prevActiveIsDay = $state<boolean | undefined>(undefined);
  $effect(() => {
    const day = activeIsDay;
    untrack(() => {
      if (prevActiveIsDay === day) return;
      prevActiveIsDay = day;
      inProgressView = day ? "grimoire" : "nightsheet";
    });
  });

  // Flip the Spotify playlist on the same day/night step transition. Tracked
  // independently of the view toggle above so neither can swallow the other's
  // transition. syncPhase() no-ops unless the Storyteller has already started
  // playback, so this never begins music on its own; the tracker is also reset
  // outside the in-progress state so entering play is not seen as a flip.
  let prevMusicIsDay = $state<boolean | undefined>(undefined);
  $effect(() => {
    const inProgress = isInProgress;
    const day = activeIsDay;
    untrack(() => {
      if (!inProgress) {
        prevMusicIsDay = undefined;
        return;
      }
      if (prevMusicIsDay === day) return;
      const isTransition = prevMusicIsDay !== undefined;
      prevMusicIsDay = day;
      if (isTransition) void syncPhase(day);
    });
  });

  // --- Grimoire state (persisted per game) ---
  let grimoirePositions = $state(new Map<string, { x: number; y: number }>());
  let grimoireNames = $state(new Map<string, string>());
  let reminderPositions = $state(new Map<string, { x: number; y: number }>());
  let reminderAttachments = $state(
    new Map<string, { playerId: string; angle: number }>(),
  );
  let grimoireGameNotes = $state(new Map<string, string>());
  let grimoireRoundNotes = $state(new Map<string, string>());
  let grimoireInitialized = $state(false);

  // Transient banner shown over the grimoire canvas for non-actionable bag-sub
  // drops (wrong team / not in play) or reassignment errors. Auto-clears.
  let grimoireHint = $state("");
  let grimoireHintTimeout: ReturnType<typeof setTimeout> | undefined;
  function showGrimoireHint(msg: string) {
    grimoireHint = msg;
    clearTimeout(grimoireHintTimeout);
    grimoireHintTimeout = setTimeout(() => (grimoireHint = ""), 3000);
  }

  // Transient banner shown near the setup roles grid, e.g. when adding a role
  // that collided with (and cleared) the Drunk's shown token. Auto-clears.
  let setupHint = $state("");
  let setupHintTimeout: ReturnType<typeof setTimeout> | undefined;
  function showSetupHint(msg: string) {
    setupHint = msg;
    clearTimeout(setupHintTimeout);
    setupHintTimeout = setTimeout(() => (setupHint = ""), 4000);
  }

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

    const tokens = game?.reminderTokens ?? [];

    // Load all persisted positions, separating player vs reminder. Reminder
    // keys are both `reminder-*` (per-token) and `bagsub-reminder-*` (the
    // synthesized "Is the Drunk" token) — the latter used to fall through to
    // player positions here, which misrouted it; route it to reminders now.
    for (const [id, pos] of Object.entries(serverPositions)) {
      if (id.startsWith("reminder-") || id.startsWith("bagsub-reminder-")) {
        newReminderPositions.set(id, { x: pos.x, y: pos.y });
      } else {
        newPositions.set(id, { x: pos.x, y: pos.y });
      }
    }
    // Lazy migration: canonicalize legacy positional `reminder-<n>` keys to the
    // stable `reminder-<charId>-<n>` scheme (bagsub / stable keys pass through).
    const canonReminderPositions = canonicalizeReminderKeys(
      newReminderPositions,
      tokens,
    );

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

    // Fill gaps for reminders without persisted positions (horizontal line at
    // bottom), keyed by the token's stable id.
    if (tokens.length > 0) {
      const stableIds = stableReminderIds(tokens);
      const reminderY = 400;
      const totalWidth = tokens.length * 80;
      const startX = -totalWidth / 2 + 40;
      for (let i = 0; i < tokens.length; i++) {
        const rid = stableIds[i];
        if (!canonReminderPositions.has(rid)) {
          canonReminderPositions.set(rid, { x: startX + i * 80, y: reminderY });
        }
      }
    }

    // Load persisted notes
    const serverGameNotes = game?.grimoireGameNotes ?? {};
    const serverRoundNotes = game?.grimoireRoundNotes ?? {};

    // Load persisted reminder attachments (encoded as "playerId:angle")
    const serverAttachments = game?.grimoireReminderAttachments ?? {};
    const newAttachments = new Map<
      string,
      { playerId: string; angle: number }
    >();
    for (const [rid, encoded] of Object.entries(serverAttachments)) {
      const colonIdx = encoded.lastIndexOf(":");
      if (colonIdx > 0) {
        const playerId = encoded.slice(0, colonIdx);
        const angle = parseFloat(encoded.slice(colonIdx + 1));
        if (!isNaN(angle)) {
          newAttachments.set(rid, { playerId, angle });
        }
      }
    }
    // Same lazy migration for attachment keys.
    const canonAttachments = canonicalizeReminderKeys(newAttachments, tokens);

    grimoirePositions = newPositions;
    reminderPositions = canonReminderPositions;
    reminderAttachments = canonAttachments;
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
    return chars.map((c, i) => {
      const pos = grimoirePositions.get(c.id) ?? { x: 0, y: 0 };
      const death = deathByRole.get(c.id);
      // A promotion wins over a bag substitution (they can't coexist): a promoted
      // seat reads as its acts-as character/team/edition and "Imp (ex Baron)".
      const promo = promotionsByRole.get(c.id);
      const sub = promo ? undefined : bagSubByRole.get(c.id);
      const displayCharId = promo ? promo.actsAsId : sub?.characterId || c.id;
      const displayCharName = promo
        ? promo.label
        : sub?.characterName || c.name;
      const displayTeam = promo ? promo.actsAsTeam : c.team;
      const displayEdition = promo ? promo.actsAsEdition : c.edition;
      return {
        id: c.id,
        name: grimoireNames.get(c.id) ?? `Player ${i + 1}`,
        characterId: displayCharId,
        characterName: displayCharName,
        team: displayTeam,
        edition: displayEdition,
        x: pos.x,
        y: pos.y,
        // Dead set follows the ACTIVE phase so the grimoire skull toggle and the
        // displayed dead state always agree (day = executions, night = kills).
        isDead: (activeIsDay ? dayDeadRoleIds : nightDeadRoleIds).has(c.id),
        ghostVoteUsed: death ? !death.ghostVote : false,
        gameNote: grimoireGameNotes.get(c.id) ?? "",
        roundNote: currentRoundNotes.get(c.id) ?? "",
        alignment: dayAlignments.get(c.id) as "good" | "evil" | undefined,
      };
    });
  });

  // Derive grimoire reminders from game data + local state
  const grimoireReminders = $derived.by((): GrimoireReminder[] => {
    if (!game) return [];
    const stableIds = stableReminderIds(game.reminderTokens ?? []);
    const reminders: GrimoireReminder[] = (game.reminderTokens ?? []).map(
      (token, i) => {
        const rid = stableIds[i];
        const char = characterById.get(token.characterId);
        const attachment = reminderAttachments.get(rid);
        let pos: { x: number; y: number };
        if (attachment) {
          const playerPos = grimoirePositions.get(attachment.playerId);
          if (playerPos) {
            pos = orbitPosition(playerPos.x, playerPos.y, attachment.angle);
          } else {
            pos = reminderPositions.get(rid) ?? { x: 0, y: 0 };
          }
        } else {
          pos = reminderPositions.get(rid) ?? { x: 0, y: 0 };
        }
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
          attachedTo: attachment?.playerId,
          orbitAngle: attachment?.angle,
        };
      },
    );

    // Auto-add reminder tokens for bag substitutions (e.g., "Is the Drunk")
    for (const bs of game.bagSubstitutions ?? []) {
      if (!bs.characterId) continue;
      const rid = `bagsub-reminder-${bs.causedById}`;
      const causedByChar = characterById.get(bs.causedById);
      const attachment = reminderAttachments.get(rid);
      let pos: { x: number; y: number };
      if (attachment) {
        const playerPos = grimoirePositions.get(attachment.playerId);
        if (playerPos) {
          pos = orbitPosition(playerPos.x, playerPos.y, attachment.angle);
        } else {
          pos = reminderPositions.get(rid) ?? { x: 0, y: 0 };
        }
      } else {
        // Default: attach to the player this substitution belongs to
        const playerPos = grimoirePositions.get(bs.causedById);
        if (playerPos) {
          pos = orbitPosition(playerPos.x, playerPos.y, Math.PI * 0.25);
        } else {
          pos = reminderPositions.get(rid) ?? { x: 0, y: 0 };
        }
      }
      reminders.push({
        id: rid,
        characterId: bs.causedById,
        characterName: bs.causedByName,
        text: `Is the ${bs.causedByName}`,
        team: causedByChar?.team ?? Team.UNSPECIFIED,
        edition: causedByChar?.edition ?? "",
        x: pos.x,
        y: pos.y,
        attachedTo: attachment?.playerId ?? bs.causedById,
        orbitAngle: attachment?.angle ?? Math.PI * 0.25,
      });
    }

    return reminders;
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
        const encodedAttachments: Record<string, string> = {};
        for (const [rid, att] of reminderAttachments) {
          encodedAttachments[rid] = `${att.playerId}:${att.angle}`;
        }
        await client.updateGrimoireState({
          gameId: game.id,
          positions: allPositions,
          playerNames: Object.fromEntries(grimoireNames),
          gameNotes: Object.fromEntries(grimoireGameNotes),
          roundNotes: Object.fromEntries(grimoireRoundNotes),
          reminderAttachments: encodedAttachments,
        });
      } catch (err) {
        console.error("Failed to save grimoire state", err);
      }
    }, 500);
  }

  // Grimoire event handlers
  function handleGrimoirePlayerMove(id: string, x: number, y: number) {
    grimoirePositions = new Map(grimoirePositions.set(id, { x, y }));
    // Attached reminders follow — their positions are derived, so just trigger reactivity
    saveGrimoireState();
  }
  function handleGrimoireReminderMove(id: string, x: number, y: number) {
    reminderPositions = new Map(reminderPositions.set(id, { x, y }));
    saveGrimoireState();
  }
  function handleReminderAttach(
    reminderId: string,
    playerId: string,
    angle: number,
  ) {
    reminderAttachments = new Map(
      reminderAttachments.set(reminderId, { playerId, angle }),
    );
    // Clear the free-floating position since it's now orbit-derived
    reminderPositions.delete(reminderId);
    reminderPositions = new Map(reminderPositions);
    saveGrimoireState();
  }
  function handleReminderDetach(reminderId: string) {
    // Compute current position from orbit before detaching
    const attachment = reminderAttachments.get(reminderId);
    if (attachment) {
      const playerPos = grimoirePositions.get(attachment.playerId);
      if (playerPos) {
        const pos = orbitPosition(playerPos.x, playerPos.y, attachment.angle);
        reminderPositions.set(reminderId, pos);
      }
    }
    reminderAttachments.delete(reminderId);
    reminderAttachments = new Map(reminderAttachments);
    reminderPositions = new Map(reminderPositions);
    saveGrimoireState();
  }
  // Map a bag substitution's stored team string to the Team enum. Defaults to
  // Townsfolk (the only bag sub today — the Drunk — is a Townsfolk token).
  function teamFromLabel(label: string | undefined): Team | undefined {
    switch ((label ?? "").toLowerCase()) {
      case "townsfolk":
        return Team.TOWNSFOLK;
      case "outsider":
        return Team.OUTSIDER;
      case "minion":
        return Team.MINION;
      case "demon":
        return Team.DEMON;
      default:
        return undefined;
    }
  }

  // Drag of the synthesized "Is the {Drunk}" token onto a DIFFERENT seat.
  // Validates the target, then either plainly re-attaches (self), bounces with a
  // transient hint (invalid), or confirms + reassigns the real roles (ok).
  function handleBagSubDrop(
    reminderId: string,
    targetPlayerId: string,
    angle: number,
  ) {
    if (!game || (!isSetup && !isInProgress)) return;
    const causedById = reminderId.slice("bagsub-reminder-".length);
    const bs = (game.bagSubstitutions ?? []).find(
      (b) => b.causedById === causedById,
    );
    const requiredTeam = teamFromLabel(bs?.team) ?? Team.TOWNSFOLK;
    // Single in-play definition shared with bagSubCollisions and the
    // start-game warnings (roles + extras + travellers).
    const selectedRoleIds = inPlayCharacterIds(game);
    const verdict = bagSubDropTarget(
      targetPlayerId,
      causedById,
      selectedRoleIds,
      (id) => characterById.get(id)?.team,
      requiredTeam,
      bs?.characterId,
    );

    const causedByName =
      bs?.causedByName ?? characterById.get(causedById)?.name ?? "role";

    if (verdict === "self") {
      handleReminderAttach(reminderId, targetPlayerId, angle);
      return;
    }
    if (verdict !== "ok") {
      showGrimoireHint(
        bagSubDropHint(
          verdict,
          causedByName,
          teamSingulars[requiredTeam] ?? "Townsfolk",
          bs?.characterName,
        ),
      );
      return;
    }

    // Valid reassignment — confirm before mutating real roles.
    const targetName =
      grimoireNames.get(targetPlayerId) ||
      characterById.get(targetPlayerId)?.name ||
      "that player";
    const shownName = bs?.characterName || "their shown character";
    confirmDialog = {
      title: `Reassign the ${causedByName}`,
      message: `${targetName} becomes the ${causedByName}, keeping their current character token. The previous ${causedByName} becomes the ${shownName}.`,
      confirmLabel: `Make ${targetName} the ${causedByName}`,
      cancelLabel: "Cancel",
      onconfirm: async () => {
        confirmDialog = null;
        if (!game) return;
        // Cancel any pending debounced save so it can't clobber the remap.
        clearTimeout(grimoireSaveTimeout);
        error = "";
        try {
          const resp = await client.reassignBagSubstitution({
            gameId: game.id,
            causedById,
            targetRoleId: targetPlayerId,
          });
          // Re-init local maps from the server-remapped state.
          grimoireInitialized = false;
          game = resp.game;
        } catch (err) {
          showGrimoireHint(
            getErrorMessage(err, "Failed to reassign the substitution"),
          );
        }
      },
      oncancel: () => {
        confirmDialog = null;
      },
    };
  }

  function handleGrimoirePlayerRename(id: string, name: string) {
    // Empty/whitespace name unassigns the seat (deletes the key) instead of
    // storing an empty string; preset-name duplicates steal from other seats.
    grimoireNames = assignNameInMap(grimoireNames, id, name, presetNames);
    saveGrimoireState();
  }
  function handleGrimoirePlayerToggleDeath(id: string) {
    // Route to the ACTIVE phase: a day-phase toggle is an execution, a
    // night-phase toggle is a night kill (cause UNSPECIFIED).
    if (activeIsDay) {
      if (dayDeadRoleIds.has(id)) undoDeathByRoleOnDay(id);
      else recordDeathOnDay(id);
    } else {
      if (nightDeadRoleIds.has(id)) undoDeathByRoleOnNight(id);
      else recordDeathOnNight(id);
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

  // --- State-aware night sheet (Feature E) ---
  // Poisoned/drunk derived from reminder-token attachments (client-owned state;
  // see night-helpers/status.ts for the single-source-of-truth escape hatch).
  const playerStatuses = $derived(
    derivePlayerStatuses(grimoireReminders, game?.bagSubstitutions ?? []),
  );

  // Reverse bag-sub map: shown character id (night entry) -> underlying seat id.
  const bagSubEntryToSeat = $derived.by(() => {
    const map = new Map<string, string>();
    for (const bs of game?.bagSubstitutions ?? []) {
      if (bs.characterId) map.set(bs.characterId, bs.causedById);
    }
    return map;
  });

  // Night-scoped players: NIGHT deaths + NIGHT alignments (distinct from the
  // day-scoped versions used by the grimoire).
  const nightHelperPlayers = $derived.by(() => {
    const map = new Map<string, HelperPlayer>();
    if (!game) return map;
    const chars = [
      ...(game.selectedCharacters ?? []),
      ...(game.selectedTravellerCharacters ?? []),
    ];
    for (const c of chars) {
      // Promotion wins over bag substitution: a promoted seat classifies as its
      // acts-as character (team Demon), so it drops out of the Minion-only star
      // pass list and registers as the Demon to Fortune Teller / Scarlet Woman.
      const promo = promotionsByRole.get(c.id);
      const sub = promo ? undefined : bagSubByRole.get(c.id);
      map.set(c.id, {
        id: c.id,
        name: grimoireNames.get(c.id) ?? "",
        characterId: promo ? promo.actsAsId : sub?.characterId || c.id,
        characterName: promo ? promo.label : sub?.characterName || c.name,
        team: promo ? promo.actsAsTeam : c.team,
        edition: promo ? promo.actsAsEdition : c.edition,
        isDead: nightDeadRoleIds.has(c.id),
        alignment: effectiveAlignment(
          c.id,
          c.team,
          nightAlignments,
          game.travellerAlignments,
        ),
      });
    }
    return map;
  });

  // Clockwise seating order derived from grimoire positions.
  const seatingIds = $derived.by(() => {
    if (!game) return [] as string[];
    const chars = [
      ...(game.selectedCharacters ?? []),
      ...(game.selectedTravellerCharacters ?? []),
    ];
    const ids = chars.map((c) => c.id);
    const positions = new Map<string, { x: number; y: number }>();
    for (const c of chars) {
      positions.set(c.id, grimoirePositions.get(c.id) ?? { x: 0, y: 0 });
    }
    return seatingOrder(positions, ids);
  });

  // Fortune Teller's Red Herring token holder, if the token is attached.
  const redHerringPlayerId = $derived(
    grimoireReminders.find(
      (r) =>
        r.characterId === "fortuneteller" &&
        r.text.trim().toLowerCase() === "red herring",
    )?.attachedTo,
  );

  // Player executed on the previous day (for the Undertaker).
  const executedToday = $derived.by(
    (): { player: HelperPlayer; heuristic: boolean } | undefined => {
      const prev = rounds[viewingRoundIndex - 1];
      const found = findExecutedToday(prev);
      if (!found) return undefined;
      const player = nightHelperPlayers.get(found.roleId);
      if (!player) return undefined;
      return { player, heuristic: found.heuristic };
    },
  );

  // Ephemeral Fortune Teller picks, keyed by night phase id (lost on reload).
  let ftPicksByPhase = $state(new Map<bigint, string[]>());

  // First-night info picks (Washerwoman/Librarian/Investigator) are NOT
  // ephemeral: they ARE the grimoire reminder-token attachments. Each helper's
  // team token text equals the team's singular label (roles.json reminders);
  // the decoy uses the shared "Wrong" token.
  const INFO_ROLE_TEAM_LABEL: Record<string, string> = {
    washerwoman: "Townsfolk",
    librarian: "Outsider",
    investigator: "Minion",
  };
  const INFO_WRONG_TEXT = "Wrong";
  // Default orbit angle for auto-attached info tokens; matches the previous
  // "Attach tokens" behaviour and avoids the bag-sub token's 0.25π default.
  const INFO_ATTACH_ANGLE = Math.PI * 0.75;
  // Orbit angle for tokens auto-attached by the reminder-token pickers (Fortune
  // Teller Red Herring, Poisoner, Butler). Distinct from the bag-sub default
  // (0.25π) and the first-night info default (0.75π) so tokens don't stack.
  const HELPER_TOKEN_ANGLE = Math.PI * 1.25;

  // Resolve a reminder token's STABLE id from (characterId, text). Shared by the
  // pick derivation and the attach/detach writer so both agree on the id.
  function stableIdForToken(
    characterId: string,
    text: string,
  ): string | undefined {
    if (!game) return undefined;
    const tokens = game.reminderTokens ?? [];
    const idx = tokens.findIndex(
      (t) => t.characterId === characterId && t.text === text,
    );
    if (idx < 0) return undefined;
    return stableReminderIds(tokens)[idx];
  }

  // Derive info picks from the current reminder attachments. A slot is "picked"
  // iff its matching token is attached to a seat, so manual grimoire
  // attach/detach shows up here automatically and picks survive reloads.
  const infoPicks = $derived.by(() => {
    const map = new Map<string, { rightId?: string; wrongId?: string }>();
    if (!game) return map;
    for (const [charId, teamLabel] of Object.entries(INFO_ROLE_TEAM_LABEL)) {
      if (!selectedRoleIdSet.has(charId)) continue; // only in-play info roles
      const rightSid = stableIdForToken(charId, teamLabel);
      const wrongSid = stableIdForToken(charId, INFO_WRONG_TEXT);
      const rightId = rightSid
        ? reminderAttachments.get(rightSid)?.playerId
        : undefined;
      const wrongId = wrongSid
        ? reminderAttachments.get(wrongSid)?.playerId
        : undefined;
      if (rightId || wrongId) map.set(charId, { rightId, wrongId });
    }
    return map;
  });

  // Apply a pick change by attaching/detaching the matching token. Only the
  // slots that actually changed are touched, so a manual grimoire edit to the
  // other slot is never clobbered.
  function setInfoPick(
    charId: string,
    picks: { rightId?: string; wrongId?: string },
  ) {
    const teamLabel = INFO_ROLE_TEAM_LABEL[charId];
    if (!teamLabel) return;
    const cur = infoPicks.get(charId) ?? {};
    applyInfoSlot(charId, teamLabel, cur.rightId, picks.rightId);
    applyInfoSlot(charId, INFO_WRONG_TEXT, cur.wrongId, picks.wrongId);
  }

  function applyInfoSlot(
    charId: string,
    text: string,
    curId: string | undefined,
    nextId: string | undefined,
  ) {
    if (curId === nextId) return;
    const sid = stableIdForToken(charId, text);
    if (!sid) {
      // Only reachable if a script's reminder texts diverge from the official
      // ones this helper keys on — the pick would otherwise vanish silently.
      console.warn(
        `info helper: no "${text}" reminder token found for ${charId}; pick not persisted`,
      );
      return;
    }
    if (nextId) handleReminderAttach(sid, nextId, INFO_ATTACH_ANGLE);
    else handleReminderDetach(sid);
  }

  // The DISPLAYED character of a seat (bag-sub aware): a substituted seat shows
  // its shown character (e.g. the Drunk shown as the Empath), else its own role.
  function displayedCharacterOf(playerId: string) {
    if (!game) return undefined;
    // Promotions win over bag substitutions: a promoted seat shows its acts-as
    // character (e.g. a Ravenkeeper learns a promoted Baron as the Imp).
    const promo = promotionsByRole.get(playerId);
    if (promo) {
      return {
        id: promo.actsAsId,
        name: promo.actsAsName,
        edition: promo.actsAsEdition,
        team: promo.actsAsTeam,
      };
    }
    const sub = bagSubByRole.get(playerId);
    if (sub?.characterId) {
      const c = characterById.get(sub.characterId);
      if (c)
        return { id: c.id, name: c.name, edition: c.edition, team: c.team };
      return {
        id: sub.characterId,
        name: sub.characterName,
        edition: "",
        team: Team.UNSPECIFIED,
      };
    }
    const own = characterById.get(playerId);
    if (own)
      return {
        id: own.id,
        name: own.name,
        edition: own.edition,
        team: own.team,
      };
    return undefined;
  }

  // Seats whose death was FIRST recorded in the current night phase (not carried
  // forward from a previous round). Drives the Ravenkeeper wake.
  const diedTonight = $derived(
    newDeathsTonight(
      nightPhase?.deaths ?? [],
      rounds[viewingRoundIndex - 1]?.day?.deaths ?? [],
    ),
  );

  const nightHelperContext = $derived.by((): NightHelperContext | undefined => {
    if (!nightPhase) return undefined;
    const phaseId = nightPhase.id;
    return {
      night: (viewingRound?.roundNumber ?? 1) === 1 ? "first" : "other",
      order: seatingIds,
      players: nightHelperPlayers,
      statuses: playerStatuses,
      playerIdForEntry: (entryId: string) => {
        const seat = bagSubEntryToSeat.get(entryId) ?? entryId;
        return nightHelperPlayers.has(seat) ? seat : undefined;
      },
      redHerringPlayerId,
      executedToday,
      ftPicks: ftPicksByPhase.get(phaseId) ?? [],
      onftpick: (picks: string[]) => {
        const next = new Map(ftPicksByPhase);
        next.set(phaseId, picks);
        ftPicksByPhase = next;
      },
      // First-night info helpers (Washerwoman / Librarian / Investigator).
      displayedCharacterOf,
      infoPicks,
      oninfopick: setInfoPick,
      onshowcard: (card: DisplayCard) => showInfoCard(card),
      // Reminder-token pickers (Fortune Teller Red Herring, Poisoner, Butler):
      // picking IS attaching the token, so it stays in sync with manual
      // grimoire edits and persists across reloads.
      onattachtoken: (
        characterId: string,
        tokenText: string,
        playerId: string | undefined,
      ) => {
        const sid = stableIdForToken(characterId, tokenText);
        if (!sid) return;
        if (playerId) handleReminderAttach(sid, playerId, HELPER_TOKEN_ANGLE);
        else handleReminderDetach(sid);
      },
      tokenHolder: (characterId: string, tokenText: string) => {
        const sid = stableIdForToken(characterId, tokenText);
        return sid ? reminderAttachments.get(sid)?.playerId : undefined;
      },
      scriptCharacters: (script?.characters ?? []).map((c) => ({
        id: c.id,
        name: c.name,
        team: c.team,
        edition: c.edition,
      })),
      // Event-driven / promotion helpers (Ravenkeeper wake, Scarlet Woman promote).
      diedTonight,
      onstarpass: openStarPass,
    };
  });

  // Demon kill: record the victim's death on the night phase (cause DEMON) and
  // mark the demon's night action complete.
  async function handleDemonKill(demonRoleId: string, victimRoleId: string) {
    if (!game || !nightPhase) return;
    await doRecordDeath(victimRoleId, nightPhase.id, true, DeathCause.DEMON);
    await toggleNightAction(demonRoleId, true);
    // Imp self-kill triggers a Star Pass — a Minion becomes the new Imp. Works
    // for a promoted seat too (a Baron acting as the Imp killing itself).
    const actsAs = promotionsByRole.get(demonRoleId)?.actsAsId ?? demonRoleId;
    if (victimRoleId === demonRoleId && actsAs === "imp") openStarPass();
  }

  // --- Star pass (Imp self-kill) ---
  let starPassOpen = $state(false);

  // Alive, in-play Minions by their REAL role — candidates to become the Imp.
  const starPassMinions = $derived(
    [...nightHelperPlayers.values()].filter(
      (p) => p.team === Team.MINION && !p.isDead,
    ),
  );

  function openStarPass() {
    starPassOpen = true;
  }

  async function doStarPass(minionRoleId: string) {
    if (!game) return;
    starPassOpen = false;
    error = "";
    try {
      // A star pass now only APPENDS a promotion marker — no seat renames, so no
      // grimoire-key rehydration or save-cancel dance is needed (unlike bag subs).
      const resp = await client.starPass({ gameId: game.id, minionRoleId });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to perform star pass");
    }
  }

  async function doUndoStarPass(roleId: string) {
    if (!game) return;
    error = "";
    try {
      const resp = await client.undoStarPass({ gameId: game.id, roleId });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to undo star pass");
    }
  }

  // --- Info cards (Feature C) ---
  let infoCardPickerOpen = $state(false);
  let activeInfoCard = $state<{
    card: DisplayCard;
    character?: DisplayCharacter;
  } | null>(null);

  function showInfoCard(
    card: DisplayCard,
    character?: DisplayCharacter | null,
  ) {
    activeInfoCard = { card, character: character ?? undefined };
    infoCardPickerOpen = false;
  }

  // Show a standard card by its "std:*" id (night-sheet shortcut buttons).
  function showStandardCardById(cardId: string) {
    if (!game) return;
    const card = generateStandardCards(game).find((c) => c.id === cardId);
    if (card) activeInfoCard = { card };
  }

  // --- Player presets ---
  let showPlayerPresets = $state(false);
  let showNameChips = $state(false);
  let presetNames = $state<string[]>([]);
  let selectedChipName = $state<string | null>(null);
  let presetsLoaded = $state(false);

  async function loadPresets() {
    try {
      const resp = await client.getPlayerPresets({});
      presetNames = [...resp.names];
    } catch {
      // silently fail
    }
  }

  // Load presets eagerly during setup so the assignment panel is ready.
  $effect(() => {
    if (isSetup && !presetsLoaded) {
      presetsLoaded = true;
      loadPresets();
    }
  });

  // Seats (roles in play) shown in the setup Players panel.
  const assignmentPanelPlayers = $derived.by(() => {
    if (!game) return [];
    const chars = [
      ...(game.selectedCharacters ?? []),
      ...(game.selectedTravellerCharacters ?? []),
    ];
    return chars.map((c) => ({
      id: c.id,
      characterName: c.name,
      edition: c.edition,
      team: c.team,
      name: grimoireNames.get(c.id),
    }));
  });

  function handleAssignPresets(names: string[]) {
    if (!game) return;
    const chars = [
      ...(game.selectedCharacters ?? []),
      ...(game.selectedTravellerCharacters ?? []),
    ];
    grimoireNames = assignInOrder(
      grimoireNames,
      chars.map((c) => c.id),
      names,
    );
    saveGrimoireState();
  }

  function assignNameToPlayer(playerId: string, name: string) {
    grimoireNames = assignNameInMap(grimoireNames, playerId, name, presetNames);
    saveGrimoireState();
    selectedChipName = null;
  }

  function unassignPlayerName(playerId: string) {
    grimoireNames = unassignName(grimoireNames, playerId);
    saveGrimoireState();
  }

  function clearAllPlayerNames() {
    grimoireNames = new Map();
    saveGrimoireState();
  }

  // Unassign whichever seat currently holds `name` (used by the name chip bar).
  function unassignNameByValue(name: string) {
    for (const [id, existingName] of grimoireNames) {
      if (existingName === name) {
        unassignPlayerName(id);
        return;
      }
    }
  }

  // Set of assigned name values (for the name chip bar's used-name detection).
  const assignedNameValues = $derived(new Set(grimoireNames.values()));

  function handleChipTap(name: string) {
    if (selectedChipName === name) {
      selectedChipName = null;
    } else {
      selectedChipName = name;
    }
  }

  function handlePlayerTapForAssign(playerId: string) {
    if (selectedChipName) {
      assignNameToPlayer(playerId, selectedChipName);
    }
  }

  // --- Player count ---
  async function updatePlayerCount(delta: number) {
    if (!game || !isSetup) return;
    const newCount = game.playerCount + delta;
    if (newCount < 5 || newCount > 15) return;
    error = "";
    try {
      const resp = await client.updatePlayerCount({
        gameId: game.id,
        playerCount: newCount,
      });
      game = resp.game;
    } catch (err) {
      error = getErrorMessage(err, "Failed to update player count");
    }
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
  <div
    class="space-y-6 {isFullscreen ? 'pb-0' : 'pb-40 2xl:pb-0'} {isSetup
      ? '2xl:mr-72'
      : ''}"
  >
    <!-- Header -->
    {#if !isFullscreen || isCompleted}
      <div
        class="no-print {isSetup
          ? 'sticky top-[57px] z-10 bg-surface border border-border rounded-lg px-4 pt-2 pb-2 shadow-sm'
          : ''}"
      >
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
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
                  class="text-2xl font-bold text-primary bg-transparent border-b-2 border-indigo-500 outline-none min-w-0 max-w-md"
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
                  class="shrink-0 whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium {stateBadge.class}"
                  >{stateBadge.label}</span
                >
              {/if}
            </div>
            <div class="mt-1 flex items-center gap-1 text-secondary">
              {#if isSetup}
                <button
                  onclick={() => updatePlayerCount(-1)}
                  disabled={game.playerCount <= 5}
                  class="rounded p-0.5 transition-colors hover:bg-hover disabled:opacity-30 disabled:cursor-default"
                  aria-label="Decrease player count"
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
                      d="M20 12H4"
                    /></svg
                  >
                </button>
              {/if}
              <span>{game.playerCount} players</span>
              {#if isSetup}
                <button
                  onclick={() => updatePlayerCount(1)}
                  disabled={game.playerCount >= 15}
                  class="rounded p-0.5 transition-colors hover:bg-hover disabled:opacity-30 disabled:cursor-default"
                  aria-label="Increase player count"
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
                      d="M12 4v16m8-8H4"
                    /></svg
                  >
                </button>
              {/if}
              {#if game.travellerCount > 0}
                <span
                  >+ {game.travellerCount}
                  {game.travellerCount === 1 ? "traveller" : "travellers"}
                  = {game.playerCount + game.travellerCount} total</span
                >
              {/if}
            </div>
          </div>
          <div class="flex flex-wrap items-center gap-2">
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
            {#if isInProgress}
              <button
                onclick={() => openCharacterPicker()}
                class="rounded-lg border border-border px-3 py-2.5 text-sm font-medium text-secondary transition-colors hover:bg-hover hover:text-primary"
                title="Add character"
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
                    d="M12 4v16m8-8H4"
                  /></svg
                >
              </button>
            {/if}
            <WakeLockToggle />
            {#if spotify.available}
              <SpotifyPanel {activeIsDay} />
            {/if}
            <button
              onclick={() => {
                showNameChips = !showNameChips;
                if (showNameChips && presetNames.length === 0) loadPresets();
              }}
              class="rounded-lg border border-border px-3 py-2.5 text-sm font-medium transition-colors {showNameChips
                ? 'bg-indigo-100 border-indigo-300 text-indigo-600 dark:bg-indigo-500/20 dark:border-indigo-600 dark:text-indigo-400'
                : 'text-secondary hover:bg-hover hover:text-primary'}"
              title="Player names"
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
                  d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
                /></svg
              >
            </button>
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
            {#if isSetup && activeTab === "setup"}
              <button
                onclick={randomize}
                disabled={randomizing}
                class="rounded-lg border border-indigo-500 px-4 py-2.5 text-sm font-medium text-indigo-500 transition-colors hover:bg-indigo-500 hover:text-white disabled:opacity-50"
              >
                {randomizing ? "Randomizing..." : "Randomize Roles"}
              </button>
            {/if}
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
        <!-- Tab bar (setup only, inside sticky wrapper) -->
        {#if isSetup}
          <div class="mt-4 flex gap-1 rounded-lg bg-element p-1">
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
          activeView={inProgressView}
          onviewchange={(v) => (inProgressView = v)}
          {isFullscreen}
          ontogglefullscreen={toggleFullscreen}
          onshowcards={() => (infoCardPickerOpen = true)}
          dayActive={activeIsDay}
        />

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
            bagSubstitutions={game.bagSubstitutions}
            playerNames={grimoireNames}
            bluffs={game.selectedBluffCharacters}
            onalignment={handleNightSheetAlignment}
            oneditbluffs={openBluffPicker}
            onshowcard={showStandardCardById}
            {playerStatuses}
            helperContext={nightHelperContext}
            ondemonkill={handleDemonKill}
            onstarpass={openStarPass}
            promotions={promotionsByRole}
            onundostarpass={doUndoStarPass}
          />

          <!-- Death tracker -->
          <DeathTracker
            {game}
            viewedPhaseDeaths={newDeathsThisRound}
            onrecord={(id) =>
              activeIsDay ? recordDeathOnDay(id) : recordDeathOnNight(id)}
            onremove={removeDeath}
            onuseghostvote={useGhostVote}
            onmove={moveDeath}
            readonly={!isViewingCurrent}
          />
        {:else}
          <!-- Name chips bar -->
          {#if showNameChips && !isFullscreen}
            <NameChipsBar
              {presetNames}
              assignedNames={assignedNameValues}
              onpickname={handleChipTap}
              onunassignname={unassignNameByValue}
              onmanagepresets={() => (showPlayerPresets = true)}
              onclose={() => (showNameChips = false)}
              selectedName={selectedChipName}
            />
          {/if}
          <!-- Active-day hint: executions are recorded via the grimoire skull. -->
          {#if activeIsDay && isViewingCurrent}
            <div
              class="flex items-center gap-2 rounded-lg border border-amber-300 bg-amber-50 px-3 py-2 text-sm font-medium text-amber-800 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-200"
            >
              <svg
                class="h-4 w-4 shrink-0 text-amber-500"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
                <circle cx="12" cy="12" r="4" />
                <path
                  stroke-linecap="round"
                  d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"
                />
              </svg>
              Day {viewingRound?.roundNumber ?? 1} — tap a player's skull in the grimoire
              to record an execution.
            </div>
          {/if}
          <!-- Grimoire view -->
          <div
            class="relative -mx-4 {isFullscreen
              ? 'h-[calc(100dvh-100px)]'
              : 'h-[calc(100dvh-240px)]'} sm:mx-0 sm:rounded-lg sm:border sm:border-border overflow-hidden"
          >
            {#if grimoireHint}
              <div
                class="pointer-events-none absolute inset-x-0 top-3 z-20 mx-auto max-w-md rounded-lg border border-amber-300 bg-amber-50 px-3 py-2 text-center text-xs font-medium text-amber-800 shadow-lg dark:border-amber-700 dark:bg-amber-950/80 dark:text-amber-200"
              >
                {grimoireHint}
              </div>
            {/if}
            <GrimoireCanvas
              players={grimoirePlayers}
              reminders={grimoireReminders}
              roundLabel="Night {viewingRound?.roundNumber ?? 1}"
              onplayermove={handleGrimoirePlayerMove}
              onremindermove={handleGrimoireReminderMove}
              onreminderattach={handleReminderAttach}
              onreminderdetach={handleReminderDetach}
              onbagsubdrop={handleBagSubDrop}
              onplayerrename={handleGrimoirePlayerRename}
              onplayertoggledeath={handleGrimoirePlayerToggleDeath}
              onplayergamenote={handleGrimoireGameNote}
              onplayerroundnote={handleGrimoireRoundNote}
              onplayeralignment={handleGrimoireAlignment}
              onplayertap={selectedChipName
                ? handlePlayerTapForAssign
                : undefined}
              ondropname={assignNameToPlayer}
            />
          </div>
        {/if}
      </div>

      <!-- ===== SETUP TABS (setup state only) ===== -->
    {:else if isSetup}
      {#if activeTab === "setup"}
        <div class="space-y-6">
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

          <!-- Players — assign names to the seats (roles) in play -->
          <PlayerAssignmentPanel
            players={assignmentPanelPlayers}
            {presetNames}
            onassign={assignNameToPlayer}
            onunassign={unassignPlayerName}
            onassigninorder={() => handleAssignPresets(presetNames)}
            onrandomize={() => handleAssignPresets(shuffled(presetNames))}
            onclearall={clearAllPlayerNames}
            onmanagepresets={() => (showPlayerPresets = true)}
          />

          <!-- Characters — click to toggle selection (script + extra merged) -->
          {#if setupHint}
            <div
              class="rounded-lg border border-amber-300 bg-amber-50 px-3 py-2 text-sm font-medium text-amber-800 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-200"
            >
              {setupHint}
            </div>
          {/if}
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
                    bagSubWarnings={bagSubCollisions}
                    onbagsubchange={openBagSubPicker}
                    onpreview={(c) => (previewCharacter = c)}
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
                        {@const isInPlay = bluffsInPlay.some(
                          (b) => b.id === char.id,
                        )}
                        {@const isShownToken = bluffsShownByBagSubs.some(
                          (b) => b.id === char.id,
                        )}
                        {@const isWarned = isInPlay || isShownToken}
                        <button
                          class="flex items-center gap-1.5 rounded-full border px-2.5 py-1 transition-colors {isWarned
                            ? 'border-amber-400 bg-amber-50 dark:border-amber-600 dark:bg-amber-950/30'
                            : 'border-border bg-surface'} hover:border-red-300 hover:bg-red-50 dark:hover:border-red-700 dark:hover:bg-red-950/30"
                          title={isInPlay
                            ? `${char.name} is in play — remove?`
                            : isShownToken
                              ? `${char.name} is the Drunk's shown token — remove?`
                              : `Remove ${char.name}`}
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
                          <span
                            class="text-xs font-medium {isWarned
                              ? 'text-amber-700 dark:text-amber-300'
                              : 'text-primary'}">{char.name}</span
                          >
                          {#if isWarned}
                            <svg
                              class="h-3.5 w-3.5 text-amber-500"
                              fill="none"
                              viewBox="0 0 24 24"
                              stroke="currentColor"
                              stroke-width="2"
                              ><path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z"
                              /></svg
                            >
                          {/if}
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
                    {#if bluffsInPlay.length > 0}
                      <p
                        class="mt-2 flex items-start gap-1.5 text-xs text-amber-700 dark:text-amber-400"
                      >
                        <svg
                          class="mt-0.5 h-3.5 w-3.5 shrink-0"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                          stroke-width="2"
                          ><path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z"
                          /></svg
                        >
                        <span>
                          {bluffsInPlay.map((b) => b.name).join(", ")}
                          {bluffsInPlay.length === 1 ? "is" : "are"} in play — the
                          demon would be bluffing a character actually in the game.
                        </span>
                      </p>
                    {/if}
                    {#if bluffsShownByBagSubs.length > 0}
                      <p
                        class="mt-2 flex items-start gap-1.5 text-xs text-amber-700 dark:text-amber-400"
                      >
                        <svg
                          class="mt-0.5 h-3.5 w-3.5 shrink-0"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                          stroke-width="2"
                          ><path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z"
                          /></svg
                        >
                        <span>
                          {bluffsShownByBagSubs.map((b) => b.name).join(", ")}
                          {bluffsShownByBagSubs.length === 1 ? "is" : "are"} the Drunk's
                          shown token — that character acts in play from the players'
                          perspective.
                        </span>
                      </p>
                    {/if}
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
                onpreview={(c) => (previewCharacter = c)}
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
          bagSubstitutions={game.bagSubstitutions}
          playerNames={grimoireNames}
          bluffs={game.selectedBluffCharacters}
          {playerStatuses}
          promotions={promotionsByRole}
          onundostarpass={doUndoStarPass}
        />
      {:else if activeTab === "grimoire"}
        <!-- Name chips bar (setup grimoire) -->
        {#if showNameChips}
          <NameChipsBar
            {presetNames}
            assignedNames={assignedNameValues}
            onpickname={handleChipTap}
            onunassignname={unassignNameByValue}
            onmanagepresets={() => (showPlayerPresets = true)}
            onclose={() => (showNameChips = false)}
            selectedName={selectedChipName}
          />
        {/if}
        <div
          class="relative -mx-4 h-[calc(100dvh-200px)] sm:mx-0 sm:rounded-lg sm:border sm:border-border overflow-hidden"
        >
          {#if grimoireHint}
            <div
              class="pointer-events-none absolute inset-x-0 top-3 z-20 mx-auto max-w-md rounded-lg border border-amber-300 bg-amber-50 px-3 py-2 text-center text-xs font-medium text-amber-800 shadow-lg dark:border-amber-700 dark:bg-amber-950/80 dark:text-amber-200"
            >
              {grimoireHint}
            </div>
          {/if}
          <GrimoireCanvas
            players={grimoirePlayers}
            reminders={grimoireReminders}
            onplayermove={handleGrimoirePlayerMove}
            onremindermove={handleGrimoireReminderMove}
            onreminderattach={handleReminderAttach}
            onreminderdetach={handleReminderDetach}
            onbagsubdrop={handleBagSubDrop}
            onplayerrename={handleGrimoirePlayerRename}
            onplayertoggledeath={handleGrimoirePlayerToggleDeath}
            onplayergamenote={handleGrimoireGameNote}
            onplayerroundnote={handleGrimoireRoundNote}
            onplayertap={selectedChipName
              ? handlePlayerTapForAssign
              : undefined}
            ondropname={assignNameToPlayer}
          />
        </div>
      {/if}
    {/if}
  </div>

  <!-- Character picker modal (setup only) -->
  {#if showCharacterPicker && (isSetup || isInProgress)}
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
  {#if bluffPickerOpen && (isSetup || isInProgress)}
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
  {#if bagSubPickerForRole && (isSetup || isInProgress)}
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
      {characterById}
      onstartgame={startGame}
      {canStartGame}
    />
  {/if}

  <!-- Player presets modal -->
  {#if showPlayerPresets}
    <PlayerPresetsModal
      onclose={() => {
        showPlayerPresets = false;
        loadPresets();
      }}
      onassign={handleAssignPresets}
      onrenamed={(o, n) => {
        grimoireNames = renameAssignedName(grimoireNames, o, n);
        saveGrimoireState();
      }}
    />
  {/if}

  <!-- Info card picker -->
  {#if infoCardPickerOpen}
    <InfoCardPicker
      {game}
      onshow={showInfoCard}
      onclose={() => (infoCardPickerOpen = false)}
    />
  {/if}

  <!-- Info card fullscreen display -->
  {#if activeInfoCard}
    <InfoCardDisplay
      card={activeInfoCard.card}
      character={activeInfoCard.character}
      onclose={() => (activeInfoCard = null)}
    />
  {/if}

  <!-- Star pass prompt (Imp self-kill → promote a Minion to Imp) -->
  {#if starPassOpen}
    <StarPassPrompt
      minions={starPassMinions}
      onpick={doStarPass}
      onskip={() => (starPassOpen = false)}
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

  <!-- Character preview popup -->
  {#if isSetup && previewCharacter}
    <CharacterPreviewPopup
      character={previewCharacter}
      onclose={() => (previewCharacter = null)}
      onstartgame={startGame}
      {canStartGame}
    />
  {/if}
{/if}
