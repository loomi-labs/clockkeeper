// Extensible registry of per-character night helpers.
//
// Adding a future helper is a two-step change: build its component and add one
// entry here (character id -> { nights, component }). This module owns the
// component references so the dispatcher (`NightEntryHelper.svelte`) only has to
// import the registry — it never imports the individual helper components.
//
// Import-cycle note: the helper components import ONLY the *type*
// `NightHelperContext` from this module. Type-only imports are erased at
// compile time, so registry -> component (value) and component -> registry
// (type-only) do not form a runtime cycle.

import type { Component } from "svelte";
import EmpathHelper from "~/lib/components/night-helpers/EmpathHelper.svelte";
import ChefHelper from "~/lib/components/night-helpers/ChefHelper.svelte";
import UndertakerHelper from "~/lib/components/night-helpers/UndertakerHelper.svelte";
import FortuneTellerHelper from "~/lib/components/night-helpers/FortuneTellerHelper.svelte";
import FirstNightInfoHelper from "~/lib/components/night-helpers/FirstNightInfoHelper.svelte";
import TokenPickHelper from "~/lib/components/night-helpers/TokenPickHelper.svelte";
import type { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
import type { DisplayCard } from "~/lib/info-cards";
import type { HelperPlayer } from "./helpers";
import type { PlayerStatus } from "./status";

/** Which night(s) a helper is relevant for. */
export type NightKind = "first" | "other";

/**
 * Everything a night-helper component needs to compute its display for the
 * current night. Built once per night in the page and passed down.
 */
export interface NightHelperContext {
  /** Which night is being run — gates helpers via `NightHelperDef.nights`. */
  night: NightKind;
  /** Clockwise seating order (role/player ids). */
  order: readonly string[];
  /** Night-scoped players (night deaths + night alignments), keyed by id. */
  players: Map<string, HelperPlayer>;
  /** Derived poisoned/drunk status per player (role/player id -> status). */
  statuses: ReadonlyMap<string, PlayerStatus>;
  /**
   * Resolve the seat/player id that a night entry (character id) belongs to.
   * Normally identity, but bag substitutions shift a shown character's row onto
   * a different underlying seat. Returns undefined if the entry has no player.
   */
  playerIdForEntry: (entryId: string) => string | undefined;
  /** Player the Fortune Teller's Red Herring token is attached to, if any. */
  redHerringPlayerId: string | undefined;
  /** Player executed on the previous day (for the Undertaker), if any. */
  executedToday: { player: HelperPlayer; heuristic: boolean } | undefined;
  /** Ephemeral Fortune Teller picks for the current night. */
  ftPicks: readonly string[];
  /** Update the Fortune Teller picks (page owns the ephemeral state). */
  onftpick: (picks: string[]) => void;

  // ── First-night info helpers (Washerwoman / Librarian / Investigator) ──
  // All optional: the page wires them in for the first night only. Helpers
  // that need them render nothing when they are absent.

  /**
   * Resolve the DISPLAYED character of a seat (bag-sub aware). Returns the
   * shown token — e.g. the Drunk shown as the Empath resolves to the Empath —
   * so a substituted seat is classified by what players see, not its real role.
   */
  displayedCharacterOf?: (
    playerId: string,
  ) => { id: string; name: string; edition: string; team: Team } | undefined;
  /**
   * First-night info picks, keyed by helper character id (the entry id).
   * `rightId` is the seat showing the revealed character; `wrongId` is the decoy
   * seat. Derived from (and written back to) the grimoire reminder-token
   * attachments — picking a slot IS attaching the matching token, so picks
   * persist across reloads and stay in sync with manual grimoire edits.
   */
  infoPicks?: ReadonlyMap<string, { rightId?: string; wrongId?: string }>;
  /** Update the info picks for a helper (attaches/detaches the tokens). */
  oninfopick?: (
    charId: string,
    picks: { rightId?: string; wrongId?: string },
  ) => void;
  /** Show a fully-built dynamic info card fullscreen. */
  onshowcard?: (card: DisplayCard) => void;

  // ── Reminder-token pickers (Fortune Teller Red Herring, Poisoner, Butler) ──
  // All optional: the page wires them in for the relevant nights. Helpers that
  // need them render nothing (or a passive hint) when they are absent.

  /**
   * Attach (playerId set) or detach (undefined) the reminder token identified
   * by (characterId, tokenText) — the same action as dragging that token onto a
   * seat in the grimoire, so picks stay in sync with manual grimoire edits.
   */
  onattachtoken?: (
    characterId: string,
    tokenText: string,
    playerId: string | undefined,
  ) => void;
  /**
   * Current holder of the reminder token identified by (characterId,
   * tokenText), if attached. Bidirectional with manual grimoire attachment.
   */
  tokenHolder?: (characterId: string, tokenText: string) => string | undefined;
  /** The current script's characters (for pickers that offer characters). */
  scriptCharacters?: ReadonlyArray<{
    id: string;
    name: string;
    team: Team;
    edition: string;
  }>;
}

/**
 * A registry entry: which nights the helper applies to plus the component that
 * renders it. The component receives `{ entryId, ctx }`.
 */
export interface NightHelperDef {
  nights: ReadonlyArray<NightKind>;
  component: Component<{ entryId: string; ctx: NightHelperContext }>;
}

/** Character id -> helper definition. */
export const NIGHT_HELPERS: Record<string, NightHelperDef> = {
  empath: { nights: ["first", "other"], component: EmpathHelper },
  chef: { nights: ["first"], component: ChefHelper },
  undertaker: { nights: ["other"], component: UndertakerHelper },
  fortuneteller: {
    nights: ["first", "other"],
    component: FortuneTellerHelper,
  },
  washerwoman: { nights: ["first"], component: FirstNightInfoHelper },
  librarian: { nights: ["first"], component: FirstNightInfoHelper },
  investigator: { nights: ["first"], component: FirstNightInfoHelper },
  poisoner: { nights: ["first", "other"], component: TokenPickHelper },
  butler: { nights: ["first", "other"], component: TokenPickHelper },
};
