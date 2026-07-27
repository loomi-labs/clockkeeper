package web

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent/user"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// userIDFromToken extracts the user ID from an issued JWT via the auth interceptor.
func userIDFromToken(t *testing.T, handler *ClockKeeperServiceHandler, token string) int {
	t.Helper()

	userID, _, err := handler.auth.validate("Bearer " + token)
	require.NoError(t, err)
	return userID
}

func TestCreateAnonymousSession_DevSingleUserSharesOneUser(t *testing.T) {
	handler := testHandler(t)
	handler.config.DevSingleUser = true
	ctx := context.Background()

	first, err := handler.CreateAnonymousSession(ctx, connect.NewRequest(&clockkeeperv1.CreateAnonymousSessionRequest{}))
	require.NoError(t, err)
	second, err := handler.CreateAnonymousSession(ctx, connect.NewRequest(&clockkeeperv1.CreateAnonymousSessionRequest{}))
	require.NoError(t, err)

	firstID := userIDFromToken(t, handler, first.Msg.Token)
	secondID := userIDFromToken(t, handler, second.Msg.Token)
	assert.Equal(t, firstID, secondID, "both sessions should resolve to the same dev user")

	count, err := handler.db.User.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "dev mode must not create extra users")

	u, err := handler.db.User.Get(ctx, firstID)
	require.NoError(t, err)
	assert.Equal(t, devSingleUserUUID, u.UUID)
	assert.False(t, u.IsAnonymous, "dev user must not be anonymous or cleanup would delete it")
}

func TestCreateAnonymousSession_CreatesDistinctUsersByDefault(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	require.False(t, handler.config.DevSingleUser, "DevSingleUser must default to false")

	first, err := handler.CreateAnonymousSession(ctx, connect.NewRequest(&clockkeeperv1.CreateAnonymousSessionRequest{}))
	require.NoError(t, err)
	second, err := handler.CreateAnonymousSession(ctx, connect.NewRequest(&clockkeeperv1.CreateAnonymousSessionRequest{}))
	require.NoError(t, err)

	firstID := userIDFromToken(t, handler, first.Msg.Token)
	secondID := userIDFromToken(t, handler, second.Msg.Token)
	assert.NotEqual(t, firstID, secondID)

	count, err := handler.db.User.Query().Where(user.IsAnonymous(true)).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestCleanupAnonymousUsers_SkipsDevSingleUser(t *testing.T) {
	handler := testHandler(t)
	handler.config.DevSingleUser = true
	ctx := context.Background()

	resp, err := handler.CreateAnonymousSession(ctx, connect.NewRequest(&clockkeeperv1.CreateAnonymousSessionRequest{}))
	require.NoError(t, err)
	devUserID := userIDFromToken(t, handler, resp.Msg.Token)

	// Make the dev user look long abandoned.
	require.NoError(t, handler.db.User.UpdateOneID(devUserID).
		SetLastActiveAt(time.Now().Add(-100*24*time.Hour)).
		Exec(ctx))

	// A genuinely stale anonymous user to prove the cleanup did run.
	stale, err := handler.db.User.Create().
		SetIsAnonymous(true).
		SetLastActiveAt(time.Now().Add(-100 * 24 * time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	cleanupAnonymousUsers(ctx, handler.db, 24*time.Hour)

	exists, err := handler.db.User.Query().Where(user.ID(devUserID)).Exist(ctx)
	require.NoError(t, err)
	assert.True(t, exists, "dev single user must survive anonymous cleanup")

	exists, err = handler.db.User.Query().Where(user.ID(stale.ID)).Exist(ctx)
	require.NoError(t, err)
	assert.False(t, exists, "stale anonymous user should have been deleted")
}
