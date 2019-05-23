package client

import (
	"time"
)

const (
	CSRs       string = "certificatesigningrequests"
	Secrets    string = "secrets"
	ConfigMaps string = "configmaps"
	Namespaces string = "namespaces"
)
const namespaceCreationMaxWaitTime time.Duration = 30 * time.Second
const namespaceCreationWaitInterval time.Duration = 100 * time.Millisecond
