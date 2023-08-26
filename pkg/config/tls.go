package config

import "fmt"

func InClusterServiceName(commonName string, namespace string) string {
	return commonName + "." + namespace + ".svc"
}

func DnsNames(commonName string, namespace string) []string {
	return []string{
		commonName,
		"svc." + commonName,
		fmt.Sprintf("%s.%s", commonName, namespace),
		InClusterServiceName(commonName, namespace),
	}
}

func GenerateTLSPairSecretName(commonName string, namespace string) string {
	return InClusterServiceName(commonName, namespace) + ".kyverno-tls-pair"
}

func GenerateRootCASecretName(commonName string, namespace string) string {
	return InClusterServiceName(commonName, namespace) + ".kyverno-tls-ca"
}
