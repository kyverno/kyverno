package constant

import "time"

const (
	CRDControllerResync             = 15 * time.Minute
	PolicyViolationControllerResync = 15 * time.Minute
	PolicyControllerResync          = 15 * time.Minute
	EventControllerResync           = 15 * time.Minute
	GenerateControllerResync        = 15 * time.Minute
	GenerateRequestControllerResync = 15 * time.Minute

	PolicyReportPolicyChangeResync   = 60 * time.Second
	PolicyReportResourceChangeResync = 60 * time.Second
)

const (
	App       string = "App"
	Namespace string = "Namespace"
	Cluster   string = "Cluster"
	All       string = "All"
)

const (
	ConfiigmapMode       string = "CONFIGMAP"
	BackgroundPolicySync string = "POLICYSYNC"
	BackgroundSync       string = "SYNC"
)
