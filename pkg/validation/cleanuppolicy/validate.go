package cleanuppolicy

import (
	"context"
	"fmt"
	"regexp"

	"github.com/go-logr/logr"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/auth"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
)

// FetchClusteredResources retieves the list of clustered resources
func FetchClusteredResources(logger logr.Logger, client dclient.Interface) (sets.Set[string], error) {
	res, err := discovery.ServerPreferredResources(client.Discovery().CachedDiscoveryInterface())
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
	clusterResources := sets.New[string]()
	for _, resList := range res {
		for _, r := range resList.APIResources {
			if !r.Namespaced {
				clusterResources.Insert(resList.GroupVersion + "/" + r.Kind)
				clusterResources.Insert(r.Kind)
			}
		}
	}
	return clusterResources, nil
}

// Validate checks policy is valid
func Validate(ctx context.Context, logger logr.Logger, client dclient.Interface, policy kyvernov2.CleanupPolicyInterface) error {
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

	if err := validateVariables(logger, policy); err != nil {
		return err
	}
	return nil
}

// validatePolicy checks the policy and rules declarations for required configurations
func validatePolicy(clusterResources sets.Set[string], policy kyvernov2.CleanupPolicyInterface) error {
	errs := policy.Validate(clusterResources)
	return errs.ToAggregate()
}

// validateAuth checks the the delete action is allowed
func validateAuth(ctx context.Context, client dclient.Interface, policy kyvernov2.CleanupPolicyInterface) error {
	namespace := policy.GetNamespace()
	spec := policy.GetSpec()
	resourceFilters := spec.MatchResources.GetResourceFilters()
	for _, res := range resourceFilters {
		for _, kind := range res.Kinds {
			names := res.Names
			if len(names) == 0 {
				names = append(names, "")
			}
			for _, name := range names {
				err := canI(ctx, client, kind, namespace, name, "")
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func canI(ctx context.Context, client dclient.Interface, kind, namespace, name, subresource string) error {
	checker := auth.NewCanI(client.Discovery(), client.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), kind, namespace, name, "delete", subresource, config.KyvernoUserName(config.KyvernoServiceAccountName()))
	allowedDeletion, _, err := checker.RunAccessCheck(ctx)
	if err != nil {
		return err
	}
	if !allowedDeletion {
		return fmt.Errorf("cleanup controller has no permission to delete kind %s", kind)
	}

	checker = auth.NewCanI(client.Discovery(), client.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), kind, namespace, name, "list", subresource, config.KyvernoUserName(config.KyvernoServiceAccountName()))
	allowedList, _, err := checker.RunAccessCheck(ctx)
	if err != nil {
		return err
	}
	if !allowedList {
		return fmt.Errorf("cleanup controller has no permission to list kind %s", kind)
	}
	return nil
}

func validateVariables(logger logr.Logger, policy kyvernov2.CleanupPolicyInterface) error {
	ctx := enginecontext.NewMockContext(allowedVariables)

	c := policy.GetSpec().Conditions
	conditionCopy := c.DeepCopy()
	if _, err := variables.SubstituteAllInType(logger, ctx, conditionCopy); !variables.CheckNotFoundErr(err) {
		return fmt.Errorf("variable substitution failed for policy %s: %s", policy.GetName(), err.Error())
	}
	return nil
}

var allowedVariables = regexp.MustCompile(`([a-z_0-9]+)|(target\.|images\.|([a-z_0-9]+\()[^{}])`)
