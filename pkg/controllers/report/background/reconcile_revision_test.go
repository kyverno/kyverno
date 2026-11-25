package background

import (
	"context"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	kyvernofake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	reportresource "github.com/kyverno/kyverno/pkg/controllers/report/resource"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/openreports"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type revisionTargetClient struct {
	dclient.Interface
	target *unstructured.Unstructured
}

func (c *revisionTargetClient) GetResource(context.Context, string, string, string, string, ...string) (*unstructured.Unstructured, error) {
	return c.target.DeepCopy(), nil
}

// GenerateEvents calls Add once for each completed scanner response, including
// responses without events. Count those calls without replacing the CEL scanner.
type revisionEventCounter struct{ scans int }

func (c *revisionEventCounter) Add(...event.Info) { c.scans++ }

func revisionValidatingPolicy(name string) *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID("uid-" + name), Generation: 1, ResourceVersion: "10"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"*"},
						},
					},
				}},
			},
			Validations: []admissionregistrationv1.Validation{{Expression: "false", Message: "denied"}},
		},
	}
}

type revisionReconcileFixture struct {
	ctrl    *controller
	client  *kyvernofake.Clientset
	indexer cache.Indexer
	target  *unstructured.Unstructured
	events  *revisionEventCounter
}

func newRevisionReconcileFixture(t *testing.T, namespace string, policies ...engineapi.GenericPolicy) *revisionReconcileFixture {
	t.Helper()
	previous := reportutils.ReportingCfg
	reportutils.ReportingCfg = nil
	reportutils.NewReportingConfig([]string{"pass", "fail", "skip", "error"}, "validate")
	t.Cleanup(func() { reportutils.ReportingCfg = previous })

	target := &unstructured.Unstructured{}
	target.SetAPIVersion("v1")
	target.SetKind("ConfigMap")
	if namespace == "" {
		target.SetKind("Namespace")
	}
	target.SetName("target")
	target.SetNamespace(namespace)
	target.SetUID("resource-uid")
	target.SetResourceVersion("1")
	observed := reportutils.NewBackgroundScanReport(namespace, "report", target.GroupVersionKind(), target.GetName(), target.GetUID())
	observed.SetName("report")
	observed.SetResourceVersion("1")
	observed.SetAnnotations(map[string]string{annotationLastScanTime: time.Now().Format(time.RFC3339)})
	reportutils.SetResourceVersionLabels(observed, target)
	var results []openreportsv1alpha1.ReportResult
	for _, policy := range policies {
		reportutils.SetPolicyLabel(observed, policy)
		results = append(results, openreportsv1alpha1.ReportResult{
			Policy: policy.GetName(), Rule: "stale", Result: openreports.StatusPass, Description: "stale result",
		})
	}
	reportutils.SetResults(observed, results...)
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	require.NoError(t, indexer.Add(observed))
	nsIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	if namespace != "" {
		require.NoError(t, nsIndexer.Add(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}))
	}
	client := kyvernofake.NewSimpleClientset(observed.(runtime.Object))
	events := &revisionEventCounter{}
	lister := cache.NewGenericLister(indexer, reportsv1.SchemeGroupVersion.WithResource("ephemeralreports").GroupResource())
	return &revisionReconcileFixture{
		client: client, indexer: indexer, target: target, events: events,
		ctrl: &controller{
			client: &revisionTargetClient{target: target}, kyvernoClient: client,
			bgscanrLister: lister, cbgscanrLister: lister, nsLister: corev1listers.NewNamespaceLister(nsIndexer),
			config: config.NewDefaultConfiguration(false), eventGen: events,
			forceDelay: time.Hour, policyReports: true, exceptionNamespace: "*",
		},
	}
}

func (f *revisionReconcileFixture) reconcile(t *testing.T, exceptions []*policiesv1beta1.PolicyException, policies ...engineapi.GenericPolicy) error {
	t.Helper()
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	if f.target.GetNamespace() == "" {
		gvr.Resource = "namespaces"
	}
	return f.ctrl.reconcileReport(t.Context(), f.target.GetNamespace(), "report", false, f.target.GetUID(),
		f.target.GroupVersionKind(), gvr, reportresource.Resource{Name: f.target.GetName(), Namespace: f.target.GetNamespace()},
		exceptions, nil, nil, policies...)
}

