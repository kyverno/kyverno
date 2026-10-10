package policyexception

import (
	"context"
	"errors"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

type exceptionDiscovery struct {
	discovery.ServerResourcesInterface
	available map[string]bool
	failed    string
	calls     int
}

func (d *exceptionDiscovery) ServerResourcesForGroupVersion(gv string) (*metav1.APIResourceList, error) {
	d.calls++
	if gv == d.failed {
		return nil, errors.New("discovery unavailable")
	}
	present, ok := d.available[gv]
	if !ok {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "apis"}, gv)
	}
	resources := &metav1.APIResourceList{GroupVersion: gv}
	if present {
		resources.APIResources = []metav1.APIResource{{Name: "policyexceptions"}}
	} else {
		resources.APIResources = []metav1.APIResource{{Name: "otherresources"}}
	}
	return resources, nil
}

func TestOptionalAPISources(t *testing.T) {
	// The constructor uses the global metrics manager, so these cases are serial.
	legacyGV := kyvernov2.SchemeGroupVersion.String()
	celGV := policiesv1beta1.SchemeGroupVersion.String()
	for _, tc := range []struct {
		name      string
		available map[string]bool
		failed    string
		want      []info
	}{
		{name: "both", available: map[string]bool{legacyGV: true, celGV: true}, want: []info{{legacyAPIGroup, "ns", "legacy", "", ""}, {celAPIGroup, "ns", "cel", "", ""}}},
		{name: "legacy only", available: map[string]bool{legacyGV: true}, want: []info{{legacyAPIGroup, "ns", "legacy", "", ""}}},
		{name: "CEL only", available: map[string]bool{celGV: true}, want: []info{{celAPIGroup, "ns", "cel", "", ""}}},
		{name: "neither"},
		{name: "group exists without resource", available: map[string]bool{legacyGV: false, celGV: false}},
		{name: "legacy discovery failure", available: map[string]bool{celGV: true}, failed: legacyGV, want: []info{{celAPIGroup, "ns", "cel", "", ""}}},
		{name: "CEL discovery failure", available: map[string]bool{legacyGV: true}, failed: celGV, want: []info{{legacyAPIGroup, "ns", "legacy", "", ""}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client := fake.NewSimpleClientset(
				&kyvernov2.PolicyException{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "legacy"}},
				&policiesv1beta1.PolicyException{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cel"}},
			)
			factory := kyvernoinformers.NewSharedInformerFactory(client, 0)
			recording := &recordingMetrics{}
			previous := metrics.GetManager()
			metrics.SetManager(testManager{exceptions: recording})
			defer metrics.SetManager(previous)
			discoveryClient := &exceptionDiscovery{available: tc.available, failed: tc.failed}
			NewController(discoveryClient, factory)
			require.Equal(t, 2, discoveryClient.calls)
			require.NotNil(t, recording.callback)
			require.NoError(t, recording.callback(ctx, nil))
			require.Empty(t, recording.observations)
			factory.Start(ctx.Done())
			synced := factory.WaitForCacheSync(ctx.Done())
			require.Len(t, synced, len(tc.want), "absent APIs must not enter the factory's startup sync requirements")
			for _, ok := range synced {
				require.True(t, ok)
			}
			cancel()
			factory.Shutdown()
			client.ClearActions()
			require.NoError(t, recording.callback(context.Background(), nil))
			require.ElementsMatch(t, tc.want, recording.observations)
			require.Equal(t, 2, discoveryClient.calls, "scrapes must not perform discovery")
			require.Empty(t, client.Actions(), "scrapes must only use caches")
		})
	}
}
