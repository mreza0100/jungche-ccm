package gate

import "hostops/pfm/internal/dream/artifact"

type VerdictResult struct {
	Normalized []artifact.NormalizedVerdict
}

func Verdicts(survivors []string, verdicts []artifact.Verdict) (VerdictResult, error) {
	normalized, err := artifact.NormalizeVerdicts(survivors, verdicts)
	if err != nil {
		return VerdictResult{}, err
	}
	return VerdictResult{Normalized: normalized}, nil
}
