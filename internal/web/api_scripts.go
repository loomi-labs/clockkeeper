package web

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	entscript "github.com/loomi-labs/clockkeeper/ent/script"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
)

func (h *ClockKeeperServiceHandler) ListScripts(ctx context.Context, req *connect.Request[clockkeeperv1.ListScriptsRequest]) (*connect.Response[clockkeeperv1.ListScriptsResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	scripts, err := h.db.Script.Query().
		Where(
			entscript.DeletedAtIsNil(),
			entscript.Or(
				entscript.IsSystem(true),
				entscript.UserIDEQ(u.ID),
			),
		).
		All(ctx)
	if err != nil {
		slog.Error("list scripts failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	result := make([]*clockkeeperv1.Script, len(scripts))
	for i, s := range scripts {
		result[i] = entScriptToProto(s, nil)
	}

	return connect.NewResponse(&clockkeeperv1.ListScriptsResponse{Scripts: result}), nil
}

func (h *ClockKeeperServiceHandler) GetScript(ctx context.Context, req *connect.Request[clockkeeperv1.GetScriptRequest]) (*connect.Response[clockkeeperv1.GetScriptResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	s, err := h.db.Script.Query().
		Where(
			entscript.ID(int(req.Msg.Id)),
			entscript.DeletedAtIsNil(),
			entscript.Or(
				entscript.IsSystem(true),
				entscript.UserIDEQ(u.ID),
			),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("script not found"))
		}
		slog.Error("get script failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.GetScriptResponse{
		Script: entScriptToProto(s, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) CreateScript(ctx context.Context, req *connect.Request[clockkeeperv1.CreateScriptRequest]) (*connect.Response[clockkeeperv1.CreateScriptResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	s, err := h.db.Script.Create().
		SetName(req.Msg.Name).
		SetCharacterIds(req.Msg.CharacterIds).
		SetUserID(u.ID).
		Save(ctx)
	if err != nil {
		slog.Error("create script failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.CreateScriptResponse{
		Script: entScriptToProto(s, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) CreateScriptFromEdition(ctx context.Context, req *connect.Request[clockkeeperv1.CreateScriptFromEditionRequest]) (*connect.Response[clockkeeperv1.CreateScriptFromEditionResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	// Find the edition.
	var editionName string
	var charIDs []string
	for _, e := range h.registry.Editions() {
		if e.ID == req.Msg.EditionId {
			editionName = e.Name
			charIDs = make([]string, len(e.Characters))
			for i, c := range e.Characters {
				charIDs[i] = c.ID
			}
			break
		}
	}
	if editionName == "" {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("edition not found"))
	}

	name := req.Msg.Name
	if name == "" {
		name = editionName
	}

	s, err := h.db.Script.Create().
		SetName(name).
		SetEdition(req.Msg.EditionId).
		SetCharacterIds(charIDs).
		SetUserID(u.ID).
		Save(ctx)
	if err != nil {
		slog.Error("create script from edition failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.CreateScriptFromEditionResponse{
		Script: entScriptToProto(s, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) UpdateScript(ctx context.Context, req *connect.Request[clockkeeperv1.UpdateScriptRequest]) (*connect.Response[clockkeeperv1.UpdateScriptResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	existing, err := h.db.Script.Get(ctx, int(req.Msg.Id))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("script not found"))
		}
		slog.Error("get script for update failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	if existing.IsSystem {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("system scripts cannot be modified"))
	}
	if existing.UserID == nil || *existing.UserID != u.ID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("script not found"))
	}

	update := h.db.Script.UpdateOneID(existing.ID)
	if req.Msg.Name != "" {
		update.SetName(req.Msg.Name)
	}
	if req.Msg.CharacterIds != nil {
		update.SetCharacterIds(req.Msg.CharacterIds)
	}

	s, err := update.Save(ctx)
	if err != nil {
		slog.Error("update script failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.UpdateScriptResponse{
		Script: entScriptToProto(s, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) DeleteScript(ctx context.Context, req *connect.Request[clockkeeperv1.DeleteScriptRequest]) (*connect.Response[clockkeeperv1.DeleteScriptResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	existing, err := h.db.Script.Get(ctx, int(req.Msg.Id))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("script not found"))
		}
		slog.Error("get script for delete failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	if existing.IsSystem {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("system scripts cannot be deleted"))
	}
	if existing.UserID == nil || *existing.UserID != u.ID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("script not found"))
	}

	now := time.Now()
	if _, err := h.db.Script.UpdateOneID(existing.ID).SetDeletedAt(now).Save(ctx); err != nil {
		slog.Error("soft delete script failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.DeleteScriptResponse{}), nil
}

func (h *ClockKeeperServiceHandler) ImportScript(ctx context.Context, req *connect.Request[clockkeeperv1.ImportScriptRequest]) (*connect.Response[clockkeeperv1.ImportScriptResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	// Parse official script JSON format: array of objects/strings.
	// First element may be a _meta object with name.
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(req.Msg.Json), &raw); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid script JSON"))
	}

	var name string
	var charIDs []string

	for _, item := range raw {
		// Try as string first (character ID).
		var id string
		if err := json.Unmarshal(item, &id); err == nil {
			if _, ok := h.registry.Character(id); ok {
				charIDs = append(charIDs, id)
			}
			continue
		}

		// Try as object.
		var obj map[string]any
		if err := json.Unmarshal(item, &obj); err != nil {
			continue
		}

		// Check for _meta object.
		if _, ok := obj["id"]; ok {
			if metaID, ok := obj["id"].(string); ok {
				if metaID == "_meta" {
					if n, ok := obj["name"].(string); ok {
						name = n
					}
					continue
				}
				if _, ok := h.registry.Character(metaID); ok {
					charIDs = append(charIDs, metaID)
				}
			}
		}
	}

	if name == "" {
		name = "Imported Script"
	}

	s, err := h.db.Script.Create().
		SetName(name).
		SetCharacterIds(charIDs).
		SetUserID(u.ID).
		Save(ctx)
	if err != nil {
		slog.Error("import script failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.ImportScriptResponse{
		Script: entScriptToProto(s, h.registry),
	}), nil
}

// currentUser looks up the authenticated user from the context.
func (h *ClockKeeperServiceHandler) currentUser(ctx context.Context) (*ent.User, error) {
	userID := UserIDFromContext(ctx)
	if userID == 0 {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	u, err := h.db.User.Get(ctx, userID)
	if err != nil {
		slog.Error("user lookup failed", "user_id", userID, "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Bump last_active_at for anonymous cleanup tracking.
	if err := h.db.User.UpdateOneID(u.ID).SetLastActiveAt(time.Now()).Exec(ctx); err != nil {
		slog.Warn("failed to update last_active_at", "user_id", u.ID, "err", err)
	}

	return u, nil
}
