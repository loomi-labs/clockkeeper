// Pure start-game warning computation, extracted from the game page so it can
// be unit-tested. Warnings are advisory: the storyteller can start the game
// anyway (some setups legitimately trip these, e.g. keeping an in-play bluff
// with the Pit-Hag).

import type { Character, Game } from "./gen/clockkeeper/v1/clockkeeper_pb";

/** Current selected role distribution (computed from the picked characters). */
export interface CurrentDistribution {
  townsfolk: number;
  outsiders: number;
  minions: number;
  demons: number;
}

/** Player count at/above which demon bluffs are dealt. */
const BLUFF_PLAYER_GATE = 7;

/** Every character id in play — selected roles, extras, and travellers. */
export function inPlayCharacterIds(game: Game): Set<string> {
  return new Set<string>([
    ...(game.selectedRoleIds ?? []),
    ...(game.extraCharacterIds ?? []),
    ...(game.selectedTravellerIds ?? []),
  ]);
}

/**
 * Demon bluff characters that are actually in play — selected as a script role,
 * an extra character, or a traveller. Advisory only: a storyteller may
 * knowingly bluff an in-play character (Pit-Hag, etc.).
 */
export function bluffCharactersInPlay(game: Game): Character[] {
  const inPlay = inPlayCharacterIds(game);
  return (game.selectedBluffCharacters ?? []).filter((c) => inPlay.has(c.id));
}

/**
 * Demon bluff characters that are some bag substitution's shown token (e.g.
 * the character the Drunk believes they are). Advisory: that character acts
 * "in play" from the players' perspective, so it makes a poor bluff.
 */
export function bluffCharactersShownByBagSubs(game: Game): Character[] {
  const shown = new Set(
    (game.bagSubstitutions ?? []).map((bs) => bs.characterId).filter(Boolean),
  );
  return (game.selectedBluffCharacters ?? []).filter((c) => shown.has(c.id));
}

/**
 * Compute the advisory warnings shown before starting a game.
 *
 * Covers: unpicked bag-substitute tokens, missing demon bluffs (7+ players),
 * demon bluffs that are actually in play or shown by a bag substitution, and
 * role-count / distribution mismatches against the expected distribution.
 */
export function getStartGameWarnings(
  game: Game,
  currentDist: CurrentDistribution,
): string[] {
  const warnings: string[] = [];

  // Bag substitutions (e.g. the Drunk hasn't picked a townsfolk token).
  const inPlay = inPlayCharacterIds(game);
  for (const bs of game.bagSubstitutions ?? []) {
    if (!bs.characterId) {
      warnings.push(`${bs.causedByName} has not picked a substitute token.`);
    } else if (inPlay.has(bs.characterId)) {
      // The shown token doubles up: the same character is a real role AND the
      // Drunk's shown token, so it would appear twice in play.
      warnings.push(
        `The ${bs.causedByName}'s shown token (${bs.characterName}) is also in play.`,
      );
    }
  }

  // Demon bluffs for games with 7+ players.
  if (game.playerCount >= BLUFF_PLAYER_GATE) {
    const bluffCount = (game.selectedBluffIds ?? []).length;
    if (bluffCount < 3) {
      warnings.push(`Only ${bluffCount} of 3 demon bluffs selected.`);
    }
  }

  // Bluffs that are actually in play.
  for (const c of bluffCharactersInPlay(game)) {
    warnings.push(`Demon bluff ${c.name} is in play.`);
  }

  // Bluffs that are a bag substitution's shown token (e.g. the Drunk's).
  for (const c of bluffCharactersShownByBagSubs(game)) {
    const bs = (game.bagSubstitutions ?? []).find(
      (b) => b.characterId === c.id,
    );
    warnings.push(
      `Demon bluff ${c.name} is the ${bs?.causedByName ?? "Drunk"}'s shown token.`,
    );
  }

  // Role count vs. player count.
  const totalSelected = (game.selectedRoleIds ?? []).length;
  if (totalSelected !== game.playerCount) {
    warnings.push(
      `${totalSelected} roles selected but ${game.playerCount} players expected.`,
    );
  }

  // Distribution match.
  if (game.distribution) {
    if (currentDist.townsfolk !== game.distribution.townsfolk)
      warnings.push(
        `Townsfolk: ${currentDist.townsfolk} selected, ${game.distribution.townsfolk} expected.`,
      );
    if (currentDist.outsiders !== game.distribution.outsiders)
      warnings.push(
        `Outsiders: ${currentDist.outsiders} selected, ${game.distribution.outsiders} expected.`,
      );
    if (currentDist.minions !== game.distribution.minions)
      warnings.push(
        `Minions: ${currentDist.minions} selected, ${game.distribution.minions} expected.`,
      );
    if (currentDist.demons !== game.distribution.demons)
      warnings.push(
        `Demons: ${currentDist.demons} selected, ${game.distribution.demons} expected.`,
      );
  }

  return warnings;
}
