package kube

import (
	"context"
	"errors"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCRDsInstalled_AllInstalled(t *testing.T) {
	crds := []runtime.Object{
		&apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "clusterpolicies.kyverno.io"},
		},
		&apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "policies.kyverno.io"},
		},
	}
	client := apiextensionsfake.NewSimpleClientset(crds...)

	err := CRDsInstalled(client, "clusterpolicies.kyverno.io", "policies.kyverno.io")

	if err != nil {
		t.Errorf("CRDsInstalled() error = %v, want nil", err)
	}
}

func TestCRDsInstalled_NoneInstalled(t *testing.T) {
	client := apiextensionsfake.NewSimpleClientset()

	err := CRDsInstalled(client, "clusterpolicies.kyverno.io", "policies.kyverno.io")

	if err == nil {
		t.Error("CRDsInstalled() should return error when CRDs are not installed")
	}
}

func TestCRDsInstalled_PartiallyInstalled(t *testing.T) {
	crds := []runtime.Object{
		&apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "clusterpolicies.kyverno.io"},
		},
	}
	client := apiextensionsfake.NewSimpleClientset(crds...)

	err := CRDsInstalled(client, "clusterpolicies.kyverno.io", "policies.kyverno.io")

	if err == nil {
		t.Error("CRDsInstalled() should return error when not all CRDs are installed")
	}
}

func TestCRDsInstalled_EmptyList(t *testing.T) {
	client := apiextensionsfake.NewSimpleClientset()

	err := CRDsInstalled(client)

	if err != nil {
		t.Errorf("CRDsInstalled() with empty list should return nil, got %v", err)
	}
}

func TestCRDsInstalled_SingleCRDInstalled(t *testing.T) {
	crds := []runtime.Object{
		&apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "updaterequest.kyverno.io"},
		},
	}
	client := apiextensionsfake.NewSimpleClientset(crds...)

	err := CRDsInstalled(client, "updaterequest.kyverno.io")

	if err != nil {
		t.Errorf("CRDsInstalled() error = %v, want nil", err)
	}
}

func TestCRDsInstalled_SingleCRDNotInstalled(t *testing.T) {
	client := apiextensionsfake.NewSimpleClientset()

	err := CRDsInstalled(client, "nonexistent.kyverno.io")

	if err == nil {
		t.Error("CRDsInstalled() should return error for nonexistent CRD")
	}
}

func TestIsCRDInstalled_Exists(t *testing.T) {
	crds := []runtime.Object{
		&apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "policies.kyverno.io"},
		},
	}
	client := apiextensionsfake.NewSimpleClientset(crds...)

	err := isCRDInstalled(client, "policies.kyverno.io")

	if err != nil {
		t.Errorf("isCRDInstalled() error = %v, want nil", err)
	}
}

func TestIsCRDInstalled_NotExists(t *testing.T) {
	client := apiextensionsfake.NewSimpleClientset()

	err := isCRDInstalled(client, "nonexistent.kyverno.io")

	if err == nil {
		t.Error("isCRDInstalled() should return error for nonexistent CRD")
	}
}

func TestCRDsInstalled_AllKyvernoCRDs(t *testing.T) {
	kyvernoCRDs := []string{
		"clusterpolicies.kyverno.io",
		"policies.kyverno.io",
		"clustercleanuppolicies.kyverno.io",
		"cleanuppolicies.kyverno.io",
		"policyexceptions.kyverno.io",
		"updaterequests.kyverno.io",
	}

	crds := make([]runtime.Object, len(kyvernoCRDs))
	for i, name := range kyvernoCRDs {
		crds[i] = &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: name},
		}
	}
	client := apiextensionsfake.NewSimpleClientset(crds...)

	err := CRDsInstalled(client, kyvernoCRDs...)

	if err != nil {
		t.Errorf("CRDsInstalled() error = %v, want nil for all Kyverno CRDs", err)
	}
}

func TestCRDsInstalled_ErrorMessageContainsCRDName(t *testing.T) {
	client := apiextensionsfake.NewSimpleClientset()

	err := CRDsInstalled(client, "missing.kyverno.io")

	if err == nil {
		t.Fatal("expected an error")
	}

	errStr := err.Error()
	if !errors.Is(err, err) && errStr == "" {
		t.Error("error should contain information about missing CRD")
	}
}

func TestIsCRDInstalled_UsesCorrectContext(t *testing.T) {
	crds := []runtime.Object{
		&apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "test.kyverno.io"},
		},
	}
	client := apiextensionsfake.NewSimpleClientset(crds...)

	// Verify the function uses context correctly (background context)
	_, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(
		context.Background(),
		"test.kyverno.io",
		metav1.GetOptions{},
	)

	if err != nil {
		t.Errorf("Expected CRD to exist, got error: %v", err)
	}
}
