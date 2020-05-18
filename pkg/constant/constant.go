package constant

import "time"

const (
	CRDControllerResync             = 10 * time.Minute
	PolicyViolationControllerResync = 5 * time.Minute
	PolicyControllerResync          = time.Second
	EventControllerResync           = time.Second
	GenerateControllerResync        = time.Second
	GenerateRequestControllerResync = time.Second
)
