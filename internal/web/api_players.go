package web

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
)

const (
	maxPlayerPresetNameLen = 100
	maxPlayerPresets       = 50
)

func sanitizePlayerPresets(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	result := make([]string, 0, len(raw))
	for _, name := range raw {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		runes := []rune(name)
		if len(runes) > maxPlayerPresetNameLen {
			name = string(runes[:maxPlayerPresetNameLen])
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
		if len(result) >= maxPlayerPresets {
			break
		}
	}
	return result
}

func (h *ClockKeeperServiceHandler) GetPlayerPresets(ctx context.Context, req *connect.Request[clockkeeperv1.GetPlayerPresetsRequest]) (*connect.Response[clockkeeperv1.GetPlayerPresetsResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.GetPlayerPresetsResponse{
		Names: u.PlayerPresets,
	}), nil
}

func (h *ClockKeeperServiceHandler) UpdatePlayerPresets(ctx context.Context, req *connect.Request[clockkeeperv1.UpdatePlayerPresetsRequest]) (*connect.Response[clockkeeperv1.UpdatePlayerPresetsResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	names := sanitizePlayerPresets(req.Msg.Names)
	u, err = u.Update().SetPlayerPresets(names).Save(ctx)
	if err != nil {
		slog.Error("update player presets failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.UpdatePlayerPresetsResponse{
		Names: u.PlayerPresets,
	}), nil
}
