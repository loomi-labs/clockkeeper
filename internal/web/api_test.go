package web

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/loomi-labs/clockkeeper/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testHandler(t *testing.T) *ClockKeeperServiceHandler {
	t.Helper()

	config := database.SetupPostgreSQLContainer(t)
	migrator := database.NewMigrator(t, config)
	migrator.ApplyN(t, -1)

	client, _, err := database.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create ent client: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	auth := NewAuthInterceptor("test-jwt-secret")

	return &ClockKeeperServiceHandler{
		config: &Config{JWTSecretKey: "test-jwt-secret"},
		db:     client,
		auth:   auth,
	}
}

func TestLogin_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	handler := testHandler(t)
	ctx := context.Background()

	// Create a user.
	hash, err := HashPassword("password123")
	require.NoError(t, err)
	_, err = handler.db.User.Create().
		SetUsername("testuser").
		SetPasswordHash(hash).
		Save(ctx)
	require.NoError(t, err)

	// Login.
	resp, err := handler.Login(ctx, connect.NewRequest(&clockkeeperv1.LoginRequest{
		Username: "testuser",
		Password: "password123",
	}))
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Token)

	// Verify returned token is valid.
	_, err = handler.auth.validate("Bearer " + resp.Msg.Token)
	assert.NoError(t, err)
}

func TestLogin_WrongPassword(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	handler := testHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("correct")
	require.NoError(t, err)
	_, err = handler.db.User.Create().
		SetUsername("testuser").
		SetPasswordHash(hash).
		Save(ctx)
	require.NoError(t, err)

	_, err = handler.Login(ctx, connect.NewRequest(&clockkeeperv1.LoginRequest{
		Username: "testuser",
		Password: "wrong",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestLogin_NonexistentUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	handler := testHandler(t)
	ctx := context.Background()

	_, err := handler.Login(ctx, connect.NewRequest(&clockkeeperv1.LoginRequest{
		Username: "nobody",
		Password: "password",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

// authedCtx returns a context with the given username set for auth.
func authedCtx(username string) context.Context {
	return context.WithValue(context.Background(), usernameKey, username)
}

func TestListScripts_IncludesSystemScripts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	handler := testHandler(t)
	ctx := context.Background()

	// Create a user and a user-owned script.
	hash, err := HashPassword("pass")
	require.NoError(t, err)
	u, err := handler.db.User.Create().
		SetUsername("testuser").
		SetPasswordHash(hash).
		Save(ctx)
	require.NoError(t, err)

	_, err = handler.db.Script.Create().
		SetName("My Custom Script").
		SetCharacterIds([]string{"washerwoman"}).
		SetUserID(u.ID).
		Save(ctx)
	require.NoError(t, err)

	// List scripts as this user.
	resp, err := handler.ListScripts(authedCtx("testuser"), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)

	// Should include 3 system scripts + 1 user script.
	assert.Len(t, resp.Msg.Scripts, 4)

	systemCount := 0
	userCount := 0
	for _, s := range resp.Msg.Scripts {
		if s.IsSystem {
			systemCount++
		} else {
			userCount++
		}
	}
	assert.Equal(t, 3, systemCount, "expected 3 system scripts")
	assert.Equal(t, 1, userCount, "expected 1 user script")
}

func TestUpdateScript_BlocksSystemScript(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	handler := testHandler(t)
	ctx := context.Background()

	// Create a user for auth context.
	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().
		SetUsername("testuser").
		SetPasswordHash(hash).
		Save(ctx)
	require.NoError(t, err)

	// Find a system script via listing.
	resp, err := handler.ListScripts(authedCtx("testuser"), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)

	var systemID int64
	for _, s := range resp.Msg.Scripts {
		if s.IsSystem {
			systemID = s.Id
			break
		}
	}
	require.NotZero(t, systemID, "expected at least one system script")

	_, err = handler.UpdateScript(authedCtx("testuser"), connect.NewRequest(&clockkeeperv1.UpdateScriptRequest{
		Id:   systemID,
		Name: "Hacked",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestDeleteScript_BlocksSystemScript(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	handler := testHandler(t)
	ctx := context.Background()

	// Create a user for auth context.
	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().
		SetUsername("testuser").
		SetPasswordHash(hash).
		Save(ctx)
	require.NoError(t, err)

	// Find a system script via listing.
	resp, err := handler.ListScripts(authedCtx("testuser"), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)

	var systemID int64
	for _, s := range resp.Msg.Scripts {
		if s.IsSystem {
			systemID = s.Id
			break
		}
	}
	require.NotZero(t, systemID, "expected at least one system script")

	_, err = handler.DeleteScript(authedCtx("testuser"), connect.NewRequest(&clockkeeperv1.DeleteScriptRequest{
		Id: systemID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}
