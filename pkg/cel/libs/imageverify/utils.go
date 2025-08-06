package imageverify

import (
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
)

func attestationMap(ivpol *v1alpha1.ImageValidatingPolicy) map[string]v1alpha1.Attestation {
	if ivpol == nil {
		return nil
	}

	return arrToMap(ivpol.Spec.Attestations)
}

type ARR_TYPE interface {
	GetKey() string
}

func arrToMap[T ARR_TYPE](arr []T) map[string]T {
	m := make(map[string]T)
	for _, v := range arr {
		m[v.GetKey()] = v
	}

	return m
}

func GetRemoteOptsFromPolicy(creds *v1alpha1.Credentials) []imagedataloader.Option {
	if creds == nil {
		return []imagedataloader.Option{}
	}

	providers := make([]string, 0, len(creds.Providers))
	if len(creds.Providers) != 0 {
		for _, v := range creds.Providers {
			providers = append(providers, string(v))
		}
	}

	return imagedataloader.BuildRemoteOpts(creds.Secrets, providers, creds.AllowInsecureRegistry)
}
