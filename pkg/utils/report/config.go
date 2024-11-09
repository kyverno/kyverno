package report

import (
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type reportingConfig struct {
	helper sets.Set[string]
	spec   v1.Spec
}

func (r *reportingConfig) ValidateReportsEnabled() bool {
	return r.helper.Has("validate")
}

func (r *reportingConfig) MutateReportsEnabled() bool {
	return r.helper.Has("mutate")
}

func (r *reportingConfig) MutateExistingReportsEnabled() bool {
	return r.helper.Has("mutateExisting")
}

func (r *reportingConfig) ImageVerificationReportsEnabled() bool {
	return r.helper.Has("imageVerify")
}

func (r *reportingConfig) GenerateReportsEnabled() bool {
	// If the DisableReportGeneration flag is set to true, report generation is disabled
	if r.spec.DisableReportGeneration != nil && *r.spec.DisableReportGeneration {
		return false
	}

	return r.helper.Has("generate")
}

func NewReportingConfig(items ...string) ReportingConfiguration {
	return &reportingConfig{
		helper: sets.New(items...),
	}
}

type ReportingConfiguration interface {
	ValidateReportsEnabled() bool
	MutateReportsEnabled() bool
	MutateExistingReportsEnabled() bool
	ImageVerificationReportsEnabled() bool
	GenerateReportsEnabled() bool
}
