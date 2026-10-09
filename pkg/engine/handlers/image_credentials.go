package handlers

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
)

// ScopeImageRegistryCredentials confines policy-supplied credentials before a
// registry client can load their secrets. ClusterPolicy credentials keep their
// existing namespace defaults.
func ScopeImageRegistryCredentials(policy kyvernov1.PolicyInterface, credentials *kyvernov1.ImageRegistryCredentials) (*kyvernov1.ImageRegistryCredentials, error) {
	policyNamespace, err := engineapi.PolicyNamespace(policy)
	if err != nil {
		return nil, err
	}
	if policyNamespace == "" || credentials == nil {
		return credentials, nil
	}
	secrets, err := kubeutils.ScopeSecretReferences(credentials.Secrets, policyNamespace)
	if err != nil {
		return nil, err
	}
	scoped := credentials.DeepCopy()
	scoped.Secrets = secrets
	return scoped, nil
}
