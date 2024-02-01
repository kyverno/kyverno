package kube

import (
	"context"
	"fmt"

	"go.uber.org/multierr"
	apiserver "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CRDsInstalled checks if the Kyverno CRDs are installed or not
func CRDsInstalled(apiserverClient apiserver.Interface) error {
	kyvernoCRDs := []string{
		"clusterpolicies.kyverno.io",
		"policies.kyverno.io",
	}
	var errs []error
	for _, crd := range kyvernoCRDs {
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

func CRDsForBackgroundControllerInstalled(apiserverClient apiserver.Interface) error {
	kyvernoCRDs := []string{
		"updaterequests.kyverno.io",
	}
	var errs []error
	for _, crd := range kyvernoCRDs {
		err := isCRDInstalled(apiserverClient, crd)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to check CRD %s is installed: %s", crd, err))
		}
	}
	return multierr.Combine(errs...)
}

func CRDsForCleanupControllerInstalled(apiserverClient apiserver.Interface) error {
	kyvernoCRDs := []string{
		"cleanuppolicies.kyverno.io",
		"clustercleanuppolicies.kyverno.io",
	}

	var errs []error
	for _, crd := range kyvernoCRDs {
		err := isCRDInstalled(apiserverClient, crd)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to check CRD %s is installed: %s", crd, err))
		}
	}
	return multierr.Combine(errs...)
}

func CRDsForReportsControllerInstalled(apiserverClient apiserver.Interface) error {
	kyvernoCRDs := []string{
		"clusterpolicyreports.wgpolicyk8s.io",
		"policyreports.wgpolicyk8s.io",
		"clusterbackgroundscanreports.kyverno.io",
		"backgroundscanreports.kyverno.io",
	}
	var errs []error
	for _, crd := range kyvernoCRDs {
		err := isCRDInstalled(apiserverClient, crd)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to check CRD %s is installed: %s", crd, err))
		}
	}
	return multierr.Combine(errs...)
}
