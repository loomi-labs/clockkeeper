-- Delete all existing users and their data (dev data only, no production users exist)
DELETE FROM "deaths";
DELETE FROM "phases";
DELETE FROM "games";
DELETE FROM "scripts" WHERE "user_id" IS NOT NULL;
DELETE FROM "users";
-- Modify "users" table
ALTER TABLE "users" DROP COLUMN "username", DROP COLUMN "password_hash", ADD COLUMN "uuid" character varying NOT NULL, ADD COLUMN "discord_id" character varying NULL, ADD COLUMN "discord_username" character varying NULL, ADD COLUMN "discord_avatar" character varying NULL, ADD COLUMN "is_anonymous" boolean NOT NULL DEFAULT false, ADD COLUMN "last_active_at" timestamptz NOT NULL;
-- Create index "users_discord_id_key" to table: "users"
CREATE UNIQUE INDEX "users_discord_id_key" ON "users" ("discord_id");
-- Create index "users_uuid_key" to table: "users"
CREATE UNIQUE INDEX "users_uuid_key" ON "users" ("uuid");
