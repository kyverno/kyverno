package generate

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/v3/assert"
	authorizationv1 "k8s.io/api/authorization/v1"
)

func TestGenerateRejectsUnresolvedAuthorizationTargets(t *testing.T) {
	t.Parallel()

	const staticError = "apiVersion and kind must be static"
	tests := []struct {
		name       string
		apiVersion string
		kind       string
		cloneKind  string
		wantError  string
	}{
		{name: "absolute kind reference", kind: "$(/name)", wantError: staticError},
		{name: "relative kind reference", kind: "$(./../data/kind)", wantError: staticError},
		{name: "embedded kind reference", kind: "Config$(/name)", wantError: staticError},
		{name: "absolute apiVersion reference", apiVersion: "$(/name)", kind: "ConfigMap", wantError: staticError},
		{name: "relative apiVersion reference", apiVersion: "$(./../data/apiVersion)", kind: "ConfigMap", wantError: staticError},
		{name: "subresource reference", kind: "ConfigMap/$(subresource)", wantError: staticError},
		{name: "subresource variable", kind: "ConfigMap/{{request.subResource}}", wantError: staticError},
		{name: "cloneList absolute reference", cloneKind: "$(/name)", wantError: staticError},
		{name: "cloneList relative reference", cloneKind: "$(./../../data/kind)", wantError: staticError},
		{name: "cloneList subresource reference", cloneKind: "v1/ConfigMap/$(/name)", wantError: staticError},
		{name: "unknown kind", kind: "UnknownTargetKind", wantError: "failed to get GVR"},
		{name: "unknown cloneList kind", cloneKind: "v1/UnknownTargetKind", wantError: "failed to get GVR"},
	}
	for _, test := range tests {
		for _, foreach := range []bool{false, true} {
			name := "direct/" + test.name
			if foreach {
				name = "foreach/" + test.name
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				pattern := generatePattern(test.kind, "target")
				if test.apiVersion != "" {
					pattern.APIVersion = test.apiVersion
				}
				if test.cloneKind != "" {
					pattern = kyvernov1.GeneratePattern{
						ResourceSpec: kyvernov1.ResourceSpec{Namespace: "target"},
						CloneList:    kyvernov1.CloneList{Kinds: []string{test.cloneKind}},
					}
				}
				generation := kyvernov1.Generation{GeneratePattern: pattern}
				if foreach {
					generation = kyvernov1.Generation{
						ForEachGeneration: []kyvernov1.ForEachGeneration{{GeneratePattern: pattern}},
					}
				}
				client, reviews := newGenerateAuthClient(t, func(authorizationv1.SubjectAccessReviewSpec) bool { return true })
				checker := NewGenerateFactoryWithGroups(client, &kyvernov1.Rule{Generation: &generation}, "alice", nil, "", logr.Discard())
				_, _, err := checker.Validate(context.Background(), nil)
				assert.ErrorContains(t, err, test.wantError)
				assert.Equal(t, len(*reviews), 0, "unresolved target kinds must not reach authorization")

				warnings, path, err := NewFakeGenerate(generation).Validate(context.Background(), nil)
				assert.NilError(t, err)
				assert.Equal(t, path, "")
				assert.Equal(t, len(warnings), 0)
			})
		}
	}
}

func TestGenerateReferenceNamespaceRequiresClusterWideAuthorization(t *testing.T) {
	t.Parallel()

	for _, namespace := range []string{"$(/name)", "$(./../name)", "prefix-$(/name)"} {
		for _, mode := range []string{"direct", "foreach", "cloneList"} {
			for _, clusterWide := range []bool{false, true} {
				authorization := "role only"
				if clusterWide {
					authorization = "cluster wide"
				}
				t.Run(mode+"/"+namespace+"/"+authorization, func(t *testing.T) {
					t.Parallel()
					pattern := generatePattern("ConfigMap", namespace)
					generation := kyvernov1.Generation{GeneratePattern: pattern}
					switch mode {
					case "foreach":
						generation = kyvernov1.Generation{
							ForEachGeneration: []kyvernov1.ForEachGeneration{{GeneratePattern: pattern}},
						}
					case "cloneList":
						generation = kyvernov1.Generation{GeneratePattern: kyvernov1.GeneratePattern{
							ResourceSpec: kyvernov1.ResourceSpec{Namespace: namespace},
							CloneList:    kyvernov1.CloneList{Kinds: []string{"v1/ConfigMap"}},
						}}
					}
					client, reviews := newGenerateAuthClient(t, func(spec authorizationv1.SubjectAccessReviewSpec) bool {
						return spec.ResourceAttributes.Namespace == "target" ||
							(clusterWide && spec.ResourceAttributes.Namespace == "")
					})
					checker := NewGenerateFactoryWithGroups(client, &kyvernov1.Rule{Generation: &generation}, "alice", nil, "", logr.Discard())
					_, _, err := checker.Validate(context.Background(), nil)
					if clusterWide {
						assert.NilError(t, err)
					} else {
						assert.ErrorContains(t, err, "requires permissions")
					}
					assert.Equal(t, len(*reviews), 2)
					for _, review := range *reviews {
						assert.Equal(t, review.ResourceAttributes.Namespace, "")
					}
				})
			}
		}
	}
}

func TestGenerateStaticTargetsAllowDynamicNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"{{request.object.metadata.name}}", "$(./../data/metadata/name)"} {
		for _, foreach := range []bool{false, true} {
			mode := "direct"
			if foreach {
				mode = "foreach"
			}
			t.Run(mode+"/"+name, func(t *testing.T) {
				t.Parallel()
				pattern := generatePattern("ConfigMap", "target")
				pattern.Name = name
				generation := kyvernov1.Generation{GeneratePattern: pattern}
				if foreach {
					generation = kyvernov1.Generation{
						ForEachGeneration: []kyvernov1.ForEachGeneration{{GeneratePattern: pattern}},
					}
				}
				client, reviews := newGenerateAuthClient(t, func(spec authorizationv1.SubjectAccessReviewSpec) bool {
					return spec.ResourceAttributes.Namespace == "target" && spec.ResourceAttributes.Resource == "configmaps"
				})
				checker := NewGenerateFactoryWithGroups(client, &kyvernov1.Rule{Generation: &generation}, "alice", nil, "", logr.Discard())
				_, _, err := checker.Validate(context.Background(), nil)
				assert.NilError(t, err)
				assert.Equal(t, len(*reviews), 2)
				for _, review := range *reviews {
					assert.Equal(t, review.ResourceAttributes.Name, "", "dynamic names must not narrow the authorization check")
				}
			})
		}
	}
}
