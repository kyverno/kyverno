package evaluator

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/config"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apiserver/pkg/admission"
	"sigs.k8s.io/yaml"
)

type proofImageContext struct {
	imagedataloader.ImageContext
	images []string
}

func (c *proofImageContext) Get(_ context.Context, image string, _ []remote.Option, _ []name.Option) (*imagedataloader.ImageData, error) {
	c.images = append(c.images, image)
	return &imagedataloader.ImageData{}, nil
}

func TestRegistryEgressProofGuardsResourceImages(t *testing.T) {
	fixture := filepath.Join("..", "..", "..", "..", "test", "conformance", "chainsaw", "image-validating-policies", "standard", "registry-egress-hard-block")
	policyBytes, err := os.ReadFile(filepath.Join(fixture, "policy.yaml"))
	require.NoError(t, err)
	policy := &policiesv1beta1.ImageValidatingPolicy{}
	require.NoError(t, yaml.Unmarshal(policyBytes, policy))
	for _, category := range []string{"containers", "initContainers", "ephemeralContainers", "custom"} {
		t.Run(category, func(t *testing.T) {
			for _, host := range []string{"127.0.0.1:5000", "169.254.169.254"} {
				t.Run(host, func(t *testing.T) {
					resource := map[string]any{"spec": map[string]any{category: []any{map[string]any{"image": host + "/app:latest"}}}}
					candidate := policy.DeepCopy()
					candidate.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "true"}}
					gvr := &metav1.GroupVersionResource{Version: "v1", Resource: "pods"}
					if category == "custom" {
						gvr = nil
						candidate.Spec.ImageExtractors = []policiesv1beta1.ImageExtractor{{Name: category, Expression: "object.spec.custom.map(item, item.image)"}}
					}
					images := &proofImageContext{}
					compiled, errs := NewCompiler(images, nil, gvr, imageverifycache.DisabledImageVerifyCache()).Compile(candidate, nil, nil)
					require.Empty(t, errs)
					_, err := compiled.Evaluate(context.Background(), images, nil, resource, nil, false, nil, libs.NewFakeContextProvider())
					require.ErrorIs(t, err, registryclient.ErrRegistryAddressBlocked)
					require.Empty(t, images.images, "author-populated image data must never satisfy a workload lookup")
				})
			}
		})
	}
}

func TestAuthorChosenImageLookupRemainsNative(t *testing.T) {
	policy := ivpol.DeepCopy()
	policy.Spec.ImageExtractors = nil
	policy.Spec.Validations = []admissionregistrationv1.Validation{{Expression: `getImageData("127.0.0.1:5000/author/image:latest") != null`}}
	images := &proofImageContext{}
	compiled, errs := NewCompiler(images, nil, nil, imageverifycache.DisabledImageVerifyCache()).Compile(policy, nil, nil)
	require.Empty(t, errs)
	result, err := compiled.Evaluate(context.Background(), images, nil, map[string]any{}, nil, false, nil, libs.NewFakeContextProvider())
	require.NoError(t, err)
	require.True(t, result.Result)
	require.Equal(t, []string{"127.0.0.1:5000/author/image:latest"}, images.images)
}

func TestResourceCacheDoesNotReuseAuthorImageData(t *testing.T) {
	server := httptest.NewServer(registry.New())
	defer server.Close()
	image := strings.TrimPrefix(server.URL, "http://") + "/author/image:latest"
	reference, err := name.ParseReference(image, name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(reference, empty.Image))
	nativeOpts, nativeNames := registryclient.GlobalImageDataOptions()
	author, err := imagedataloader.NewImageContext(nil, nativeOpts, nativeNames)
	require.NoError(t, err)
	cached, err := author.Get(context.Background(), image, nil, []name.Option{name.Insecure})
	require.NoError(t, err)
	opts, names := registryclient.ResourceImageOptions()
	resources, err := imageverify.NewResourceImageContext(author, []string{image}, opts, names)
	require.NoError(t, err)
	_, err = resources.Get(context.Background(), image, nil, nil)
	require.ErrorIs(t, err, registryclient.ErrRegistryAddressBlocked)
	retained, err := author.Get(context.Background(), image, nil, nil)
	require.NoError(t, err)
	require.Same(t, cached, retained)
}

