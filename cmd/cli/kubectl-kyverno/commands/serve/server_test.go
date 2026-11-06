package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/evaluator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtures = "../../../../../test/cli/serve"

var summaryRegexp = regexp.MustCompile(`pass: (\d+), fail: (\d+), warn: (\d+), error: (\d+), skip: (\d+)`)

func summaryFromMatch(m []string) Summary {
	atoi := func(s string) int { n, _ := strconv.Atoi(s); return n }
	return Summary{Pass: atoi(m[1]), Fail: atoi(m[2]), Warn: atoi(m[3]), Error: atoi(m[4]), Skip: atoi(m[5])}
}

func fixture(parts ...string) string {
	return filepath.Join(append([]string{fixtures}, parts...)...)
}

func allPolicyDirs() []string {
	return []string{fixture("policies", "kubernetes"), fixture("policies", "json"), fixture("policies", "http"), fixture("policies", "envoy")}
}

func newTestServer(t *testing.T, modes []evaluator.Mode, dirs ...string) *Server {
	t.Helper()
	s, err := newServerFromPaths(dirs, Config{Modes: modes})
	require.NoError(t, err)
	return s
}

func do(t *testing.T, h http.Handler, method, path string, body []byte) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

func evaluate(t *testing.T, h http.Handler, mode evaluator.Mode, payload string) (int, *EvaluationResponse) {
	t.Helper()
	body, err := os.ReadFile(payload)
	require.NoError(t, err)
	code, raw := do(t, h, http.MethodPost, "/v1alpha1/evaluate/"+string(mode), body)
	var resp EvaluationResponse
	if code == http.StatusOK {
		require.NoError(t, json.Unmarshal(raw, &resp), string(raw))
	}
	return code, &resp
}

//nolint:paralleltest // newServerFromPaths sets the process-global CRD processor
func TestEvaluateModes(t *testing.T) {
	s := newTestServer(t, nil, allPolicyDirs()...)
	h := s.Handler()
	tests := []struct {
		mode     evaluator.Mode
		payload  string
		decision Decision
		summary  Summary
		failed   []string
	}{
		{evaluator.ModeKubernetes, fixture("payloads", "kubernetes", "deployment-pass.yaml"), DecisionAllow, Summary{Pass: 2, Skip: 1}, nil},
		{evaluator.ModeKubernetes, fixture("payloads", "kubernetes", "deployment-fail.yaml"), DecisionDeny, Summary{Pass: 1, Fail: 1, Skip: 1}, []string{"check-deployment-labels"}},
		{evaluator.ModeKubernetes, fixture("payloads", "kubernetes", "configmap.yaml"), DecisionNoMatch, Summary{Skip: 1}, nil},
		{evaluator.ModeJSON, fixture("payloads", "json", "dockerfile-pass.json"), DecisionAllow, Summary{Pass: 1}, nil},
		{evaluator.ModeJSON, fixture("payloads", "json", "dockerfile-fail.json"), DecisionDeny, Summary{Fail: 1}, []string{"check-dockerfile-disallow-wget"}},
		{evaluator.ModeHTTP, fixture("payloads", "http", "get.json"), DecisionAllow, Summary{Pass: 1}, nil},
		{evaluator.ModeHTTP, fixture("payloads", "http", "delete.json"), DecisionDeny, Summary{Fail: 1}, []string{"allow-only-get"}},
		{evaluator.ModeEnvoy, fixture("payloads", "envoy", "public.json"), DecisionAllow, Summary{Pass: 1}, nil},
		{evaluator.ModeEnvoy, fixture("payloads", "envoy", "admin.json"), DecisionDeny, Summary{Fail: 1}, []string{"forbid-admin-path"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode)+"/"+filepath.Base(tt.payload), func(t *testing.T) {
			code, resp := evaluate(t, h, tt.mode, tt.payload)
			require.Equal(t, http.StatusOK, code)
			assert.Equal(t, APIVersion, resp.APIVersion)
			assert.Equal(t, "EvaluationResponse", resp.Kind)
			assert.Equal(t, tt.mode, resp.Mode)
			assert.Equal(t, s.digest, resp.PolicySetDigest)
			assert.NotEmpty(t, resp.KyvernoVersion)
			assert.Equal(t, tt.decision, resp.Decision)
			assert.Equal(t, tt.summary, resp.Summary)
			var failed []string
			for _, r := range resp.Results {
				if r.Status == "fail" {
					failed = append(failed, r.Policy.Name)
					assert.NotEmpty(t, r.Message)
				}
			}
			assert.Equal(t, tt.failed, failed)
		})
	}
}

