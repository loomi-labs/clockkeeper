-- Modify "games" table
ALTER TABLE "games" ADD COLUMN "grimoire_game_notes" jsonb NULL, ADD COLUMN "grimoire_round_notes" jsonb NULL;
