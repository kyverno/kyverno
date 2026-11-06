package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	clicontext "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/context"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/evaluator"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/exception"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DefaultMaxBodyBytes caps a request body.
const DefaultMaxBodyBytes int64 = 10 << 20

// evaluateFunc is the evaluation entrypoint; a variable so tests can inject failures and panics.
type evaluateFunc func(context.Context, *evaluator.PolicySet, evaluator.Input, evaluator.Options) (*evaluator.Result, error)

// Config configures a Server.
type Config struct {
	// Modes restricts which modes can be evaluated; empty enables all of them.
	Modes []evaluator.Mode
	// AllowRequestPolicies lets POST /v1alpha1/evaluate carry its own policies, which are then
	// evaluated instead of the server's policy set.
	AllowRequestPolicies bool
}

// Server serves the v1alpha1 evaluation API.
//
// Evaluations are serialized: the CLI processors behind evaluator.Evaluate rely on
// process-global state (the CRD processor, the CEL library context), so running two at once could
// leak one request's state into another. Throughput is deliberately limited; kyverno serve is a
// local development and testing tool.
type Server struct {
	// set is the policy set loaded at startup; nil when the server only evaluates request policies.
	set                  *evaluator.PolicySet
	digest               string
	version              string
	modes                []evaluator.Mode
	allowRequestPolicies bool
	maxBodyBytes         int64
	evaluate             evaluateFunc

	mu sync.Mutex
}

// NewServer builds a server for set. set may be nil only when cfg.AllowRequestPolicies is true.
func NewServer(set *evaluator.PolicySet, version string, cfg Config) (*Server, error) {
	s := &Server{
		version:              version,
		modes:                cfg.Modes,
		allowRequestPolicies: cfg.AllowRequestPolicies,
		maxBodyBytes:         DefaultMaxBodyBytes,
		evaluate:             evaluator.Evaluate,
	}
	if len(s.modes) == 0 {
		s.modes = slices.Clone(evaluator.Modes)
	}
	if set == nil || set.PolicyCount() == 0 {
		if !cfg.AllowRequestPolicies {
			return nil, errors.New("policy set is empty")
		}
		return s, nil
	}
	digest, err := set.Digest()
	if err != nil {
		return nil, err
	}
	s.set, s.digest = set, digest
	return s, nil
}

// Handler returns the HTTP handler for the v1alpha1 API.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1alpha1/evaluate", s.handleEvaluateRequest)
	mux.HandleFunc("POST /v1alpha1/evaluate/{mode}", s.handleEvaluate)
	mux.HandleFunc("GET /v1alpha1/policies", s.handlePolicies)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /readyz", s.handleHealth)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeStatus(w, http.StatusNotFound, fmt.Sprintf("no route for %s %s", r.Method, r.URL.Path))
	})
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, Status{APIVersion: APIVersion, Kind: "Status", Status: "ok", Code: http.StatusOK})
}

func (s *Server) handlePolicies(w http.ResponseWriter, _ *http.Request) {
	list := PolicyList{
		APIVersion:      APIVersion,
		Kind:            "PolicyList",
		KyvernoVersion:  s.version,
		PolicySetDigest: s.digest,
		Modes:           s.modes,
		Policies:        []evaluator.PolicyRef{},
	}
	if s.set != nil {
		list.Policies = s.set.Policies()
	}
	writeJSON(w, http.StatusOK, list)
}

// handleEvaluate serves POST /v1alpha1/evaluate/{mode}: the raw payload is the body, and the
// server's policy set is used.
func (s *Server) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	mode, err := evaluator.ParseMode(r.PathValue("mode"))
	if err != nil {
		writeStatus(w, http.StatusNotFound, err.Error())
		return
	}
	if !slices.Contains(s.modes, mode) {
		writeStatus(w, http.StatusNotFound, fmt.Sprintf("mode %s is not enabled on this server (enabled: %v)", mode, s.modes))
		return
	}
	if s.set == nil {
		writeStatus(w, http.StatusBadRequest, "the server has no policies loaded; send them with POST /v1alpha1/evaluate")
		return
	}
	body, ok := s.readBody(w, r)
	if !ok {
		return
	}
	in, err := evaluator.DecodeInput(mode, body)
	if err != nil {
		writeStatus(w, http.StatusBadRequest, err.Error())
		return
	}
	s.respond(w, r.Context(), mode, s.set, s.digest, in, nil, "")
}

