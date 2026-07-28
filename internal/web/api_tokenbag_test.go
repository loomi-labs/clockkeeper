package web

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent/registration"
	"github.com/loomi-labs/clockkeeper/ent/user"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bagGame is a game with an open token bag, plus the codes handed to players.
type bagGame struct {
	ownerID    int
	gameID     int64
	joinCode   string
	sharedCode string
}

// bagTestRoles are the roles these tests put in play. Reveal only deals roles
// the storyteller selected, so every grimoire name a test writes has to name one
// of these — see setSelectedRoles for a test that needs a different set.
var bagTestRoles = []string{"chef", "imp", "mayor", "drunk"}

// setSelectedRoles puts an exact set of roles in play, bypassing the RPC so a
// test can also select a role id the registry does not know.
func setSelectedRoles(t *testing.T, h *ClockKeeperServiceHandler, gameID int64, roleIDs ...string) {
	t.Helper()

	_, err := h.db.Game.UpdateOneID(int(gameID)).SetSelectedRoles(roleIDs).Save(context.Background())
	require.NoError(t, err)
}

// createBagGame creates a user, a game with bagTestRoles in play, and opens its
// token bag.
func createBagGame(t *testing.T, h *ClockKeeperServiceHandler) bagGame {
	t.Helper()

	ownerID, gameID := createTestGame(t, h)
	setSelectedRoles(t, h, gameID, bagTestRoles...)

	resp, err := h.OpenTokenBagRegistration(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.OpenTokenBagRegistrationRequest{
		GameId: gameID,
	}))
	require.NoError(t, err)
	require.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_OPEN, resp.Msg.TokenBag.Phase)
	require.NotEmpty(t, resp.Msg.TokenBag.JoinCode)
	require.NotEmpty(t, resp.Msg.TokenBag.SharedCode)

	return bagGame{
		ownerID:    ownerID,
		gameID:     gameID,
		joinCode:   resp.Msg.TokenBag.JoinCode,
		sharedCode: resp.Msg.TokenBag.SharedCode,
	}
}

// joinBag registers a player and returns their registration id and raw secret.
func joinBag(t *testing.T, h *ClockKeeperServiceHandler, joinCode, name string) (int64, string) {
	t.Helper()

	resp, err := h.JoinTokenBag(context.Background(), connect.NewRequest(&clockkeeperv1.JoinTokenBagRequest{
		JoinCode: joinCode,
		Name:     name,
	}))
	require.NoError(t, err)
	return resp.Msg.RegistrationId, resp.Msg.RegistrationSecret
}

// setGrimoireNames writes the grimoire's role_id -> player name map.
func setGrimoireNames(t *testing.T, h *ClockKeeperServiceHandler, ownerID int, gameID int64, names map[string]string) {
	t.Helper()

	_, err := h.UpdateGrimoireState(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.UpdateGrimoireStateRequest{
		GameId:      gameID,
		PlayerNames: names,
	}))
	require.NoError(t, err)
}

func closeBag(t *testing.T, h *ClockKeeperServiceHandler, bag bagGame) {
	t.Helper()

	_, err := h.CloseTokenBagRegistration(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.CloseTokenBagRegistrationRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)
}

func revealBag(t *testing.T, h *ClockKeeperServiceHandler, bag bagGame) {
	t.Helper()

	_, err := h.RevealTokenBag(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.RevealTokenBagRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)
}

func getBag(t *testing.T, h *ClockKeeperServiceHandler, bag bagGame) *clockkeeperv1.TokenBag {
	t.Helper()

	resp, err := h.GetTokenBag(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.GetTokenBagRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)
	return resp.Msg.TokenBag
}

// --- Open / Close / Reset ---

func TestOpenTokenBag_StartsFromInactiveAndSurfacesOnGame(t *testing.T) {
	h := testHandler(t)
	ownerID, gameID := createTestGame(t, h)

	before, err := h.GetGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.GetGameRequest{Id: gameID}))
	require.NoError(t, err)
	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_INACTIVE, before.Msg.Game.TokenBagPhase)
	assert.Empty(t, before.Msg.Game.TokenBagJoinCode)
	assert.Empty(t, before.Msg.Game.TokenBagSharedCode)

	openResp, err := h.OpenTokenBagRegistration(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.OpenTokenBagRegistrationRequest{
		GameId: gameID,
	}))
	require.NoError(t, err)
	assert.NotEqual(t, openResp.Msg.TokenBag.JoinCode, openResp.Msg.TokenBag.SharedCode)
	assert.Empty(t, openResp.Msg.TokenBag.Players)

	after, err := h.GetGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.GetGameRequest{Id: gameID}))
	require.NoError(t, err)
	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_OPEN, after.Msg.Game.TokenBagPhase)
	assert.Equal(t, openResp.Msg.TokenBag.JoinCode, after.Msg.Game.TokenBagJoinCode)
	assert.Equal(t, openResp.Msg.TokenBag.SharedCode, after.Msg.Game.TokenBagSharedCode)
}

func TestOpenTokenBag_AlreadyOpenIsANoOp(t *testing.T) {
	h := testHandler(t)
	bag := createBagGame(t, h)

	resp, err := h.OpenTokenBagRegistration(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.OpenTokenBagRegistrationRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)
	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_OPEN, resp.Msg.TokenBag.Phase)
	assert.Equal(t, bag.joinCode, resp.Msg.TokenBag.JoinCode, "re-opening must not rotate the join code")
	assert.Equal(t, bag.sharedCode, resp.Msg.TokenBag.SharedCode)
}

