package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	fakediscovery "k8s.io/client-go/discovery/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestValidateCELParamKindScopeAdmission(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name           string
		apiVersion     string
		kind           string
		clusterPolicy  bool
		discoveryError bool
		wantError      string
	}{
		{name: "preferred cluster parameter", apiVersion: "cluster.example.com/v1", kind: "Parameter", wantError: "cluster-scoped paramKind is not allowed"},
		{name: "nonpreferred cluster parameter", apiVersion: "cluster.example.com/v1beta1", kind: "Parameter", wantError: "cluster-scoped paramKind is not allowed"},
		{name: "namespaced parameter with same kind in another group", apiVersion: "tenant.example.com/v1", kind: "Parameter"},
		{name: "core namespaced parameter", apiVersion: "v1", kind: "ConfigMap"},
		{name: "unknown parameter kind", apiVersion: "tenant.example.com/v1", kind: "UnknownParameter", wantError: "cannot resolve paramKind"},
		{name: "unknown parameter version", apiVersion: "tenant.example.com/v2", kind: "Parameter", wantError: "failed to resolve paramKind"},
		{name: "malformed api version", apiVersion: "tenant.example.com/v1/extra", kind: "Parameter", wantError: "invalid paramKind.apiVersion"},
		{name: "empty api version", kind: "Parameter", wantError: "invalid paramKind.apiVersion"},
		{name: "missing version", apiVersion: "tenant.example.com/", kind: "Parameter", wantError: "invalid paramKind.apiVersion"},
		{name: "empty kind", apiVersion: "tenant.example.com/v1", wantError: "paramKind.kind must not be empty"},
		{name: "discovery failure", apiVersion: "tenant.example.com/v1", kind: "Parameter", discoveryError: true, wantError: "parameter discovery unavailable"},
		{name: "cluster policy preferred parameter", apiVersion: "cluster.example.com/v1", kind: "Parameter", clusterPolicy: true},
		{name: "cluster policy nonpreferred parameter", apiVersion: "cluster.example.com/v1beta1", kind: "Parameter", clusterPolicy: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client, cached := newParameterScopeClient()
			if test.discoveryError {
				cached.failedGroupVersion = test.apiVersion
				cached.lookupError = errors.New("parameter discovery unavailable")
			}
			policy := parameterScopePolicy(t, test.apiVersion, test.kind)
			var policyToValidate kyvernov1.PolicyInterface = policy
			if test.clusterPolicy {
				policyToValidate = &kyvernov1.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: policy.Name}, Spec: policy.Spec}
			}
			warnings, err := Validate(policyToValidate, nil, client, false, "", "")
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
			} else {
				require.NoError(t, err)
				require.Empty(t, warnings)
			}
			require.Zero(t, client.parameterReads)
		})
	}
}

func TestValidateCELParamKindScopeRefreshesDiscovery(t *testing.T) {
	t.Parallel()
	client, cached := newParameterScopeClient()
	cached.staleGroupVersion = "tenant.example.com/v1"
	rule := parameterScopePolicy(t, "tenant.example.com/v1", "Parameter").Spec.Rules[0]
	require.NoError(t, validateCELParamKindScope(rule, true, client))
	require.Equal(t, int32(1), cached.invalidations.Load())
	require.Equal(t, int32(2), cached.lookups.Load())
}

func TestValidateCELParamKindScopeSkipsClusterAndUnparameterizedRules(t *testing.T) {
	t.Parallel()
	rule := parameterScopePolicy(t, "cluster.example.com/v1beta1", "Parameter").Spec.Rules[0]
	require.NoError(t, validateCELParamKindScope(rule, false, nil))
	require.NoError(t, validateCELParamKindScope(kyvernov1.Rule{}, true, nil))
	rule.Validation.CEL.ParamRef = nil
	require.NoError(t, validateCELParamKindScope(rule, true, nil))
}

func TestValidateCELParamKindScopeRejectsUnavailableDiscovery(t *testing.T) {
	t.Parallel()
	rule := parameterScopePolicy(t, "tenant.example.com/v1", "Parameter").Spec.Rules[0]
	require.ErrorContains(t, validateCELParamKindScope(rule, true, nil), "discovery client is unavailable")
}

func TestValidateCELParamKindScopeRejectsAmbiguousAndSubresourceMatches(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		resources []metav1.APIResource
	}{
		{name: "ambiguous kind", resources: []metav1.APIResource{
			{Name: "parameters", Kind: "Parameter", Namespaced: true},
			{Name: "otherparameters", Kind: "Parameter", Namespaced: true},
		}},
		{name: "subresource only", resources: []metav1.APIResource{
			{Name: "parameters/status", Kind: "Parameter", Namespaced: true},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client, cached := newParameterScopeClient()
			cached.resultOverride = &metav1.APIResourceList{GroupVersion: "tenant.example.com/v1", APIResources: test.resources}
			rule := parameterScopePolicy(t, "tenant.example.com/v1", "Parameter").Spec.Rules[0]
			require.ErrorContains(t, validateCELParamKindScope(rule, true, client), "cannot resolve paramKind")
			require.Equal(t, int32(1), cached.invalidations.Load())
		})
	}
}

