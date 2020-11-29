package constant

import "time"

// Resync period for Kyverno controllers
const (
	CRDControllerResync             = 15 * time.Minute
	PolicyReportControllerResync    = 15 * time.Minute
	PolicyControllerResync          = 15 * time.Minute
	EventControllerResync           = 15 * time.Minute
	GenerateControllerResync        = 15 * time.Minute
	GenerateRequestControllerResync = 15 * time.Minute
)
