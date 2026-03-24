import type { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";

export interface GrimoirePlayer {
  id: string;
  name: string;
  characterId: string;
  characterName: string;
  team: Team;
  edition: string;
  x: number;
  y: number;
  isDead: boolean;
  ghostVoteUsed: boolean;
  gameNote: string;
  roundNote: string;
  alignment: "good" | "evil" | undefined;
}

export interface GrimoireReminder {
  id: string;
  characterId: string;
  characterName: string;
  text: string;
  team: Team;
  edition: string;
  x: number;
  y: number;
  alignment?: "good" | "evil";
}
