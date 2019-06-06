package client

import (
	"time"
)

const (
	//CSRs certificatesigningrequests
	CSRs string = "certificatesigningrequests"
	// Secrets secrets
	Secrets string = "secrets"
	// ConfigMaps configmaps
	ConfigMaps string = "configmaps"
	// Namespaces namespaces
	Namespaces string = "namespaces"
)
const namespaceCreationMaxWaitTime time.Duration = 30 * time.Second
const namespaceCreationWaitInterval time.Duration = 100 * time.Millisecond
