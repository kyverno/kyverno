package cleanuppolicy

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1alpha1 "github.com/kyverno/kyverno/api/kyverno/v1alpha1"
	"github.com/kyverno/kyverno/pkg/auth"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
)

// FetchClusteredResources retieves the list of clustered resources
func FetchClusteredResources(logger logr.Logger, client dclient.Interface) (sets.String, error) {
	res, err := discovery.ServerPreferredResources(client.Discovery().DiscoveryInterface())
	if err != nil {
		if discovery.IsGroupDiscoveryFailedError(err) {
			err := err.(*discovery.ErrGroupDiscoveryFailed)
			for gv, err := range err.Groups {
				logger.Error(err, "failed to list api resources", "group", gv)
			}
		} else {
			return nil, err
		}
	}
	clusterResources := sets.NewString()
	for _, resList := range res {
		for _, r := range resList.APIResources {
			if !r.Namespaced {
				clusterResources.Insert(r.Kind)
			}
		}
	}
	return clusterResources, nil
}

// Validate checks policy is valid
func Validate(ctx context.Context, logger logr.Logger, client dclient.Interface, policy kyvernov1alpha1.CleanupPolicyInterface) error {
	clusteredResources, err := FetchClusteredResources(logger, client)
	if err != nil {
		return err
	}
	if err := validatePolicy(clusteredResources, policy); err != nil {
		return err
	}
	if err := validateAuth(ctx, client, policy); err != nil {
		return err
	}
	return nil
}

// validatePolicy checks the policy and rules declarations for required configurations
func validatePolicy(clusterResources sets.String, policy kyvernov1alpha1.CleanupPolicyInterface) error {
	errs := policy.Validate(clusterResources)
	return errs.ToAggregate()
}

// validateAuth checks the the delete action is allowed
func validateAuth(ctx context.Context, client dclient.Interface, policy kyvernov1alpha1.CleanupPolicyInterface) error {
	namespace := policy.GetNamespace()
	spec := policy.GetSpec()
	kinds := sets.NewString(spec.MatchResources.GetKinds()...)
	for kind := range kinds {
		checker := auth.NewCanI(client, kind, namespace, "delete")
		allowed, err := checker.RunAccessCheck(ctx)
		if err != nil {
			return err
		}
		if !allowed {
			return fmt.Errorf("cleanup controller has no permission to delete kind %s", kind)
		}
	}
	return nil
}
