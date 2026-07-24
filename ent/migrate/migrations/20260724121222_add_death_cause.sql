-- Modify "deaths" table
ALTER TABLE "deaths" ADD COLUMN "cause" character varying NOT NULL DEFAULT 'unspecified';
