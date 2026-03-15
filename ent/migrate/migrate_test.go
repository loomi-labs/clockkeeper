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
	_ "github.com/loomi-labs/clockkeeper/ent/runtime"
	"github.com/loomi-labs/clockkeeper/ent/user"
	"github.com/loomi-labs/clockkeeper/internal/database"

	_ "github.com/lib/pq"
)

// migrationValidators maps each migration filename (without .sql) to a
// validation function. When a new migration is added without a corresponding
// validator, TestMigrationCoverage fails — forcing you to write one.
var migrationValidators = map[string]func(t *testing.T, ctx context.Context, db *sql.DB, client *ent.Client){
	"20260315162621_initial": validateInitialSchema,
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