//nolint:paralleltest // newServerFromPaths sets the process-global CRD processor
func TestEvaluateArtifacts(t *testing.T) {
	h := newTestServer(t, nil, fixture("policies", "kubernetes")).Handler()

	code, resp := evaluate(t, h, evaluator.ModeKubernetes, fixture("payloads", "kubernetes", "pod.yaml"))
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, resp.Artifacts)
	require.Len(t, resp.Artifacts.Patched, 1)
	containers := resp.Artifacts.Patched[0]["spec"].(map[string]any)["containers"].([]any)
	assert.Equal(t, "registry.example.com/nginx:1.27", containers[0].(map[string]any)["image"])
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "MutatingPolicy", resp.Results[0].Policy.Kind)
	assert.Equal(t, &ObjectRef{APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "web"}, resp.Results[0].Resource)
	assert.Equal(t, resp.Artifacts.Patched[0], resp.Results[0].PatchedResource)

	code, resp = evaluate(t, h, evaluator.ModeKubernetes, fixture("payloads", "kubernetes", "namespace.yaml"))
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, resp.Artifacts)
	require.Len(t, resp.Artifacts.Generated, 1)
	generated := resp.Artifacts.Generated[0]
	assert.Equal(t, "ConfigMap", generated["kind"])
	assert.Equal(t, "team-a", generated["metadata"].(map[string]any)["namespace"])
	require.Len(t, resp.Results, 1)
	assert.Equal(t, resp.Artifacts.Generated, resp.Results[0].GeneratedResources)
	assert.Equal(t, DecisionAllow, resp.Decision)
}

// TestApplyParity checks that kyverno apply and kyverno serve agree on the same fixtures: both now
// go through evaluator.Evaluate, so their rule counts must match exactly.
//
//nolint:paralleltest // apply and newServerFromPaths set process-global state
func TestApplyParity(t *testing.T) {
	flags := map[evaluator.Mode]string{
		evaluator.ModeKubernetes: "--resource",
		evaluator.ModeJSON:       "--json",
		evaluator.ModeHTTP:       "--http-payload",
		evaluator.ModeEnvoy:      "--envoy-payload",
	}
	type policySet struct {
		name string
		dirs func(dir string) []string
	}
	sets := []policySet{
		{"scoped", func(dir string) []string { return []string{fixture("policies", dir)} }},
		{"all", func(string) []string { return allPolicyDirs() }},
	}
	for _, set := range sets {
		for _, mode := range evaluator.Modes {
			dir := strings.ToLower(string(mode))
			payloads, err := filepath.Glob(fixture("payloads", dir, "*"))
			require.NoError(t, err)
			require.NotEmpty(t, payloads)
			policyDirs := set.dirs(dir)
			s := newTestServer(t, []evaluator.Mode{mode}, policyDirs...)
			for _, payload := range payloads {
				t.Run(set.name+"/"+string(mode)+"/"+filepath.Base(payload), func(t *testing.T) {
					want := applySummary(t, append(slices.Clone(policyDirs), flags[mode], payload)...)

					code, resp := evaluate(t, s.Handler(), mode, payload)
					require.Equal(t, http.StatusOK, code)
					assert.Equal(t, want, resp.Summary)
				})
			}
		}
	}
}

