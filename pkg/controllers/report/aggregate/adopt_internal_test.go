package aggregate

import (
	"testing"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestAdoptChecksResourceUID(t *testing.T) {
	t.Parallel()

	const liveUID types.UID = "live-uid"
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}

	for _, tc := range []struct {
		name      string
		reportUID types.UID
		wantAdopt bool
	}{
		{name: "same UID", reportUID: liveUID, wantAdopt: true},
		{name: "UID not yet assigned", wantAdopt: true},
		{name: "resource replaced", reportUID: "deleted-uid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resource := &unstructured.Unstructured{}
			resource.SetGroupVersionKind(gvk)
			resource.SetNamespace("default")
			resource.SetName("settings")
			resource.SetUID(liveUID)
			dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), resource)

			report := reportutils.NewAdmissionReport(resource.GetNamespace(), "request", gvr, gvk, *resource).(*reportsv1.EphemeralReport)
			report.SetName("admission-report")
			reportutils.SetResourceUid(report, tc.reportUID)
			client := versionedfake.NewSimpleClientset(report)
			c := &controller{client: client, dclient: dclient.NewFakeClientWithDisco(dynamicClient, nil, nil)}

			adopted, forbidden := c.adopt(t.Context(), &metav1.PartialObjectMetadata{ObjectMeta: *report.ObjectMeta.DeepCopy()})
			require.False(t, forbidden)
			require.Equal(t, tc.wantAdopt, adopted)
			got, err := client.ReportsV1().EphemeralReports(report.GetNamespace()).Get(t.Context(), report.GetName(), metav1.GetOptions{})
			require.NoError(t, err)
			if tc.wantAdopt {
				require.Equal(t, liveUID, reportutils.GetResourceUid(got))
				require.Len(t, got.OwnerReferences, 1)
				require.Equal(t, liveUID, got.OwnerReferences[0].UID)
			} else {
				require.Equal(t, tc.reportUID, reportutils.GetResourceUid(got))
				require.Empty(t, got.OwnerReferences)
			}
		})
	}
}
