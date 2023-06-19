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
		"admissionreports.kyverno.io",
		"backgroundscanreports.kyverno.io",
		"cleanuppolicies.kyverno.io",
		"clusteradmissionreports.kyverno.io",
		"clusterbackgroundscanreports.kyverno.io",
		"clustercleanuppolicies.kyverno.io",
		"clusterpolicies.kyverno.io",
		"clusterpolicyreports.wgpolicyk8s.io",
		"policies.kyverno.io",
		"policyexceptions.kyverno.io",
		"policyreports.wgpolicyk8s.io",
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

func isCRDInstalled(apiserverClient apiserver.Interface, kind string) error {
	_, err := apiserverClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), kind, metav1.GetOptions{})
	return err
}
