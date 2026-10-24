package admissionpolicygenerator

import (
	"context"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	"k8s.io/client-go/tools/cache"
)

type denyResourceAuthChecker struct{ resource string }

func (c denyResourceAuthChecker) Check(ctx context.Context, group, version, resource, subresource, namespace, name, verb string) (*checker.AuthResult, error) {
	return &checker.AuthResult{Allowed: resource != c.resource}, nil
}

func TestVAPExpiryBeforePermissionChecks(t *testing.T) {
	for _, resource := range []string{"validatingadmissionpolicies", "validatingadmissionpolicybindings"} {
		for _, mode := range []string{"eligible", "disabled", "autogen"} {
			t.Run(resource+"/"+mode, func(t *testing.T) {
				vpol := disallowPrivilegeEscalationVpol(nil)
				vpol.Kind = "ValidatingPolicy"
				if mode != "autogen" {
					vpol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
				}
				if mode == "disabled" {
					*vpol.Spec.AutogenConfiguration.ValidatingAdmissionPolicy.Enabled = false
				}
				c, kubeClient, _ := newVAPTestController(t, vpol)
				c.checker = denyResourceAuthChecker{resource: resource}
				queue := &delayRecorder{delays: map[any]time.Duration{}}
				c.queue = queue
				c.polexEnabled, c.exceptionNamespace = true, "*"
				c.celpolexLister = newCELPolexLister(t, &policiesv1beta1.PolicyException{
					ObjectMeta: metav1.ObjectMeta{Name: "exception", Namespace: "default"},
					Spec: policiesv1beta1.PolicyExceptionSpec{
						PolicyRefs: []policiesv1beta1.PolicyRef{{Name: vpol.Name, Kind: vpol.Kind}},
						ExpiresAt:  &metav1.Time{Time: time.Now().Add(time.Hour)},
					},
				})

				require.NoError(t, c.handleVAPGeneration(context.Background(), engineapi.NewValidatingPolicy(vpol)))
				assert.Empty(t, kubeClient.Actions(), "permission denial must prevent native API writes")
				if mode == "eligible" {
					require.Contains(t, queue.delays, "ValidatingPolicy/"+vpol.Name, "expiry must be armed despite permission denial")
					assert.Positive(t, queue.delays["ValidatingPolicy/"+vpol.Name])
				} else {
					assert.Empty(t, queue.delays, "ineligible policies must not arm an expiry timer")
				}
			})
		}
	}
}

func TestMAPExpiryBeforePermissionChecks(t *testing.T) {
	for _, version := range []string{"v1", "v1beta1", "v1alpha1"} {
		for _, resource := range []string{"mutatingadmissionpolicies", "mutatingadmissionpolicybindings"} {
			for _, mode := range []string{"eligible", "disabled", "autogen", "server side apply", "API unavailable"} {
				t.Run(version+"/"+resource+"/"+mode, func(t *testing.T) {
					mpol := &policiesv1beta1.MutatingPolicy{
						TypeMeta:   metav1.TypeMeta{Kind: "MutatingPolicy"},
						ObjectMeta: metav1.ObjectMeta{Name: "policy"},
						Spec: policiesv1beta1.MutatingPolicySpec{
							AutogenConfiguration: mapGenEnabled(),
						},
					}
					switch mode {
					case "disabled":
						*mpol.Spec.AutogenConfiguration.MutatingAdmissionPolicy.Enabled = false
					case "autogen":
						mpol.Spec.AutogenConfiguration = mapGenEnabledWithPodControllerAutogen()
						mpol.Spec.MatchConstraints = podsMatchConstraints()
					case "server side apply":
						mpol.Spec.EvaluationConfiguration = &policiesv1beta1.MutatingPolicyEvaluationConfiguration{UseServerSideApply: true}
					}
					c, kubeClient, _ := newVAPTestController(t, disallowPrivilegeEscalationVpol(nil))
					c.kyvernoClient = versionedfake.NewSimpleClientset(mpol)
					if mode != "API unavailable" {
						switch version {
						case "v1":
							indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
							c.mapV1Lister = admissionregistrationv1listers.NewMutatingAdmissionPolicyLister(indexer)
							c.mapbindingV1Lister = admissionregistrationv1listers.NewMutatingAdmissionPolicyBindingLister(indexer)
						case "v1beta1":
							c.mapBetaLister, c.mapbindingBetaLister = &mockMAPBetaLister{}, &mockMAPBindingBetaLister{}
						case "v1alpha1":
							c.mapAlphaLister, c.mapbindingAlphaLister = &mockMAPAlphaLister{}, &mockMAPBindingAlphaLister{}
						}
					}
					c.checker = denyResourceAuthChecker{resource: resource}
					queue := &delayRecorder{delays: map[any]time.Duration{}}
					c.queue = queue
					c.polexEnabled, c.exceptionNamespace = true, "*"
					c.celpolexLister = newCELPolexLister(t, &policiesv1beta1.PolicyException{
						ObjectMeta: metav1.ObjectMeta{Name: "exception", Namespace: "default"},
						Spec: policiesv1beta1.PolicyExceptionSpec{
							PolicyRefs: []policiesv1beta1.PolicyRef{{Name: mpol.Name, Kind: mpol.Kind}},
							ExpiresAt:  &metav1.Time{Time: time.Now().Add(time.Hour)},
						},
					})

					require.NoError(t, c.handleMAPGeneration(context.Background(), mpol))
					assert.Empty(t, kubeClient.Actions(), "permission denial must prevent native API writes")
					if mode == "eligible" {
						require.Contains(t, queue.delays, "MutatingPolicy/"+mpol.Name, "expiry must be armed despite permission denial")
						assert.Positive(t, queue.delays["MutatingPolicy/"+mpol.Name])
					} else {
						assert.Empty(t, queue.delays, "ineligible policies must not arm an expiry timer")
					}
				})
			}
		}
	}
}
