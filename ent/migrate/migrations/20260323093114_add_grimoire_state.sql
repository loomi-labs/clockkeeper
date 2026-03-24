-- Modify "games" table
ALTER TABLE "games" ADD COLUMN "grimoire_positions" jsonb NULL, ADD COLUMN "grimoire_player_names" jsonb NULL;