func TestOpenTokenBag_ReopenAfterCloseKeepsPlayersAndCodes(t *testing.T) {
	h := testHandler(t)
	bag := createBagGame(t, h)
	joinBag(t, h, bag.joinCode, "Alice")
	closeBag(t, h, bag)

	resp, err := h.OpenTokenBagRegistration(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.OpenTokenBagRegistrationRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)
	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_OPEN, resp.Msg.TokenBag.Phase)
	assert.Equal(t, bag.joinCode, resp.Msg.TokenBag.JoinCode)
	assert.Equal(t, bag.sharedCode, resp.Msg.TokenBag.SharedCode)
	require.Len(t, resp.Msg.TokenBag.Players, 1)
	assert.Equal(t, "Alice", resp.Msg.TokenBag.Players[0].Name)
}

func TestOpenTokenBag_RejectedAfterReveal(t *testing.T) {
	h := testHandler(t)
	bag := createBagGame(t, h)
	joinBag(t, h, bag.joinCode, "Alice")
	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{"chef": "Alice"})
	closeBag(t, h, bag)
	revealBag(t, h, bag)

	_, err := h.OpenTokenBagRegistration(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.OpenTokenBagRegistrationRequest{
		GameId: bag.gameID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Contains(t, err.Error(), "reset the token bag")
}

func TestOpenTokenBag_RejectedOnCompletedGame(t *testing.T) {
	h := testHandler(t)
	ownerID, g := startedGame(t, h)

	_, err := h.EndGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.EndGameRequest{GameId: g.Id}))
	require.NoError(t, err)

	_, err = h.OpenTokenBagRegistration(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.OpenTokenBagRegistrationRequest{
		GameId: g.Id,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestCloseTokenBag_OnlyFromOpen(t *testing.T) {
	h := testHandler(t)
	bag := createBagGame(t, h)

	closeBag(t, h, bag)
	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_CLOSED, getBag(t, h, bag).Phase)

	_, err := h.CloseTokenBagRegistration(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.CloseTokenBagRegistrationRequest{
		GameId: bag.gameID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestResetTokenBag_WipesPlayersAndCodes(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	joinBag(t, h, bag.joinCode, "Alice")
	joinBag(t, h, bag.joinCode, "Bob")

	resp, err := h.ResetTokenBag(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.ResetTokenBagRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)
	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_INACTIVE, resp.Msg.TokenBag.Phase)
	assert.Empty(t, resp.Msg.TokenBag.JoinCode)
	assert.Empty(t, resp.Msg.TokenBag.SharedCode)
	assert.Empty(t, resp.Msg.TokenBag.Players)

	count, err := h.db.Registration.Query().Where(registration.GameID(int(bag.gameID))).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count, "reset must delete every registration")

	// The old codes stop working.
	_, err = h.JoinTokenBag(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagRequest{
		JoinCode: bag.joinCode,
		Name:     "Carol",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestResetThenOpenTokenBag_RotatesCodes(t *testing.T) {
	h := testHandler(t)
	bag := createBagGame(t, h)

	_, err := h.ResetTokenBag(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.ResetTokenBagRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)

	resp, err := h.OpenTokenBagRegistration(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.OpenTokenBagRegistrationRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)
	assert.NotEqual(t, bag.joinCode, resp.Msg.TokenBag.JoinCode, "reset must rotate the join code")
	assert.NotEqual(t, bag.sharedCode, resp.Msg.TokenBag.SharedCode, "reset must rotate the shared code")
}

// --- Authorization: every storyteller RPC must be owner-only ---

func TestTokenBagStorytellerRPCs_BlockNonOwner(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	regID, _ := joinBag(t, h, bag.joinCode, "Alice")

	attacker, err := h.db.User.Create().Save(ctx)
	require.NoError(t, err)
	attackerCtx := authedCtx(attacker.ID)

	calls := map[string]func() error{
		"OpenTokenBagRegistration": func() error {
			_, err := h.OpenTokenBagRegistration(attackerCtx, connect.NewRequest(&clockkeeperv1.OpenTokenBagRegistrationRequest{GameId: bag.gameID}))
			return err
		},
		"CloseTokenBagRegistration": func() error {
			_, err := h.CloseTokenBagRegistration(attackerCtx, connect.NewRequest(&clockkeeperv1.CloseTokenBagRegistrationRequest{GameId: bag.gameID}))
			return err
		},
		"AddTokenBagRegistration": func() error {
			_, err := h.AddTokenBagRegistration(attackerCtx, connect.NewRequest(&clockkeeperv1.AddTokenBagRegistrationRequest{GameId: bag.gameID, Name: "Mallory"}))
			return err
		},
		"RemoveTokenBagRegistration": func() error {
			_, err := h.RemoveTokenBagRegistration(attackerCtx, connect.NewRequest(&clockkeeperv1.RemoveTokenBagRegistrationRequest{GameId: bag.gameID, RegistrationId: regID}))
			return err
		},
		"RevealTokenBag": func() error {
			_, err := h.RevealTokenBag(attackerCtx, connect.NewRequest(&clockkeeperv1.RevealTokenBagRequest{GameId: bag.gameID}))
			return err
		},
		"ResetTokenBag": func() error {
			_, err := h.ResetTokenBag(attackerCtx, connect.NewRequest(&clockkeeperv1.ResetTokenBagRequest{GameId: bag.gameID}))
			return err
		},
		"GetTokenBag": func() error {
			_, err := h.GetTokenBag(attackerCtx, connect.NewRequest(&clockkeeperv1.GetTokenBagRequest{GameId: bag.gameID}))
			return err
		},
		"GetTokenBagSeating": func() error {
			_, err := h.GetTokenBagSeating(attackerCtx, connect.NewRequest(&clockkeeperv1.GetTokenBagSeatingRequest{GameId: bag.gameID}))
			return err
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call()
			require.Error(t, err)
			assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
		})
	}

	// Nothing leaked or changed.
	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_OPEN, getBag(t, h, bag).Phase)
}

// --- Join ---

func TestJoinTokenBag_ReturnsSecretAndStoresOnlyItsHash(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)

	resp, err := h.JoinTokenBag(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagRequest{
		JoinCode: bag.joinCode,
		Name:     "  Alice   Smith ",
	}))
	require.NoError(t, err)
	require.NotZero(t, resp.Msg.RegistrationId)
	require.NotEmpty(t, resp.Msg.RegistrationSecret)

	require.NotNil(t, resp.Msg.Snapshot)
	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_OPEN, resp.Msg.Snapshot.Phase)
	assert.Equal(t, resp.Msg.RegistrationId, resp.Msg.Snapshot.SelfRegistrationId)
	assert.Nil(t, resp.Msg.Snapshot.SelfToken, "no token before the reveal")
	require.Len(t, resp.Msg.Snapshot.Players, 1)
	assert.Equal(t, "Alice Smith", resp.Msg.Snapshot.Players[0].Name, "whitespace must be collapsed and trimmed")

	r, err := h.db.Registration.Get(ctx, int(resp.Msg.RegistrationId))
	require.NoError(t, err)
	assert.NotEqual(t, resp.Msg.RegistrationSecret, r.SecretHash, "the raw secret must not be stored")
	assert.Equal(t, hashSecret(resp.Msg.RegistrationSecret), r.SecretHash)
	assert.Equal(t, "alice smith", r.NameNormalized)
	assert.False(t, r.ViaSharedDevice)
}

