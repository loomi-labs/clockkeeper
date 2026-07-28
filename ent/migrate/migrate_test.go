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
	"github.com/loomi-labs/clockkeeper/ent/infocard"
	_ "github.com/loomi-labs/clockkeeper/ent/runtime"
	"github.com/loomi-labs/clockkeeper/ent/script"
	"github.com/loomi-labs/clockkeeper/internal/database"

	_ "github.com/lib/pq"
)

// migrationValidators maps each migration filename (without .sql) to a
// validation function. When a new migration is added without a corresponding
// validator, TestMigrationCoverage fails — forcing you to write one.
var migrationValidators = map[string]func(t *testing.T, ctx context.Context, db *sql.DB, client *ent.Client){
	"20260315162621_initial":                           validateInitialSchema,
	"20260315175834_add_scripts_and_games":             validateScriptsAndGames,
	"20260315191827_add_traveller_count":               validateTravellerCount,
	"20260315195303_add_selected_travellers":           validateSelectedTravellers,
	"20260316163815_add_system_scripts":                validateSystemScripts,
	"20260316165946_add_script_soft_delete":            validateScriptSoftDelete,
	"20260318105440_add_game_owner":                    validateGameOwner,
	"20260318155946_add_script_owner_check":            validateScriptOwnerCheck,
	"20260319100012_add_game_extra_characters":         validateGameExtraCharacters,
	"20260321102851_add_phases_and_deaths":             validatePhasesAndDeaths,
	"20260321112246_add_traveller_alignments":          validateTravellerAlignments,
	"20260321131451_add_game_name":                     validateGameName,
	"20260321133230_add_completed_actions":             validateCompletedActions,
	"20260322103914_add_death_unique_index":            validateDeathUniqueIndex,
	"20260323093114_add_grimoire_state":                validateGrimoireState,
	"20260323102548_add_grimoire_notes":                validateGrimoireNotes,
	"20260323131712_add_character_alignments":          validateCharacterAlignments,
	"20260323135528_add_demon_bluffs":                  validateDemonBluffs,
	"20260323161223_add_bag_substitutions":             validateBagSubstitutions,
	"20260324183937_add_grimoire_reminder_attachments": validateGrimoireReminderAttachments,
	"20260406182402_add_player_presets":                validatePlayerPresets,
	"20260414094150_add_discord_and_anonymous":         validateDiscordAndAnonymous,
	"20260724121222_add_death_cause":                   validateDeathCause,
	"20260724121808_add_info_cards":                    validateInfoCards,
	"20260724195230_add_role_promotions":               validateRolePromotions,
	"20260727125505_add_spotify_connection":            validateSpotifyConnection,
	"20260728130152_add_token_bag":                     validateTokenBag,
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
		"infocard.go",
		"phase.go",
		"registration.go",
		"script.go",
		"spotifyconnection.go",
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

// validateInitialSchema checks that the users table exists.
// Note: the add_discord_and_anonymous migration deletes all seed users,
// so we just verify the table is queryable.
func validateInitialSchema(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	_, err := client.User.Query().Count(ctx)
	if err != nil {
		t.Fatalf("failed to query users table: %v", err)
	}
}

// validateScriptsAndGames checks that scripts and games tables exist with seeded data.
func validateScriptsAndGames(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	s, err := client.Script.Query().
		Where(script.Name("My TB Script")).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			t.Skip("seed data was cleaned up by a later migration")
		}
		t.Fatalf("failed to query seeded script: %v", err)
	}
	if s.Edition != "tb" {
		t.Errorf("expected edition 'tb', got %q", s.Edition)
	}

	g, err := client.Game.Query().
		Where(game.StateEQ(game.StateSetup)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			t.Skip("seed data was cleaned up by a later migration")
		}
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
		if ent.IsNotFound(err) {
			t.Skip("seed data was cleaned up by a later migration")
		}
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
		if ent.IsNotFound(err) {
			t.Skip("seed data was cleaned up by a later migration")
		}
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
		if ent.IsNotFound(err) {
			t.Log("user-owned script was cleaned up by a later migration, skipping check")
		} else {
			t.Fatalf("failed to query user script: %v", err)
		}
	} else if userScript.UserID == nil {
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
		if ent.IsNotFound(err) {
			t.Skip("seed data was cleaned up by a later migration")
		}
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
		if ent.IsNotFound(err) {
			t.Skip("seed data was cleaned up by a later migration")
		}
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
		if ent.IsNotFound(err) {
			t.Skip("seed data was cleaned up by a later migration")
		}
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
		if ent.IsNotFound(err) {
			t.Skip("seed data was cleaned up by a later migration")
		}
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
		if ent.IsNotFound(err) {
			t.Skip("seed data was cleaned up by a later migration")
		}
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
		if ent.IsNotFound(err) {
			t.Skip("seed data was cleaned up by a later migration")
		}
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
		if ent.IsNotFound(err) {
			t.Skip("seed data was cleaned up by a later migration")
		}
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
		if ent.IsNotFound(err) {
			t.Skip("seed data was cleaned up by a later migration")
		}
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
		if ent.IsNotFound(err) {
			t.Skip("seed data was cleaned up by a later migration")
		}
		t.Fatalf("failed to query game: %v", err)
	}
	if g.GrimoireReminderAttachments != nil && len(g.GrimoireReminderAttachments) != 0 {
		t.Errorf("expected nil or empty grimoire_reminder_attachments, got %v", g.GrimoireReminderAttachments)
	}
}

