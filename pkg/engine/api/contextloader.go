package api

import (
	"context"
	"fmt"
	"reflect"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

type RegistryClientFactory interface {
	GetClient(ctx context.Context, creds *kyvernov1.ImageRegistryCredentials, resourceNamespace string, imagePullSecrets []string) (RegistryClient, error)
}

// ScopedRegistryClientFactory enables tenant Secret access only after policy
// credential references have been confined to the policy namespace.
type ScopedRegistryClientFactory interface {
	GetClientForPolicy(ctx context.Context, creds *kyvernov1.ImageRegistryCredentials, resourceNamespace string, imagePullSecrets []string, policyNamespace string) (RegistryClient, error)
}

// RegistryClientForPolicy preserves ordinary factories while allowing the live
// controller factory to resolve credentials in an explicitly scoped namespace.
func RegistryClientForPolicy(ctx context.Context, factory RegistryClientFactory, creds *kyvernov1.ImageRegistryCredentials, resourceNamespace string, imagePullSecrets []string, policyNamespace string) (RegistryClient, error) {
	if scoped, ok := factory.(ScopedRegistryClientFactory); ok && policyNamespace != "" {
		return scoped.GetClientForPolicy(ctx, creds, resourceNamespace, imagePullSecrets, policyNamespace)
	}
	return factory.GetClient(ctx, creds, resourceNamespace, imagePullSecrets)
}

type Initializer = func(jsonContext enginecontext.Interface) error

// PolicyScope is the only part of a policy a ContextLoader needs: its identity and whether
// it is namespace-scoped. Both kyvernov1.PolicyInterface and kyvernov2.CleanupPolicyInterface
// satisfy it, so no caller has to pass nil and silently lose the namespace clamp.
type PolicyScope interface {
	metav1.Object
	IsNamespaced() bool
}

// PolicyNamespace returns the confinement namespace for a policy. An empty
// namespace is valid only for an explicitly cluster-scoped policy.
func PolicyNamespace(policy PolicyScope) (string, error) {
	if policy == nil || (reflect.ValueOf(policy).Kind() == reflect.Pointer && reflect.ValueOf(policy).IsNil()) {
		return "", fmt.Errorf("policy scope must not be nil")
	}
	if !policy.IsNamespaced() {
		return "", nil
	}
	namespace := policy.GetNamespace()
	if namespace == "" {
		return "", fmt.Errorf("policy namespace must not be empty")
	}
	if len(validation.IsDNS1123Label(namespace)) != 0 {
		return "", fmt.Errorf("policy namespace %q is invalid", namespace)
	}
	return namespace, nil
}

// ContextLoaderFactory provides a ContextLoader given a policy context and rule name
type ContextLoaderFactory = func(policy PolicyScope, rule kyvernov1.Rule) ContextLoader

// ContextLoader abstracts the mechanics to load context entries in the underlying json context
type ContextLoader interface {
	Load(
		ctx context.Context,
		jp jmespath.Interface,
		client RawClient,
		rclientFactory RegistryClientFactory,
		contextEntries []kyvernov1.ContextEntry,
		jsonContext enginecontext.Interface,
	) error
}