// Refresh the informer from the persisted object, as the real watch would do.
func (f *revisionReconcileFixture) stored(t *testing.T) reportsv1.ReportInterface {
	t.Helper()
	report, err := f.ctrl.getReport(t.Context(), f.target.GetNamespace(), "report")
	require.NoError(t, err)
	require.NoError(t, f.indexer.Update(report))
	return report
}

func (f *revisionReconcileFixture) writes() int {
	n := 0
	for _, action := range f.client.Actions() {
		if action.GetVerb() == "create" || action.GetVerb() == "update" || action.GetVerb() == "delete" {
			n++
		}
	}
	return n
}

func TestReconcileReport_LegacyRevisionMigration(t *testing.T) {
	for _, namespace := range []string{"", "default"} {
		t.Run("namespace="+namespace, func(t *testing.T) {
			vp := revisionValidatingPolicy("policy")
			policy := engineapi.NewValidatingPolicy(vp)
			f := newRevisionReconcileFixture(t, namespace, policy)
			observed := f.stored(t)
			observed.GetLabels()[reportutils.PolicyLabel(policy)] = vp.ResourceVersion
			_, err := reportutils.UpdateReport(t.Context(), observed, f.client, nil)
			require.NoError(t, err)
			f.stored(t)
			f.client.ClearActions()

			// A failed migration must leave both the old label and result intact.
			broken := vp.DeepCopy()
			broken.Spec.Validations[0].Expression = "invalid("
			require.Error(t, f.reconcile(t, nil, engineapi.NewValidatingPolicy(broken)))
			require.Zero(t, f.writes())
			require.Equal(t, observed.GetLabels(), f.stored(t).GetLabels())
			require.Equal(t, observed.GetResults(), f.stored(t).GetResults())

			require.NoError(t, f.reconcile(t, nil, policy))
			migrated := f.stored(t)
			require.Equal(t, reportutils.PolicyRevision(policy), migrated.GetLabels()[reportutils.PolicyLabel(policy)])
			require.Len(t, migrated.GetResults(), 1)
			require.EqualValues(t, openreports.StatusFail, migrated.GetResults()[0].Result)
			require.NotEqual(t, "stale result", migrated.GetResults()[0].Description)
			require.Equal(t, 1, f.writes())
			require.Equal(t, 1, f.events.scans)

			// Even direct periodic reconciliation must neither scan nor write.
			metadataOnly := vp.DeepCopy()
			metadataOnly.ResourceVersion = "11"
			metadataOnly.Annotations = map[string]string{"example.com/note": "updated"}
			metadataOnly.Status.ConditionStatus.Message = "ready"
			current := engineapi.NewValidatingPolicy(metadataOnly)
			_, needs, _, err := f.ctrl.needsReconcile(namespace, "report", reportutils.CalculateResourceHash(*f.target), nil, nil, current)
			require.NoError(t, err)
			require.False(t, needs)
			require.NoError(t, f.reconcile(t, nil, current))
			require.Equal(t, 1, f.writes())
			require.Equal(t, 1, f.events.scans)
			require.Equal(t, migrated.GetResults(), f.stored(t).GetResults())

			changed := metadataOnly.DeepCopy()
			changed.Generation++
			changed.ResourceVersion = "12"
			changed.Spec.Validations[0].Expression = "true"
			require.NoError(t, f.reconcile(t, nil, engineapi.NewValidatingPolicy(changed)))
			require.Len(t, f.stored(t).GetResults(), 1)
			require.EqualValues(t, openreports.StatusPass, f.stored(t).GetResults()[0].Result)
			require.Equal(t, 2, f.writes())
			require.Equal(t, 2, f.events.scans)
		})
	}
}

