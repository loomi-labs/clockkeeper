-- Modify "games" table: add owner (user_id) column.
-- Step 1: Add as nullable.
ALTER TABLE "games" ADD COLUMN "user_id" bigint NULL;
-- Step 2: Backfill existing games with the script owner.
UPDATE "games" SET "user_id" = "scripts"."user_id" FROM "scripts" WHERE "games"."script_id" = "scripts"."id" AND "games"."user_id" IS NULL;
-- Step 3: Set NOT NULL and add FK.
ALTER TABLE "games" ALTER COLUMN "user_id" SET NOT NULL;
ALTER TABLE "games" ADD CONSTRAINT "games_users_games" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
