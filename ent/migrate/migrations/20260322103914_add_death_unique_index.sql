-- Create index "death_role_id_phase_id" to table: "deaths"
CREATE UNIQUE INDEX "death_role_id_phase_id" ON "deaths" ("role_id", "phase_id");
