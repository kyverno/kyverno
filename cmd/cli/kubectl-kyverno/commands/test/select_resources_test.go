package test

import (
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
)

func TestKindMatches(t *testing.T) {
	t.Parallel()
	key := strings.Split("apps/v1,StatefulSet,default,good-pinned-tag", ",")
	tests := []struct {
		name      string
		expected  string
		nameParts []string
		want      bool
	}{
		{name: "same kind", expected: "StatefulSet", nameParts: key, want: true},
		{name: "different kind", expected: "Deployment", nameParts: key, want: false},
		{name: "empty kind matches any kind", expected: "", nameParts: key, want: true},
		{name: "short key", expected: "StatefulSet", nameParts: []string{"good-pinned-tag"}, want: false},
		{name: "three fields", expected: "StatefulSet", nameParts: []string{"StatefulSet", "default", "good-pinned-tag"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, kindMatches(tt.expected, tt.nameParts))
		})
	}
}

func TestSelectResourcesFiltersByKind(t *testing.T) {
	t.Parallel()
	deploymentKey := "apps/v1,Deployment,default,good-pinned-tag"
	statefulSetKey := "apps/v1,StatefulSet,default,good-pinned-tag"
	commaKey := "rbac.authorization.k8s.io/v1,ClusterRole,,team,payments-reader"
	trigger := map[string][]engineapi.EngineResponse{
		deploymentKey:  nil,
		statefulSetKey: nil,
		commaKey:       nil,
	}
	spec := func(kind string) []v1alpha1.TestResourceSpec {
		return []v1alpha1.TestResourceSpec{{Group: "apps", Version: "v1", Kind: kind, Namespace: "default", Name: "good-pinned-tag"}}
	}
	tests := []struct {
		name   string
		result v1alpha1.TestResult
		want   []string
	}{
		{
			name: "resources with kind Deployment",
			result: v1alpha1.TestResult{
				TestResultBase: v1alpha1.TestResultBase{Kind: "Deployment"},
				TestResultData: v1alpha1.TestResultData{Resources: []string{"good-pinned-tag"}},
			},
			want: []string{deploymentKey},
		},
		{
			name: "resources with kind StatefulSet",
			result: v1alpha1.TestResult{
				TestResultBase: v1alpha1.TestResultBase{Kind: "StatefulSet"},
				TestResultData: v1alpha1.TestResultData{Resources: []string{"good-pinned-tag"}},
			},
			want: []string{statefulSetKey},
		},
		{
			// resourceSpecs are only read when resources is set, so an empty list is used
			name: "resourceSpecs with kind Deployment",
			result: v1alpha1.TestResult{
				TestResultData: v1alpha1.TestResultData{Resources: []string{}, ResourceSpecs: spec("Deployment")},
			},
			want: []string{deploymentKey},
		},
		{
			name: "resourceSpecs with kind StatefulSet",
			result: v1alpha1.TestResult{
				TestResultData: v1alpha1.TestResultData{Resources: []string{}, ResourceSpecs: spec("StatefulSet")},
			},
			want: []string{statefulSetKey},
		},
		{
			name: "resources with a comma in the name",
			result: v1alpha1.TestResult{
				TestResultBase: v1alpha1.TestResultBase{Kind: "ClusterRole"},
				TestResultData: v1alpha1.TestResultData{Resources: []string{"team,payments-reader"}},
			},
			want: []string{commaKey},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, selectResources(tt.result, nil, trigger))
		})
	}
}
