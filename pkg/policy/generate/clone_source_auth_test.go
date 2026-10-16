package generate

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/v3/assert"
	authorizationv1 "k8s.io/api/authorization/v1"
)

func cloneSourcePattern(kind string, cloneList bool) kyvernov1.GeneratePattern {
	pattern := generatePattern(kind, "target")
	if cloneList {
		pattern.CloneList = kyvernov1.CloneList{Namespace: "sources", Kinds: []string{"v1/" + kind}}
	} else {
		pattern.Clone = kyvernov1.CloneFrom{Namespace: "sources", Name: "source-object"}
	}
	return pattern
}

func TestGenerateCloneSourcesRequireReadAccess(t *testing.T) {
	t.Parallel()

	for _, cloneList := range []bool{false, true} {
		for _, foreach := range []bool{false, true} {
			for _, synchronize := range []bool{false, true} {
				for _, sourceAllowed := range []bool{false, true} {
					t.Run(fmt.Sprintf("cloneList=%t/foreach=%t/synchronize=%t/allowed=%t", cloneList, foreach, synchronize, sourceAllowed), func(t *testing.T) {
						t.Parallel()
						pattern := cloneSourcePattern("Secret", cloneList)
						if cloneList {
							pattern.CloneList.Kinds = []string{"ConfigMap", "v1/Secret"}
						}
						generation := kyvernov1.Generation{GeneratePattern: pattern, Synchronize: synchronize}
						if foreach {
							generation.GeneratePattern = kyvernov1.GeneratePattern{}
							generation.ForEachGeneration = []kyvernov1.ForEachGeneration{
								{GeneratePattern: cloneSourcePattern("ConfigMap", cloneList)},
								{GeneratePattern: pattern},
							}
						}
						groups := []string{"policy-authors"}
						client, reviews := newGenerateAuthClient(t, func(spec authorizationv1.SubjectAccessReviewSpec) bool {
							attr := spec.ResourceAttributes
							if attr.Namespace == "target" {
								return attr.Verb != "patch"
							}
							if attr.Namespace != "sources" || (attr.Resource == "secrets" && !sourceAllowed) {
								return false
							}
							if cloneList {
								return attr.Verb == "list" && attr.Name == ""
							}
							return attr.Verb == "get" && attr.Name == "source-object"
						})
						checker := NewGenerateFactoryWithGroups(client, &kyvernov1.Rule{Generation: &generation}, "alice", groups, "", logr.Discard())
						_, _, err := checker.Validate(context.Background(), nil)
						if sourceAllowed {
							assert.NilError(t, err)
						} else {
							assert.ErrorContains(t, err, "source: alice requires permissions")
							assert.ErrorContains(t, err, "in namespace sources")
						}
						sawSecretSource := false
						for _, review := range *reviews {
							assert.Equal(t, review.User, "alice")
							assert.DeepEqual(t, review.Groups, groups)
							if review.ResourceAttributes.Namespace == "sources" && review.ResourceAttributes.Resource == "secrets" {
								sawSecretSource = true
							}
						}
						assert.Assert(t, sawSecretSource, "later kinds and foreach elements must authorize their own source")
					})
				}
			}
		}
	}
}

func TestGenerateCloneSourceReadPermissionsByCaller(t *testing.T) {
	t.Parallel()

	for _, cloneList := range []bool{false, true} {
		for _, controller := range []bool{false, true} {
			for _, allowGet := range []bool{false, true} {
				t.Run(fmt.Sprintf("cloneList=%t/controller=%t/get=%t", cloneList, controller, allowGet), func(t *testing.T) {
					t.Parallel()
					generation := kyvernov1.Generation{GeneratePattern: cloneSourcePattern("ConfigMap", cloneList), Synchronize: true}
					client, reviews := newGenerateAuthClient(t, func(spec authorizationv1.SubjectAccessReviewSpec) bool {
						attr := spec.ResourceAttributes
						return attr.Namespace == "target" || (attr.Namespace == "sources" &&
							(attr.Verb == "list" || (attr.Verb == "get" && allowGet)))
					})
					var verbs []string
					user := "alice"
					if controller {
						user = "system:serviceaccount:kyverno:background-controller"
						verbs = []string{"get", "create", "patch", "update", "delete"}
					}
					checker := NewGenerateFactory(client, &kyvernov1.Rule{Generation: &generation}, user, "", logr.Discard())
					_, _, err := checker.Validate(context.Background(), verbs)
					if (controller || !cloneList) && !allowGet {
						assert.ErrorContains(t, err, "source: "+user+" requires permissions get")
					} else {
						assert.NilError(t, err)
					}
					var sourceVerbs []string
					for _, review := range *reviews {
						if review.ResourceAttributes.Namespace == "sources" {
							sourceVerbs = append(sourceVerbs, review.ResourceAttributes.Verb)
						}
					}
					want := []string{"get"}
					if cloneList {
						want = []string{"list"}
						if controller {
							want = append(want, "get")
						}
					}
					assert.DeepEqual(t, sourceVerbs, want, "source writes must not be required")
				})
			}
		}
	}
}

