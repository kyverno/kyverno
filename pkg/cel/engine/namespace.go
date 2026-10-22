package engine

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type NamespaceResolver = func(string) *corev1.Namespace

// NewNamespaceResolver returns a NamespaceResolver that resolves namespaces from the
// given lister and, in case of a cache miss, falls back to a live API call. Resolution
// failures are logged and result in a nil namespace, which is exposed to CEL as null.
func NewNamespaceResolver(logger logr.Logger, nsLister corev1listers.NamespaceLister, client kubernetes.Interface) NamespaceResolver {
	return func(name string) *corev1.Namespace {
		ns, err := nsLister.Get(name)
		if err == nil {
			return ns
		}
		if apierrors.IsNotFound(err) {
			// in case of cache latency, make a live call to verify whether the namespace truly exists
			ns, err = client.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
			if err == nil {
				return ns
			}
		}
		logger.Error(err, "failed to resolve namespace, namespaceObject will be null", "namespace", name)
		return nil
	}
}