func TestJoinTokenBag_DuplicateNameIsCaseAndWhitespaceInsensitive(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	joinBag(t, h, bag.joinCode, "Alice Smith")

	for _, name := range []string{"alice smith", "  ALICE    SMITH  ", "Alice Smith"} {
		_, err := h.JoinTokenBag(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagRequest{
			JoinCode: bag.joinCode,
			Name:     name,
		}))
		require.Error(t, err, "name %q should collide", name)
		assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
	}
}

func TestJoinTokenBag_RejectedWhenNotOpen(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	closeBag(t, h, bag)

	_, err := h.JoinTokenBag(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagRequest{
		JoinCode: bag.joinCode,
		Name:     "Alice",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestJoinTokenBag_RejectsUnknownAndSharedCode(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)

	for _, code := range []string{"", "not-a-code", bag.sharedCode} {
		_, err := h.JoinTokenBag(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagRequest{
			JoinCode: code,
			Name:     "Alice",
		}))
		require.Error(t, err, "code %q must not be usable to join", code)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	}
}

func TestJoinTokenBag_RejectsInvalidNames(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)

	// strings.Repeat("界", 17) is 17 runes but 51 bytes: within the rune bound a
	// player perceives, over the schema's MaxLen(50), which counts bytes.
	wideName := strings.Repeat("界", 17)
	require.Len(t, wideName, 51)
	require.Equal(t, 17, utf8.RuneCountInString(wideName))

	names := map[string]string{
		"empty":                  "",
		"whitespace only":        "   \t  ",
		"control chars only":     "\x00\x01\x07",
		"too many characters":    strings.Repeat("a", 51),
		"too many bytes":         wideName,
		"too many bytes (emoji)": strings.Repeat("🎲", 13),
	}
	for label, name := range names {
		t.Run(label, func(t *testing.T) {
			// Both public join paths must reject it as a bad request, never a 500.
			_, err := h.JoinTokenBag(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagRequest{
				JoinCode: bag.joinCode,
				Name:     name,
			}))
			require.Error(t, err)
			assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

			_, err = h.JoinTokenBagShared(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagSharedRequest{
				SharedCode: bag.sharedCode,
				Name:       name,
			}))
			require.Error(t, err)
			assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
		})
	}
}

