// Player-facing info cards — digital versions of the official BotC info tokens
// the Storyteller shows players across the table.
//
// This module is pure (no Svelte imports): it turns game/InfoCard state into
// `DisplayCard`s that the UI renders. Standard cards are computed here and are
// never stored; custom cards come from the backend (`listInfoCards`).

import type {
  Character,
  Game,
  InfoCard,
} from "./gen/clockkeeper/v1/clockkeeper_pb";
// Type-only: `CharacterRef` is the canonical `{ id, name, edition, team }` shape
// (see `night-helpers/helpers.ts`). Erased at compile time, so this module stays
// pure and no runtime import cycle is created.
import type { CharacterRef } from "./night-helpers/helpers";
import { iconSuffix } from "./team-styles";

/** The standard info tokens (eight official + the ad-hoc character token). */
export type StandardCardId =
  | "std:notinplay"
  | "std:thisisthedemon"
  | "std:minions"
  | "std:youare"
  | "std:thisplayeris"
  | "std:selectedyou"
  | "std:vote"
  | "std:nominate"
  | "std:character";

/** Accent colour family, mirroring the physical token colours. */
export type Accent = "gold" | "purple" | "green" | "blue" | "red" | "neutral";

/** A character reduced to what an info card needs to render its icon + name. */
export interface DisplayCharacter {
  id: string;
  name: string;
  edition: string;
  /**
   * Icon filename suffix ("" | "_g" | "_e") derived from the source
   * character's team. Standard character icons only exist as `<id>_g.webp` /
   * `<id>_e.webp`; only travellers have a bare `<id>.webp`. Icon URLs must be
   * built as `{id}{iconSuffix}.webp`.
   */
  iconSuffix: string;
}

export interface DisplayCard {
  id: StandardCardId | `custom:${string}` | `dyn:${string}`;
  /** Uppercase display text. */
  title: string;
  body: string;
  /** Pre-filled character icons (bluffs, custom cards). */
  characters: DisplayCharacter[];
  kind: "standard" | "custom";
  accent: Accent;
  /**
   * When true the card is shown paired with a single character chosen at
   * show-time ("You are", "This player is", "This character selected you").
   */
  needsCharacterPick?: boolean;
  /**
   * When true the show-time character pick draws from ALL script characters
   * (not just those in play) — used by the ad-hoc "Character token" card.
   */
  pickFromAllCharacters?: boolean;
}

/**
 * Minimum player count at which the demon learns its minions (and the minions
 * learn each other / the demon). Same gate the night sheet uses for the
 * `minioninfo` / `demoninfo` entries.
 */
export const MINION_INFO_PLAYER_GATE = 7;

/** Visual styling per accent. Data-only so it can live in this pure module. */
export interface AccentStyle {
  /** Full-bleed layered CSS `background` value (radial colour + subtle noise). */
  background: string;
  /** Decorative border / ornament colour. */
  border: string;
  /** Foreground text colour. */
  text: string;
}

export const ACCENT_STYLES: Record<Accent, AccentStyle> = {
  gold: {
    background:
      "repeating-linear-gradient(45deg, rgba(255,255,255,0.04) 0 2px, transparent 2px 7px)," +
      "radial-gradient(circle at 50% 32%, #c79a3f 0%, #8a5f22 48%, #4d3312 100%)",
    border: "#f3e2b0",
    text: "#fbf3d9",
  },
  purple: {
    background:
      "repeating-linear-gradient(45deg, rgba(255,255,255,0.04) 0 2px, transparent 2px 7px)," +
      "radial-gradient(circle at 50% 32%, #8a3fb0 0%, #571d78 48%, #2c0f42 100%)",
    border: "#e9cdf5",
    text: "#f6ecfb",
  },
  green: {
    background:
      "repeating-linear-gradient(45deg, rgba(255,255,255,0.04) 0 2px, transparent 2px 7px)," +
      "radial-gradient(circle at 50% 32%, #3aa15c 0%, #1f6d3a 48%, #0e3a1e 100%)",
    border: "#c7edd2",
    text: "#eefaf1",
  },
  blue: {
    background:
      "repeating-linear-gradient(45deg, rgba(255,255,255,0.04) 0 2px, transparent 2px 7px)," +
      "radial-gradient(circle at 50% 32%, #3f6fc7 0%, #1f4285 48%, #0e2044 100%)",
    border: "#c5d6f2",
    text: "#eaf1fb",
  },
  red: {
    background:
      "repeating-linear-gradient(45deg, rgba(255,255,255,0.04) 0 2px, transparent 2px 7px)," +
      "radial-gradient(circle at 50% 32%, #a83232 0%, #741f1f 48%, #3c0e0e 100%)",
    border: "#f2c9c9",
    text: "#fbeaea",
  },
  neutral: {
    background:
      "repeating-linear-gradient(45deg, rgba(255,255,255,0.04) 0 2px, transparent 2px 7px)," +
      "radial-gradient(circle at 50% 32%, #556077 0%, #333a4c 48%, #191d28 100%)",
    border: "#d3d9e6",
    text: "#eef1f7",
  },
};

