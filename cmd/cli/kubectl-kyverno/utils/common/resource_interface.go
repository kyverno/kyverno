package common

import (
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ResourceInterface abstracts the matched resources by either Kyverno Policies or Validating Admission Policies
type ResourceInterface interface {
	FetchResourcesFromPolicy(resourcePaths []string, dClient dclient.Interface, namespace string, policyReport bool) ([]*unstructured.Unstructured, error)
}