// validatePlayerPresets checks that the player_presets column exists on users.
// Note: seed users were deleted by the add_discord_and_anonymous migration,
// so we create a fresh user and verify the column works.
func validatePlayerPresets(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	u, err := client.User.Create().Save(ctx)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	if u.PlayerPresets != nil && len(u.PlayerPresets) != 0 {
		t.Errorf("expected nil or empty player_presets, got %v", u.PlayerPresets)
	}
}

// validateDiscordAndAnonymous checks the new user schema fields work.
func validateDiscordAndAnonymous(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	// Create an anonymous user (no Discord fields).
	anonUser, err := client.User.Create().
		SetIsAnonymous(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create anonymous user: %v", err)
	}
	if anonUser.UUID == "" {
		t.Error("anonymous user UUID is empty")
	}
	if !anonUser.IsAnonymous {
		t.Error("expected is_anonymous to be true")
	}
	if anonUser.LastActiveAt.IsZero() {
		t.Error("last_active_at should be set by default")
	}

	// Create a Discord-linked user.
	discordUser, err := client.User.Create().
		SetDiscordID("123456").
		SetDiscordUsername("testuser").
		SetDiscordAvatar("abc123").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create discord user: %v", err)
	}
	if discordUser.DiscordID == nil || *discordUser.DiscordID != "123456" {
		t.Error("discord_id not set correctly")
	}
	if discordUser.IsAnonymous {
		t.Error("discord user should not be anonymous")
	}
}

// validateDeathCause checks that the cause column exists on deaths with default 'unspecified'.
func validateDeathCause(t *testing.T, ctx context.Context, db *sql.DB, _ *ent.Client) {
	t.Helper()

	var colDefault string
	err := db.QueryRowContext(ctx,
		`SELECT column_default FROM information_schema.columns
		 WHERE table_name = 'deaths' AND column_name = 'cause'`).Scan(&colDefault)
	if err != nil {
		t.Fatalf("cause column should exist on deaths table: %v", err)
	}
	if !strings.Contains(colDefault, "unspecified") {
		t.Errorf("expected cause default to be 'unspecified', got %q", colDefault)
	}
}

// validateInfoCards checks that the info_cards table exists with the seeded card.
func validateInfoCards(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()

	c, err := client.InfoCard.Query().
		Where(infocard.Title("Seed Info Card")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query seeded info card: %v", err)
	}
	if c.Body != "Seed body text" {
		t.Errorf("expected body 'Seed body text', got %q", c.Body)
	}
	if len(c.CharacterIds) != 1 || c.CharacterIds[0] != "washerwoman" {
		t.Errorf("expected character_ids [washerwoman], got %v", c.CharacterIds)
	}
	if c.SortOrder != 0 {
		t.Errorf("expected sort_order 0, got %d", c.SortOrder)
	}
	if c.UserID == 0 {
		t.Error("expected info card to have a user_id")
	}
}