// handleEvaluateRequest serves POST /v1alpha1/evaluate: the body is an EvaluationRequest carrying
// the payload, its context and, optionally, the policies.
func (s *Server) handleEvaluateRequest(w http.ResponseWriter, r *http.Request) {
	body, ok := s.readBody(w, r)
	if !ok {
		return
	}
	var req EvaluationRequest
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeStatus(w, http.StatusBadRequest, fmt.Sprintf("failed to decode EvaluationRequest: %v", err))
		return
	}
	if (req.APIVersion != "" && req.APIVersion != APIVersion) || (req.Kind != "" && req.Kind != "EvaluationRequest") {
		writeStatus(w, http.StatusBadRequest, fmt.Sprintf("unsupported apiVersion/kind %q/%q, want %q/%q", req.APIVersion, req.Kind, APIVersion, "EvaluationRequest"))
		return
	}
	mode, err := evaluator.ParseMode(string(req.Mode))
	if err != nil {
		writeStatus(w, http.StatusBadRequest, err.Error())
		return
	}
	if !slices.Contains(s.modes, mode) {
		writeStatus(w, http.StatusBadRequest, fmt.Sprintf("mode %s is not enabled on this server (enabled: %v)", mode, s.modes))
		return
	}
	in, err := evaluator.DecodeInput(mode, []byte(req.Payload))
	if err != nil {
		writeStatus(w, http.StatusBadRequest, "payload: "+err.Error())
		return
	}
	operation, err := processor.NormalizeOperation(strings.ToUpper(strings.TrimSpace(req.Operation)))
	if err != nil {
		writeStatus(w, http.StatusBadRequest, err.Error())
		return
	}
	in.Operation = operation

	set, digest := s.set, s.digest
	if strings.TrimSpace(req.Policies) != "" {
		if !s.allowRequestPolicies {
			writeStatus(w, http.StatusForbidden, "request policies are disabled; start kyverno serve with --allow-request-policies")
			return
		}
		set, err = evaluator.LoadStrictBytes([]byte(req.Policies))
		if err != nil {
			writeStatus(w, http.StatusBadRequest, "policies: "+err.Error())
			return
		}
		if digest, err = set.Digest(); err != nil {
			writeStatus(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else if set == nil {
		writeStatus(w, http.StatusBadRequest, "the server has no policies loaded and the request has none")
		return
	}

	if in.ParameterResources, err = decodeResources("parameterResources", req.ParameterResources); err != nil {
		writeStatus(w, http.StatusBadRequest, err.Error())
		return
	}
	clusterResources, err := decodeResources("clusterResources", req.ClusterResources)
	if err != nil {
		writeStatus(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Exceptions) != "" {
		objects, err := decodeResources("exceptions", req.Exceptions)
		if err != nil {
			writeStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		results, err := exception.SelectFrom(objects)
		if err != nil {
			writeStatus(w, http.StatusBadRequest, "exceptions: "+err.Error())
			return
		}
		if len(results.CELExceptions) != len(objects) {
			writeStatus(w, http.StatusBadRequest, "exceptions: every document must be a CEL PolicyException (policies.kyverno.io)")
			return
		}
		in.Exceptions = results.CELExceptions
	}
	contextPath := ""
	if strings.TrimSpace(req.Context) != "" {
		path, cleanup, err := writeContext(req.Context)
		if err != nil {
			writeStatus(w, http.StatusBadRequest, "context: "+err.Error())
			return
		}
		defer cleanup()
		contextPath = path
	}
	s.respond(w, r.Context(), mode, set, digest, in, clusterResources, contextPath)
}

// writeContext validates a Context document and writes it to a private temp file, because the
// evaluator reads the context the way apply does, from a --context-file path.
func writeContext(content string) (string, func(), error) {
	f, err := os.CreateTemp("", "kyverno-serve-context-*.yaml")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.Remove(f.Name()) }
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		cleanup()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	ctx, err := clicontext.Load(nil, f.Name())
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if ctx.Kind != "Context" {
		cleanup()
		return "", nil, fmt.Errorf("kind must be Context, got %q", ctx.Kind)
	}
	return f.Name(), cleanup, nil
}

func decodeResources(field, content string) ([]*unstructured.Unstructured, error) {
	if strings.TrimSpace(content) == "" {
		return nil, nil
	}
	objects, err := resource.GetUnstructuredResources([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	return evaluator.ProcessResources(objects), nil
}

func (s *Server) readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, s.maxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeStatus(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("request body exceeds %d bytes", tooLarge.Limit))
			return nil, false
		}
		writeStatus(w, http.StatusBadRequest, fmt.Sprintf("failed to read request body: %v", err))
		return nil, false
	}
	return body, true
}

func (s *Server) respond(w http.ResponseWriter, ctx context.Context, mode evaluator.Mode, set *evaluator.PolicySet, digest string, in evaluator.Input, clusterResources []*unstructured.Unstructured, contextPath string) {
	resp, err := s.evaluateWith(ctx, mode, set, digest, in, clusterResources, contextPath)
	if err != nil {
		log.Log.V(2).Info("evaluation failed", "mode", mode, "error", err)
		writeStatus(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// Evaluate runs one decoded request against the server's policy set.
func (s *Server) Evaluate(ctx context.Context, mode evaluator.Mode, in evaluator.Input) (*EvaluationResponse, error) {
	if s.set == nil {
		return nil, errors.New("the server has no policies loaded")
	}
	return s.evaluateWith(ctx, mode, s.set, s.digest, in, nil, "")
}

// evaluateWith builds fresh per-request state (store, variables, in-memory cluster seeded with the
// payload, cluster and parameter resources) and holds the server lock for the evaluation.
func (s *Server) evaluateWith(ctx context.Context, mode evaluator.Mode, set *evaluator.PolicySet, digest string, in evaluator.Input, clusterResources []*unstructured.Unstructured, contextPath string) (resp *EvaluationResponse, err error) {
	st := &store.Store{}
	st.SetLocal(true)
	if len(in.Resources)+len(clusterResources)+len(in.ParameterResources) > 0 {
		client, err := evaluator.NewFakeClient(in.Resources, clusterResources, in.ParameterResources)
		if err != nil {
			return nil, fmt.Errorf("failed to create local resource client: %w", err)
		}
		in.Client = client
		st.AllowApiCall(true)
	}
	vars, err := variables.New(io.Discard, nil, "", "", nil)
	if err != nil {
		return nil, err
	}
	in.Store = st
	in.Variables = vars
	in.Exceptions = append(slices.Clone(set.PolicyCelExceptions), in.Exceptions...)

	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		if p := recover(); p != nil {
			log.Log.Error(fmt.Errorf("%v", p), "evaluation panicked", "stack", string(debug.Stack()))
			resp, err = nil, fmt.Errorf("evaluation panicked: %v", p)
		}
	}()
	result, err := s.evaluate(ctx, set, in, evaluator.Options{Out: io.Discard, ContextPath: contextPath})
	if err != nil {
		return nil, err
	}
	return newEvaluationResponse(mode, s.version, digest, result.Responses), nil
}

// ListenAndServe serves on address until ctx is cancelled, then shuts down gracefully.
// ready, if non-nil, receives the bound address once the listener is accepting connections.
func (s *Server) ListenAndServe(ctx context.Context, address string, ready func(string)) error {
	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       time.Minute,
		WriteTimeout:      5 * time.Minute,
	}
	ln, err := listen(ctx, address)
	if err != nil {
		return err
	}
	if ready != nil {
		ready(ln.Addr().String())
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func writeStatus(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, Status{APIVersion: APIVersion, Kind: "Status", Status: "Failure", Code: code, Message: message})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
