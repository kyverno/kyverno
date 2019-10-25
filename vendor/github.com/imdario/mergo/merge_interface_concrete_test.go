package mergo

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type ifaceTypesTest struct {
	N       int
	Handler http.Handler
}

type ifaceTypesHandler int

func (*ifaceTypesHandler) ServeHTTP(rw http.ResponseWriter, _ *http.Request) {
	rw.Header().Set("Test", "ifaceTypesHandler")
}

func TestMergeInterfaceWithDifferentConcreteTypes(t *testing.T) {
	dst := ifaceTypesTest{
		Handler: new(ifaceTypesHandler),
	}

	src := ifaceTypesTest{
		N: 42,
		Handler: http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
			rw.Header().Set("Test", "handlerFunc")
		}),
	}

	if err := Merge(&dst, src); err != nil {
		t.Errorf("Error while merging %s", err)
	}

	rw := httptest.NewRecorder()
	dst.Handler.ServeHTTP(rw, nil)

	if got, want := rw.Header().Get("Test"), "ifaceTypesHandler"; got != want {
		t.Errorf("Handler not merged in properly: got %q header value %q, want %q", "Test", got, want)
	}
}
