package extract

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetAtPath(t *testing.T) {
	tests := []struct {
		name    string
		root    map[string]any
		path    string
		value   map[string]any
		want    map[string]any
		wantErr string
	}{
		{
			name: "single map key",
			root: map[string]any{
				"spec": map[string]any{"containers": []any{}},
			},
			path:  "spec",
			value: map[string]any{"replaced": true},
			want: map[string]any{
				"spec": map[string]any{"replaced": true},
			},
		},
		{
			name: "nested map keys",
			root: map[string]any{
				"spec": map[string]any{
					"template": map[string]any{"spec": map[string]any{"containers": []any{}}},
				},
			},
			path:  "spec.template",
			value: map[string]any{"metadata": map[string]any{"labels": map[string]any{"a": "b"}}},
			want: map[string]any{
				"spec": map[string]any{
					"template": map[string]any{"metadata": map[string]any{"labels": map[string]any{"a": "b"}}},
				},
			},
		},
		{
			name: "array index in the middle of the path",
			root: map[string]any{
				"spec": map[string]any{
					"replicatedJobs": []any{
						map[string]any{
							"name":     "workers",
							"template": map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{}}}},
						},
					},
				},
			},
			path:  "spec.replicatedJobs[0].template.spec.template",
			value: map[string]any{"spec": map[string]any{"containers": []any{"mutated"}}},
			want: map[string]any{
				"spec": map[string]any{
					"replicatedJobs": []any{
						map[string]any{
							"name":     "workers",
							"template": map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{"mutated"}}}}},
						},
					},
				},
			},
		},
		{
			name: "only the addressed array element is touched",
			root: map[string]any{
				"spec": map[string]any{
					"jobs": []any{
						map[string]any{"template": map[string]any{"marker": "untouched-0"}},
						map[string]any{"template": map[string]any{"marker": "untouched-1"}},
					},
				},
			},
			path:  "spec.jobs[1].template",
			value: map[string]any{"marker": "replaced-1"},
			want: map[string]any{
				"spec": map[string]any{
					"jobs": []any{
						map[string]any{"template": map[string]any{"marker": "untouched-0"}},
						map[string]any{"template": map[string]any{"marker": "replaced-1"}},
					},
				},
			},
		},
		{
			name:    "empty path",
			root:    map[string]any{},
			path:    "",
			value:   map[string]any{},
			wantErr: "empty path",
		},
		{
			name:    "missing intermediate key",
			root:    map[string]any{"spec": map[string]any{}},
			path:    "spec.template.metadata",
			value:   map[string]any{},
			wantErr: `missing key "template"`,
		},
		{
			name:    "type mismatch: expected object, found array",
			root:    map[string]any{"spec": []any{}},
			path:    "spec.template",
			value:   map[string]any{},
			wantErr: `expected an object at "template"`,
		},
		{
			name:    "type mismatch: expected array for index",
			root:    map[string]any{"spec": map[string]any{"jobs": map[string]any{}}},
			path:    "spec.jobs[0]",
			value:   map[string]any{},
			wantErr: "expected an array at index 0",
		},
		{
			name:    "array index out of range",
			root:    map[string]any{"spec": map[string]any{"jobs": []any{map[string]any{}}}},
			path:    "spec.jobs[3]",
			value:   map[string]any{},
			wantErr: "index 3 out of range (len 1)",
		},
		{
			name:    "malformed path: unterminated bracket",
			root:    map[string]any{"spec": map[string]any{}},
			path:    "spec.jobs[0",
			value:   map[string]any{},
			wantErr: "unterminated '['",
		},
		{
			name:    "malformed path: non-numeric index",
			root:    map[string]any{"spec": map[string]any{}},
			path:    "spec.jobs[x]",
			value:   map[string]any{},
			wantErr: "invalid index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetAtPath(tt.root, tt.path, tt.value)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, tt.root)
		})
	}
}

// TestSetAtPath_RoundTripWithExtractPodTemplates verifies SetAtPath is a
// true inverse of the paths ExtractPodTemplates discovers: writing a new
// template back at its own Path and re-extracting finds the new content at
// the same Path, with sibling templates left untouched.
func TestSetAtPath_RoundTripWithExtractPodTemplates(t *testing.T) {
	obj := map[string]any{
		"apiVersion": "jobset.x-k8s.io/v1alpha2",
		"kind":       "JobSet",
		"spec": map[string]any{
			"replicatedJobs": []any{
				map[string]any{
					"name":     "workers",
					"template": map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{container("worker", "bash:latest")}}}}},
				},
				map[string]any{
					"name":     "drivers",
					"template": map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{container("driver", "bash:1.0")}}}}},
				},
			},
		},
	}

	before := ExtractPodTemplates(obj)
	require.Len(t, before, 2)

	mutated := map[string]any{
		"spec": map[string]any{"containers": []any{container("worker", "bash:1.0")}},
	}
	require.NoError(t, SetAtPath(obj, before[0].Path, mutated))

	after := ExtractPodTemplates(obj)
	require.Len(t, after, 2)
	assert.Equal(t, before[0].Path, after[0].Path)
	assert.Equal(t, mutated, after[0].Template)
	// the sibling template must be untouched
	assert.Equal(t, before[1].Template, after[1].Template)
}
