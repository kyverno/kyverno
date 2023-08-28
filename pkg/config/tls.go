package config

import "fmt"

func InClusterServiceName(commonName string, namespace string) string {
	return commonName + "." + namespace + ".svc"
}

func DnsNames(commonName string, namespace string) []string {
	return []string{
		commonName,
		fmt.Sprintf("%s.%s", commonName, namespace),
		InClusterServiceName(commonName, namespace),
	}
}
