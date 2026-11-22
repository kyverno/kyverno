package background

import (
	"testing"

	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
)

func TestKeepResultForEffectiveRevision(t *testing.T) {
	labelsByName := map[string]string{
		"policy": "vpol.kyverno.io/policy",
		"binding": "validatingadmissionpolicybinding.apiserver.io/binding",
		"exception": "polex.kyverno.io/exception",
	}
	expected := map[string]string{
		labelsByName["policy"]:    "e1-2-revision",
		labelsByName["binding"]:   "42",
		labelsByName["exception"]: "12",
	}
	for _, tc := range []struct {
		name       string
		result     openreportsv1alpha1.ReportResult
		actual     map[string]string
		wantRetain bool
	}{
		{
			name: "unchanged result",
			result: openreportsv1alpha1.ReportResult{
				Policy: "policy",
			},
			actual:     map[string]string{labelsByName["policy"]: "e1-2-revision"},
			wantRetain: true,
		},
		{
			name: "legacy label migration must replace stale result",
			result: openreportsv1alpha1.ReportResult{
				Policy: "policy",
			},
			actual: map[string]string{labelsByName["policy"]: "27"},
		},
		{
			name: "recreated policy must replace stale result",
			result: openreportsv1alpha1.ReportResult{
				Policy: "policy",
			},
			actual: map[string]string{labelsByName["policy"]: "e1-2-old-uid"},
		},
		{
			name: "unchanged native policy and binding",
			result: openreportsv1alpha1.ReportResult{
				Policy: "policy", Properties: map[string]string{"binding": "binding"},
			},
			actual: map[string]string{
				labelsByName["policy"]: "e1-2-revision",
				labelsByName["binding"]: "42",
			},
			wantRetain: true,
		},
		{
			name: "changed binding must invalidate result",
			result: openreportsv1alpha1.ReportResult{
				Policy: "policy", Properties: map[string]string{"binding": "binding"},
			},
			actual: map[string]string{
				labelsByName["policy"]: "e1-2-revision",
				labelsByName["binding"]: "41",
			},
		},
		{
			name: "changed exception must invalidate result",
			result: openreportsv1alpha1.ReportResult{
				Policy: "policy", Properties: map[string]string{"exceptions": "exception"},
			},
			actual: map[string]string{
				labelsByName["policy"]: "e1-2-revision",
				labelsByName["exception"]: "11",
			},
		},
		{
			name: "unknown policy is not safe to keep",
			result: openreportsv1alpha1.ReportResult{
				Policy: "unknown",
			},
			actual: map[string]string{labelsByName["policy"]: "e1-2-revision"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := keepResultForEffectiveRevision(tc.result, labelsByName, expected, tc.actual); got != tc.wantRetain {
				t.Fatalf("retain=%v, want %v", got, tc.wantRetain)
			}
		})
	}
}
