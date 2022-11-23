package validate

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1alpha1 "github.com/kyverno/kyverno/api/kyverno/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cleanup-controller/logger"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/policy/generate"
	"github.com/kyverno/kyverno/pkg/utils/admission"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
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

func listNameSpaces(coreClient kubernetes.Interface) ([]string, error) {
	var namespaceList []string
	nsList, err := coreClient.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return namespaceList, err
	}
	for _, n := range nsList.Items {
		namespaceList = append(namespaceList, n.Name)
	}
	return namespaceList, nil
}

// Validate checks the policy and rules declarations for required configurations
func ValidateCleanupPolicy(cleanuppolicy kyvernov1alpha1.CleanupPolicyInterface, dclient dclient.Interface, client kubernetes.Interface, mock bool) error {
	var res []*metav1.APIResourceList
	clusterResources := sets.NewString()
	// Get all the cluster type kind supported by cluster
	res, err := discovery.ServerPreferredResources(dclient.Discovery().DiscoveryInterface())
	if err != nil {
		if discovery.IsGroupDiscoveryFailedError(err) {
			err := err.(*discovery.ErrGroupDiscoveryFailed)
			for gv, err := range err.Groups {
				logger.Logger.Error(err, "failed to list api resources", "group", gv)
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

	kinds := admission.FetchUniqueKinds(*cleanuppolicy.GetSpec())
	for _, kind := range kinds {
		checker := NewCleanup(dclient, *cleanuppolicy.GetSpec(), logging.GlobalLogger())
		namespaces, err := listNameSpaces(client)
		if err != nil {
			return err
		}
		for _, namespace := range namespaces {
			if err := checker.CanIDelete(kind, namespace); err != nil {
				return fmt.Errorf("cannot delete kind %s in namespace %s", kind, namespace)
			}
		}
	}

	return nil
}
