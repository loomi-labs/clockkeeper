-- Modify "scripts" table
ALTER TABLE "scripts" ADD COLUMN "deleted_at" timestamptz NULL;
