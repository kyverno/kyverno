package cleanuppolicy

import (
	"context"
	"fmt"
	"regexp"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/auth"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
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
func Validate(ctx context.Context, logger logr.Logger, client dclient.Interface, cleanupPolicy kyvernov2.CleanupPolicyInterface) error {
	clusteredResources, err := FetchClusteredResources(logger, client)
	if err != nil {
		return err
	}
	if err := validatePolicy(clusteredResources, cleanupPolicy); err != nil {
		return err
	}
	if err := validateGlobLengths(cleanupPolicy); err != nil {
		return err
	}
	if err := validateAuth(ctx, client, cleanupPolicy); err != nil {
		return err
	}

	if err := validateVariables(logger, cleanupPolicy); err != nil {
		return err
	}
	return nil
}

// validateGlobLengths caps the length of every wildcard-bearing string
// field under match/exclude, reusing the same caps pkg/validation/policy
// applies to Policy/ClusterPolicy rules -- CleanupPolicy has its own
// validator and does not otherwise go through that package.
func validateGlobLengths(cleanupPolicy kyvernov2.CleanupPolicyInterface) error {
	spec := cleanupPolicy.GetSpec()

	checkFilter := func(prefix string, rf kyvernov1.ResourceFilter) error {
		if err := policyvalidation.ValidateResourceDescriptionGlobLengths(prefix, rf.ResourceDescription); err != nil {
			return err
		}
		return policyvalidation.ValidateUserInfoGlobLengths(prefix, rf.UserInfo)
	}

	for i, rmr := range spec.MatchResources.Any {
		if err := checkFilter(fmt.Sprintf("match.any[%d]", i), rmr); err != nil {
			return err
		}
	}
	for i, rmr := range spec.MatchResources.All {
		if err := checkFilter(fmt.Sprintf("match.all[%d]", i), rmr); err != nil {
			return err
		}
	}

	if exclude := spec.ExcludeResources; exclude != nil {
		for i, rmr := range exclude.Any {
			if err := checkFilter(fmt.Sprintf("exclude.any[%d]", i), rmr); err != nil {
				return err
			}
		}
		for i, rmr := range exclude.All {
			if err := checkFilter(fmt.Sprintf("exclude.all[%d]", i), rmr); err != nil {
				return err
			}
		}
	}

	return nil
}

// validatePolicy checks the policy and rules declarations for required configurations
func validatePolicy(clusterResources sets.Set[string], policy kyvernov2.CleanupPolicyInterface) error {
	errs := policy.Validate(clusterResources)
	return errs.ToAggregate()
}

// validateAuth checks the delete action is allowed
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
