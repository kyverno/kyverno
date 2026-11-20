package webhook

import "testing"

func TestBuildRecorderKeyAuthorizingPolicy(t *testing.T) {
	got := BuildRecorderKey(AuthorizingPolicyType, "apol-sample", "")
	want := "AuthorizingPolicy/apol-sample"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestParseRecorderKeyAuthorizingPolicy(t *testing.T) {
	kind, name, namespace := ParseRecorderKey("AuthorizingPolicy/apol-sample")
	if kind != AuthorizingPolicyType {
		t.Fatalf("expected policy type %q, got %q", AuthorizingPolicyType, kind)
	}
	if name != "apol-sample" {
		t.Fatalf("expected name %q, got %q", "apol-sample", name)
	}
	if namespace != "" {
		t.Fatalf("expected empty namespace, got %q", namespace)
	}
}
