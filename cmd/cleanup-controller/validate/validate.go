package validate

import (
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1alpha1 "github.com/kyverno/kyverno/api/kyverno/v1alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/policy/generate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
)

// Cleanup provides implementation to validate permission for using DELETE operation by CleanupPolicy
type Cleanup struct {
	// rule to hold CleanupPolicy specifications
	spec kyvernov1alpha1.CleanupPolicySpec
	// authCheck to check access for operations
	authCheck generate.Operations
	// logger
	log logr.Logger
}

// NewCleanup returns a new instance of Cleanup validation checker
func NewCleanup(client dclient.Interface, cleanup kyvernov1alpha1.CleanupPolicySpec, log logr.Logger) *Cleanup {
	c := Cleanup{
		spec:      cleanup,
		authCheck: generate.NewAuth(client, log),
		log:       log,
	}

	return &c
}

// canIDelete returns a error if kyverno cannot perform operations
func (c *Cleanup) CanIDelete(kind, namespace string) error {
	// Skip if there is variable defined
	authCheck := c.authCheck
	if !variables.IsVariable(kind) && !variables.IsVariable(namespace) {
		// DELETE
		ok, err := authCheck.CanIDelete(kind, namespace)
		if err != nil {
			// machinery error
			return err
		}
		if !ok {
			return fmt.Errorf("kyverno does not have permissions to 'delete' resource %s/%s. Update permissions in ClusterRole", kind, namespace)
		}
	} else {
		c.log.V(4).Info("name & namespace uses variables, so cannot be resolved. Skipping Auth Checks.")
	}

	return nil
}

// Validate checks the policy and rules declarations for required configurations
func ValidateCleanupPolicy(logger logr.Logger, cleanuppolicy kyvernov1alpha1.CleanupPolicyInterface, client dclient.Interface, mock bool) error {
	// namespace := cleanuppolicy.GetNamespace()
	var res []*metav1.APIResourceList
	clusterResources := sets.NewString()

	// Get all the cluster type kind supported by cluster
	res, err := discovery.ServerPreferredResources(client.Discovery().DiscoveryInterface())
	if err != nil {
		if discovery.IsGroupDiscoveryFailedError(err) {
			err := err.(*discovery.ErrGroupDiscoveryFailed)
			for gv, err := range err.Groups {
				logger.Error(err, "failed to list api resources", "group", gv)
			}
		} else {
			return err
		}
	}
	for _, resList := range res {
		for _, r := range resList.APIResources {
			if !r.Namespaced {
				clusterResources.Insert(r.Kind)
			}
		}
	}

	if errs := cleanuppolicy.Validate(clusterResources); len(errs) != 0 {
		return errs.ToAggregate()
	}

	// for kind := range clusterResources {
	// 	checker := NewCleanup(client, *cleanuppolicy.GetSpec(), logging.GlobalLogger())
	// 	if err := checker.CanIDelete(kind, namespace); err != nil {
	// 		return fmt.Errorf("cannot delete kind %s in namespace %s", kind, namespace)
	// 	}
	// }
	return nil
}