func TestResourceImageCallbacksAreGuardedBeforeExtraction(t *testing.T) {
	for _, category := range []string{"containers", "initContainers", "ephemeralContainers"} {
		for _, phase := range []string{"match", "variable", "validation", "mutation match", "mutation"} {
			t.Run(category+"/"+phase, func(t *testing.T) {
				policy := ivpol.DeepCopy()
				policy.Spec.EvaluationConfiguration = nil
				policy.Spec.MatchImageReferences = []policiesv1beta1.MatchImageReference{{Glob: "*"}}
				policy.Spec.ImageExtractors = nil
				policy.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "true"}}
				expression := "getImageData(object.spec." + category + "[0].image) != null"
				switch phase {
				case "match", "mutation match":
					policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "lookup", Expression: expression}}
				case "variable":
					policy.Spec.Variables = []admissionregistrationv1.Variable{{Name: "lookup", Expression: expression}}
					policy.Spec.Validations[0].Expression = "variables.lookup"
				case "validation":
					policy.Spec.Validations[0].Expression = expression
				}
				resource := unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "v1", "kind": "Pod", "metadata": map[string]any{"name": "resource-scope"},
					"spec": map[string]any{category: []any{map[string]any{"name": "app", "image": "127.0.0.1:5000/app:latest"}}},
				}}
				request, attr, _ := buildRequestMapHoistRequestAndAttr(t, admissionv1.Create)
				attr.GetObject().(*unstructured.Unstructured).Object = resource.Object
				images := &proofImageContext{}
				compiled, errs := NewCompiler(images, nil, nil, imageverifycache.DisabledImageVerifyCache()).Compile(policy, nil, nil)
				require.Empty(t, errs)
				var err error
				if phase == "mutation" || phase == "mutation match" {
					_, err = compiled.MutateDigest(context.Background(), images, attr, request, nil, resource, nil, config.NewDefaultConfiguration(false))
				} else {
					_, err = compiled.Evaluate(context.Background(), images, attr, request, nil, true, nil, libs.NewFakeContextProvider())
				}
				require.Error(t, err)
				require.Contains(t, err.Error(), "connection blocked to network host")
				require.Empty(t, images.images)
			})
		}
	}
}

func TestResourceOldImageCallbackIsGuarded(t *testing.T) {
	policy := ivpol.DeepCopy()
	policy.Spec.EvaluationConfiguration = nil
	policy.Spec.ImageExtractors = nil
	policy.Spec.Validations = []admissionregistrationv1.Validation{{Expression: `getImageData(oldObject.spec.ephemeralContainers[0].image) != null`}}
	request, current, _ := buildRequestMapHoistRequestAndAttr(t, admissionv1.Update)
	old := current.GetObject().(*unstructured.Unstructured).DeepCopy()
	old.Object["spec"] = map[string]any{"ephemeralContainers": []any{map[string]any{"name": "debug", "image": "127.0.0.1:5000/app:latest"}}}
	attr := admission.NewAttributesRecord(current.GetObject(), old, current.GetKind(), current.GetNamespace(), current.GetName(), current.GetResource(), current.GetSubresource(), admission.Update, nil, false, nil)
	images := &proofImageContext{}
	compiled, errs := NewCompiler(images, nil, nil, imageverifycache.DisabledImageVerifyCache()).Compile(policy, nil, nil)
	require.Empty(t, errs)
	_, err := compiled.Evaluate(context.Background(), images, attr, request, nil, true, nil, libs.NewFakeContextProvider())
	require.ErrorContains(t, err, "connection blocked to network host")
	require.Empty(t, images.images)
}
