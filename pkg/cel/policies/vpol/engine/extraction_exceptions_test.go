package engine

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TestHandle_ExtractionMode_ExemptTemplateNextToPassingTemplate: an exception that exempts one pod
// template must still be reported when another template passes, the same as when the exempted
// template is the only one. A failing template still wins over both.
func TestHandle_ExtractionMode_ExemptTemplateNextToPassingTemplate(t *testing.T) {
	tests := []struct {
		name       string
		images     []string
		wantStatus engineapi.RuleStatus
		wantMsg    string
		wantExcs   int
	}{{
		name:       "exempted template only",
		images:     []string{"legacy/app:latest"},
		wantStatus: engineapi.RuleStatusSkip,
		wantMsg:    "rule is skipped due to policy exception: default/polex",
		wantExcs:   1,
	}, {
		name:       "exempted template before a passing one",
		images:     []string{"legacy/app:latest", "bash:1.0"},
		wantStatus: engineapi.RuleStatusSkip,
		wantMsg:    "rule is skipped due to policy exception: default/polex",
		wantExcs:   1,
	}, {
		name:       "exempted template after a passing one",
		images:     []string{"bash:1.0", "legacy/app:latest"},
		wantStatus: engineapi.RuleStatusSkip,
		wantMsg:    "rule is skipped due to policy exception: default/polex",
		wantExcs:   1,
	}, {
		name:       "a failing template is not hidden by an exempted one",
		images:     []string{"legacy/app:latest", "bash:1.0", "nginx:latest"},
		wantStatus: engineapi.RuleStatusFail,
		wantMsg:    "CEL expression validation failed at index 0 (pod template at spec.replicatedJobs[2].template.spec.template)",
	}, {
		name:       "no exempted template",
		images:     []string{"bash:1.0", "nginx:1.0"},
		wantStatus: engineapi.RuleStatusPass,
		wantMsg:    "success",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := buildDisallowLatestTagPolicy()
			polex := buildException("default", "polex", policy.GetName())
			polex.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{
				Name:       "legacy",
				Expression: "object.spec.containers.exists(c, c.image.startsWith('legacy/'))",
			}}
			provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.ValidatingPolicyLike{policy}, []*policiesv1beta1.PolicyException{polex})
			require.NoError(t, err)
			eng := NewEngine(provider, func(string) *corev1.Namespace { return nil }, matching.NewMatcher())
			req := celengine.Request(
				nil,
				schema.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
				schema.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
				"",
				"multi-template-jobset",
				"default",
				admissionv1.Create,
				authenticationv1.UserInfo{},
				jobSetWithImages(tt.images...),
				nil,
				false,
				nil,
			)
			resp, err := eng.Handle(context.Background(), req, nil)
			require.NoError(t, err)
			var rules []engineapi.RuleResponse
			for _, p := range resp.Policies {
				rules = append(rules, p.Rules...)
			}
			require.Len(t, rules, 1)
			assert.Equal(t, tt.wantStatus, rules[0].Status())
			assert.Equal(t, tt.wantMsg, rules[0].Message())
			assert.Len(t, rules[0].Exceptions(), tt.wantExcs)
		})
	}
}
