package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/infocard"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
)

const (
	maxInfoCardTitleLen     = 100
	maxInfoCardBodyLen      = 2000
	maxInfoCardCharacterIDs = 6
	maxInfoCardsPerUser     = 50
)

// getOwnedInfoCard fetches an info card by ID and verifies the current user owns it.
// Returns CodeNotFound for a missing card or one owned by another user.
func (h *ClockKeeperServiceHandler) getOwnedInfoCard(ctx context.Context, id int) (*ent.InfoCard, error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	c, err := h.db.InfoCard.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("info card not found"))
		}
		slog.Error("get info card failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if c.UserID != u.ID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("info card not found"))
	}

	return c, nil
}

// validateInfoCardInput validates the fields of a create/update request.
func (h *ClockKeeperServiceHandler) validateInfoCardInput(title, body string, characterIDs []string) error {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("title is required"))
	}
	if len([]rune(trimmed)) > maxInfoCardTitleLen {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("title must be at most %d characters", maxInfoCardTitleLen))
	}
	if len([]rune(body)) > maxInfoCardBodyLen {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("body must be at most %d characters", maxInfoCardBodyLen))
	}
	if len(characterIDs) > maxInfoCardCharacterIDs {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("at most %d characters are allowed", maxInfoCardCharacterIDs))
	}
	for _, id := range characterIDs {
		if _, ok := h.registry.Character(id); !ok {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown character %q", id))
		}
	}
	return nil
}

func (h *ClockKeeperServiceHandler) ListInfoCards(ctx context.Context, req *connect.Request[clockkeeperv1.ListInfoCardsRequest]) (*connect.Response[clockkeeperv1.ListInfoCardsResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	cards, err := h.db.InfoCard.Query().
		Where(infocard.UserID(u.ID)).
		Order(ent.Asc(infocard.FieldSortOrder), ent.Asc(infocard.FieldID)).
		All(ctx)
	if err != nil {
		slog.Error("list info cards failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	result := make([]*clockkeeperv1.InfoCard, len(cards))
	for i, c := range cards {
		result[i] = entInfoCardToProto(c, h.registry)
	}

	return connect.NewResponse(&clockkeeperv1.ListInfoCardsResponse{Cards: result}), nil
}

func (h *ClockKeeperServiceHandler) CreateInfoCard(ctx context.Context, req *connect.Request[clockkeeperv1.CreateInfoCardRequest]) (*connect.Response[clockkeeperv1.CreateInfoCardResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.validateInfoCardInput(req.Msg.Title, req.Msg.Body, req.Msg.CharacterIds); err != nil {
		return nil, err
	}

	count, err := h.db.InfoCard.Query().Where(infocard.UserID(u.ID)).Count(ctx)
	if err != nil {
		slog.Error("count info cards failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	if count >= maxInfoCardsPerUser {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("maximum of %d info cards reached", maxInfoCardsPerUser))
	}

	characterIDs := req.Msg.CharacterIds
	if characterIDs == nil {
		characterIDs = []string{}
	}

	c, err := h.db.InfoCard.Create().
		SetTitle(strings.TrimSpace(req.Msg.Title)).
		SetBody(req.Msg.Body).
		SetCharacterIds(characterIDs).
		SetSortOrder(count).
		SetUserID(u.ID).
		Save(ctx)
	if err != nil {
		slog.Error("create info card failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.CreateInfoCardResponse{
		Card: entInfoCardToProto(c, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) UpdateInfoCard(ctx context.Context, req *connect.Request[clockkeeperv1.UpdateInfoCardRequest]) (*connect.Response[clockkeeperv1.UpdateInfoCardResponse], error) {
	existing, err := h.getOwnedInfoCard(ctx, int(req.Msg.Id))
	if err != nil {
		return nil, err
	}

	if err := h.validateInfoCardInput(req.Msg.Title, req.Msg.Body, req.Msg.CharacterIds); err != nil {
		return nil, err
	}

	characterIDs := req.Msg.CharacterIds
	if characterIDs == nil {
		characterIDs = []string{}
	}

	c, err := h.db.InfoCard.UpdateOneID(existing.ID).
		SetTitle(strings.TrimSpace(req.Msg.Title)).
		SetBody(req.Msg.Body).
		SetCharacterIds(characterIDs).
		Save(ctx)
	if err != nil {
		slog.Error("update info card failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.UpdateInfoCardResponse{
		Card: entInfoCardToProto(c, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) DeleteInfoCard(ctx context.Context, req *connect.Request[clockkeeperv1.DeleteInfoCardRequest]) (*connect.Response[clockkeeperv1.DeleteInfoCardResponse], error) {
	existing, err := h.getOwnedInfoCard(ctx, int(req.Msg.Id))
	if err != nil {
		return nil, err
	}

	if err := h.db.InfoCard.DeleteOneID(existing.ID).Exec(ctx); err != nil {
		slog.Error("delete info card failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.DeleteInfoCardResponse{}), nil
}
