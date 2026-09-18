package kube

import (
	"context"
	"fmt"

	"go.uber.org/multierr"
	apiserver "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CRDsInstalled checks if the Kyverno CRDs are installed or not
func CRDsInstalled(apiserverClient apiserver.Interface, names ...string) error {
	var errs []error
	for _, crd := range names {
		err := isCRDInstalled(apiserverClient, crd)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to check CRD %s is installed: %s", crd, err))
		}
	}
	return multierr.Combine(errs...)
}

func isCRDInstalled(apiserverClient apiserver.Interface, kind string) error {
	_, err := apiserverClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), kind, metav1.GetOptions{})
	return err
}

// CRDsExist reports whether all the named CRDs are installed. A missing CRD
// (NotFound) is not an error: it returns (false, nil). Any other error while
// checking, for example the API server being unreachable, is returned so callers
// can distinguish "absent" from "could not determine".
func CRDsExist(apiserverClient apiserver.Interface, names ...string) (bool, error) {
	for _, crd := range names {
		if err := isCRDInstalled(apiserverClient, crd); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, fmt.Errorf("failed to check CRD %s is installed: %w", crd, err)
		}
	}
	return true, nil
}
