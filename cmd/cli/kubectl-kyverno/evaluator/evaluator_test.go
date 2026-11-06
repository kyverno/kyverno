package evaluator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const fixtures = "../../../../test/cli/serve"

func fixture(parts ...string) string {
	return filepath.Join(append([]string{fixtures}, parts...)...)
}

func readFixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	b, err := os.ReadFile(fixture(parts...))
	require.NoError(t, err)
	return b
}

func TestParseMode(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]Mode{"Kubernetes": ModeKubernetes, "kubernetes": ModeKubernetes, " JSON ": ModeJSON, "http": ModeHTTP, "ENVOY": ModeEnvoy} {
		got, err := ParseMode(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got)
	}
	_, err := ParseMode("grpc")
	assert.ErrorContains(t, err, "unsupported mode")
}

func TestDecodeInput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mode    Mode
		body    []byte
		wantErr string
		check   func(t *testing.T, in Input)
	}{{
		name: "kubernetes yaml",
		mode: ModeKubernetes,
		body: readFixture(t, "payloads", "kubernetes", "pod.yaml"),
		check: func(t *testing.T, in Input) {
			require.Len(t, in.Resources, 1)
			assert.Equal(t, "Pod", in.Resources[0].GetKind())
		},
	}, {
		name: "kubernetes multi-document",
		mode: ModeKubernetes,
		body: append(append(readFixture(t, "payloads", "kubernetes", "pod.yaml"), []byte("\n---\n")...), readFixture(t, "payloads", "kubernetes", "namespace.yaml")...),
		check: func(t *testing.T, in Input) {
			assert.Len(t, in.Resources, 2)
		},
	}, {
		name: "kubernetes json list",
		mode: ModeKubernetes,
		body: []byte(`{"apiVersion":"v1","kind":"List","items":[{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"a"}},{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"b"}}]}`),
		check: func(t *testing.T, in Input) {
			assert.Len(t, in.Resources, 2)
		},
	}, {
		name:    "kubernetes missing kind",
		mode:    ModeKubernetes,
		body:    []byte("apiVersion: v1\nmetadata:\n  name: x\n"),
		wantErr: "'Kind' is missing",
	}, {
		name:    "kubernetes not yaml",
		mode:    ModeKubernetes,
		body:    []byte("{not yaml"),
		wantErr: "failed to decode Kubernetes resources",
	}, {
		name: "json object",
		mode: ModeJSON,
		body: readFixture(t, "payloads", "json", "dockerfile-pass.json"),
		check: func(t *testing.T, in Input) {
			require.Len(t, in.JSONPayloads, 1)
			assert.Contains(t, in.JSONPayloads[0].Object, "Stages")
		},
	}, {
		name: "json as yaml",
		mode: ModeJSON,
		body: []byte("foo: bar\n"),
		check: func(t *testing.T, in Input) {
			require.Len(t, in.JSONPayloads, 1)
			assert.Equal(t, "bar", in.JSONPayloads[0].Object["foo"])
		},
	}, {
		name:    "json array rejected",
		mode:    ModeJSON,
		body:    []byte(`[1,2]`),
		wantErr: "single top-level object",
	}, {
		name:    "json malformed",
		mode:    ModeJSON,
		body:    []byte(`{"a":`),
		wantErr: "failed to decode JSON payload",
	}, {
		name: "http",
		mode: ModeHTTP,
		body: readFixture(t, "payloads", "http", "get.json"),
		check: func(t *testing.T, in Input) {
			require.Len(t, in.HTTPRequests, 1)
			assert.Equal(t, "GET", in.HTTPRequests[0].Attributes.Method)
		},
	}, {
		name:    "http malformed",
		mode:    ModeHTTP,
		body:    []byte(`{"attributes":`),
		wantErr: "failed to decode HTTP check request",
	}, {
		name: "envoy",
		mode: ModeEnvoy,
		body: readFixture(t, "payloads", "envoy", "admin.json"),
		check: func(t *testing.T, in Input) {
			require.Len(t, in.EnvoyRequests, 1)
			assert.Equal(t, "/admin/users", in.EnvoyRequests[0].GetAttributes().GetRequest().GetHttp().GetPath())
		},
	}, {
		name:    "envoy unknown field",
		mode:    ModeEnvoy,
		body:    []byte(`{"notAField":true}`),
		wantErr: "failed to decode Envoy check request",
	}, {
		name:    "empty body",
		mode:    ModeKubernetes,
		body:    []byte("  \n"),
		wantErr: "request body is empty",
	}, {
		name:    "unknown mode",
		mode:    Mode("gRPC"),
		body:    []byte(`{}`),
		wantErr: "unsupported mode",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in, err := DecodeInput(tt.mode, tt.body)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			tt.check(t, in)
		})
	}
}

func TestLoadStrict(t *testing.T) {
	t.Parallel()
	set, err := LoadStrict(fixture("policies", "kubernetes"), fixture("policies", "http"))
	require.NoError(t, err)
	assert.Equal(t, 4, set.PolicyCount())
	assert.Len(t, set.GenericPolicies(), 3)

	_, err = LoadStrict(fixture("policies", "invalid"))
	assert.ErrorContains(t, err, "failed to load policies")

	_, err = LoadStrict(fixture("policies", "does-not-exist"))
	assert.Error(t, err)

	_, err = LoadStrict()
	assert.ErrorContains(t, err, "at least one policy path")

	empty := t.TempDir()
	_, err = LoadStrict(empty)
	assert.ErrorContains(t, err, "no policies found")

	legacy := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(legacy, "cpol.yaml"), []byte("apiVersion: kyverno.io/v1\nkind: ClusterPolicy\nmetadata:\n  name: legacy\nspec:\n  rules: []\n"), 0o600))
	_, err = LoadStrict(legacy)
	assert.Error(t, err)
}