//nolint:paralleltest // newServerFromPaths sets the process-global CRD processor
func TestRequestErrors(t *testing.T) {
	s := newTestServer(t, []evaluator.Mode{evaluator.ModeKubernetes, evaluator.ModeJSON}, allPolicyDirs()...)
	s.maxBodyBytes = 64 << 10
	h := s.Handler()
	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
		code   int
		msg    string
	}{
		{"empty body", http.MethodPost, "/v1alpha1/evaluate/Kubernetes", nil, http.StatusBadRequest, "request body is empty"},
		{"malformed yaml", http.MethodPost, "/v1alpha1/evaluate/Kubernetes", []byte("{not yaml"), http.StatusBadRequest, "failed to decode"},
		{"json array", http.MethodPost, "/v1alpha1/evaluate/JSON", []byte("[1]"), http.StatusBadRequest, "single top-level object"},
		{"unknown mode", http.MethodPost, "/v1alpha1/evaluate/gRPC", []byte("{}"), http.StatusNotFound, "unsupported mode"},
		{"disabled mode", http.MethodPost, "/v1alpha1/evaluate/Envoy", []byte("{}"), http.StatusNotFound, "not enabled"},
		{"too large", http.MethodPost, "/v1alpha1/evaluate/JSON", []byte(`{"a":"` + strings.Repeat("x", 70<<10) + `"}`), http.StatusRequestEntityTooLarge, "exceeds"},
		{"unknown route", http.MethodGet, "/v1/evaluate", nil, http.StatusNotFound, "no route"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, raw := do(t, h, tt.method, tt.path, tt.body)
			assert.Equal(t, tt.code, code, string(raw))
			var st Status
			require.NoError(t, json.Unmarshal(raw, &st), string(raw))
			assert.Equal(t, "Failure", st.Status)
			assert.Equal(t, tt.code, st.Code)
			assert.Contains(t, st.Message, tt.msg)
		})
	}
	t.Run("wrong method", func(t *testing.T) {
		code, _ := do(t, h, http.MethodGet, "/v1alpha1/evaluate/Kubernetes", nil)
		assert.GreaterOrEqual(t, code, 400)
		assert.Less(t, code, 500)
	})
}

//nolint:paralleltest // newServerFromPaths sets the process-global CRD processor
func TestEvaluationFailuresAreNeverAllow(t *testing.T) {
	s := newTestServer(t, nil, allPolicyDirs()...)
	h := s.Handler()
	body, err := os.ReadFile(fixture("payloads", "http", "get.json"))
	require.NoError(t, err)

	s.evaluate = func(context.Context, *evaluator.PolicySet, evaluator.Input, evaluator.Options) (*evaluator.Result, error) {
		return nil, errors.New("boom")
	}
	code, raw := do(t, h, http.MethodPost, "/v1alpha1/evaluate/HTTP", body)
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Contains(t, string(raw), "boom")

	s.evaluate = func(context.Context, *evaluator.PolicySet, evaluator.Input, evaluator.Options) (*evaluator.Result, error) {
		panic("kaboom")
	}
	code, raw = do(t, h, http.MethodPost, "/v1alpha1/evaluate/HTTP", body)
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Contains(t, string(raw), "evaluation panicked: kaboom")

	// the lock was released by the panicking request, so the server keeps serving
	s.evaluate = evaluator.Evaluate
	code, raw = do(t, h, http.MethodPost, "/v1alpha1/evaluate/HTTP", body)
	assert.Equal(t, http.StatusOK, code, string(raw))
}

func TestDecide(t *testing.T) {
	t.Parallel()
	assert.Equal(t, DecisionNoMatch, decide(Summary{}))
	assert.Equal(t, DecisionNoMatch, decide(Summary{Skip: 3}))
	assert.Equal(t, DecisionAllow, decide(Summary{Pass: 1, Skip: 1}))
	assert.Equal(t, DecisionAllow, decide(Summary{Warn: 1}))
	assert.Equal(t, DecisionDeny, decide(Summary{Pass: 4, Fail: 1}))
	assert.Equal(t, DecisionError, decide(Summary{Pass: 1, Fail: 1, Error: 1}))
}

