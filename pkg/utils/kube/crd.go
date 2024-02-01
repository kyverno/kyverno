package kube

import (
	"context"
	"fmt"

	"go.uber.org/multierr"
	apiserver "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
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