function toDisplayCharacters(characters: Character[]): DisplayCharacter[] {
  return characters.map((c) => ({
    id: c.id,
    name: c.name,
    edition: c.edition,
    iconSuffix: iconSuffix(c.team),
  }));
}

/**
 * Compute the standard set of info cards for a game.
 *
 * - "These characters are not in play" is omitted when the game has no
 *   selected demon bluffs.
 * - "These are your minions" only appears for games with
 *   {@link MINION_INFO_PLAYER_GATE}+ players (same gate as the night sheet's
 *   minion/demon info step).
 */
export function generateStandardCards(game: Game): DisplayCard[] {
  const cards: DisplayCard[] = [];

  // Bluffs — pre-filled from game state, hidden when none are selected.
  if (game.selectedBluffCharacters.length > 0) {
    cards.push({
      id: "std:notinplay",
      title: "THESE CHARACTERS ARE NOT IN PLAY",
      body: "",
      characters: toDisplayCharacters(game.selectedBluffCharacters),
      kind: "standard",
      accent: "blue",
    });
  }

  cards.push({
    id: "std:thisisthedemon",
    title: "THIS IS THE DEMON",
    body: "",
    characters: [],
    kind: "standard",
    accent: "red",
  });

  if (game.playerCount >= MINION_INFO_PLAYER_GATE) {
    cards.push({
      id: "std:minions",
      title: "THESE ARE YOUR MINIONS",
      body: "",
      characters: [],
      kind: "standard",
      accent: "red",
    });
  }

  cards.push(
    {
      id: "std:youare",
      title: "YOU ARE",
      body: "",
      characters: [],
      kind: "standard",
      accent: "purple",
      needsCharacterPick: true,
    },
    {
      id: "std:thisplayeris",
      title: "THIS PLAYER IS",
      body: "",
      characters: [],
      kind: "standard",
      accent: "purple",
      needsCharacterPick: true,
    },
    {
      id: "std:selectedyou",
      title: "THIS CHARACTER SELECTED YOU",
      body: "",
      characters: [],
      kind: "standard",
      accent: "green",
      needsCharacterPick: true,
    },
    {
      id: "std:vote",
      title: "DID YOU VOTE TODAY?",
      body: "",
      characters: [],
      kind: "standard",
      accent: "gold",
    },
    {
      id: "std:nominate",
      title: "DID YOU NOMINATE TODAY?",
      body: "",
      characters: [],
      kind: "standard",
      accent: "gold",
    },
    {
      // Ad-hoc single-character token. The chosen character IS the card, so it
      // has no title/body — the icon renders extra-large in the display.
      id: "std:character",
      title: "",
      body: "",
      characters: [],
      kind: "standard",
      accent: "neutral",
      needsCharacterPick: true,
      pickFromAllCharacters: true,
    },
  );

  return cards;
}

/**
 * Dynamic first-night "one of these players is X" card (Washerwoman / Librarian
 * / Investigator). Shows the revealed character's icon under an uppercase title
 * — no player names at all (the Storyteller points at the two seats in person).
 * The shown character is the DISPLAYED character of the picked "right" seat
 * (bag-sub aware), whatever that seat's team.
 */
export function firstNightInfoCard(shownCharacter: CharacterRef): DisplayCard {
  return {
    id: `dyn:firstnight-${shownCharacter.id}`,
    title: `ONE OF THESE PLAYERS IS THE ${shownCharacter.name.toUpperCase()}`,
    body: "",
    characters: [
      {
        id: shownCharacter.id,
        name: shownCharacter.name,
        edition: shownCharacter.edition,
        iconSuffix: iconSuffix(shownCharacter.team),
      },
    ],
    kind: "standard",
    accent: "purple",
  };
}

/** Librarian's "no Outsiders in play" card — no players, no icons. */
export function noOutsidersCard(): DisplayCard {
  return {
    id: "dyn:no-outsiders",
    title: "THERE ARE NO OUTSIDERS IN PLAY",
    body: "",
    characters: [],
    kind: "standard",
    accent: "blue",
  };
}

/** Bare character-token card: the icon IS the card (empty title/body → icon-only display). */
export function characterTokenCard(
  char: CharacterRef,
  idPrefix: string,
): DisplayCard {
  return {
    id: `dyn:${idPrefix}-${char.id}`,
    title: "",
    body: "",
    characters: [
      {
        id: char.id,
        name: char.name,
        edition: char.edition,
        iconSuffix: iconSuffix(char.team),
      },
    ],
    kind: "standard",
    accent: "neutral",
  };
}

/**
 * Map a stored custom {@link InfoCard} to a {@link DisplayCard}.
 *
 * Accent choice: custom cards always use the "neutral" accent. Deriving a
 * colour from the first character's team was considered, but neutral keeps the
 * mapping deterministic and independent of team data (which `DisplayCharacter`
 * deliberately does not carry). Custom cards never need a show-time pick — any
 * character icons are baked into the card itself.
 */
export function customCardToDisplay(card: InfoCard): DisplayCard {
  return {
    id: `custom:${card.id.toString()}`,
    title: card.title,
    body: card.body,
    characters: toDisplayCharacters(card.characters),
    kind: "custom",
    accent: "neutral",
  };
}
