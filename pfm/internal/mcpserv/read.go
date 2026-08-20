package mcpserv

import (
	"context"
	"fmt"
	"path/filepath"
)

func (current *backend) read(
	ctx context.Context,
	input ReadInput,
) (ReadOutput, error) {
	if current.operations.Read == nil {
		return ReadOutput{}, fmt.Errorf("chat_read shared CLI operation is not configured")
	}
	return current.operations.Read(ctx, input)
}

func (current *backend) resolveReadSource(
	ctx context.Context,
	value string,
) (searchable, error) {
	rows, err := current.searchableRows(ctx)
	if err != nil {
		return searchable{}, err
	}
	clean := filepath.Clean(value)
	for _, row := range rows {
		if row.id == value || filepath.Clean(row.path) == clean {
			return row, nil
		}
	}
	rollouts, err := current.database.Rollouts(ctx)
	if err != nil {
		return searchable{}, err
	}
	for _, rollout := range rollouts {
		if rollout.SessionID == value {
			for _, row := range rows {
				if row.id == rollout.ID {
					return row, nil
				}
			}
		}
	}
	return searchable{}, fmt.Errorf("source %q is not an indexed transcript", value)
}
