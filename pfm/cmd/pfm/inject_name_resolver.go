package main

import (
	"context"
	"errors"
	"io"

	"hostops/pfm/internal/compose"
	"hostops/pfm/internal/inject"
	"hostops/pfm/internal/resolve"
)

// fleetNameResolver is the roster rung shared by preview and delivery. It
// returns unknown for a roster miss so inject.Engine can continue through the
// raw pane fallbacks needed before a fresh Codex seat writes its first rollout.
type fleetNameResolver struct {
	runtimes []commandRuntime
}

func (resolver fleetNameResolver) ResolveName(
	ctx context.Context,
	name, requiredEngine string,
) (inject.Target, int, string, error) {
	rows, err := composedChatRows(ctx, io.Discard, resolver.runtimes...)
	if err != nil {
		return inject.Target{}, inject.CodeUndelivered, "", err
	}
	liveRows := rows[:0]
	for _, row := range rows {
		if !isLiveKind(row.Kind) || row.Socket == "" ||
			(row.PaneID == "" && row.SessionName == "") ||
			(requiredEngine != "" && string(compose.EngineForKind(row.Kind)) != requiredEngine) {
			continue
		}
		liveRows = append(liveRows, row)
	}
	chat, found, err := matchChat(liveRows, name)
	if err != nil {
		var ambiguous *resolve.RosterAmbiguityError
		if errors.As(err, &ambiguous) {
			return inject.Target{}, inject.CodeAmbiguous, ambiguous.Error(), nil
		}
		return inject.Target{}, inject.CodeUndelivered, "", err
	}
	if !found || !chat.Live || chat.Socket == "" {
		return inject.Target{}, inject.CodeUnknown, "", nil
	}
	socketPath, err := chatSocketPath(chat.Socket)
	if err != nil {
		return inject.Target{}, inject.CodeUndelivered, "", err
	}
	pane := chat.Pane
	if pane == "" {
		pane = chat.Session
	}
	if pane == "" {
		return inject.Target{}, inject.CodeUnknown, "", nil
	}
	return inject.Target{
		SocketPath: socketPath,
		Pane:       pane,
		Engine:     string(chat.Engine),
		Name:       chat.Name,
		ID:         chat.ID,
		Session:    chat.Session,
	}, 0, "", nil
}
