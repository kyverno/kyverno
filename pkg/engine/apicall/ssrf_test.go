package apicall

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/toggle"
	"gotest.tools/v3/assert"
)

func TestMain(m *testing.M) {
	// Package tests use httptest on loopback. Keep metadata host blocks, drop loopback CIDRs
	// so functional ServiceCall tests still dial localhost. Security tests that need the
	// full default blocklist call toggle.HTTPBlocklist.Reset() explicitly.
	_ = toggle.HTTPBlocklist.Parse("169.254.169.254,169.254.169.253,metadata.google.internal")
	os.Exit(m.Run())
}

func Test_ServiceCall_BlocksMetadataHostname(t *testing.T) {
	toggle.HTTPBlocklist.Reset()
	t.Cleanup(func() {
		_ = toggle.HTTPBlocklist.Parse("169.254.169.254,169.254.169.253,metadata.google.internal")
	})

	ctx := enginecontext.NewContext(jp)
	entry := kyvernov1.ContextEntry{
		Name: "svc",
		APICall: &kyvernov1.ContextAPICall{
			APICall: kyvernov1.APICall{
				Method: "GET",
				Service: &kyvernov1.ServiceCall{
					URL: "http://169.254.169.254/latest/meta-data/",
				},
			},
		},
	}
	call, err := New(logr.Discard(), jp, entry, ctx, nil, NewAPICallConfiguration(0, 0), "")
	assert.NilError(t, err)
	_, err = call.Fetch(context.Background())
	assert.ErrorContains(t, err, "blocked")
}

func Test_ServiceCall_BlocksLoopbackCIDR(t *testing.T) {
	toggle.HTTPBlocklist.Reset()
	t.Cleanup(func() {
		_ = toggle.HTTPBlocklist.Parse("169.254.169.254,169.254.169.253,metadata.google.internal")
	})

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer s.Close()

	ctx := enginecontext.NewContext(jp)
	entry := kyvernov1.ContextEntry{
		Name: "svc",
		APICall: &kyvernov1.ContextAPICall{
			APICall: kyvernov1.APICall{
				Method: "GET",
				Service: &kyvernov1.ServiceCall{
					URL: s.URL + "/resource",
				},
			},
		},
	}
	call, err := New(logr.Discard(), jp, entry, ctx, nil, NewAPICallConfiguration(0, 0), "")
	assert.NilError(t, err)
	_, err = call.Fetch(context.Background())
	assert.ErrorContains(t, err, "blocked")
}

func Test_ServiceCall_StillInjectsScopedTokenWhenAllowed(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "token")
	assert.NilError(t, os.WriteFile(tokenPath, []byte("scoped-sa-token\n"), 0o600))
	old := scopedTokenPath
	scopedTokenPath = tokenPath
	t.Cleanup(func() { scopedTokenPath = old })

	var gotAuth, gotPath string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer s.Close()

	ctx := enginecontext.NewContext(jp)
	entry := kyvernov1.ContextEntry{
		Name: "svc",
		APICall: &kyvernov1.ContextAPICall{
			APICall: kyvernov1.APICall{
				Method: "GET",
				Service: &kyvernov1.ServiceCall{
					URL: s.URL + "/latest/meta-data/",
				},
			},
		},
	}
	call, err := New(logr.Discard(), jp, entry, ctx, nil, NewAPICallConfiguration(0, 0), "")
	assert.NilError(t, err)
	data, err := call.Fetch(context.Background())
	assert.NilError(t, err)
	assert.Equal(t, string(data), `{"ok":true}`)
	assert.Equal(t, gotPath, "/latest/meta-data/")
	assert.Equal(t, gotAuth, "Bearer scoped-sa-token")
}
