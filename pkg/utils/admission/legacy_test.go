package admission

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestIsFinalizerRemovalOnTerminatingObject(t *testing.T) {
	now := metav1.NewTime(time.Now().Truncate(time.Second))

	terminating := func(finalizers []string, extra string) []byte {
		return []byte(`{
			"metadata": {
				"name": "test",
				"namespace": "",
				"uid": "abc",
				"generation": 1,
				"resourceVersion": "1",
				"deletionTimestamp": "` + now.Format(time.RFC3339) + `",
				"finalizers": ` + toJSONArray(finalizers) + `,
				"labels": {"a":"b"}
			},
			"spec": {"foo": "bar"` + extra + `}
		}`)
	}

	tests := []struct {
		name       string
		request    admissionv1.AdmissionRequest
		wantAllow  bool
		wantErrSet bool
	}{
		{
			name: "valid single finalizer removal",
			request: admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				OldObject: runtime.RawExtension{Raw: terminating([]string{"kyverno.io/finalizer"}, "")},
				Object:    runtime.RawExtension{Raw: terminating([]string{}, "")},
			},
			wantAllow: true,
		},
		{
			name: "valid removal to empty from multiple",
			request: admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				OldObject: runtime.RawExtension{Raw: terminating([]string{"a", "b"}, "")},
				Object:    runtime.RawExtension{Raw: terminating([]string{}, "")},
			},
			wantAllow: true,
		},
		{
			name: "removal plus spec change denied",
			request: admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				OldObject: runtime.RawExtension{Raw: terminating([]string{"a"}, "")},
				Object:    runtime.RawExtension{Raw: terminating([]string{}, `,"baz":"qux"`)},
			},
			wantAllow: false,
		},
		{
			name: "finalizer add denied",
			request: admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				OldObject: runtime.RawExtension{Raw: terminating([]string{"a"}, "")},
				Object:    runtime.RawExtension{Raw: terminating([]string{"a", "b"}, "")},
			},
			wantAllow: false,
		},
		{
			name: "without deletionTimestamp denied",
			request: admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				OldObject: runtime.RawExtension{Raw: []byte(`{"metadata":{"name":"test","finalizers":["a"]},"spec":{"foo":"bar"}}`)},
				Object:    runtime.RawExtension{Raw: []byte(`{"metadata":{"name":"test","finalizers":[]},"spec":{"foo":"bar"}}`)},
			},
			wantAllow: false,
		},
		{
			name: "not an update denied",
			request: admissionv1.AdmissionRequest{
				Operation: admissionv1.Create,
				Object:    runtime.RawExtension{Raw: terminating([]string{}, "")},
			},
			wantAllow: false,
		},
		{
			name: "subresource update denied",
			request: admissionv1.AdmissionRequest{
				Operation:   admissionv1.Update,
				SubResource: "status",
				OldObject:   runtime.RawExtension{Raw: terminating([]string{"a"}, "")},
				Object:      runtime.RawExtension{Raw: terminating([]string{}, "")},
			},
			wantAllow: false,
		},
		{
			name: "decode error on old object denies with error",
			request: admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				OldObject: runtime.RawExtension{Raw: []byte(`not json`)},
				Object:    runtime.RawExtension{Raw: terminating([]string{}, "")},
			},
			wantAllow:  false,
			wantErrSet: true,
		},
		{
			name: "removal plus label change denied by zero-list",
			request: admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				OldObject: runtime.RawExtension{Raw: terminating([]string{"a"}, "")},
				Object: runtime.RawExtension{Raw: []byte(`{
					"metadata": {
						"name": "test",
						"namespace": "",
						"uid": "abc",
						"generation": 1,
						"resourceVersion": "1",
						"deletionTimestamp": "` + now.Format(time.RFC3339) + `",
						"finalizers": [],
						"labels": {"a":"different"}
					},
					"spec": {"foo": "bar"}
				}`)},
			},
			wantAllow: false,
		},
		{
			name: "managedFields and resourceVersion drift ignored in positive case",
			request: admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				OldObject: runtime.RawExtension{Raw: []byte(`{
					"metadata": {
						"name": "test",
						"namespace": "",
						"uid": "abc",
						"generation": 1,
						"resourceVersion": "1",
						"deletionTimestamp": "` + now.Format(time.RFC3339) + `",
						"finalizers": ["a"],
						"labels": {"a":"b"},
						"managedFields": [{"manager":"kubectl"}]
					},
					"spec": {"foo": "bar"}
				}`)},
				Object: runtime.RawExtension{Raw: []byte(`{
					"metadata": {
						"name": "test",
						"namespace": "",
						"uid": "abc",
						"generation": 1,
						"resourceVersion": "2",
						"deletionTimestamp": "` + now.Format(time.RFC3339) + `",
						"finalizers": [],
						"labels": {"a":"b"},
						"managedFields": [{"manager":"different"}]
					},
					"spec": {"foo": "bar"}
				}`)},
			},
			wantAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allow, err := IsFinalizerRemovalOnTerminatingObject(tt.request)
			assert.Equal(t, tt.wantAllow, allow)
			if tt.wantErrSet {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func toJSONArray(values []string) string {
	out := "["
	for i, v := range values {
		if i > 0 {
			out += ","
		}
		out += `"` + v + `"`
	}
	return out + "]"
}

// TestIsFinalizerRemovalOnTerminatingObjectNullPayload pins the nil guard: a JSON "null" body
// decodes to a nil pointer with no error, so without the guard this panics in the admission path
// instead of denying, contradicting the function's documented contract.
func TestIsFinalizerRemovalOnTerminatingObjectNullPayload(t *testing.T) {
	terminating := []byte(`{"metadata":{"deletionTimestamp":"2026-01-01T00:00:00Z","finalizers":["a"]}}`)
	for _, tt := range []struct {
		name      string
		oldRaw    []byte
		objectRaw []byte
	}{
		{"null old object", []byte(`null`), terminating},
		{"null new object", terminating, []byte(`null`)},
		{"both null", []byte(`null`), []byte(`null`)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				OldObject: runtime.RawExtension{Raw: tt.oldRaw},
				Object:    runtime.RawExtension{Raw: tt.objectRaw},
			}
			allowed, err := IsFinalizerRemovalOnTerminatingObject(request)
			assert.NoError(t, err)
			assert.False(t, allowed)
		})
	}
}
