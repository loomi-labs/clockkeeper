package web

import (
	"context"
	"log/slog"
	"time"

	"github.com/shifty11/clockkeeper/ent"
	"github.com/shifty11/clockkeeper/ent/death"
	"github.com/shifty11/clockkeeper/ent/game"
	"github.com/shifty11/clockkeeper/ent/phase"
	"github.com/shifty11/clockkeeper/ent/script"
	"github.com/shifty11/clockkeeper/ent/user"
)

// startCleanup periodically deletes anonymous users that have been inactive
// for longer than maxAge. It blocks until ctx is cancelled.
func startCleanup(ctx context.Context, db *ent.Client, maxAge time.Duration) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanupAnonymousUsers(ctx, db, maxAge)
		}
	}
}

func cleanupAnonymousUsers(ctx context.Context, db *ent.Client, maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)

	staleUsers, err := db.User.Query().
		Where(
			user.IsAnonymous(true),
			user.LastActiveAtLT(cutoff),
		).
		All(ctx)
	if err != nil {
		slog.Error("cleanup: failed to query stale anonymous users", "err", err)
		return
	}

	if len(staleUsers) == 0 {
		return
	}

	var deleted int
	for _, u := range staleUsers {
		if err := deleteUserCascade(ctx, db, u.ID); err != nil {
			slog.Error("cleanup: failed to delete anonymous user", "user_id", u.ID, "err", err)
			continue
		}
		deleted++
	}

	slog.Info("cleanup: deleted stale anonymous users", "count", deleted)
}

// deleteUserCascade deletes a user and all their related data.
func deleteUserCascade(ctx context.Context, db *ent.Client, userID int) error {
	// Get all game IDs for this user.
	gameIDs, err := db.Game.Query().
		Where(game.UserID(userID)).
		IDs(ctx)
	if err != nil {
		return err
	}

	// Delete deaths and phases for those games.
	if len(gameIDs) > 0 {
		phaseIDs, err := db.Phase.Query().
			Where(phase.GameIDIn(gameIDs...)).
			IDs(ctx)
		if err != nil {
			return err
		}
		if len(phaseIDs) > 0 {
			if _, err := db.Death.Delete().
				Where(death.PhaseIDIn(phaseIDs...)).
				Exec(ctx); err != nil {
				return err
			}
		}
		if _, err := db.Phase.Delete().
			Where(phase.GameIDIn(gameIDs...)).
			Exec(ctx); err != nil {
			return err
		}
		if _, err := db.Game.Delete().
			Where(game.UserID(userID)).
			Exec(ctx); err != nil {
			return err
		}
	}

	// Delete user's scripts.
	if _, err := db.Script.Delete().
		Where(script.UserID(userID)).
		Exec(ctx); err != nil {
		return err
	}

	// Delete the user.
	return db.User.DeleteOneID(userID).Exec(ctx)
}
