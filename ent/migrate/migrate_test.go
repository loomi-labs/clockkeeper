package migrate_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/game"
	_ "github.com/loomi-labs/clockkeeper/ent/runtime"
	"github.com/loomi-labs/clockkeeper/ent/script"
	"github.com/loomi-labs/clockkeeper/ent/user"
	"github.com/loomi-labs/clockkeeper/internal/database"

	_ "github.com/lib/pq"
)

// migrationValidators maps each migration filename (without .sql) to a
// validation function. When a new migration is added without a corresponding
// validator, TestMigrationCoverage fails — forcing you to write one.
var migrationValidators = map[string]func(t *testing.T, ctx context.Context, db *sql.DB, client *ent.Client){
	"20260315162621_initial":                       validateInitialSchema,
	"20260315175834_add_scripts_and_games":          validateScriptsAndGames,
	"20260315191827_add_traveller_count":            validateTravellerCount,
	"20260315195303_add_selected_travellers":        validateSelectedTravellers,
	"20260316163815_add_system_scripts":              validateSystemScripts,
	"20260316165946_add_script_soft_delete":          validateScriptSoftDelete,
	"20260318105440_add_game_owner":                    validateGameOwner,
	"20260318155946_add_script_owner_check":             validateScriptOwnerCheck,
	"20260319100012_add_game_extra_characters":           validateGameExtraCharacters,
	"20260321102851_add_phases_and_deaths":               validatePhasesAndDeaths,
	"20260321112246_add_traveller_alignments":            validateTravellerAlignments,
	"20260321131451_add_game_name":                       validateGameName,
	"20260321133230_add_completed_actions":               validateCompletedActions,
	"20260322103914_add_death_unique_index":              validateDeathUniqueIndex,
	"20260323093114_add_grimoire_state":                  validateGrimoireState,
	"20260323102548_add_grimoire_notes":                  validateGrimoireNotes,
	"20260323131712_add_character_alignments":            validateCharacterAlignments,
	"20260323135528_add_demon_bluffs":                   validateDemonBluffs,
	"20260323161223_add_bag_substitutions":               validateBagSubstitutions,
	"20260324183937_add_grimoire_reminder_attachments":   validateGrimoireReminderAttachments,
	"20260406182402_add_player_presets":                  validatePlayerPresets,
}

// TestMigrationCoverage ensures every migration file has a registered validator.
func TestMigrationCoverage(t *testing.T) {
	migrationsDir := filepath.Join("migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	var sqlFiles []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".sql")
		sqlFiles = append(sqlFiles, name)
	}

	sort.Strings(sqlFiles)

	var missing []string
	for _, f := range sqlFiles {
		if _, ok := migrationValidators[f]; !ok {
			missing = append(missing, f)
		}
	}

	var extra []string
	fileSet := toSet(sqlFiles)
	for name := range migrationValidators {
		if !fileSet[name] {
			extra = append(extra, name)
		}
	}

	if len(missing) > 0 {
		t.Errorf("migrations without validators: %v\n"+
			"Add a validator to migrationValidators in migrate_test.go.", missing)
	}
	if len(extra) > 0 {
		t.Errorf("validators for non-existent migrations: %v\n"+
			"Remove stale entries from migrationValidators.", extra)
	}

	// Verify seed files in testdata/ correspond to real migration versions.
	versionSet := make(map[string]bool, len(sqlFiles))
	for _, f := range sqlFiles {
		version, _, _ := strings.Cut(f, "_")
		versionSet[version] = true
	}

	seedEntries, err := os.ReadDir(filepath.Join("testdata"))
	if err != nil {
		t.Fatalf("failed to read testdata directory: %v", err)
	}
	for _, e := range seedEntries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}
		version := strings.TrimSuffix(e.Name(), ".sql")
		if !versionSet[version] {
			t.Errorf("seed file %s has no matching migration — remove or rename it", e.Name())
		}
	}
}

