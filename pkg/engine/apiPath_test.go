package engine

import (
	"testing"
)

func Test_Paths(t *testing.T) {
	f := func(path, expected string) {
		p, err := NewAPIPath(path)
		if err != nil {
			t.Error(err)
			return
		}

		if p.String() != expected {
			t.Errorf("expected %s got %s", expected, p.String())
		}
	}

	f("/api/v1/namespace/{{ request.namespace }}", "/api/v1/namespace/{{ request.namespace }}")
	f("/api/v1/namespace/{{ request.namespace }}/", "/api/v1/namespace/{{ request.namespace }}")
	f("/api/v1/namespace/{{ request.namespace }}/  ", "/api/v1/namespace/{{ request.namespace }}")
	f("  /api/v1/namespace/{{ request.namespace }}", "/api/v1/namespace/{{ request.namespace }}")
	f("/apis/gloo.solo.io/v1/namespaces/gloo-system/upstreams", "/apis/gloo.solo.io/v1/namespaces/gloo-system/upstreams")
	f("/apis/gloo.solo.io/v1/namespaces/gloo-system/upstreams/", "/apis/gloo.solo.io/v1/namespaces/gloo-system/upstreams")
	f("/apis/gloo.solo.io/v1/namespaces/gloo-system/upstreams/  ", "/apis/gloo.solo.io/v1/namespaces/gloo-system/upstreams")
	f("  /apis/gloo.solo.io/v1/namespaces/gloo-system/upstreams", "/apis/gloo.solo.io/v1/namespaces/gloo-system/upstreams")
}

func Test_GroupVersions(t *testing.T) {
	f := func(path, expected string) {
		p, err := NewAPIPath(path)
		if err != nil {
			t.Error(err)
			return
		}

		if p.Root == "api" {
			if p.Group != expected {
				t.Errorf("expected %s got %s", expected, p.Group)
			}
		} else {
			if p.Version != expected {
				t.Errorf("expected %s got %s", expected, p.Version)
			}
		}
	}

	f("/api/v1/namespace/{{ request.namespace }}", "v1")
	f("/apis/extensions/v1beta1/namespaces/example/ingresses", "extensions/v1beta1")
}
