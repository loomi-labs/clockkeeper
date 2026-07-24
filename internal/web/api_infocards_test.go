package web

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateInfoCard_Success(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	u, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)

	resp, err := handler.CreateInfoCard(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.CreateInfoCardRequest{
		Title:        "These characters are not in play",
		Body:         "The following characters are bluffs.",
		CharacterIds: []string{"washerwoman", "imp"},
	}))
	require.NoError(t, err)

	card := resp.Msg.Card
	assert.NotZero(t, card.Id)
	assert.Equal(t, "These characters are not in play", card.Title)
	assert.Equal(t, "The following characters are bluffs.", card.Body)
	assert.Equal(t, []string{"washerwoman", "imp"}, card.CharacterIds)

	// Resolved characters should be non-empty and in the same order.
	require.Len(t, card.Characters, 2)
	assert.Equal(t, "washerwoman", card.Characters[0].Id)
	assert.Equal(t, "imp", card.Characters[1].Id)
}

func TestCreateInfoCard_RejectsEmptyTitle(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	u, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)

	_, err = handler.CreateInfoCard(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.CreateInfoCardRequest{
		Title: "   ",
		Body:  "body",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateInfoCard_RejectsUnknownCharacter(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	u, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)

	_, err = handler.CreateInfoCard(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.CreateInfoCardRequest{
		Title:        "Card",
		CharacterIds: []string{"washerwoman", "nonexistent_character"},
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateInfoCard_RejectsTooManyCharacters(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	u, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)

	_, err = handler.CreateInfoCard(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.CreateInfoCardRequest{
		Title: "Card",
		CharacterIds: []string{
			"washerwoman", "librarian", "investigator",
			"chef", "empath", "fortuneteller", "undertaker",
		},
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateInfoCard_RejectsTooLongTitle(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	u, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)

	_, err = handler.CreateInfoCard(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.CreateInfoCardRequest{
		Title: strings.Repeat("a", maxInfoCardTitleLen+1),
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateInfoCard_CapReached(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	u, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)

	// Create the maximum number of cards directly via Ent for speed.
	bulk := make([]*ent.InfoCardCreate, maxInfoCardsPerUser)
	for i := range bulk {
		bulk[i] = handler.db.InfoCard.Create().
			SetTitle("Card").
			SetUserID(u.ID).
			SetSortOrder(i)
	}
	_, err = handler.db.InfoCard.CreateBulk(bulk...).Save(ctx)
	require.NoError(t, err)

	_, err = handler.CreateInfoCard(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.CreateInfoCardRequest{
		Title: "One too many",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestListInfoCards_OnlyOwn(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	userA, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)
	userB, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)

	// User A creates two cards (created in order → sort_order 0, then 1).
	_, err = handler.CreateInfoCard(authedCtx(userA.ID), connect.NewRequest(&clockkeeperv1.CreateInfoCardRequest{Title: "A First"}))
	require.NoError(t, err)
	_, err = handler.CreateInfoCard(authedCtx(userA.ID), connect.NewRequest(&clockkeeperv1.CreateInfoCardRequest{Title: "A Second"}))
	require.NoError(t, err)

	// User B creates one card.
	_, err = handler.CreateInfoCard(authedCtx(userB.ID), connect.NewRequest(&clockkeeperv1.CreateInfoCardRequest{Title: "B Only"}))
	require.NoError(t, err)

	respA, err := handler.ListInfoCards(authedCtx(userA.ID), connect.NewRequest(&clockkeeperv1.ListInfoCardsRequest{}))
	require.NoError(t, err)
	require.Len(t, respA.Msg.Cards, 2)
	// Ordered by sort_order then id.
	assert.Equal(t, "A First", respA.Msg.Cards[0].Title)
	assert.Equal(t, "A Second", respA.Msg.Cards[1].Title)

	respB, err := handler.ListInfoCards(authedCtx(userB.ID), connect.NewRequest(&clockkeeperv1.ListInfoCardsRequest{}))
	require.NoError(t, err)
	require.Len(t, respB.Msg.Cards, 1)
	assert.Equal(t, "B Only", respB.Msg.Cards[0].Title)
}

func TestUpdateInfoCard_OwnerSucceeds(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	u, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)

	createResp, err := handler.CreateInfoCard(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.CreateInfoCardRequest{
		Title:        "Original",
		Body:         "original body",
		CharacterIds: []string{"washerwoman"},
	}))
	require.NoError(t, err)

	resp, err := handler.UpdateInfoCard(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.UpdateInfoCardRequest{
		Id:           createResp.Msg.Card.Id,
		Title:        "Updated",
		Body:         "updated body",
		CharacterIds: []string{"imp", "empath"},
	}))
	require.NoError(t, err)
	assert.Equal(t, "Updated", resp.Msg.Card.Title)
	assert.Equal(t, "updated body", resp.Msg.Card.Body)
	assert.Equal(t, []string{"imp", "empath"}, resp.Msg.Card.CharacterIds)
	require.Len(t, resp.Msg.Card.Characters, 2)
	assert.Equal(t, "imp", resp.Msg.Card.Characters[0].Id)
}

func TestUpdateInfoCard_BlocksOtherUser(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	userA, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)
	userB, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)

	createResp, err := handler.CreateInfoCard(authedCtx(userA.ID), connect.NewRequest(&clockkeeperv1.CreateInfoCardRequest{
		Title: "A's Card",
	}))
	require.NoError(t, err)

	_, err = handler.UpdateInfoCard(authedCtx(userB.ID), connect.NewRequest(&clockkeeperv1.UpdateInfoCardRequest{
		Id:    createResp.Msg.Card.Id,
		Title: "Hacked",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestDeleteInfoCard_OwnerSucceeds(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	u, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)

	createResp, err := handler.CreateInfoCard(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.CreateInfoCardRequest{
		Title: "To Delete",
	}))
	require.NoError(t, err)

	_, err = handler.DeleteInfoCard(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.DeleteInfoCardRequest{
		Id: createResp.Msg.Card.Id,
	}))
	require.NoError(t, err)

	listResp, err := handler.ListInfoCards(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.ListInfoCardsRequest{}))
	require.NoError(t, err)
	for _, c := range listResp.Msg.Cards {
		assert.NotEqual(t, createResp.Msg.Card.Id, c.Id, "deleted card should not be listed")
	}
	assert.Empty(t, listResp.Msg.Cards)
}

func TestDeleteInfoCard_BlocksOtherUser(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	userA, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)
	userB, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)

	createResp, err := handler.CreateInfoCard(authedCtx(userA.ID), connect.NewRequest(&clockkeeperv1.CreateInfoCardRequest{
		Title: "A's Card",
	}))
	require.NoError(t, err)

	_, err = handler.DeleteInfoCard(authedCtx(userB.ID), connect.NewRequest(&clockkeeperv1.DeleteInfoCardRequest{
		Id: createResp.Msg.Card.Id,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
