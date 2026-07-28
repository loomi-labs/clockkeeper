package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"unicode"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/game"
	"github.com/loomi-labs/clockkeeper/ent/registration"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/loomi-labs/clockkeeper/internal/botc"
)

// maxTokenBagRegistrations caps the number of players in one game's token bag
// (15 seats + travellers + slack).
const maxTokenBagRegistrations = 25

// A display name is bounded twice. The rune bound is what a player perceives as
// "50 characters"; the byte bound mirrors the Registration.name schema's
// MaxLen(50), which counts BYTES. Without the byte bound a short but wide name
// (CJK, emoji) would pass validation here and only fail inside Save() as an ent
// validation error.
const (
	maxTokenBagNameRunes = 50
	maxTokenBagNameBytes = 50
)

// newBagCode returns a fresh join or shared code for a game's token bag. Codes
// are handed out in QR links, so they must be unguessable.
func newBagCode() (string, error) {
	return randomToken(16)
}

// newRegistrationSecret returns the bearer credential a player's device stores
// after joining. Only its hash is persisted.
func newRegistrationSecret() (string, error) {
	return randomToken(32)
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashSecret returns the hex-encoded SHA-256 of a registration secret. Secrets
// are high-entropy random tokens, so a plain hash (no KDF) is sufficient.
func hashSecret(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// normalizeName cleans a player-typed name into its display form (control
// characters dropped, whitespace runs collapsed to single spaces, trimmed) and
// the case-folded form used to enforce uniqueness within a game.
func normalizeName(raw string) (display string, normalized string, err error) {
	var b strings.Builder
	pendingSpace := false
	for _, r := range raw {
		switch {
		case unicode.IsSpace(r):
			pendingSpace = true
		case unicode.IsControl(r):
			// Dropped: control characters have no place in a display name.
		default:
			if pendingSpace && b.Len() > 0 {
				b.WriteRune(' ')
			}
			pendingSpace = false
			b.WriteRune(r)
		}
	}

	display = b.String()
	if display == "" {
		return "", "", errors.New("name must not be empty")
	}
	if utf8.RuneCountInString(display) > maxTokenBagNameRunes {
		return "", "", errors.New("name must be at most 50 characters")
	}
	if len(display) > maxTokenBagNameBytes {
		return "", "", errors.New("name is too long — please use a shorter one")
	}
	return display, strings.ToLower(display), nil
}

// gameByBagCode looks up the game a token bag code belongs to. isShared reports
// whether the code is the shared-device code rather than the player join code.
// Unknown and empty codes are indistinguishable to the caller.
func (h *ClockKeeperServiceHandler) gameByBagCode(ctx context.Context, code string) (g *ent.Game, isShared bool, err error) {
	notFound := connect.NewError(connect.CodeNotFound, errors.New("token bag not found"))
	if code == "" {
		return nil, false, notFound
	}

	g, err = h.db.Game.Query().
		Where(game.Or(
			game.TokenBagJoinCode(code),
			game.TokenBagSharedCode(code),
		)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, false, notFound
		}
		slog.Error("get game by token bag code failed", "err", err)
		return nil, false, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	isShared = g.TokenBagSharedCode != nil && *g.TokenBagSharedCode == code
	return g, isShared, nil
}

// registrationBySecret resolves a player's registration (and its game) from the
// raw secret their device holds. A wrong code and a wrong secret both yield the
// same CodeNotFound so the API is not an oracle for either.
func (h *ClockKeeperServiceHandler) registrationBySecret(ctx context.Context, secret string) (*ent.Registration, *ent.Game, error) {
	notFound := connect.NewError(connect.CodeNotFound, errors.New("registration not found"))
	if secret == "" {
		return nil, nil, notFound
	}

	r, err := h.db.Registration.Query().
		Where(registration.SecretHash(hashSecret(secret))).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil, notFound
		}
		if ctx.Err() != nil {
			return nil, nil, abandonedRequestError(ctx)
		}
		slog.Error("get registration by secret failed", "err", err)
		return nil, nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	g, err := h.db.Game.Get(ctx, r.GameID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil, notFound
		}
		if ctx.Err() != nil {
			return nil, nil, abandonedRequestError(ctx)
		}
		slog.Error("get game for registration failed", "err", err)
		return nil, nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return r, g, nil
}

// bagRegistrations lists a game's registrations in join order. Every listing of
// token bag players uses this order.
func (h *ClockKeeperServiceHandler) bagRegistrations(ctx context.Context, gameID int) ([]*ent.Registration, error) {
	regs, err := h.db.Registration.Query().
		Where(registration.GameID(gameID)).
		Order(ent.Asc(registration.FieldID)).
		All(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, abandonedRequestError(ctx)
		}
		slog.Error("list token bag registrations failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	return regs, nil
}

// abandonedRequestError classifies a query that failed because its own request
// context ended. Callers must only reach it with ctx.Err() != nil.
//
// It is not logged: a caller that hung up is not a server fault, and the watch
// stream re-queries on every tick, so an ordinary player closing their phone
// would otherwise log an error every time.
func abandonedRequestError(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return connect.NewError(connect.CodeDeadlineExceeded, ctx.Err())
	}
	return connect.NewError(connect.CodeCanceled, ctx.Err())
}

// buildWatchSnapshot builds the player-facing view of a token bag. selfReg is
// the requesting player's registration, when known. The self token is included
// only once the bag is revealed and a role was assigned to that player.
func (h *ClockKeeperServiceHandler) buildWatchSnapshot(g *ent.Game, regs []*ent.Registration, selfReg *ent.Registration) (*clockkeeperv1.WatchTokenBagResponse, error) {
	snapshot := &clockkeeperv1.WatchTokenBagResponse{
		Phase:    tokenBagPhaseToProto(g.TokenBagPhase),
		GameName: g.Name,
		Players:  make([]*clockkeeperv1.TokenBagPlayer, len(regs)),
	}
	for i, r := range regs {
		snapshot.Players[i] = registrationToProto(r)
	}

	if selfReg != nil {
		snapshot.SelfRegistrationId = int64(selfReg.ID)
		if g.TokenBagPhase == game.TokenBagPhaseRevealed && selfReg.AssignedRoleID != "" {
			c, err := resolveTokenCharacter(g, selfReg.AssignedRoleID, h.registry)
			if err != nil {
				return nil, err
			}
			snapshot.SelfToken = characterToProto(c)
		}
	}

	return snapshot, nil
}

// resolveTokenCharacter maps a registration's assigned role id to the character
// the player actually holds. Bag substitutions win: a seat that drew the Drunk
// holds the townsfolk token the substitution names, and must never learn the
// Drunk id.
//
// role_promotions are intentionally ignored — reveal happens at game start,
// before any star pass; the snapshot stores the original role.
func resolveTokenCharacter(g *ent.Game, roleID string, registry *botc.Registry) (*botc.Character, error) {
	if roleID == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no token assigned yet"))
	}

	resolved := roleID
	for _, sub := range g.BagSubstitutions {
		if sub.CausedByID == roleID && sub.CharacterID != "" {
			resolved = sub.CharacterID
			break
		}
	}

	c, ok := registry.Character(resolved)
	if !ok {
		slog.Error("token character not in registry", "role_id", resolved, "game_id", g.ID)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	return c, nil
}
