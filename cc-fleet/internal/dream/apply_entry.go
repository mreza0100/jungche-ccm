package dream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"hostops/cc-fleet/internal/dream/apply"
	"hostops/cc-fleet/internal/dream/artifact"
	"hostops/cc-fleet/internal/dream/lane"
	"hostops/cc-fleet/internal/dream/organ"
)

type ApplyRequest struct {
	RepoRoot      string
	AgentType     string
	RegistryBase  string
	ResourcesRoot string
	Stage         string
}

// Apply is the production explicit-apply entry point. It deliberately has no
// autonomous call from Night: a signed operator action is the only bridge
// from HOLD to organ mutation.
func Apply(ctx context.Context, request ApplyRequest) (result apply.Result, returnErr error) {
	if ctx == nil {
		return apply.Result{}, errors.New("dream apply requires a context")
	}
	repository, err := organ.Resolve(request.RepoRoot, request.RegistryBase)
	if err != nil {
		return apply.Result{}, err
	}
	if _, err := organ.Validate(repository); err != nil {
		return apply.Result{}, err
	}
	// Alias-aware, same resolver Night staged with — Night writes lane.txt via
	// the lane profiles' Serves declarations, so an alias-blind Apply would
	// reject Night's own signed apply command.
	laneName, err := lane.FromAgentTypeIn(request.AgentType, repository.Organ, request.ResourcesRoot)
	if err != nil {
		return apply.Result{}, err
	}
	layout, err := organ.ValidateStage(repository, request.Stage)
	if err != nil {
		return apply.Result{}, err
	}
	release, err := acquireRunnerLock(repository.Organ)
	if err != nil {
		return apply.Result{}, err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("release Dreamer runner lock: %w", releaseErr))
		}
	}()
	return apply.Run(ctx, apply.Request{
		Repo:  repository,
		Lane:  artifact.LaneContext{AgentType: request.AgentType, Lane: laneName},
		Stage: layout,
	}, apply.Dependencies{
		Git:   apply.CommandGitReader{Repo: repository.RepoRoot},
		Clock: time.Now,
	})
}