// TestConcurrentRequestsDoNotLeakState fires interleaved passing and failing requests at the
// server in parallel. Each response must describe only its own request; run with -race.
//
//nolint:paralleltest // newServerFromPaths sets the process-global CRD processor
func TestConcurrentRequestsDoNotLeakState(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t, nil, allPolicyDirs()...).Handler())
	defer srv.Close()
	cases := []struct {
		mode     evaluator.Mode
		payload  string
		decision Decision
		resource string
	}{
		{evaluator.ModeKubernetes, fixture("payloads", "kubernetes", "deployment-pass.yaml"), DecisionAllow, "good-deployment"},
		{evaluator.ModeKubernetes, fixture("payloads", "kubernetes", "deployment-fail.yaml"), DecisionDeny, "bad-deployment"},
		{evaluator.ModeJSON, fixture("payloads", "json", "dockerfile-fail.json"), DecisionDeny, ""},
		{evaluator.ModeEnvoy, fixture("payloads", "envoy", "public.json"), DecisionAllow, ""},
		{evaluator.ModeHTTP, fixture("payloads", "http", "delete.json"), DecisionDeny, ""},
	}
	bodies := make([][]byte, len(cases))
	for i, c := range cases {
		b, err := os.ReadFile(c.payload)
		require.NoError(t, err)
		bodies[i] = b
	}
	var wg sync.WaitGroup
	errs := make(chan error, 100)
	for i := range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := cases[i%len(cases)]
			res, err := http.Post(srv.URL+"/v1alpha1/evaluate/"+string(c.mode), "application/yaml", bytes.NewReader(bodies[i%len(cases)]))
			if err != nil {
				errs <- err
				return
			}
			defer res.Body.Close()
			var resp EvaluationResponse
			if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
				errs <- err
				return
			}
			if resp.Decision != c.decision || resp.Mode != c.mode {
				errs <- fmt.Errorf("request %d (%s): got decision %s mode %s, want %s", i, filepath.Base(c.payload), resp.Decision, resp.Mode, c.decision)
				return
			}
			for _, r := range resp.Results {
				if c.resource != "" && (r.Resource == nil || r.Resource.Name != c.resource) {
					errs <- fmt.Errorf("request %d (%s): result for foreign resource %+v", i, filepath.Base(c.payload), r.Resource)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

//nolint:paralleltest // newServerFromPaths sets the process-global CRD processor
func TestPoliciesAndHealth(t *testing.T) {
	s := newTestServer(t, []evaluator.Mode{evaluator.ModeHTTP}, allPolicyDirs()...)
	h := s.Handler()
	for _, path := range []string{"/healthz", "/readyz"} {
		code, raw := do(t, h, http.MethodGet, path, nil)
		assert.Equal(t, http.StatusOK, code)
		assert.Contains(t, string(raw), `"status": "ok"`)
	}
	code, raw := do(t, h, http.MethodGet, "/v1alpha1/policies", nil)
	require.Equal(t, http.StatusOK, code)
	var list PolicyList
	require.NoError(t, json.Unmarshal(raw, &list))
	assert.Equal(t, "PolicyList", list.Kind)
	assert.Equal(t, s.digest, list.PolicySetDigest)
	assert.Equal(t, []evaluator.Mode{evaluator.ModeHTTP}, list.Modes)
	assert.Len(t, list.Policies, 6)
}

//nolint:paralleltest // newServerFromPaths sets the process-global CRD processor
func TestStartupFailures(t *testing.T) {
	_, err := newServerFromPaths([]string{fixture("policies", "invalid")}, Config{})
	assert.ErrorContains(t, err, "failed to load policies")
	_, err = newServerFromPaths([]string{t.TempDir()}, Config{})
	assert.ErrorContains(t, err, "no policies found")
	_, err = parseModes([]string{"Kubernetes", "SOAP"})
	assert.ErrorContains(t, err, "unsupported mode")
	modes, err := parseModes([]string{"json", "JSON", "envoy"})
	require.NoError(t, err)
	assert.Equal(t, []evaluator.Mode{evaluator.ModeJSON, evaluator.ModeEnvoy}, modes)
	_, err = NewServer(&evaluator.PolicySet{}, "v0", Config{})
	assert.Error(t, err)
	_, err = NewServer(nil, "v0", Config{AllowRequestPolicies: true})
	assert.NoError(t, err)

	cmd := Command()
	cmd.SetArgs([]string{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	assert.ErrorContains(t, cmd.Execute(), "--policies is required unless --allow-request-policies is set")

	cmd = Command()
	cmd.SetArgs([]string{"--policies", fixture("policies", "invalid")})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	assert.ErrorContains(t, cmd.Execute(), "failed to load policies")
}

//nolint:paralleltest // newServerFromPaths sets the process-global CRD processor
func TestListenAndServe(t *testing.T) {
	s := newTestServer(t, nil, fixture("policies", "envoy"))
	ctx, cancel := context.WithCancel(context.Background())
	addrCh := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- s.ListenAndServe(ctx, "127.0.0.1:0", func(a string) { addrCh <- a }) }()
	addr := <-addrCh
	res, err := http.Get("http://" + addr + "/healthz")
	require.NoError(t, err)
	res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	cancel()
	assert.NoError(t, <-done)

	_, err = listen(context.Background(), "256.0.0.1:1")
	assert.Error(t, err)
	assert.True(t, isLoopback("localhost"))
	assert.True(t, isLoopback("::1"))
	assert.False(t, isLoopback("0.0.0.0"))
}
