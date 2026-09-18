package extract

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func container(name, image string) map[string]any {
	return map[string]any{"name": name, "image": image}
}

func stripSegments(in []Extracted) []Extracted {
	out := make([]Extracted, len(in))
	for i, e := range in {
		e.Segments = nil
		out[i] = e
	}
	return out
}

func TestExtractPodTemplates(t *testing.T) {
	tests := []struct {
		name string
		obj  map[string]any
		want []Extracted
	}{
		{
			name: "jobset with a single replicatedJob, no metadata on the pod template",
			obj: map[string]any{
				"apiVersion": "jobset.x-k8s.io/v1alpha2",
				"kind":       "JobSet",
				"metadata":   map[string]any{"name": "latest-tag-jobset"},
				"spec": map[string]any{
					"replicatedJobs": []any{
						map[string]any{
							"name": "workers",
							"template": map[string]any{
								"spec": map[string]any{
									"parallelism": int64(1),
									"completions": int64(1),
									"template": map[string]any{
										"spec": map[string]any{
											"containers":    []any{container("worker", "bash:latest")},
											"restartPolicy": "Never",
										},
									},
								},
							},
						},
					},
				},
			},
			want: []Extracted{
				{
					Path: "spec.replicatedJobs[0].template.spec.template",
					Template: map[string]any{
						"spec": map[string]any{
							"containers":    []any{container("worker", "bash:latest")},
							"restartPolicy": "Never",
						},
					},
				},
			},
		},
		{
			name: "deployment - sanity check against the existing built-in shape",
			obj: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"spec": map[string]any{
					"template": map[string]any{
						"metadata": map[string]any{"labels": map[string]any{"app": "nginx"}},
						"spec": map[string]any{
							"containers": []any{container("nginx", "nginx:1.27")},
						},
					},
				},
			},
			want: []Extracted{
				{
					Path: "spec.template",
					Template: map[string]any{
						"metadata": map[string]any{"labels": map[string]any{"app": "nginx"}},
						"spec": map[string]any{
							"containers": []any{container("nginx", "nginx:1.27")},
						},
					},
				},
			},
		},
		{
			name: "pod-shaped subtree under status is ignored - only spec is searched",
			obj: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Rollout",
				"metadata":   map[string]any{"name": "canary-rollout"},
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{container("app", "app:v2")},
						},
					},
				},
				"status": map[string]any{
					"canary": map[string]any{
						"currentStepPodTemplate": map[string]any{
							"spec": map[string]any{
								"containers": []any{container("app", "app:v1-stale")},
							},
						},
					},
				},
			},
			want: []Extracted{
				{
					Path: "spec.template",
					Template: map[string]any{
						"spec": map[string]any{
							"containers": []any{container("app", "app:v2")},
						},
					},
				},
			},
		},
		{
			name: "no spec at all yields no templates",
			obj: map[string]any{
				"apiVersion": "example.io/v1",
				"kind":       "Widget",
				"metadata":   map[string]any{"name": "widget"},
			},
			want: nil,
		},
		{
			name: "decoy object with no pod-shaped subtree",
			obj: map[string]any{
				"apiVersion": "example.io/v1",
				"kind":       "Widget",
				"spec": map[string]any{
					"replicas": int64(3),
					"selector": map[string]any{"app": "widget"},
				},
			},
			want: nil,
		},
		{
			name: "jobset with multiple replicatedJobs yields one Extracted per entry",
			obj: map[string]any{
				"apiVersion": "jobset.x-k8s.io/v1alpha2",
				"kind":       "JobSet",
				"spec": map[string]any{
					"replicatedJobs": []any{
						map[string]any{
							"name": "leader",
							"template": map[string]any{
								"spec": map[string]any{
									"template": map[string]any{
										"spec": map[string]any{
											"containers": []any{container("leader", "leader:v1")},
										},
									},
								},
							},
						},
						map[string]any{
							"name": "workers",
							"template": map[string]any{
								"spec": map[string]any{
									"template": map[string]any{
										"spec": map[string]any{
											"containers": []any{container("worker", "worker:v1")},
										},
									},
								},
							},
						},
					},
				},
			},
			want: []Extracted{
				{
					Path: "spec.replicatedJobs[0].template.spec.template",
					Template: map[string]any{
						"spec": map[string]any{
							"containers": []any{container("leader", "leader:v1")},
						},
					},
				},
				{
					Path: "spec.replicatedJobs[1].template.spec.template",
					Template: map[string]any{
						"spec": map[string]any{
							"containers": []any{container("worker", "worker:v1")},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPodTemplates(tt.obj)
			assert.ElementsMatch(t, tt.want, stripSegments(got))
		})
	}
}

func TestIsPodTemplateSpec(t *testing.T) {
	tests := []struct {
		name string
		v    map[string]any
		want bool
	}{
		{
			name: "valid, with metadata",
			v: map[string]any{
				"metadata": map[string]any{},
				"spec":     map[string]any{"containers": []any{container("a", "img:v1")}},
			},
			want: true,
		},
		{
			name: "valid, metadata absent entirely",
			v: map[string]any{
				"spec": map[string]any{"containers": []any{container("a", "img:v1")}},
			},
			want: true,
		},
		{
			name: "metadata present but wrong type",
			v: map[string]any{
				"metadata": "not-an-object",
				"spec":     map[string]any{"containers": []any{container("a", "img:v1")}},
			},
			want: false,
		},
		{
			name: "no spec",
			v:    map[string]any{"metadata": map[string]any{}},
			want: false,
		},
		{
			name: "empty containers list",
			v:    map[string]any{"spec": map[string]any{"containers": []any{}}},
			want: false,
		},
		{
			name: "container missing image",
			v:    map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"name": "a"}}}},
			want: false,
		},
		{
			name: "container missing name",
			v:    map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"image": "img:v1"}}}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isPodTemplateSpec(tt.v))
		})
	}
}