func TestGenerateDynamicCloneSourcesRequireUnrestrictedReadAccess(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		namespace string
		source    string
		wantNS    string
		wantName  string
	}{
		{name: "variable namespace", namespace: "{{request.object.metadata.namespace}}", source: "source-object", wantName: "source-object"},
		{name: "reference namespace", namespace: "$(./../namespace)", source: "source-object", wantName: "source-object"},
		{name: "empty namespace", source: "source-object", wantName: "source-object"},
		{name: "variable name", namespace: "sources", source: "{{request.object.metadata.name}}", wantNS: "sources"},
		{name: "reference name", namespace: "sources", source: "$(./../name)", wantNS: "sources"},
	} {
		for _, cloneList := range []bool{false, true} {
			if cloneList && test.wantNS != "" {
				continue // cloneList does not have a source name.
			}
			for _, allowUnrestricted := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/cloneList=%t/allowed=%t", test.name, cloneList, allowUnrestricted), func(t *testing.T) {
					t.Parallel()
					pattern := cloneSourcePattern("ConfigMap", cloneList)
					pattern.Clone.Namespace, pattern.Clone.Name = test.namespace, test.source
					if cloneList {
						pattern.Clone = kyvernov1.CloneFrom{}
						pattern.CloneList.Namespace = test.namespace
					}
					generation := kyvernov1.Generation{GeneratePattern: pattern}
					wantName := test.wantName
					if cloneList {
						wantName = ""
					}
					client, reviews := newGenerateAuthClient(t, func(spec authorizationv1.SubjectAccessReviewSpec) bool {
						attr := spec.ResourceAttributes
						if attr.Namespace == "target" {
							return true
						}
						// Literal expression strings and narrower grants must not pass.
						return allowUnrestricted && attr.Namespace == test.wantNS &&
							attr.Name == wantName
					})
					checker := NewGenerateFactoryWithGroups(client, &kyvernov1.Rule{Generation: &generation}, "alice", nil, "", logr.Discard())
					_, _, err := checker.Validate(context.Background(), nil)
					if allowUnrestricted {
						assert.NilError(t, err)
					} else {
						assert.ErrorContains(t, err, "requires permissions")
					}
					assert.Equal(t, len(*reviews), 3)
					attr := (*reviews)[2].ResourceAttributes
					assert.Equal(t, attr.Namespace, test.wantNS)
					assert.Equal(t, attr.Name, wantName)
				})
			}
		}
	}
}

func TestGenerateCloneSourceReadsNotRequiredForAdmissionOrOffline(t *testing.T) {
	t.Parallel()

	for _, cloneList := range []bool{false, true} {
		t.Run(fmt.Sprintf("cloneList=%t", cloneList), func(t *testing.T) {
			t.Parallel()
			generation := kyvernov1.Generation{GeneratePattern: cloneSourcePattern("ConfigMap", cloneList), Synchronize: true}
			client, reviews := newGenerateAuthClient(t, func(spec authorizationv1.SubjectAccessReviewSpec) bool {
				return spec.ResourceAttributes.Namespace == "target"
			})
			checker := NewGenerateFactory(client, &kyvernov1.Rule{Generation: &generation}, "admission", "", logr.Discard())
			_, _, err := checker.ValidateTargets(context.Background(), []string{"list", "get"})
			assert.NilError(t, err)
			assert.Equal(t, len(*reviews), 2)

			checker.offline = true
			_, _, err = checker.Validate(context.Background(), nil)
			assert.NilError(t, err)
			assert.Equal(t, len(*reviews), 2, "offline validation must not submit access reviews")
		})
	}
}
