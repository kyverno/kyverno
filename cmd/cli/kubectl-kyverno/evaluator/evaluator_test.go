package evaluator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
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
		name:    "http unknown field",
		mode:    ModeHTTP,
		body:    []byte(`{"attribute":{"method":"GET"}}`),
		wantErr: `unknown field "attribute"`,
	}, {
		name:    "http second yaml document",
		mode:    ModeHTTP,
		body:    []byte("attributes:\n  method: GET\n---\nattributes:\n  method: DELETE\n"),
		wantErr: "more than one YAML document",
	}, {
		name: "http yaml with a leading document separator and comments",
		mode: ModeHTTP,
		body: []byte("# a GET request\n---\nattributes:\n  method: GET\n"),
		check: func(t *testing.T, in Input) {
			require.Len(t, in.HTTPRequests, 1)
			assert.Equal(t, "GET", in.HTTPRequests[0].Attributes.Method)
		},
	}, {
		name:    "envoy second yaml document",
		mode:    ModeEnvoy,
		body:    []byte("attributes:\n  request:\n    http:\n      path: /\n---\nattributes:\n  request:\n    http:\n      path: /admin\n"),
		wantErr: "more than one YAML document",
	}, {
		name:    "http trailing data",
		mode:    ModeHTTP,
		body:    append(readFixture(t, "payloads", "http", "get.json"), []byte(`{"unexpected":true}`)...),
		wantErr: "unexpected data after the request object",
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

	_, err = LoadStrict(fixture("policies", "invalid-cel"))
	assert.ErrorContains(t, err, "failed to compile ValidatingPolicy bad-expression")

	_, err = LoadStrictBytes([]byte(`apiVersion: policies.kyverno.io/v1beta1
kind: MutatingPolicy
metadata:
  name: bad-mutation
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [CREATE]
      resources: [pods]
  mutations:
  - patchType: JSONPatch
    jsonPatch:
      expression: "[JSONPatch{op: "
`))
	assert.ErrorContains(t, err, "failed to compile MutatingPolicy bad-mutation")

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

// TestLoadStrictRejectsUncompilablePolicies covers each compiler PolicySet.Compile dispatches to:
// every policy below has a valid schema and a CEL expression that does not compile.
func TestLoadStrictRejectsUncompilablePolicies(t *testing.T) {
	t.Parallel()
	const broken = `"object.metadata.name =="`
	match := `
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [CREATE]
      resources: [pods]`
	tests := []struct {
		name   string
		policy string
	}{{
		name: "ValidatingPolicy",
		policy: `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: broken
spec:` + match + `
  validations:
  - expression: ` + broken,
	}, {
		name: "MutatingPolicy",
		policy: `apiVersion: policies.kyverno.io/v1beta1
kind: MutatingPolicy
metadata:
  name: broken
spec:` + match + `
  mutations:
  - patchType: JSONPatch
    jsonPatch:
      expression: ` + broken,
	}, {
		name: "GeneratingPolicy",
		policy: `apiVersion: policies.kyverno.io/v1beta1
kind: GeneratingPolicy
metadata:
  name: broken
spec:` + match + `
  generate:
  - expression: ` + broken,
	}, {
		name: "DeletingPolicy",
		policy: `apiVersion: policies.kyverno.io/v1beta1
kind: DeletingPolicy
metadata:
  name: broken
spec:
  schedule: "*/1 * * * *"` + match + `
  conditions:
  - name: broken
    expression: ` + broken,
	}, {
		name: "ImageValidatingPolicy",
		policy: `apiVersion: policies.kyverno.io/v1beta1
kind: ImageValidatingPolicy
metadata:
  name: broken
spec:` + match + `
  matchImageReferences:
  - glob: "registry.example.com/*"
  attestors:
  - name: unused
    cosign:
      key:
        data: unused
  validations:
  - expression: ` + broken,
	}, {
		name: "HTTP ValidatingPolicy",
		policy: `apiVersion: policies.kyverno.io/v1alpha1
kind: ValidatingPolicy
metadata:
  name: broken
spec:
  evaluation:
    mode: HTTP
  validations:
  - expression: ` + broken,
	}, {
		name: "Envoy ValidatingPolicy",
		policy: `apiVersion: policies.kyverno.io/v1alpha1
kind: ValidatingPolicy
metadata:
  name: broken
spec:
  evaluation:
    mode: Envoy
  validations:
  - expression: ` + broken,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// the policy itself loads: only compiling it fails
			results, err := policy.LoadBytes("test", []byte(tt.policy))
			require.NoError(t, err)
			require.Empty(t, results.NonFatalErrors)

			set, err := LoadStrictBytes([]byte(tt.policy))
			require.ErrorContains(t, err, "failed to compile "+tt.name+" broken")
			assert.Nil(t, set)

			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "policy.yaml"), []byte(tt.policy), 0o600))
			set, err = LoadStrict(dir)
			require.ErrorContains(t, err, "failed to compile "+tt.name+" broken")
			assert.Nil(t, set)
		})
	}
}