func TestExtractPodTemplates_Segments(t *testing.T) {
	obj := map[string]any{
		"apiVersion": "jobset.x-k8s.io/v1alpha2",
		"kind":       "JobSet",
		"spec": map[string]any{
			"replicatedJobs": []any{
				map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{container("leader", "leader:v1")},
								},
							},
						},
					},
				},
				map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{container("worker", "worker:v1")},
								},
							},
						},
					},
				},
			},
		},
	}

	got := ExtractPodTemplates(obj)
	if !assert.Len(t, got, 2) {
		return
	}

	// find leader/worker regardless of walk order (maps don't guarantee order)
	var leader, worker *Extracted
	for i := range got {
		containers, _, _ := unstructuredContainers(got[i].Template)
		if len(containers) > 0 && containers[0]["name"] == "leader" {
			leader = &got[i]
		}
		if len(containers) > 0 && containers[0]["name"] == "worker" {
			worker = &got[i]
		}
	}
	if !assert.NotNil(t, leader) || !assert.NotNil(t, worker) {
		return
	}

	assert.Equal(t, []PathSegment{
		{Key: "spec"},
		{Key: "replicatedJobs"},
		{Index: 0, IsIdx: true},
		{Key: "template"},
		{Key: "spec"},
		{Key: "template"},
	}, leader.Segments)
	assert.Equal(t, "/spec/replicatedJobs/0/template/spec/template", leader.JSONPointerPrefix())

	assert.Equal(t, []PathSegment{
		{Key: "spec"},
		{Key: "replicatedJobs"},
		{Index: 1, IsIdx: true},
		{Key: "template"},
		{Key: "spec"},
		{Key: "template"},
	}, worker.Segments)
	assert.Equal(t, "/spec/replicatedJobs/1/template/spec/template", worker.JSONPointerPrefix())
}

func unstructuredContainers(tpl map[string]any) ([]map[string]any, bool, error) {
	spec, ok := tpl["spec"].(map[string]any)
	if !ok {
		return nil, false, nil
	}
	raw, ok := spec["containers"].([]any)
	if !ok {
		return nil, false, nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, c := range raw {
		if m, ok := c.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, true, nil
}
