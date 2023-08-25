package config

import "fmt"

func InClusterServiceName() string {
	return KyvernoServiceName() + "." + KyvernoNamespace() + ".svc"
}

func DnsNames() []string {
	return []string{
		KyvernoServiceName(),
		fmt.Sprintf("%s.%s", KyvernoServiceName(), KyvernoNamespace()),
		InClusterServiceName(),
	}
}

func GenerateTLSPairSecretName() string {
	return InClusterServiceName() + ".kyverno-tls-pair"
}

func GenerateRootCASecretName() string {
	return InClusterServiceName() + ".kyverno-tls-ca"
}
