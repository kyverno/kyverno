package internal

import (
	"strings"

	"github.com/go-logr/logr"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
)

func setupReporting(logger logr.Logger) reportutils.ReportingConfiguration {
	logger = logger.WithName("setup-reporting").WithValues("enableReporting", enableReporting)
	cfg := reportutils.NewReportingConfig(strings.Split(enableReporting, ",")...)
	logger.Info("setting up reporting...", "validate", cfg.ValidateReportsEnabled(), "mutate", cfg.MutateReportsEnabled(), "mutateExisiting", cfg.MutateExistingReportsEnabled(), "imageVerify", cfg.ImageVerificationReportsEnabled(), "generate", cfg.GenerateReportsEnabled())
	return cfg
}
