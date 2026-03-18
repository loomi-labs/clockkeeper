-- Modify "games" table
ALTER TABLE "games" ADD COLUMN "selected_travellers" jsonb NOT NULL DEFAULT '[]';