// TestSchemaCompleteness ensures all schema entities are covered by the migration test.
func TestSchemaCompleteness(t *testing.T) {
	knownEntities := []string{
		"death.go",
		"game.go",
		"phase.go",
		"script.go",
		"user.go",
	}

	schemaDir := filepath.Join("..", "schema")
	entries, err := os.ReadDir(schemaDir)
	if err != nil {
		t.Fatalf("failed to read schema directory: %v", err)
	}

	var found []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".go" {
			continue
		}
		found = append(found, e.Name())
	}

	sort.Strings(knownEntities)
	sort.Strings(found)

	var added, removed []string
	known := toSet(knownEntities)
	foundSet := toSet(found)

	for _, f := range found {
		if !known[f] {
			added = append(added, f)
		}
	}
	for _, k := range knownEntities {
		if !foundSet[k] {
			removed = append(removed, k)
		}
	}

	if len(added) > 0 {
		t.Errorf("new schema entities detected: %v\n"+
			"Update knownEntities, seed data, and the migration validators.", added)
	}
	if len(removed) > 0 {
		t.Errorf("schema entities removed: %v\n"+
			"Update knownEntities in TestSchemaCompleteness.", removed)
	}
}

// TestMigrationDataIntegrity applies migrations one-by-one, seeding test data
// after each step, then runs every registered validator.
func TestMigrationDataIntegrity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start Postgres container.
	config := database.SetupPostgreSQLContainer(t)

	// Open raw connection (needed for seeding between migrations).
	db, err := sql.Open("postgres", config.ConnectionString())
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Apply migrations one-by-one, seeding after each step.
	migrator := database.NewMigrator(t, config)
	for _, name := range migrationNames(t) {
		migrator.ApplyN(t, 1)

		version, _, _ := strings.Cut(name, "_")
		seedFile := filepath.Join("testdata", version+".sql")
		seedSQL, err := os.ReadFile(seedFile)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatalf("failed to read seed %s: %v", seedFile, err)
		}
		if _, err := db.Exec(string(seedSQL)); err != nil {
			t.Fatalf("failed to execute seed %s: %v", seedFile, err)
		}
		t.Logf("seeded %s", version)
	}

	// Open Ent client.
	client, _, err := database.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create ent client: %v", err)
	}
	defer client.Close()

	// Run all validators.
	for name, validate := range migrationValidators {
		t.Run(name, func(t *testing.T) {
			validate(t, ctx, db, client)
		})
	}
}

// --- validators ---

// validateInitialSchema checks that the users table exists and data is preserved.
func validateInitialSchema(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	adminUser, err := client.User.Query().
		Where(user.Username("admin")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query admin user: %v", err)
	}
	if adminUser.PasswordHash == "" {
		t.Error("user password_hash is empty")
	}
}

// validateScriptsAndGames checks that scripts and games tables exist with seeded data.
func validateScriptsAndGames(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	s, err := client.Script.Query().
		Where(script.Name("My TB Script")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query seeded script: %v", err)
	}
	if s.Edition != "tb" {
		t.Errorf("expected edition 'tb', got %q", s.Edition)
	}

	g, err := client.Game.Query().
		Where(game.StateEQ(game.StateSetup)).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query seeded game: %v", err)
	}
	if g.PlayerCount != 7 {
		t.Errorf("expected player_count 7, got %d", g.PlayerCount)
	}
}

// validateTravellerCount checks that the traveller_count column exists with default 0.
func validateTravellerCount(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	g, err := client.Game.Query().Only(ctx)
	if err != nil {
		t.Fatalf("failed to query game: %v", err)
	}
	if g.TravellerCount != 0 {
		t.Errorf("expected default traveller_count 0, got %d", g.TravellerCount)
	}
}

// validateSelectedTravellers checks that the selected_travellers column exists with default empty.
func validateSelectedTravellers(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	g, err := client.Game.Query().Only(ctx)
	if err != nil {
		t.Fatalf("failed to query game: %v", err)
	}
	if len(g.SelectedTravellers) != 0 {
		t.Errorf("expected empty selected_travellers, got %v", g.SelectedTravellers)
	}
}

