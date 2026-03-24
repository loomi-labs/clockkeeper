import { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";

/** Plural display names for each team. */
export const teamLabels: Record<number, string> = {
  [Team.TOWNSFOLK]: "Townsfolk",
  [Team.OUTSIDER]: "Outsiders",
  [Team.MINION]: "Minions",
  [Team.DEMON]: "Demons",
  [Team.TRAVELLER]: "Travellers",
  [Team.FABLED]: "Fabled",
  [Team.LORIC]: "Lorics",
};

/** Singular display names for each team. */
export const teamSingulars: Record<number, string> = {
  [Team.TOWNSFOLK]: "Townsfolk",
  [Team.OUTSIDER]: "Outsider",
  [Team.MINION]: "Minion",
  [Team.DEMON]: "Demon",
  [Team.TRAVELLER]: "Traveller",
  [Team.FABLED]: "Fabled",
  [Team.LORIC]: "Loric",
};

/** Header text color classes for team section headings. */
export const teamHeaderColors: Record<number, string> = {
  [Team.TOWNSFOLK]: "text-blue-600 dark:text-blue-400",
  [Team.OUTSIDER]: "text-cyan-600 dark:text-cyan-400",
  [Team.MINION]: "text-orange-600 dark:text-orange-400",
  [Team.DEMON]: "text-red-600 dark:text-red-400",
  [Team.TRAVELLER]:
    "bg-gradient-to-r from-blue-600 to-red-600 bg-clip-text text-transparent dark:from-blue-400 dark:to-red-400",
  [Team.FABLED]: "text-yellow-500 dark:text-yellow-400",
  [Team.LORIC]: "text-green-600 dark:text-green-400",
};

/** Card border/background classes for the default (selected) state. */
export const teamCardColors: Record<number, string> = {
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

/** Card border/background classes for unselected state with reduced opacity. */
export const teamCardColorsUnselected: Record<number, string> = {
  [Team.TOWNSFOLK]:
    "border-blue-100 bg-blue-50/50 dark:border-blue-800/50 dark:bg-blue-950/20",
  [Team.OUTSIDER]:
    "border-cyan-100 bg-cyan-50/50 dark:border-cyan-800/50 dark:bg-cyan-950/20",
  [Team.MINION]:
    "border-orange-100 bg-orange-50/50 dark:border-orange-800/50 dark:bg-orange-950/20",
  [Team.DEMON]:
    "border-red-100 bg-red-50/50 dark:border-red-800/50 dark:bg-red-950/20",
  [Team.TRAVELLER]: "card-traveller opacity-60",
  [Team.FABLED]:
    "border-yellow-200 bg-yellow-50/50 dark:border-yellow-700/50 dark:bg-yellow-950/20",
  [Team.LORIC]:
    "border-green-100 bg-green-50/50 dark:border-green-800/50 dark:bg-green-950/20",
};

/** Card colors for the selected (stronger) state in pickers. */
export const teamCardColorsSelected: Record<number, string> = {
  [Team.TOWNSFOLK]: "border-blue-500 bg-blue-100 dark:bg-blue-500/20",
  [Team.OUTSIDER]: "border-cyan-500 bg-cyan-100 dark:bg-cyan-500/20",
  [Team.MINION]: "border-orange-500 bg-orange-100 dark:bg-orange-500/20",
  [Team.DEMON]: "border-red-500 bg-red-100 dark:bg-red-500/20",
  [Team.TRAVELLER]: "card-traveller",
  [Team.FABLED]: "border-yellow-500 bg-yellow-100 dark:bg-yellow-500/20",
  [Team.LORIC]: "border-green-500 bg-green-100 dark:bg-green-500/20",
};

/** Name text color classes (700/300 weight). */
export const teamNameColors: Record<number, string> = {
  [Team.TOWNSFOLK]: "text-blue-700 dark:text-blue-300",
  [Team.OUTSIDER]: "text-cyan-700 dark:text-cyan-300",
  [Team.MINION]: "text-orange-700 dark:text-orange-300",
  [Team.DEMON]: "text-red-700 dark:text-red-300",
  [Team.TRAVELLER]:
    "bg-gradient-to-r from-blue-700 to-red-700 bg-clip-text text-transparent dark:from-blue-300 dark:to-red-300",
  [Team.FABLED]: "text-yellow-700 dark:text-yellow-300",
  [Team.LORIC]: "text-green-700 dark:text-green-300",
};

/** Checkmark color classes for selected indicators. */
export const teamCheckColors: Record<number, string> = {
  [Team.TOWNSFOLK]: "text-blue-600 dark:text-blue-400",
  [Team.OUTSIDER]: "text-cyan-600 dark:text-cyan-400",
  [Team.MINION]: "text-orange-600 dark:text-orange-400",
  [Team.DEMON]: "text-red-600 dark:text-red-400",
  [Team.TRAVELLER]: "text-blue-600 dark:text-blue-400",
  [Team.FABLED]: "text-yellow-600 dark:text-yellow-400",
  [Team.LORIC]: "text-green-600 dark:text-green-400",
};

/** Data attribute string identifiers for each team. */
export const teamDataAttr: Record<number, string> = {
  [Team.TOWNSFOLK]: "townsfolk",
  [Team.OUTSIDER]: "outsider",
  [Team.MINION]: "minion",
  [Team.DEMON]: "demon",
  [Team.TRAVELLER]: "traveller",
  [Team.FABLED]: "fabled",
  [Team.LORIC]: "loric",
};

/** Add button card colors for each team. */
export const addCardColors: Record<number, string> = {
  [Team.TOWNSFOLK]:
    "border-blue-300 text-blue-400 hover:bg-blue-50 dark:border-blue-700 dark:text-blue-500 dark:hover:bg-blue-950/30",
  [Team.OUTSIDER]:
    "border-cyan-300 text-cyan-400 hover:bg-cyan-50 dark:border-cyan-700 dark:text-cyan-500 dark:hover:bg-cyan-950/30",
  [Team.MINION]:
    "border-orange-300 text-orange-400 hover:bg-orange-50 dark:border-orange-700 dark:text-orange-500 dark:hover:bg-orange-950/30",
  [Team.DEMON]:
    "border-red-300 text-red-400 hover:bg-red-50 dark:border-red-700 dark:text-red-500 dark:hover:bg-red-950/30",
  [Team.TRAVELLER]: "card-traveller-add",
  [Team.FABLED]:
    "border-yellow-300 text-yellow-500 hover:bg-yellow-50 dark:border-yellow-700 dark:text-yellow-500 dark:hover:bg-yellow-950/30",
  [Team.LORIC]:
    "border-green-300 text-green-400 hover:bg-green-50 dark:border-green-700 dark:text-green-500 dark:hover:bg-green-950/30",
};

/** Alignment color classes for good-aligned tokens. */
export const goodColors =
  "border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-950/40";

/** Alignment color classes for evil-aligned tokens. */
export const evilColors =
  "border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950/40";

/** Returns the icon filename suffix based on team: '_g' for good, '_e' for evil, '' for others. */
export function iconSuffix(team: number): string {
  if (team === Team.TOWNSFOLK || team === Team.OUTSIDER) return "_g";
  if (team === Team.MINION || team === Team.DEMON) return "_e";
  return "";
}
