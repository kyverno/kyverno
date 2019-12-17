package signal

import (
	"os"
	"syscall"
)

var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
