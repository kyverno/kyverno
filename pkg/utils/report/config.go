package report

import "k8s.io/apimachinery/pkg/util/sets"

type reportingConfig struct {
	helper sets.Set[string]
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
