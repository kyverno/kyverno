package compiler

import (
	"testing"

	"github.com/google/cel-go/cel"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestImageExtractorProfilesMatchRequestCompilation(t *testing.T) {
	t.Parallel()
	env, err := cel.NewEnv(append(DefaultEnvOptions(), cel.Variable(ObjectKey, cel.DynType), cel.Variable(OldObjectKey, cel.DynType))...)
	require.NoError(t, err)
	for _, override := range []bool{false, true} {
		custom := []policiesv1beta1.ImageExtractor{{Name: "custom", Expression: "['custom-image']"}}
		if override {
			custom = append(custom, policiesv1beta1.ImageExtractor{Name: "containers", Expression: "['override-image']"})
		}
		profiles, errs := CompileImageExtractorProfiles(field.NewPath("images"), env, custom...)
		require.Empty(t, errs)
		for _, gvr := range []*metav1.GroupVersionResource{nil, &pods, &jobs, &deployments, &statefulsets, &daemonsets, &replicasets, &cronjobs, {Group: "example.com", Version: "v1", Resource: "workloads"}} {
			legacy, errs := CompileImageExtractors(field.NewPath("images"), env, gvr, custom...)
			require.Empty(t, errs)
			spec := map[string]any{"containers": []any{map[string]any{"image": "pod-image"}}}
			object := map[string]any{"spec": map[string]any{
				"containers":  spec["containers"],
				"template":    map[string]any{"spec": spec},
				"jobTemplate": map[string]any{"spec": map[string]any{"template": map[string]any{"spec": spec}}},
			}}
			for _, deleting := range []bool{false, true} {
				data := map[string]any{ObjectKey: object, OldObjectKey: nil}
				if deleting {
					data[ObjectKey], data[OldObjectKey] = nil, object
				}
				expected, err := ExtractImages(data, legacy)
				require.NoError(t, err)
				actual, err := ExtractImages(data, profiles.ForResource(gvr))
				require.NoError(t, err)
				require.Equal(t, expected, actual)
			}
		}
	}
}
