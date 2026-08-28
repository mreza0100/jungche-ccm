//go:build darwin

package spawn

import (
	"context"
	"os/exec"
)

func serviceScopeCommand(
	ctx context.Context,
	binary string,
	arguments, environment []string,
) (*exec.Cmd, error) {
	command := exec.CommandContext(ctx, binary, arguments...)
	command.Env = environment
	return command, nil
}
