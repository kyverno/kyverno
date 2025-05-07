package profiling

import (
	"net/http"
	_ "net/http/pprof" // #nosec
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/logging"
)

func Start(logger logr.Logger, address string) {
	logger.Info("Enable profiling, see details at https://github.com/kyverno/kyverno/wiki/Profiling-Kyverno-on-Kubernetes")
	go func() {
		s := http.Server{
			Addr:              address,
			ErrorLog:          logging.StdLogger(logger, ""),
			ReadHeaderTimeout: 30 * time.Second,
		}
		if err := s.ListenAndServe(); err != nil {
			logger.Error(err, "failed to enable profiling")
			os.Exit(1)
		}
	}()
}