// TestEvaluateKeepsResponsesWhenALaterPassFails: the ValidatingPolicy pass completes and is
// counted, then the DeletingPolicy pass fails (its policy does not compile; Evaluate is handed the
// set without LoadStrict's checks). The error must come back with the completed response.
//
//nolint:paralleltest // data.InjectProcessor sets process-global state
func TestEvaluateKeepsResponsesWhenALaterPassFails(t *testing.T) {
	data.InjectProcessor(data.NewCRDProcessor(nil))
	validating, err := policy.LoadBytes("validating", readFixture(t, "policies", "kubernetes", "check-deployment-labels.yaml"))
	require.NoError(t, err)
	deleting, err := policy.LoadBytes("deleting", []byte(`apiVersion: policies.kyverno.io/v1beta1
kind: DeletingPolicy
metadata:
  name: broken
spec:
  schedule: "*/1 * * * *"
  matchConstraints:
    resourceRules:
    - apiGroups: [apps]
      apiVersions: [v1]
      resources: [deployments]
  conditions:
  - name: broken
    expression: "object.metadata.name =="
`))
	require.NoError(t, err)
	set := &PolicySet{}
	set.Add(validating)
	set.Add(deleting)
	require.Error(t, set.Compile(), "the set is only usable because LoadStrict was skipped")

	in, err := DecodeInput(ModeKubernetes, readFixture(t, "payloads", "kubernetes", "deployment-fail.yaml"))
	require.NoError(t, err)
	in.Client, err = NewFakeClient(in.Resources, nil, nil)
	require.NoError(t, err)
	result, err := Evaluate(context.Background(), set, in, Options{})
	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Counts.Fail)
	require.Len(t, result.Responses, 1, "the completed ValidatingPolicy response is kept")
	assert.Equal(t, "check-deployment-labels", result.Responses[0].Policy().GetName())
}

// TestEvaluateJSONImageValidatingPolicyException: a PolicyException that matches a JSON payload
// exempts a JSON-mode ImageValidatingPolicy, as it does for resources. The policy always fails and
// calls no attestor, so nothing is fetched from a registry.
//
//nolint:paralleltest // data.InjectProcessor sets process-global state
func TestEvaluateJSONImageValidatingPolicyException(t *testing.T) {
	data.InjectProcessor(data.NewCRDProcessor(nil))
	const ivpol = `apiVersion: policies.kyverno.io/v1beta1
kind: ImageValidatingPolicy
metadata:
  name: json-images
spec:
  evaluation:
    mode: JSON
  matchImageReferences:
  - glob: "registry.example.com/*"
  validationConfigurations:
    verifyDigest: false
    required: false
  attestors:
  - name: unused
    cosign:
      key:
        data: unused
  images:
  - name: app
    expression: "[object.image]"
  validations:
  - expression: "false"
    message: always fails
`
	const exception = `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: exempt-sandbox
spec:
  policyRefs:
  - name: json-images
    kind: ImageValidatingPolicy
  matchConditions:
  - name: is-sandbox
    expression: "object.env == 'sandbox'"
`
	tests := []struct {
		name       string
		payload    string
		exceptions bool
		want       engineapi.RuleStatus
	}{
		{"no exception fails", `{"env":"sandbox","image":"registry.example.com/app:1"}`, false, engineapi.RuleStatusFail},
		{"matching exception skips", `{"env":"sandbox","image":"registry.example.com/app:1"}`, true, engineapi.RuleStatusSkip},
		{"exception that does not match still fails", `{"env":"production","image":"registry.example.com/app:1"}`, true, engineapi.RuleStatusFail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := ivpol
			if tt.exceptions {
				content += "---\n" + exception
			}
			set, err := LoadStrictBytes([]byte(content))
			require.NoError(t, err)
			in, err := DecodeInput(ModeJSON, []byte(tt.payload))
			require.NoError(t, err)
			in.Exceptions = set.PolicyCelExceptions
			result, err := Evaluate(context.Background(), set, in, Options{})
			require.NoError(t, err)
			require.Len(t, result.Responses, 1)
			rules := result.Responses[0].PolicyResponse.Rules
			require.Len(t, rules, 1)
			assert.Equal(t, tt.want, rules[0].Status(), rules[0].Message())
			if tt.want == engineapi.RuleStatusSkip {
				assert.Contains(t, rules[0].Message(), "exempt-sandbox")
				assert.Len(t, rules[0].Exceptions(), 1)
			}
		})
	}
}
