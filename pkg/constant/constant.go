package constant

import "time"

const (
	CRDControllerResync             = 15 * time.Minute
	PolicyReportControllerResync    = 15 * time.Minute
	PolicyControllerResync          = 15 * time.Minute
	EventControllerResync           = 15 * time.Minute
	GenerateControllerResync        = 15 * time.Minute
	GenerateRequestControllerResync = 15 * time.Minute

	PolicyReportPolicyChangeResync   = 60 * time.Second
	PolicyReportResourceChangeResync = 60 * time.Second
)

const (
	Namespace string = "Namespace"
	Cluster   string = "Cluster"
	All       string = "All"
)

const (
	ConfigmapMode        string = "CONFIGMAP"
	BackgroundPolicySync string = "POLICYSYNC"
	BackgroundSync       string = "SYNC"
)