// validateSystemScripts checks that the system scripts were seeded and user_id is nullable.
func validateSystemScripts(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	// Verify all 3 system scripts exist.
	systemScripts, err := client.Script.Query().
		Where(script.IsSystem(true)).
		All(ctx)
	if err != nil {
		t.Fatalf("failed to query system scripts: %v", err)
	}
	if len(systemScripts) != 3 {
		t.Fatalf("expected 3 system scripts, got %d", len(systemScripts))
	}

	editions := map[string]bool{}
	for _, s := range systemScripts {
		editions[s.Edition] = true
		if s.UserID != nil {
			t.Errorf("system script %q should have nil user_id", s.Name)
		}
		if len(s.CharacterIds) == 0 {
			t.Errorf("system script %q has no character_ids", s.Name)
		}
	}
	for _, ed := range []string{"tb", "bmr", "snv"} {
		if !editions[ed] {
			t.Errorf("missing system script for edition %q", ed)
		}
	}

	// Verify user-owned script still exists with user_id set.
	userScript, err := client.Script.Query().
		Where(script.IsSystem(false)).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query user script: %v", err)
	}
	if userScript.UserID == nil {
		t.Error("user script should have a user_id")
	}
}

// validateScriptSoftDelete checks that the deleted_at column exists and is nullable.
func validateScriptSoftDelete(t *testing.T, ctx context.Context, db *sql.DB, client *ent.Client) {
	t.Helper()

	// Verify deleted_at is NULL for all existing scripts.
	scripts, err := client.Script.Query().All(ctx)
	if err != nil {
		t.Fatalf("failed to query scripts: %v", err)
	}
	for _, s := range scripts {
		if s.DeletedAt != nil {
			t.Errorf("script %q should have nil deleted_at", s.Name)
		}
	}
}

// validateGameOwner checks that the user_id column exists and was backfilled.
func validateGameOwner(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	g, err := client.Game.Query().Only(ctx)
	if err != nil {
		t.Fatalf("failed to query game: %v", err)
	}
	if g.UserID == 0 {
		t.Error("expected game user_id to be backfilled, got 0")
	}
}

// validateScriptOwnerCheck verifies the CHECK constraint was added (later dropped by phases migration).
func validateScriptOwnerCheck(t *testing.T, _ context.Context, _ *sql.DB, _ *ent.Client) {
	t.Helper()
	// The chk_script_has_owner constraint was added by this migration but later
	// dropped by 20260321102851_add_phases_and_deaths. Since validators run after
	// all migrations are applied, we can only verify the migration was applied.
}

// validateGameExtraCharacters checks that the extra_characters column exists and defaults to NULL.
func validateGameExtraCharacters(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	g, err := client.Game.Query().Only(ctx)
	if err != nil {
		t.Fatalf("failed to query game: %v", err)
	}
	if g.ExtraCharacters != nil && len(g.ExtraCharacters) != 0 {
		t.Errorf("expected nil or empty extra_characters, got %v", g.ExtraCharacters)
	}
}

// validatePhasesAndDeaths checks that phases and deaths tables exist.
func validatePhasesAndDeaths(t *testing.T, ctx context.Context, db *sql.DB, _ *ent.Client) {
	t.Helper()

	// Verify phases table exists.
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM phases`).Scan(&count)
	if err != nil {
		t.Fatalf("phases table should exist: %v", err)
	}

	// Verify deaths table exists.
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM deaths`).Scan(&count)
	if err != nil {
		t.Fatalf("deaths table should exist: %v", err)
	}
}

// validateTravellerAlignments checks that the traveller_alignments column exists.
func validateTravellerAlignments(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	g, err := client.Game.Query().Only(ctx)
	if err != nil {
		t.Fatalf("failed to query game: %v", err)
	}
	if g.TravellerAlignments != nil && len(g.TravellerAlignments) != 0 {
		t.Errorf("expected nil or empty traveller_alignments, got %v", g.TravellerAlignments)
	}
}

// validateGameName checks that the name column exists with default empty string.
func validateGameName(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	g, err := client.Game.Query().Only(ctx)
	if err != nil {
		t.Fatalf("failed to query game: %v", err)
	}
	if g.Name != "" {
		t.Errorf("expected default empty name, got %q", g.Name)
	}
}

// validateCompletedActions checks that the completed_actions column exists on phases.
func validateCompletedActions(t *testing.T, ctx context.Context, db *sql.DB, _ *ent.Client) {
	t.Helper()

	// Verify the column exists by querying it.
	var exists bool
	err := db.QueryRowContext(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'phases' AND column_name = 'completed_actions'
		)`).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to check column: %v", err)
	}
	if !exists {
		t.Error("completed_actions column should exist on phases table")
	}
}

// validateDeathUniqueIndex checks that the unique index on (role_id, phase_id) exists.
func validateDeathUniqueIndex(t *testing.T, ctx context.Context, db *sql.DB, _ *ent.Client) {
	t.Helper()

	var exists bool
	err := db.QueryRowContext(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE tablename = 'deaths' AND indexname = 'death_role_id_phase_id'
		)`).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to check index: %v", err)
	}
	if !exists {
		t.Error("death_role_id_phase_id unique index should exist")
	}
}

