package imageverify

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/sdk/extensions/regcreds"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
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

func GetRemoteOptsFromPolicy(lister k8scorev1.SecretInterface, creds *v1beta1.Credentials) ([]remote.Option, []name.Option) {
	if creds == nil {
		return nil, nil
	}

	providers := make([]string, 0, len(creds.Providers))
	if len(creds.Providers) != 0 {
		for _, v := range creds.Providers {
			providers = append(providers, string(v))
		}
	}

	var authOpts []remote.Option
	var keychains []authn.Keychain
	if len(creds.Secrets) > 0 && lister != nil {
		secretLister := registryclient.SecretListerFromInterface(lister, config.KyvernoNamespace())
		keychains = append(keychains, regcreds.NewSecretsKeychain(secretLister, config.KyvernoNamespace(), creds.Secrets...))
	}
	if len(providers) > 0 {
		keychains = append(keychains, regcreds.KeychainsForProviders(providers...)...)
	}
	if len(keychains) > 0 {
		authOpts = append(authOpts, remote.WithAuthFromKeychain(authn.NewMultiKeychain(keychains...)))
	}

	nameOpts := []name.Option{}
	if creds.AllowInsecureRegistry {
		nameOpts = append(nameOpts, name.Insecure)
	}
	return authOpts, nameOpts
}