func TestCELParamScopeRuntimeRejectsAllClusterVersionsBeforeReads(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"v1", "v1beta1"} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()
			client, _ := newParameterScopeClient()
			policy := parameterScopePolicy(t, "cluster.example.com/"+version, "Parameter")
			for _, ref := range []*admissionregistrationv1.ParamRef{
				{Name: "params"},
				{Selector: &metav1.LabelSelector{}},
			} {
				params, err := admissionpolicy.CollectParamsForPolicy(context.Background(), adapters.Client(client), policy.Spec.Rules[0].Validation.CEL.ParamKind, ref, "tenant-a", policy)
				require.ErrorContains(t, err, "cluster-scoped paramKind is not allowed")
				require.Nil(t, params)
				require.Zero(t, client.parameterReads)
			}
		})
	}
}

func parameterScopePolicy(t *testing.T, apiVersion, kind string) *kyvernov1.Policy {
	t.Helper()
	// The name wildcard is a valid legacy match which does not generate a native
	// VAP, keeping these tests focused on the legacy admission validation path.
	raw := fmt.Sprintf(`{
		"apiVersion":"kyverno.io/v1","kind":"Policy",
		"metadata":{"name":"check-parameter-scope","namespace":"tenant-a"},
		"spec":{"background":false,"rules":[{
			"name":"check-param",
			"match":{"any":[{"resources":{"kinds":["v1/ConfigMap"],"names":["sample-*"]}}]},
			"validate":{"failureAction":"Enforce","cel":{
				"paramKind":{"apiVersion":%q,"kind":%q},
				"paramRef":{"name":"params","parameterNotFoundAction":"Deny"},
				"expressions":[{"expression":"true","message":"parameter scope check"}]
			}}
		}]}
	}`, apiVersion, kind)
	var policy kyvernov1.Policy
	require.NoError(t, json.Unmarshal([]byte(raw), &policy))
	return &policy
}

type parameterScopeDiscovery struct {
	dclient.IDiscovery
	cached    discovery.CachedDiscoveryInterface
	resources []*metav1.APIResourceList
}

func (d *parameterScopeDiscovery) CachedDiscoveryInterface() discovery.CachedDiscoveryInterface {
	return d.cached
}

func (d *parameterScopeDiscovery) FindResources(group, version, kind, subresource string) (map[dclient.TopLevelApiDescription]metav1.APIResource, error) {
	found := map[dclient.TopLevelApiDescription]metav1.APIResource{}
	for _, list := range d.resources {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			return nil, err
		}
		if group != "*" && group != gv.Group {
			continue
		}
		if version != "" && version != "*" && version != gv.Version {
			continue
		}
		for _, resource := range list.APIResources {
			if resource.Kind == kind {
				found[dclient.TopLevelApiDescription{GroupVersion: gv, Kind: kind, Resource: resource.Name, SubResource: subresource}] = resource
			}
		}
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("unknown kind %s/%s/%s", group, version, kind)
	}
	return found, nil
}

type parameterScopeCachedDiscovery struct {
	discovery.CachedDiscoveryInterface
	failedGroupVersion string
	lookupError        error
	staleGroupVersion  string
	resultOverride     *metav1.APIResourceList
	invalidations      atomic.Int32
	lookups            atomic.Int32
}

func (d *parameterScopeCachedDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	d.lookups.Add(1)
	if groupVersion == d.failedGroupVersion && d.lookupError != nil {
		return nil, d.lookupError
	}
	if groupVersion == d.staleGroupVersion && d.invalidations.Load() == 0 {
		return &metav1.APIResourceList{GroupVersion: groupVersion}, nil
	}
	if d.resultOverride != nil && d.resultOverride.GroupVersion == groupVersion {
		return d.resultOverride, nil
	}
	return d.CachedDiscoveryInterface.ServerResourcesForGroupVersion(groupVersion)
}

func (d *parameterScopeCachedDiscovery) Invalidate() {
	d.invalidations.Add(1)
	d.CachedDiscoveryInterface.Invalidate()
}

type parameterScopeClient struct {
	dclient.Interface
	parameterReads int
}

func (c *parameterScopeClient) GetResource(context.Context, string, string, string, string, ...string) (*unstructured.Unstructured, error) {
	c.parameterReads++
	return nil, fmt.Errorf("unexpected parameter GET")
}

func (c *parameterScopeClient) ListResource(context.Context, string, string, string, *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	c.parameterReads++
	return nil, fmt.Errorf("unexpected parameter LIST")
}

func newParameterScopeClient() (*parameterScopeClient, *parameterScopeCachedDiscovery) {
	resources := []*metav1.APIResourceList{
		{GroupVersion: "v1", APIResources: []metav1.APIResource{{Name: "configmaps", Kind: "ConfigMap", Namespaced: true}}},
		{GroupVersion: "cluster.example.com/v1", APIResources: []metav1.APIResource{{Name: "parameters", Kind: "Parameter", Namespaced: false}}},
		{GroupVersion: "cluster.example.com/v1beta1", APIResources: []metav1.APIResource{{Name: "parameters", Kind: "Parameter", Namespaced: false}}},
		{GroupVersion: "tenant.example.com/v1", APIResources: []metav1.APIResource{{Name: "parameters", Kind: "Parameter", Namespaced: true}}},
	}
	fake := &fakediscovery.FakeDiscovery{Fake: &k8stesting.Fake{Resources: resources}}
	cached := &parameterScopeCachedDiscovery{CachedDiscoveryInterface: memory.NewMemCacheClient(fake)}
	disco := &parameterScopeDiscovery{cached: cached, resources: resources}
	kube := kubefake.NewSimpleClientset()
	kube.PrependReactor("create", "selfsubjectaccessreviews", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authorizationv1.SelfSubjectAccessReview{Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true}}, nil
	})
	return &parameterScopeClient{Interface: dclient.NewFakeClientWithDisco(nil, kube, disco)}, cached
}
