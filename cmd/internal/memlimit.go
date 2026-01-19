package internal

import (
	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/go-logr/logr"
)

func setupMemLimit(logger logr.Logger) {
	if !autoMemLimitEnabled {
		return
	}
	logger = logger.WithName("memlimit")
	logger.V(2).Info("setup memlimit...", "ratio", autoMemLimitRatio)
	if _, err := memlimit.SetGoMemLimitWithOpts(
		memlimit.WithRatio(autoMemLimitRatio),
		memlimit.WithProvider(
			memlimit.ApplyFallback(
				memlimit.FromCgroup,
				memlimit.FromSystem,
			),
		),
	); err != nil {
		logger.Error(err, "failed to set GOMEMLIMIT automatically")
	}
}