func TestReconcileReport_ExceptionRevisionAndFailedRetry(t *testing.T) {
	first := revisionValidatingPolicy("first")
	second := revisionValidatingPolicy("second")
	policies := []engineapi.GenericPolicy{engineapi.NewValidatingPolicy(first), engineapi.NewValidatingPolicy(second)}
	f := newRevisionReconcileFixture(t, "", policies...)
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "exception", UID: "exception-uid", ResourceVersion: "10"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs:      []policiesv1beta1.PolicyRef{{Name: first.Name, Kind: "ValidatingPolicy"}},
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "match", Expression: "false"}},
		},
	}
	require.NoError(t, f.reconcile(t, []*policiesv1beta1.PolicyException{exception}, policies...))
	initial := f.stored(t)
	require.Len(t, initial.GetResults(), 2)
	require.EqualValues(t, 2, reportutils.CalculateSummary(initial.GetResults()).Fail)

	updated := exception.DeepCopy()
	updated.ResourceVersion = "11"
	updated.Spec.MatchConditions[0].Expression = "true"
	broken := second.DeepCopy()
	broken.Generation++
	broken.Spec.Validations[0].Expression = "invalid("
	scansBefore := f.events.scans
	f.client.ClearActions()
	require.Error(t, f.reconcile(t, []*policiesv1beta1.PolicyException{updated}, policies[0], engineapi.NewValidatingPolicy(broken)))
	require.Equal(t, scansBefore+1, f.events.scans, "the first policy must complete before the second scan fails")
	require.Zero(t, f.writes())
	require.Equal(t, initial.GetLabels(), f.stored(t).GetLabels())
	require.Equal(t, initial.GetResults(), f.stored(t).GetResults())

	require.NoError(t, f.reconcile(t, []*policiesv1beta1.PolicyException{updated}, policies...))
	refreshed := f.stored(t)
	require.Equal(t, celExceptionRevision([]*policiesv1beta1.PolicyException{updated}), refreshed.GetLabels()[celExceptionRevisionLabel])
	require.Len(t, refreshed.GetResults(), 2, "updated results must replace the previous ones")
	require.EqualValues(t, 1, reportutils.CalculateSummary(refreshed.GetResults()).Skip)
	require.EqualValues(t, 1, reportutils.CalculateSummary(refreshed.GetResults()).Fail)
	require.Equal(t, 1, f.writes())

	// Deleting the final exception invalidates a previous skip and removes its label.
	require.NoError(t, f.reconcile(t, nil, policies...))
	deleted := f.stored(t)
	require.NotContains(t, deleted.GetLabels(), celExceptionRevisionLabel)
	require.Len(t, deleted.GetResults(), 2)
	require.EqualValues(t, 2, reportutils.CalculateSummary(deleted.GetResults()).Fail)
	require.Equal(t, 2, f.writes())
	scansBefore = f.events.scans
	require.NoError(t, f.reconcile(t, nil, policies...))
	require.Equal(t, scansBefore, f.events.scans)
	require.Equal(t, 2, f.writes())
}

func TestReconcileReport_ChangedNativeBindingDoesNotDuplicateResults(t *testing.T) {
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "native-policy", ResourceVersion: "10"},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			MatchConstraints: revisionValidatingPolicy("template").Spec.MatchConstraints,
			Validations:      []admissionregistrationv1.Validation{{Expression: "true"}},
		},
	}
	f := newRevisionReconcileFixture(t, "", engineapi.NewValidatingAdmissionPolicy(vap))
	f.ctrl.client.(*revisionTargetClient).Interface = dclient.NewEmptyFakeClient()
	observed := f.stored(t)
	reportutils.SetResults(observed)
	_, err := reportutils.UpdateReport(t.Context(), observed, f.client, nil)
	require.NoError(t, err)
	bindings := []admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		{ObjectMeta: metav1.ObjectMeta{Name: "first-binding", ResourceVersion: "20"},
			Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
				PolicyName: vap.Name, ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
			}},
		{ObjectMeta: metav1.ObjectMeta{Name: "second-binding", ResourceVersion: "30"},
			Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
				PolicyName: vap.Name, ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
			}},
	}
	reconcile := func() {
		t.Helper()
		require.NoError(t, f.ctrl.reconcileReport(t.Context(), "", "report", false, f.target.GetUID(),
			f.target.GroupVersionKind(), schema.GroupVersionResource{Version: "v1", Resource: "namespaces"},
			reportresource.Resource{Name: f.target.GetName()}, nil, bindings, nil,
			engineapi.NewValidatingAdmissionPolicy(vap.DeepCopy())))
	}
	reconcile()
	initial := f.stored(t).GetResults()
	require.NotEmpty(t, initial)
	bindings[0].ResourceVersion = "21"
	reconcile()
	results := f.stored(t).GetResults()
	require.Len(t, results, len(initial), "a policy-wide binding rescan must replace all previous results")
	counts := map[string]int{}
	for _, result := range results {
		counts[result.Properties["binding"]]++
	}
	for binding, count := range counts {
		require.Equal(t, 1, count, "duplicate result for binding %s", binding)
	}
}
