package config

import "k8s.io/client-go/kubernetes"

func NewFakeConfig() Configuration {
	return &configuration{}
}

func NewFakeMetricsConfig(client kubernetes.Interface) *MetricsConfigData {
	return &MetricsConfigData{
		client: client,
		cmName: "",
		metricsConfig: MetricsConfig{
			metricsRefreshInterval: 0,
			namespaces: namespacesConfig{
				IncludeNamespaces: []string{},
				ExcludeNamespaces: []string{},
			},
		},
	}
}
