package web

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/game"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
)

// defaultWatchHeartbeat is how often an idle watch stream re-sends its snapshot.
// The traffic keeps NATs and proxies from dropping an idle connection, and the
// failing Send is how the server notices a client that vanished without closing.
const defaultWatchHeartbeat = 30 * time.Second

// WatchTokenBag streams the player-facing view of a token bag: one snapshot up
// front, then a fresh one whenever the game changes.
//
// There are no sequence numbers or deltas — reconnecting simply re-dials and the
// first message is complete state.
func (h *ClockKeeperServiceHandler) WatchTokenBag(ctx context.Context, req *connect.Request[clockkeeperv1.WatchTokenBagRequest], stream *connect.ServerStream[clockkeeperv1.WatchTokenBagResponse]) error {
	g, _, err := h.gameByBagCode(ctx, req.Msg.Code)
	if err != nil {
		return err
	}

	tick, cancel := h.hub.Subscribe(g.ID)
	defer cancel()

	heartbeat := time.NewTicker(h.watchHeartbeat())
	defer heartbeat.Stop()

	// Subscribe happened before this first read, so no change can slip through
	// the gap between the initial snapshot and the loop.
	for {
		snapshot, ended, err := h.watchSnapshot(ctx, g.ID, req.Msg.Code, req.Msg.RegistrationSecret)
		if err != nil {
			if ctx.Err() != nil {
				return nil // the client hung up mid-read
			}
			return err
		}
		if err := stream.Send(snapshot); err != nil {
			return err // client disconnected
		}
		if ended {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-h.hub.Done():
			return nil // server shutting down
		case <-tick:
		case <-heartbeat.C:
		}
	}
}

// watchSnapshot re-reads a game and renders one subscriber's view of its token
// bag. ended reports that the bag this subscriber watches is gone — the game was
// deleted, or the code is not one of its codes — in which case the returned
// snapshot is the final one and the stream must end.
func (h *ClockKeeperServiceHandler) watchSnapshot(ctx context.Context, gameID int, code, secret string) (snapshot *clockkeeperv1.WatchTokenBagResponse, ended bool, err error) {
	g, err := h.db.Game.Get(ctx, gameID)
	if err != nil {
		if ent.IsNotFound(err) {
			return endedWatchSnapshot(""), true, nil
		}
		if ctx.Err() != nil {
			return nil, false, err // watcher disconnected — not a server fault
		}
		slog.Error("get game for token bag watch failed", "err", err)
		return nil, false, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Defensive: no path takes an opened bag back to inactive or changes its
	// codes any more (a reset keeps both — see ResetTokenBag), so this only fires
	// for a bag that never existed. A watcher whose code has stopped matching
	// must still be told, not left hanging on a stale snapshot.
	if g.TokenBagPhase == game.TokenBagPhaseInactive || !bagCodeMatches(g, code) {
		return endedWatchSnapshot(g.Name), true, nil
	}

	regs, err := h.bagRegistrations(ctx, g.ID)
	if err != nil {
		return nil, false, err
	}

	selfReg, err := h.watchSelfRegistration(ctx, g, secret)
	if err != nil {
		return nil, false, err
	}

	snapshot, err = h.buildWatchSnapshot(g, regs, selfReg)
	if err != nil {
		return nil, false, err
	}
	return snapshot, false, nil
}

// watchSelfRegistration resolves the watcher's own registration from the secret
// their device holds. A missing, unknown or foreign secret yields no self and no
// error: a removed player keeps watching the bag as an anonymous spectator, and a
// shared device watches without a secret at all.
//
// Anything else — a cancelled request, a database failure — is returned. Zeroing
// out the self fields on a transient error would make a revealed player's token
// vanish from their screen, indistinguishable from being kicked.
func (h *ClockKeeperServiceHandler) watchSelfRegistration(ctx context.Context, g *ent.Game, secret string) (*ent.Registration, error) {
	if secret == "" {
		return nil, nil
	}

	r, regGame, err := h.registrationBySecret(ctx, secret)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, err
	}
	if regGame.ID != g.ID {
		// The secret belongs to another game's bag.
		return nil, nil
	}
	return r, nil
}

// endedWatchSnapshot is the last message a stream sends: the bag is gone, so
// there are no players and no token, whatever the watcher saw before.
func endedWatchSnapshot(gameName string) *clockkeeperv1.WatchTokenBagResponse {
	return &clockkeeperv1.WatchTokenBagResponse{
		Phase:    clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_INACTIVE,
		GameName: gameName,
	}
}

// bagCodeMatches reports whether a code is still one of the game's token bag
// codes.
func bagCodeMatches(g *ent.Game, code string) bool {
	if code == "" {
		return false
	}
	if g.TokenBagJoinCode != nil && *g.TokenBagJoinCode == code {
		return true
	}
	return g.TokenBagSharedCode != nil && *g.TokenBagSharedCode == code
}

// watchHeartbeat returns the heartbeat interval, overridable per handler so
// tests don't have to wait half a minute.
func (h *ClockKeeperServiceHandler) watchHeartbeat() time.Duration {
	if h.watchHeartbeatInterval > 0 {
		return h.watchHeartbeatInterval
	}
	return defaultWatchHeartbeat
}

// publishTokenBag wakes the watchers of a game's token bag. Safe to call on a
// handler built without a hub.
func (h *ClockKeeperServiceHandler) publishTokenBag(gameID int) {
	if h.hub != nil {
		h.hub.Publish(gameID)
	}
}