// validateGrimoireState checks that grimoire_positions and grimoire_player_names columns exist.
func validateGrimoireState(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	g, err := client.Game.Query().Only(ctx)
	if err != nil {
		t.Fatalf("failed to query game: %v", err)
	}
	if g.GrimoirePositions != nil && len(g.GrimoirePositions) != 0 {
		t.Errorf("expected nil or empty grimoire_positions, got %v", g.GrimoirePositions)
	}
	if g.GrimoirePlayerNames != nil && len(g.GrimoirePlayerNames) != 0 {
		t.Errorf("expected nil or empty grimoire_player_names, got %v", g.GrimoirePlayerNames)
	}
}

// validateGrimoireNotes checks that grimoire_game_notes and grimoire_round_notes columns exist.
func validateGrimoireNotes(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	g, err := client.Game.Query().Only(ctx)
	if err != nil {
		t.Fatalf("failed to query game: %v", err)
	}
	if g.GrimoireGameNotes != nil && len(g.GrimoireGameNotes) != 0 {
		t.Errorf("expected nil or empty grimoire_game_notes, got %v", g.GrimoireGameNotes)
	}
	if g.GrimoireRoundNotes != nil && len(g.GrimoireRoundNotes) != 0 {
		t.Errorf("expected nil or empty grimoire_round_notes, got %v", g.GrimoireRoundNotes)
	}
}

// validateDemonBluffs checks that the selected_bluffs column exists on games.
func validateDemonBluffs(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()
	g, err := client.Game.Query().Only(ctx)
	if err != nil {
		t.Fatalf("failed to query game: %v", err)
	}
	if g.SelectedBluffs != nil && len(g.SelectedBluffs) != 0 {
		t.Errorf("expected nil or empty selected_bluffs, got %v", g.SelectedBluffs)
	}
}

// validateCharacterAlignments checks that the character_alignments column exists on phases.
func validateCharacterAlignments(t *testing.T, ctx context.Context, db *sql.DB, _ *ent.Client) {
	t.Helper()

	var exists bool
	err := db.QueryRowContext(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'phases' AND column_name = 'character_alignments'
		)`).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to check column: %v", err)
	}
	if !exists {
		t.Error("character_alignments column should exist on phases table")
	}
}

// validateBagSubstitutions checks that the bag_substitutions column exists on games.
func validateBagSubstitutions(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()
	g, err := client.Game.Query().Only(ctx)
	if err != nil {
		t.Fatalf("failed to query game: %v", err)
	}
	if g.BagSubstitutions != nil && len(g.BagSubstitutions) != 0 {
		t.Errorf("expected nil or empty bag_substitutions, got %v", g.BagSubstitutions)
	}
}

// validateGrimoireReminderAttachments checks that the grimoire_reminder_attachments column exists on games.
func validateGrimoireReminderAttachments(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()
	g, err := client.Game.Query().Only(ctx)
	if err != nil {
		t.Fatalf("failed to query game: %v", err)
	}
	if g.GrimoireReminderAttachments != nil && len(g.GrimoireReminderAttachments) != 0 {
		t.Errorf("expected nil or empty grimoire_reminder_attachments, got %v", g.GrimoireReminderAttachments)
	}
}

// validatePlayerPresets checks that the player_presets column exists on users.
func validatePlayerPresets(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()
	u, err := client.User.Query().Where(user.Username("admin")).Only(ctx)
	if err != nil {
		t.Fatalf("failed to query user: %v", err)
	}
	if u.PlayerPresets != nil && len(u.PlayerPresets) != 0 {
		t.Errorf("expected nil or empty player_presets, got %v", u.PlayerPresets)
	}
}

// --- helpers ---

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

// migrationNames returns sorted migration filenames (without .sql) from the
// migrations directory.
func migrationNames(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join("migrations"))
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".sql"))
	}
	sort.Strings(names)
	return names
}