func TestPolicySetPoliciesAndDigest(t *testing.T) {
	t.Parallel()
	a, err := LoadStrict(fixture("policies", "kubernetes"), fixture("policies", "json"), fixture("policies", "http"), fixture("policies", "envoy"))
	require.NoError(t, err)
	b, err := LoadStrict(fixture("policies", "envoy"), fixture("policies", "http"), fixture("policies", "json"), fixture("policies", "kubernetes"))
	require.NoError(t, err)
	da, err := a.Digest()
	require.NoError(t, err)
	db, err := b.Digest()
	require.NoError(t, err)
	assert.Equal(t, da, db, "digest must not depend on load order")
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, da)

	c, err := LoadStrict(fixture("policies", "kubernetes"))
	require.NoError(t, err)
	dc, err := c.Digest()
	require.NoError(t, err)
	assert.NotEqual(t, da, dc)

	modes := map[string][]Mode{}
	for _, p := range a.Policies() {
		modes[p.Kind+"/"+p.Name] = p.Modes
	}
	assert.Equal(t, map[string][]Mode{
		"GeneratingPolicy/generate-cm":                    {ModeKubernetes},
		"MutatingPolicy/rewrite-image-registry":           {ModeKubernetes},
		"ValidatingPolicy/allow-only-get":                 {ModeHTTP},
		"ValidatingPolicy/check-deployment-labels":        {ModeKubernetes},
		"ValidatingPolicy/check-dockerfile-disallow-wget": {ModeJSON},
		"ValidatingPolicy/forbid-admin-path":              {ModeEnvoy},
	}, modes)
}

//nolint:paralleltest // Evaluate is documented as not safe for concurrent use
func TestEvaluate(t *testing.T) {
	data.InjectProcessor(data.NewCRDProcessor(nil))
	set, err := LoadStrict(fixture("policies", "kubernetes"), fixture("policies", "json"), fixture("policies", "http"), fixture("policies", "envoy"))
	require.NoError(t, err)

	t.Run("kubernetes fail", func(t *testing.T) {
		in, err := DecodeInput(ModeKubernetes, readFixture(t, "payloads", "kubernetes", "deployment-fail.yaml"))
		require.NoError(t, err)
		in.Client, err = NewFakeClient(in.Resources, nil, nil)
		require.NoError(t, err)
		result, err := Evaluate(context.Background(), set, in, Options{})
		require.NoError(t, err)
		assert.Equal(t, 1, result.Counts.Fail)
		var failed []string
		for _, r := range result.Responses {
			for _, rule := range r.PolicyResponse.Rules {
				if rule.Status() == engineapi.RuleStatusFail {
					failed = append(failed, r.Policy().GetName())
				}
			}
		}
		assert.Equal(t, []string{"check-deployment-labels"}, failed)
	})
	t.Run("envoy deny", func(t *testing.T) {
		in, err := DecodeInput(ModeEnvoy, readFixture(t, "payloads", "envoy", "admin.json"))
		require.NoError(t, err)
		result, err := Evaluate(context.Background(), set, in, Options{})
		require.NoError(t, err)
		assert.Equal(t, 1, result.Counts.Fail)
		require.Len(t, result.Responses, 1)
	})
	t.Run("json pass", func(t *testing.T) {
		in, err := DecodeInput(ModeJSON, readFixture(t, "payloads", "json", "dockerfile-pass.json"))
		require.NoError(t, err)
		result, err := Evaluate(context.Background(), set, in, Options{})
		require.NoError(t, err)
		assert.Equal(t, 1, result.Counts.Pass)
		assert.Equal(t, 0, result.Counts.Fail)
	})
}

func TestNewFakeClientRejectsMissingTypeMeta(t *testing.T) {
	t.Parallel()
	in, err := DecodeInput(ModeKubernetes, readFixture(t, "payloads", "kubernetes", "pod.yaml"))
	require.NoError(t, err)
	in.Resources[0].SetKind("")
	_, err = NewFakeClient(in.Resources, nil, nil)
	assert.ErrorContains(t, err, "missing TypeMeta")
}

func TestNewFakeClientDeduplicates(t *testing.T) {
	t.Parallel()
	in, err := DecodeInput(ModeKubernetes, readFixture(t, "payloads", "kubernetes", "pod.yaml"))
	require.NoError(t, err)
	dup := in.Resources[0].DeepCopy()
	dup.SetLabels(map[string]string{"copy": "last"})
	client, err := NewFakeClient(in.Resources, []*unstructured.Unstructured{dup}, nil)
	require.NoError(t, err)
	got, err := client.GetResource(context.Background(), "v1", "Pod", "default", "web")
	require.NoError(t, err)
	assert.Equal(t, "last", got.GetLabels()["copy"])
}

// TestOfflineNamespaceResolver checks that offline namespace lookups (namespaceSelector,
// namespaceObject) fall back to Namespace objects in the in-memory cluster.
func TestOfflineNamespaceResolver(t *testing.T) {
	t.Parallel()
	in, err := DecodeInput(ModeKubernetes, []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: team-a\n  labels:\n    env: prod\n"))
	require.NoError(t, err)
	client, err := NewFakeClient(in.Resources, nil, nil)
	require.NoError(t, err)
	resolve := processor.GetNamespaceResolver(nil, client, false)
	ns := resolve("team-a")
	require.NotNil(t, ns)
	assert.Equal(t, "prod", ns.Labels["env"])
	assert.Nil(t, resolve("missing"))
	assert.Nil(t, resolve(""))
}
