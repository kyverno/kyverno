package internal

import (
	"flag"
	"fmt"
	"os"

	"github.com/kyverno/kyverno/pkg/logging"
)

var (
	// logging
	loggingFormat string
	// profiling
	profilingEnabled bool
	profilingAddress string
	profilingPort    string
)

func InitFlags(profile bool) {
	// logging
	logging.InitFlags(nil)
	flag.StringVar(&loggingFormat, "loggingFormat", logging.TextFormat, "This determines the output format of the logger.")
	if err := flag.Set("v", "2"); err != nil {
		fmt.Println("failed to init flags", err)
		os.Exit(1)
	}
	// profiling
	if profile {
		flag.BoolVar(&profilingEnabled, "profile", false, "Set this flag to 'true', to enable profiling.")
		flag.StringVar(&profilingAddress, "profileAddress", "", "Profiling server address, defaults to ''.")
		flag.StringVar(&profilingPort, "profilePort", "6060", "Profiling server port, defaults to 6060.")
	}
}
