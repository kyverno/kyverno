package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"slices"
	"sync"
	"time"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/evaluator"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
)

// DefaultMaxBodyBytes caps a request body.
const DefaultMaxBodyBytes int64 = 10 << 20

// evaluateFunc is the evaluation entrypoint; a variable so tests can inject failures and panics.
type evaluateFunc func(context.Context, *evaluator.PolicySet, evaluator.Input, evaluator.Options) (*evaluator.Result, error)

// Server serves the v1alpha1 evaluation API over a fixed, pre-loaded policy set.
//
// Evaluations are serialized: the CLI processors behind evaluator.Evaluate rely on
// process-global state (the CRD processor, the CEL library context), so running two at once could
// leak one request's state into another. Throughput is deliberately limited; kyverno serve is a
// local development and testing tool.
type Server struct {
	set          *evaluator.PolicySet
	digest       string
	version      string
	modes        []evaluator.Mode
	maxBodyBytes int64
	evaluate     evaluateFunc

	mu sync.Mutex
}

// NewServer builds a server for set. modes restricts which evaluate endpoints are enabled; empty
// enables all of them.
func NewServer(set *evaluator.PolicySet, version string, modes []evaluator.Mode) (*Server, error) {
	if set == nil || set.PolicyCount() == 0 {
		return nil, errors.New("policy set is empty")
	}
	digest, err := set.Digest()
	if err != nil {
		return nil, err
	}
	if len(modes) == 0 {
		modes = slices.Clone(evaluator.Modes)
	}
	return &Server{
		set:          set,
		digest:       digest,
		version:      version,
		modes:        modes,
		maxBodyBytes: DefaultMaxBodyBytes,
		evaluate:     evaluator.Evaluate,
	}, nil
}

// Handler returns the HTTP handler for the v1alpha1 API.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
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
	writeJSON(w, http.StatusOK, PolicyList{
		APIVersion:      APIVersion,
		Kind:            "PolicyList",
		KyvernoVersion:  s.version,
		PolicySetDigest: s.digest,
		Modes:           s.modes,
		Policies:        s.set.Policies(),
	})
}

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
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, s.maxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeStatus(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("request body exceeds %d bytes", tooLarge.Limit))
			return
		}
		writeStatus(w, http.StatusBadRequest, fmt.Sprintf("failed to read request body: %v", err))
		return
	}
	in, err := evaluator.DecodeInput(mode, body)
	if err != nil {
		writeStatus(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.Evaluate(r.Context(), mode, in)
	if err != nil {
		log.Log.V(2).Info("evaluation failed", "mode", mode, "error", err)
		writeStatus(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// Evaluate runs one decoded request against the server's policy set. It builds fresh per-request
// state (store, in-memory cluster client) and holds the server lock for the evaluation.
func (s *Server) Evaluate(ctx context.Context, mode evaluator.Mode, in evaluator.Input) (resp *EvaluationResponse, err error) {
	st := &store.Store{}
	st.SetLocal(true)
	if len(in.Resources) > 0 {
		client, err := evaluator.NewFakeClient(in.Resources, nil, nil)
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
	in.Exceptions = s.set.PolicyCelExceptions

	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		if p := recover(); p != nil {
			log.Log.Error(fmt.Errorf("%v", p), "evaluation panicked", "stack", string(debug.Stack()))
			resp, err = nil, fmt.Errorf("evaluation panicked: %v", p)
		}
	}()
	result, err := s.evaluate(ctx, s.set, in, evaluator.Options{Out: io.Discard})
	if err != nil {
		return nil, err
	}
	return newEvaluationResponse(mode, s.version, s.digest, result.Responses), nil
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