// The QR codes handed out at the start of the evening are still lying on the
// table when the game ends. None of them may keep working.
func TestTokenBagRPCs_RejectedOnCompletedGame(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	ownerID, g := startedGame(t, h)

	open, err := h.OpenTokenBagRegistration(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.OpenTokenBagRegistrationRequest{
		GameId: g.Id,
	}))
	require.NoError(t, err)
	joinCode := open.Msg.TokenBag.JoinCode
	sharedCode := open.Msg.TokenBag.SharedCode

	_, err = h.EndGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.EndGameRequest{GameId: g.Id}))
	require.NoError(t, err)

	t.Run("join", func(t *testing.T) {
		_, err := h.JoinTokenBag(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagRequest{
			JoinCode: joinCode,
			Name:     "Alice",
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
		assert.Contains(t, err.Error(), "completed")
	})

	t.Run("join via shared device", func(t *testing.T) {
		_, err := h.JoinTokenBagShared(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagSharedRequest{
			SharedCode: sharedCode,
			Name:       "Bob",
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
		assert.Contains(t, err.Error(), "completed")
	})

	t.Run("add player as the storyteller", func(t *testing.T) {
		_, err := h.AddTokenBagRegistration(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.AddTokenBagRegistrationRequest{
			GameId: g.Id,
			Name:   "Carol",
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
		assert.Contains(t, err.Error(), "completed")
	})

	t.Run("close registration", func(t *testing.T) {
		_, err := h.CloseTokenBagRegistration(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.CloseTokenBagRegistrationRequest{
			GameId: g.Id,
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
		assert.Contains(t, err.Error(), "completed")
	})

	t.Run("reveal", func(t *testing.T) {
		_, err := h.RevealTokenBag(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.RevealTokenBagRequest{
			GameId: g.Id,
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
		assert.Contains(t, err.Error(), "completed")
	})
}

// Format characters are invisible, so two names that differ only by one read as
// the same name at a physical table. Mirrored by web/src/lib/tokenbag.ts.
func TestNormalizeName_DropsFormatCharacters(t *testing.T) {
	plainDisplay, plain, err := normalizeName("Ana")
	require.NoError(t, err)

	cases := map[string]string{
		"zero width space":       "An\u200ba",
		"zero width joiner":      "An\u200da",
		"zero width non-joiner":  "An\u200ca",
		"soft hyphen":            "An\u00ada",
		"right-to-left override": "\u202eAna",
		"byte order mark":        "\ufeffAna",
	}
	for label, raw := range cases {
		t.Run(label, func(t *testing.T) {
			require.NotEqual(t, "Ana", raw, "the raw name must actually differ")
			display, normalized, err := normalizeName(raw)
			require.NoError(t, err)
			assert.Equal(t, plainDisplay, display)
			assert.Equal(t, plain, normalized, "must collide with a plain %q", plainDisplay)
		})
	}
}

// The collision above is what makes the uniqueness constraint hold: an invisible
// character must not buy a second registration under someone else's name.
func TestJoinTokenBag_RejectsAZeroWidthDuplicate(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	joinBag(t, h, bag.joinCode, "Ana")

	_, err := h.JoinTokenBag(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagRequest{
		JoinCode: bag.joinCode,
		Name:     "An\u200ba",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
}

func TestNormalizeName_BoundsMatchTheSchema(t *testing.T) {
	// The longest accepted name is bounded by whichever limit bites first.
	display, normalized, err := normalizeName(strings.Repeat("a", maxTokenBagNameRunes))
	require.NoError(t, err)
	assert.Len(t, display, maxTokenBagNameBytes)
	assert.Equal(t, display, normalized)

	// 16 CJK runes = 48 bytes: comfortably inside both bounds.
	display, _, err = normalizeName(strings.Repeat("界", 16))
	require.NoError(t, err)
	assert.LessOrEqual(t, len(display), maxTokenBagNameBytes)

	_, _, err = normalizeName(strings.Repeat("界", 17))
	require.Error(t, err, "17 CJK runes exceed the schema's 50-byte limit")
}

func TestJoinTokenBag_EnforcesCap(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)

	for i := range maxTokenBagRegistrations {
		joinBag(t, h, bag.joinCode, fmt.Sprintf("Player %d", i))
	}

	_, err := h.JoinTokenBag(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagRequest{
		JoinCode: bag.joinCode,
		Name:     "One Too Many",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err))
}

// --- Add (storyteller) ---

func TestAddTokenBagRegistration_RegistersLikeASharedDevice(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)

	resp, err := h.AddTokenBagRegistration(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.AddTokenBagRegistrationRequest{
		GameId: bag.gameID,
		Name:   "  Alice   Smith ",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.TokenBag.Players, 1)

	added := resp.Msg.TokenBag.Players[0]
	assert.Equal(t, "Alice Smith", added.Name, "whitespace must be collapsed and trimmed")
	assert.True(t, added.ViaSharedDevice, "a storyteller-added player holds no device of their own")

	r, err := h.db.Registration.Get(ctx, int(added.RegistrationId))
	require.NoError(t, err)
	assert.Equal(t, "alice smith", r.NameNormalized)
	assert.True(t, r.ViaSharedDevice)
}

// Adding a player after registration closed is the whole point: the storyteller
// types the seats nobody registered for right before revealing.
func TestAddTokenBagRegistration_WorksWhileClosed(t *testing.T) {
	h := testHandler(t)
	bag := createBagGame(t, h)
	joinBag(t, h, bag.joinCode, "Alice")
	closeBag(t, h, bag)

	resp, err := h.AddTokenBagRegistration(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.AddTokenBagRegistrationRequest{
		GameId: bag.gameID,
		Name:   "Bob",
	}))
	require.NoError(t, err)
	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_CLOSED, resp.Msg.TokenBag.Phase, "adding a player must not re-open registration")

	var names []string
	for _, p := range resp.Msg.TokenBag.Players {
		names = append(names, p.Name)
	}
	assert.ElementsMatch(t, []string{"Alice", "Bob"}, names)
}

func TestAddTokenBagRegistration_RejectedOutsideOpenAndClosed(t *testing.T) {
	h := testHandler(t)

	t.Run("inactive", func(t *testing.T) {
		ownerID, gameID := createTestGame(t, h)

		_, err := h.AddTokenBagRegistration(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.AddTokenBagRegistrationRequest{
			GameId: gameID,
			Name:   "Alice",
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	})

	t.Run("revealed", func(t *testing.T) {
		bag := createBagGame(t, h)
		joinBag(t, h, bag.joinCode, "Alice")
		setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{"chef": "Alice"})
		closeBag(t, h, bag)
		revealBag(t, h, bag)

		_, err := h.AddTokenBagRegistration(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.AddTokenBagRegistrationRequest{
			GameId: bag.gameID,
			Name:   "Bob",
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
		assert.Len(t, getBag(t, h, bag).Players, 1, "a player added after the reveal would never get a token")
	})
}

// The storyteller typing a name someone already registered from their phone must
// not create a second seat for the same person.
func TestAddTokenBagRegistration_RejectsDuplicateName(t *testing.T) {
	h := testHandler(t)
	bag := createBagGame(t, h)
	joinBag(t, h, bag.joinCode, "Alice Smith")

	for _, name := range []string{"alice smith", "  ALICE    SMITH  ", "Alice Smith"} {
		_, err := h.AddTokenBagRegistration(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.AddTokenBagRegistrationRequest{
			GameId: bag.gameID,
			Name:   name,
		}))
		require.Error(t, err, "name %q should collide", name)
		assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
	}
}

func TestAddTokenBagRegistration_RejectsInvalidName(t *testing.T) {
	h := testHandler(t)
	bag := createBagGame(t, h)

	for _, name := range []string{"", "   \t  ", strings.Repeat("a", 51)} {
		_, err := h.AddTokenBagRegistration(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.AddTokenBagRegistrationRequest{
			GameId: bag.gameID,
			Name:   name,
		}))
		require.Error(t, err, "name %q must be rejected", name)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	}
}

func TestAddTokenBagRegistration_EnforcesCap(t *testing.T) {
	h := testHandler(t)
	bag := createBagGame(t, h)

	for i := range maxTokenBagRegistrations {
		joinBag(t, h, bag.joinCode, fmt.Sprintf("Player %d", i))
	}

	_, err := h.AddTokenBagRegistration(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.AddTokenBagRegistrationRequest{
		GameId: bag.gameID,
		Name:   "One Too Many",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err))
}

// --- Remove ---

func TestRemoveTokenBagRegistration_ClearsNeighborReferences(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	aliceID, _ := joinBag(t, h, bag.joinCode, "Alice")
	bobID, bobSecret := joinBag(t, h, bag.joinCode, "Bob")
	carolID, carolSecret := joinBag(t, h, bag.joinCode, "Carol")
	closeBag(t, h, bag)

	// Bob and Carol both point at Alice.
	_, err := h.SetTokenBagNeighbors(ctx, connect.NewRequest(&clockkeeperv1.SetTokenBagNeighborsRequest{
		RegistrationSecret:  bobSecret,
		LeftRegistrationId:  aliceID,
		RightRegistrationId: carolID,
	}))
	require.NoError(t, err)
	_, err = h.SetTokenBagNeighbors(ctx, connect.NewRequest(&clockkeeperv1.SetTokenBagNeighborsRequest{
		RegistrationSecret:  carolSecret,
		LeftRegistrationId:  bobID,
		RightRegistrationId: aliceID,
	}))
	require.NoError(t, err)

	resp, err := h.RemoveTokenBagRegistration(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.RemoveTokenBagRegistrationRequest{
		GameId:         bag.gameID,
		RegistrationId: aliceID,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.TokenBag.Players, 2)

	for _, p := range resp.Msg.TokenBag.Players {
		assert.NotEqual(t, aliceID, p.LeftNeighborId, "%s still points at the removed player", p.Name)
		assert.NotEqual(t, aliceID, p.RightNeighborId, "%s still points at the removed player", p.Name)
	}
	// Bob's right pick (Carol) is untouched.
	bob, err := h.db.Registration.Get(ctx, int(bobID))
	require.NoError(t, err)
	assert.Zero(t, bob.LeftNeighborID)
	assert.Equal(t, int(carolID), bob.RightNeighborID)

	exists, err := h.db.Registration.Query().Where(registration.ID(int(aliceID))).Exist(ctx)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestRemoveTokenBagRegistration_RejectsForeignRegistration(t *testing.T) {
	h := testHandler(t)
	bagA := createBagGame(t, h)
	bagB := createBagGame(t, h)
	foreignID, _ := joinBag(t, h, bagB.joinCode, "Alice")

	_, err := h.RemoveTokenBagRegistration(authedCtx(bagA.ownerID), connect.NewRequest(&clockkeeperv1.RemoveTokenBagRegistrationRequest{
		GameId:         bagA.gameID,
		RegistrationId: foreignID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	assert.Len(t, getBag(t, h, bagB).Players, 1, "the other game's registration must survive")
}

// --- Neighbors ---

func TestSetTokenBagNeighbors_OnlyWhileClosed(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	aliceID, aliceSecret := joinBag(t, h, bag.joinCode, "Alice")
	_, bobSecret := joinBag(t, h, bag.joinCode, "Bob")

	// While open: too early.
	_, err := h.SetTokenBagNeighbors(ctx, connect.NewRequest(&clockkeeperv1.SetTokenBagNeighborsRequest{
		RegistrationSecret: bobSecret,
		LeftRegistrationId: aliceID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{"chef": "Alice", "imp": "Bob"})
	closeBag(t, h, bag)

	_, err = h.SetTokenBagNeighbors(ctx, connect.NewRequest(&clockkeeperv1.SetTokenBagNeighborsRequest{
		RegistrationSecret: bobSecret,
		LeftRegistrationId: aliceID,
	}))
	require.NoError(t, err)

	revealBag(t, h, bag)

	// After the reveal: too late.
	_, err = h.SetTokenBagNeighbors(ctx, connect.NewRequest(&clockkeeperv1.SetTokenBagNeighborsRequest{
		RegistrationSecret: aliceSecret,
		LeftRegistrationId: 0,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestSetTokenBagNeighbors_RejectsSelfAndForeignPlayers(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	aliceID, aliceSecret := joinBag(t, h, bag.joinCode, "Alice")
	closeBag(t, h, bag)

	other := createBagGame(t, h)
	foreignID, _ := joinBag(t, h, other.joinCode, "Stranger")

	cases := map[string]*clockkeeperv1.SetTokenBagNeighborsRequest{
		"self on the left":     {RegistrationSecret: aliceSecret, LeftRegistrationId: aliceID},
		"self on the right":    {RegistrationSecret: aliceSecret, RightRegistrationId: aliceID},
		"other game's player":  {RegistrationSecret: aliceSecret, LeftRegistrationId: foreignID},
		"unknown registration": {RegistrationSecret: aliceSecret, RightRegistrationId: 999999},
	}
	for label, req := range cases {
		t.Run(label, func(t *testing.T) {
			_, err := h.SetTokenBagNeighbors(ctx, connect.NewRequest(req))
			require.Error(t, err)
			assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
		})
	}
}

func TestSetTokenBagNeighbors_SameNeighborBothSidesAndClearing(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	aliceID, aliceSecret := joinBag(t, h, bag.joinCode, "Alice")
	bobID, _ := joinBag(t, h, bag.joinCode, "Bob")
	closeBag(t, h, bag)

	// A circle of two: the same player sits on both sides.
	resp, err := h.SetTokenBagNeighbors(ctx, connect.NewRequest(&clockkeeperv1.SetTokenBagNeighborsRequest{
		RegistrationSecret:  aliceSecret,
		LeftRegistrationId:  bobID,
		RightRegistrationId: bobID,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Snapshot)
	assert.Equal(t, aliceID, resp.Msg.Snapshot.SelfRegistrationId)

	alice, err := h.db.Registration.Get(ctx, int(aliceID))
	require.NoError(t, err)
	assert.Equal(t, int(bobID), alice.LeftNeighborID)
	assert.Equal(t, int(bobID), alice.RightNeighborID)

	// Zero clears both picks.
	_, err = h.SetTokenBagNeighbors(ctx, connect.NewRequest(&clockkeeperv1.SetTokenBagNeighborsRequest{
		RegistrationSecret: aliceSecret,
	}))
	require.NoError(t, err)

	alice, err = h.db.Registration.Get(ctx, int(aliceID))
	require.NoError(t, err)
	assert.Zero(t, alice.LeftNeighborID)
	assert.Zero(t, alice.RightNeighborID)
}

// --- Seating ---

func TestGetTokenBagSeating_CompleteCircleAndNamedConflicts(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	aliceID, aliceSecret := joinBag(t, h, bag.joinCode, "Alice")
	bobID, bobSecret := joinBag(t, h, bag.joinCode, "Bob")
	carolID, carolSecret := joinBag(t, h, bag.joinCode, "Carol")
	closeBag(t, h, bag)

	// Before any picks: incomplete, everyone listed as a gap by name.
	resp, err := h.GetTokenBagSeating(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.GetTokenBagSeatingRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)
	assert.False(t, resp.Msg.Complete)
	assert.Empty(t, resp.Msg.OrderedRegistrationIds)
	require.Len(t, resp.Msg.Conflicts, 1)
	assert.Equal(t, "no neighbor picks yet: Alice, Bob, Carol", resp.Msg.Conflicts[0])

	// Every player names the next one as sitting on their RIGHT: Bob is on
	// Alice's right, Carol on Bob's, Alice on Carol's. A right-hand neighbor sits
	// counter-clockwise seen from above, so the clockwise ring is the other way
	// round: Alice, Carol, Bob.
	for secret, right := range map[string]int64{aliceSecret: bobID, bobSecret: carolID, carolSecret: aliceID} {
		_, err := h.SetTokenBagNeighbors(ctx, connect.NewRequest(&clockkeeperv1.SetTokenBagNeighborsRequest{
			RegistrationSecret:  secret,
			RightRegistrationId: right,
		}))
		require.NoError(t, err)
	}

	resp, err = h.GetTokenBagSeating(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.GetTokenBagSeatingRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)
	assert.True(t, resp.Msg.Complete)
	assert.Empty(t, resp.Msg.Conflicts)
	assert.Equal(t, []int64{aliceID, carolID, bobID}, resp.Msg.OrderedRegistrationIds)

	// Carol contradicts Alice about who sits clockwise of Alice.
	_, err = h.SetTokenBagNeighbors(ctx, connect.NewRequest(&clockkeeperv1.SetTokenBagNeighborsRequest{
		RegistrationSecret: carolSecret,
		LeftRegistrationId: aliceID,
	}))
	require.NoError(t, err)

	resp, err = h.GetTokenBagSeating(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.GetTokenBagSeatingRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)
	assert.False(t, resp.Msg.Complete)
	require.NotEmpty(t, resp.Msg.Conflicts)
	assert.Contains(t, resp.Msg.Conflicts[0], "Alice")
	assert.NotContains(t, strings.Join(resp.Msg.Conflicts, " "), "#", "conflicts must name players, not ids")
}

// --- Reveal ---

func TestRevealTokenBag_RequiresClosedRegistration(t *testing.T) {
	h := testHandler(t)
	bag := createBagGame(t, h)
	joinBag(t, h, bag.joinCode, "Alice")
	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{"chef": "Alice"})

	_, err := h.RevealTokenBag(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.RevealTokenBagRequest{
		GameId: bag.gameID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestRevealTokenBag_RejectsEmptyBag(t *testing.T) {
	h := testHandler(t)
	bag := createBagGame(t, h)
	closeBag(t, h, bag)

	_, err := h.RevealTokenBag(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.RevealTokenBagRequest{
		GameId: bag.gameID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Contains(t, err.Error(), "no registered players")
}

func TestRevealTokenBag_ListsEveryUnassignedName(t *testing.T) {
	h := testHandler(t)
	bag := createBagGame(t, h)
	joinBag(t, h, bag.joinCode, "Alice")
	joinBag(t, h, bag.joinCode, "Bob")
	joinBag(t, h, bag.joinCode, "Carol")
	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{"chef": "Alice"})
	closeBag(t, h, bag)

	_, err := h.RevealTokenBag(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.RevealTokenBagRequest{
		GameId: bag.gameID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Contains(t, err.Error(), "Bob")
	assert.Contains(t, err.Error(), "Carol")
	assert.NotContains(t, err.Error(), "Alice")

	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_CLOSED, getBag(t, h, bag).Phase, "a failed reveal must not change the phase")
}

func TestRevealTokenBag_RejectsAmbiguousName(t *testing.T) {
	h := testHandler(t)
	bag := createBagGame(t, h)
	joinBag(t, h, bag.joinCode, "Alice")
	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{"chef": "Alice", "imp": "alice"})
	closeBag(t, h, bag)

	_, err := h.RevealTokenBag(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.RevealTokenBagRequest{
		GameId: bag.gameID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Contains(t, err.Error(), "Alice")
}

func TestRevealTokenBag_AssignsRolesIgnoringExtraGrimoireNames(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	aliceID, aliceSecret := joinBag(t, h, bag.joinCode, "Alice Smith")
	bobID, bobSecret := joinBag(t, h, bag.joinCode, "Bob")

	// "Dana" plays without a device, so her grimoire name matches no registration.
	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{
		"chef":  "  ALICE   smith ",
		"imp":   "Bob",
		"mayor": "Dana",
	})
	closeBag(t, h, bag)

	resp, err := h.RevealTokenBag(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.RevealTokenBagRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)
	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_REVEALED, resp.Msg.TokenBag.Phase)

	alice, err := h.db.Registration.Get(ctx, int(aliceID))
	require.NoError(t, err)
	assert.Equal(t, "chef", alice.AssignedRoleID, "names must match case- and whitespace-insensitively")
	bob, err := h.db.Registration.Get(ctx, int(bobID))
	require.NoError(t, err)
	assert.Equal(t, "imp", bob.AssignedRoleID)

	aliceToken, err := h.GetMyToken(ctx, connect.NewRequest(&clockkeeperv1.GetMyTokenRequest{RegistrationSecret: aliceSecret}))
	require.NoError(t, err)
	assert.Equal(t, "chef", aliceToken.Msg.Character.Id)

	bobToken, err := h.GetMyToken(ctx, connect.NewRequest(&clockkeeperv1.GetMyTokenRequest{RegistrationSecret: bobSecret}))
	require.NoError(t, err)
	assert.Equal(t, "imp", bobToken.Msg.Character.Id, "one player's secret must never reveal another's token")
}

// grimoire_player_names keeps the seats of characters the storyteller swapped
// out of the script. Dealing a token off one of those would hand a player a
// character nobody is holding, so a stale key counts for nothing.
func TestRevealTokenBag_IgnoresNamesOnRolesNoLongerInPlay(t *testing.T) {
	h := testHandler(t)
	bag := createBagGame(t, h)
	joinBag(t, h, bag.joinCode, "Alice")

	// "washerwoman" was in the script when Alice's name was typed, and has since
	// been swapped out — its grimoire entry is left behind.
	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{"washerwoman": "Alice"})
	closeBag(t, h, bag)

	_, err := h.RevealTokenBag(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.RevealTokenBagRequest{
		GameId: bag.gameID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Contains(t, err.Error(), "no role assigned to")
	assert.Contains(t, err.Error(), "Alice")

	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_CLOSED, getBag(t, h, bag).Phase)
}

// The same stale entry used to make an honest assignment look ambiguous, which
// blocked the reveal with a conflict the storyteller could not see or fix.
func TestRevealTokenBag_StaleSeatDoesNotCreateAmbiguity(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	aliceID, _ := joinBag(t, h, bag.joinCode, "Alice")

	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{
		"chef":        "Alice", // in play
		"washerwoman": "Alice", // left over from a character that is not
	})
	closeBag(t, h, bag)

	resp, err := h.RevealTokenBag(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.RevealTokenBagRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)
	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_REVEALED, resp.Msg.TokenBag.Phase)

	alice, err := h.db.Registration.Get(ctx, int(aliceID))
	require.NoError(t, err)
	assert.Equal(t, "chef", alice.AssignedRoleID, "only the seat in play can be dealt")
}

// An in-play role id the registry cannot resolve would surface as a CodeInternal
// the moment its player asked for their token. Refuse the reveal and name it.
func TestRevealTokenBag_RejectsInPlayRoleUnknownToTheRegistry(t *testing.T) {
	h := testHandler(t)
	bag := createBagGame(t, h)
	joinBag(t, h, bag.joinCode, "Alice")
	setSelectedRoles(t, h, bag.gameID, "chef", "no-such-character")
	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{"chef": "Alice"})
	closeBag(t, h, bag)

	_, err := h.RevealTokenBag(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.RevealTokenBagRequest{
		GameId: bag.gameID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err), "a broken script selection is the storyteller's to fix, not a 500")
	assert.Contains(t, err.Error(), "no-such-character")

	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_CLOSED, getBag(t, h, bag).Phase)
}

// Travellers hold seats too, so a traveller's grimoire name must be dealable.
func TestRevealTokenBag_DealsSelectedTravellers(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	scapegoatID, _ := joinBag(t, h, bag.joinCode, "Trav")

	_, err := h.db.Game.UpdateOneID(int(bag.gameID)).SetSelectedTravellers([]string{"scapegoat"}).Save(ctx)
	require.NoError(t, err)
	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{"scapegoat": "Trav"})
	closeBag(t, h, bag)
	revealBag(t, h, bag)

	trav, err := h.db.Registration.Get(ctx, int(scapegoatID))
	require.NoError(t, err)
	assert.Equal(t, "scapegoat", trav.AssignedRoleID)
}

func TestRevealTokenBag_BagSubstitutionHidesTheDrunk(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	_, aliceSecret := joinBag(t, h, bag.joinCode, "Alice")

	// Alice drew the Drunk, so she believes she is the Chef.
	_, err := h.UpdateBagSubstitutions(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.UpdateBagSubstitutionsRequest{
		GameId: bag.gameID,
		BagSubstitutions: []*clockkeeperv1.BagSubstitution{
			{CausedById: "drunk", CharacterId: "chef"},
		},
	}))
	require.NoError(t, err)
	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{"drunk": "Alice"})
	closeBag(t, h, bag)
	revealBag(t, h, bag)

	resp, err := h.GetMyToken(ctx, connect.NewRequest(&clockkeeperv1.GetMyTokenRequest{RegistrationSecret: aliceSecret}))
	require.NoError(t, err)
	assert.Equal(t, "chef", resp.Msg.Character.Id, "the substituted token must be shown")
	assert.NotEqual(t, "drunk", resp.Msg.Character.Id)
	assert.NotContains(t, strings.ToLower(resp.Msg.Character.Name), "drunk")
}

func TestGetMyToken_UnaffectedByGrimoireEditsAfterReveal(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	_, aliceSecret := joinBag(t, h, bag.joinCode, "Alice")
	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{"chef": "Alice"})
	closeBag(t, h, bag)
	revealBag(t, h, bag)

	// The storyteller reshuffles names in the grimoire after the reveal.
	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{"imp": "Alice", "chef": "Dana"})

	resp, err := h.GetMyToken(ctx, connect.NewRequest(&clockkeeperv1.GetMyTokenRequest{RegistrationSecret: aliceSecret}))
	require.NoError(t, err)
	assert.Equal(t, "chef", resp.Msg.Character.Id, "the revealed token is a snapshot and must not follow grimoire edits")
}

// --- GetMyToken ---

func TestGetMyToken_BeforeRevealAndWithBadSecret(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	_, aliceSecret := joinBag(t, h, bag.joinCode, "Alice")

	_, err := h.GetMyToken(ctx, connect.NewRequest(&clockkeeperv1.GetMyTokenRequest{RegistrationSecret: aliceSecret}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	for _, secret := range []string{"", "not-a-secret"} {
		_, err := h.GetMyToken(ctx, connect.NewRequest(&clockkeeperv1.GetMyTokenRequest{RegistrationSecret: secret}))
		require.Error(t, err, "secret %q must not resolve", secret)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	}
}

// --- Shared device ---

func TestJoinTokenBagShared_ReturnsOnlyTheRegistrationId(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)

	resp, err := h.JoinTokenBagShared(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagSharedRequest{
		SharedCode: bag.sharedCode,
		Name:       "Phoneless Pat",
	}))
	require.NoError(t, err)
	require.NotZero(t, resp.Msg.RegistrationId)

	r, err := h.db.Registration.Get(ctx, int(resp.Msg.RegistrationId))
	require.NoError(t, err)
	assert.True(t, r.ViaSharedDevice)
	assert.NotEmpty(t, r.SecretHash, "the schema requires a secret hash even for shared-device players")

	// Shared joins share the name space with device joins.
	_, err = h.JoinTokenBag(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagRequest{
		JoinCode: bag.joinCode,
		Name:     "phoneless pat",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
}

func TestSharedTokenBagRPCs_RejectTheJoinCode(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	regID, _ := joinBag(t, h, bag.joinCode, "Alice")

	_, err := h.JoinTokenBagShared(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagSharedRequest{
		SharedCode: bag.joinCode,
		Name:       "Bob",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	_, err = h.RevealTokenShared(ctx, connect.NewRequest(&clockkeeperv1.RevealTokenSharedRequest{
		SharedCode:     bag.joinCode,
		RegistrationId: regID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestRevealTokenShared_PhaseAndOwnershipChecks(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	aliceID, _ := joinBag(t, h, bag.joinCode, "Alice")

	// Before the reveal.
	_, err := h.RevealTokenShared(ctx, connect.NewRequest(&clockkeeperv1.RevealTokenSharedRequest{
		SharedCode:     bag.sharedCode,
		RegistrationId: aliceID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	other := createBagGame(t, h)
	foreignID, _ := joinBag(t, h, other.joinCode, "Stranger")

	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{"chef": "Alice"})
	closeBag(t, h, bag)
	revealBag(t, h, bag)

	resp, err := h.RevealTokenShared(ctx, connect.NewRequest(&clockkeeperv1.RevealTokenSharedRequest{
		SharedCode:     bag.sharedCode,
		RegistrationId: aliceID,
	}))
	require.NoError(t, err)
	assert.Equal(t, "Alice", resp.Msg.Name)
	assert.Equal(t, "chef", resp.Msg.Character.Id)

	// A registration from another game is not revealable here.
	_, err = h.RevealTokenShared(ctx, connect.NewRequest(&clockkeeperv1.RevealTokenSharedRequest{
		SharedCode:     bag.sharedCode,
		RegistrationId: foreignID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- Cascades ---

func TestDeleteGame_RemovesRegistrations(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	joinBag(t, h, bag.joinCode, "Alice")
	joinBag(t, h, bag.joinCode, "Bob")

	_, err := h.DeleteGame(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.DeleteGameRequest{
		Id: bag.gameID,
	}))
	require.NoError(t, err)

	count, err := h.db.Registration.Query().Where(registration.GameID(int(bag.gameID))).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestDuplicateGame_DoesNotCopyTokenBag(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	joinBag(t, h, bag.joinCode, "Alice")

	resp, err := h.DuplicateGame(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.DuplicateGameRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)

	copyGame := resp.Msg.Game
	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_INACTIVE, copyGame.TokenBagPhase)
	assert.Empty(t, copyGame.TokenBagJoinCode)
	assert.Empty(t, copyGame.TokenBagSharedCode)

	count, err := h.db.Registration.Query().Where(registration.GameID(int(copyGame.Id))).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count, "a duplicated game starts with an empty token bag")
}

func TestCleanupAnonymousUsers_RemovesRegistrations(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	joinBag(t, h, bag.joinCode, "Alice")

	require.NoError(t, h.db.User.UpdateOneID(bag.ownerID).
		SetIsAnonymous(true).
		SetLastActiveAt(time.Now().Add(-100*24*time.Hour)).
		Exec(ctx))

	cleanupAnonymousUsers(ctx, h.db, 24*time.Hour)

	exists, err := h.db.User.Query().Where(user.ID(bag.ownerID)).Exist(ctx)
	require.NoError(t, err)
	assert.False(t, exists, "the stale anonymous user should have been deleted")

	count, err := h.db.Registration.Query().Where(registration.GameID(int(bag.gameID))).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count, "the user's registrations must be cascaded away")
}
