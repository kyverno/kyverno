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
