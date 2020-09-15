package constant

import "time"

const (
	CRDControllerResync             = 15 * time.Minute
	PolicyViolationControllerResync = 15 * time.Minute
	PolicyControllerResync          = 15 * time.Minute
	EventControllerResync           = 15 * time.Minute
	GenerateControllerResync        = 15 * time.Minute
	GenerateRequestControllerResync = 15 * time.Minute

	PolicyReportPolicyChangeResync =  120 * time.Second
	PolicyReportResourceChangeResync =  120 * time.Second
)
