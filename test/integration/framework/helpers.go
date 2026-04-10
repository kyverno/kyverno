package framework

import (
	"context"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	event "github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// MockEventGen captures events for test assertions.
type MockEventGen struct {
	Events []event.Info
}

func (m *MockEventGen) Add(infoList ...event.Info) {
	m.Events = append(m.Events, infoList...)
}

// PodAdmissionRequest builds a handlers.AdmissionRequest for a Pod CREATE operation.
func PodAdmissionRequest(name, namespace string, raw []byte) handlers.AdmissionRequest {
	return handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID(uuid.New().String()),
			Operation: admissionv1.Create,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Name:      name,
			Namespace: namespace,
			Object:    runtime.RawExtension{Raw: raw},
			UserInfo:  authenticationv1.UserInfo{Username: "test-user"},
		},
	}
}

// ContextWithPolicies injects policy names into the context using httprouter params,
// matching how the real webhook server passes policy names to handlers.
func ContextWithPolicies(ctx context.Context, policyNames ...string) context.Context {
	return context.WithValue(ctx, httprouter.ParamsKey, httprouter.Params{
		{Key: "policies", Value: "/" + strings.Join(policyNames, "/")},
	})
}

// MockURGenerator captures UpdateRequest specs created by the gpol handler.
type MockURGenerator struct {
	mu    sync.Mutex
	Specs []kyvernov2.UpdateRequestSpec
}

func (m *MockURGenerator) Apply(_ context.Context, spec kyvernov2.UpdateRequestSpec) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Specs = append(m.Specs, spec)
	return nil
}

// GetSpecs returns a snapshot of captured UpdateRequestSpecs (thread-safe).
func (m *MockURGenerator) GetSpecs() []kyvernov2.UpdateRequestSpec {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]kyvernov2.UpdateRequestSpec, len(m.Specs))
	copy(result, m.Specs)
	return result
}

// ProcessingURGenerator extends MockURGenerator by running each captured URSpec
// through a processor function before marking it as captured. This simulates
// the background controller: handler fires UR → processor creates downstream
// resources → spec appears in GetSpecs() only after processing completes.
type ProcessingURGenerator struct {
	mu        sync.Mutex
	Specs     []kyvernov2.UpdateRequestSpec
	processor func(kyvernov2.UpdateRequestSpec) error
	errMu     sync.Mutex
	errors    []error
}

func NewProcessingURGenerator(processor func(kyvernov2.UpdateRequestSpec) error) *ProcessingURGenerator {
	return &ProcessingURGenerator{
		processor: processor,
	}
}

func (p *ProcessingURGenerator) Apply(_ context.Context, spec kyvernov2.UpdateRequestSpec) error {
	// Process first — creates downstream resources in envtest.
	if err := p.processor(spec); err != nil {
		p.errMu.Lock()
		p.errors = append(p.errors, err)
		p.errMu.Unlock()
	}
	// Then record the spec. Tests polling GetSpecs() see it only after
	// processing is done, so downstream resources are guaranteed to exist.
	p.mu.Lock()
	p.Specs = append(p.Specs, spec)
	p.mu.Unlock()
	return nil
}

// GetSpecs returns a snapshot of captured UpdateRequestSpecs (thread-safe).
func (p *ProcessingURGenerator) GetSpecs() []kyvernov2.UpdateRequestSpec {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]kyvernov2.UpdateRequestSpec, len(p.Specs))
	copy(result, p.Specs)
	return result
}

// ProcessingErrors returns any errors from UR processing.
func (p *ProcessingURGenerator) ProcessingErrors() []error {
	p.errMu.Lock()
	defer p.errMu.Unlock()
	result := make([]error, len(p.errors))
	copy(result, p.errors)
	return result
}

// PodMatchRules returns MatchResources that match pods on CREATE operations.
func PodMatchRules() *admissionregistrationv1.MatchResources {
	return PodMatchRulesWithOps(admissionregistrationv1.Create)
}

// PodMatchRulesWithOps returns MatchResources that match pods on the given operations.
func PodMatchRulesWithOps(ops ...admissionregistrationv1.OperationType) *admissionregistrationv1.MatchResources {
	return &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: ops,
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods"},
				},
			},
		}},
	}
}

// PodAdmissionRequestWithOp builds a handlers.AdmissionRequest for a Pod with the given operation.
// For DELETE, the resource is placed in OldObject (matching real K8s behavior).
func PodAdmissionRequestWithOp(name, namespace string, op admissionv1.Operation, raw []byte) handlers.AdmissionRequest {
	req := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID(uuid.New().String()),
			Operation: op,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Name:      name,
			Namespace: namespace,
			UserInfo:  authenticationv1.UserInfo{Username: "test-user"},
		},
	}
	if op == admissionv1.Delete {
		req.OldObject = runtime.RawExtension{Raw: raw}
	} else {
		req.Object = runtime.RawExtension{Raw: raw}
	}
	return req
}
