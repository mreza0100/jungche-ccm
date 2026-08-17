package dream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"hostops/pfm/internal/dream/apply"
	"hostops/pfm/internal/dream/artifact"
	"hostops/pfm/internal/dream/lane"
	"hostops/pfm/internal/dream/organ"
	"hostops/pfm/internal/dream/resources"
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
	if err := validateResourcesRoot(request.ResourcesRoot); err != nil {
		return apply.Result{}, err
	}
	// Alias-aware, same resolver Night staged with — Night writes lane.txt via
	// the lane profiles' Serves declarations, so an alias-blind Apply would
	// reject Night's own signed apply command.
	laneName, err := lane.FromAgentTypeIn(
		request.AgentType,
		resources.NewResources(request.ResourcesRoot, repository.Organ),
	)
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