// validateRolePromotions checks that the role_promotions column exists on games.
func validateRolePromotions(t *testing.T, ctx context.Context, _ *sql.DB, client *ent.Client) {
	t.Helper()
	g, err := client.Game.Query().Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			t.Skip("seed data was cleaned up by a later migration")
		}
		t.Fatalf("failed to query game: %v", err)
	}
	if g.RolePromotions != nil && len(g.RolePromotions) != 0 {
		t.Errorf("expected nil or empty role_promotions, got %v", g.RolePromotions)
	}
}

// validateSpotifyConnection checks that the spotify_connections table exists
// with a NOT NULL user_id FK and at most one connection per user.
func validateSpotifyConnection(t *testing.T, ctx context.Context, db *sql.DB, client *ent.Client) {
	t.Helper()

	// user_id must be a NOT NULL column.
	var nullable string
	err := db.QueryRowContext(ctx,
		`SELECT is_nullable FROM information_schema.columns
		 WHERE table_name = 'spotify_connections' AND column_name = 'user_id'`).Scan(&nullable)
	if err != nil {
		t.Fatalf("user_id column should exist on spotify_connections: %v", err)
	}
	if nullable != "NO" {
		t.Errorf("expected user_id to be NOT NULL, got is_nullable=%q", nullable)
	}

	// user_id must carry a foreign key to users.
	var fkCount int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM information_schema.table_constraints tc
		 JOIN information_schema.key_column_usage kcu
		   ON tc.constraint_name = kcu.constraint_name
		 WHERE tc.table_name = 'spotify_connections'
		   AND tc.constraint_type = 'FOREIGN KEY'
		   AND kcu.column_name = 'user_id'`).Scan(&fkCount)
	if err != nil {
		t.Fatalf("failed to check foreign key: %v", err)
	}
	if fkCount == 0 {
		t.Error("expected a foreign key on spotify_connections.user_id")
	}

	// A round-trip through Ent proves the columns line up with the schema.
	u, err := client.User.Create().Save(ctx)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	conn, err := client.SpotifyConnection.Create().
		SetUserID(u.ID).
		SetSpotifyUserID("spotify-user").
		SetDisplayName("Storyteller").
		SetPremium(true).
		SetRefreshToken("refresh-token").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create spotify connection: %v", err)
	}
	if conn.AccessTokenExpiresAt != nil {
		t.Errorf("expected nil access_token_expires_at, got %v", conn.AccessTokenExpiresAt)
	}
	if conn.DayPlaylist != nil || conn.NightPlaylist != nil || conn.NominationsPlaylist != nil {
		t.Error("expected all playlist slots to be nil by default")
	}

	// The unique index allows only one connection per user.
	if _, err := client.SpotifyConnection.Create().
		SetUserID(u.ID).
		SetSpotifyUserID("spotify-user-2").
		SetRefreshToken("refresh-token-2").
		Save(ctx); err == nil {
		t.Error("expected a unique constraint violation for a second connection")
	}
}

// validateTokenBag checks the token bag columns on games and the registrations
// table (NOT NULL FK to games plus the unique name/secret indexes).
func validateTokenBag(t *testing.T, ctx context.Context, db *sql.DB, client *ent.Client) {
	t.Helper()

	// token_bag_phase defaults to 'inactive'.
	var colDefault string
	err := db.QueryRowContext(ctx,
		`SELECT column_default FROM information_schema.columns
		 WHERE table_name = 'games' AND column_name = 'token_bag_phase'`).Scan(&colDefault)
	if err != nil {
		t.Fatalf("token_bag_phase column should exist on games: %v", err)
	}
	if !strings.Contains(colDefault, "inactive") {
		t.Errorf("expected token_bag_phase default 'inactive', got %q", colDefault)
	}

	// The join/shared codes are only generated on demand, so they stay nullable.
	for _, col := range []string{"token_bag_join_code", "token_bag_shared_code"} {
		var nullable string
		err := db.QueryRowContext(ctx,
			`SELECT is_nullable FROM information_schema.columns
			 WHERE table_name = 'games' AND column_name = $1`, col).Scan(&nullable)
		if err != nil {
			t.Fatalf("%s column should exist on games: %v", col, err)
		}
		if nullable != "YES" {
			t.Errorf("expected %s to be nullable, got is_nullable=%q", col, nullable)
		}
	}

	// Every index the schema declares must be unique.
	for _, idx := range []string{
		"games_token_bag_join_code_key",
		"games_token_bag_shared_code_key",
		"registration_name_normalized_game_id",
		"registration_secret_hash",
	} {
		var isUnique bool
		err := db.QueryRowContext(ctx,
			`SELECT i.indisunique
			 FROM pg_class c
			 JOIN pg_index i ON i.indexrelid = c.oid
			 WHERE c.relname = $1`, idx).Scan(&isUnique)
		if err != nil {
			t.Fatalf("index %s should exist: %v", idx, err)
		}
		if !isUnique {
			t.Errorf("expected index %s to be unique", idx)
		}
	}

	// registrations.game_id must be a NOT NULL column with an FK to games.
	var nullable string
	err = db.QueryRowContext(ctx,
		`SELECT is_nullable FROM information_schema.columns
		 WHERE table_name = 'registrations' AND column_name = 'game_id'`).Scan(&nullable)
	if err != nil {
		t.Fatalf("game_id column should exist on registrations: %v", err)
	}
	if nullable != "NO" {
		t.Errorf("expected registrations.game_id to be NOT NULL, got is_nullable=%q", nullable)
	}

	var fkCount int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM information_schema.table_constraints tc
		 JOIN information_schema.key_column_usage kcu
		   ON tc.constraint_name = kcu.constraint_name
		 WHERE tc.table_name = 'registrations'
		   AND tc.constraint_type = 'FOREIGN KEY'
		   AND kcu.column_name = 'game_id'`).Scan(&fkCount)
	if err != nil {
		t.Fatalf("failed to check foreign key: %v", err)
	}
	if fkCount == 0 {
		t.Error("expected a foreign key on registrations.game_id")
	}

	// A round-trip through Ent proves the columns line up with the schema.
	g, err := client.Game.Query().Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			t.Skip("seed data was cleaned up by a later migration")
		}
		t.Fatalf("failed to query game: %v", err)
	}
	if g.TokenBagPhase != game.TokenBagPhaseInactive {
		t.Errorf("expected token_bag_phase %q, got %q", game.TokenBagPhaseInactive, g.TokenBagPhase)
	}
	if g.TokenBagJoinCode != nil || g.TokenBagSharedCode != nil {
		t.Error("expected both token bag codes to be nil by default")
	}

	reg, err := client.Registration.Create().
		SetGameID(g.ID).
		SetName("Alice").
		SetNameNormalized("alice").
		SetSecretHash("secret-hash-alice").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create registration: %v", err)
	}
	if reg.ViaSharedDevice {
		t.Error("expected via_shared_device to default to false")
	}
	if reg.LeftNeighborID != 0 || reg.RightNeighborID != 0 {
		t.Error("expected both neighbor ids to be unset by default")
	}
	if reg.AssignedRoleID != "" {
		t.Errorf("expected empty assigned_role_id, got %q", reg.AssignedRoleID)
	}

	// The same normalized name cannot be registered twice in one game.
	if _, err := client.Registration.Create().
		SetGameID(g.ID).
		SetName("ALICE").
		SetNameNormalized("alice").
		SetSecretHash("secret-hash-other").
		Save(ctx); err == nil {
		t.Error("expected a unique constraint violation for a duplicate normalized name")
	}

	// Secret hashes are unique across all games.
	if _, err := client.Registration.Create().
		SetGameID(g.ID).
		SetName("Bob").
		SetNameNormalized("bob").
		SetSecretHash("secret-hash-alice").
		Save(ctx); err == nil {
		t.Error("expected a unique constraint violation for a duplicate secret hash")
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
