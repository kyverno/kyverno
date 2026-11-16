package kube

import (
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
)

// TestCRDsExist_AbsentIsNoOp verifies that when the named CRDs are not installed,
// CRDsExist reports (false, nil) so callers treat a missing CRD as "nothing to
// clean up" rather than a fatal error.
func TestCRDsExist_AbsentIsNoOp(t *testing.T) {
	client := fake.NewSimpleClientset()
	exists, err := CRDsExist(client, "clusterpolicyreports.wgpolicyk8s.io", "policyreports.wgpolicyk8s.io")
	if err != nil {
		t.Fatalf("expected no error for absent CRDs, got: %v", err)
	}
	if exists {
		t.Fatalf("expected exists=false for absent CRDs, got true")
	}
}
