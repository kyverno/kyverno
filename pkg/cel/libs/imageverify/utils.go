package imageverify

import (
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
)

func attestationMap(ivpol v1beta1.ImageValidatingPolicyLike) map[string]v1beta1.Attestation {
	if ivpol == nil {
		return nil
	}
	spec := ivpol.GetSpec()
	return arrToMap(spec.Attestations)
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

func GetRemoteOptsFromPolicy(creds *v1beta1.Credentials) []imagedataloader.Option {
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
