package internal

import (
	"strings"

	"github.com/go-logr/logr"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"k8s.io/apimachinery/pkg/util/sets"
)

type reportingConfig struct {
	helper sets.Set[string]
}

func setupReporting(logger logr.Logger) reportutils.ReportingConfiguration {
	logger = logger.WithName("setup-reporting").WithValues("enableReporting", enableReporting)
	helper := sets.New(strings.Split(enableReporting, ",")...)
	cfg := &reportingConfig{
		helper: helper,
	}
	logger.Info("setting up reporting...", "validate", cfg.ValidateReportsEnabled(), "mutate", cfg.MutateReportsEnabled(), "mutateExisiting", cfg.MutateExistingReportsEnabled(), "imageVerify", cfg.ImageVerificationReportsEnabled(), "generate", cfg.GenerateReportsEnabled())
	return cfg
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
