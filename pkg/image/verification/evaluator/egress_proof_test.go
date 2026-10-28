package evaluator

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engine "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/config"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"
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
					spec := map[string]any{"containers": []any{}}
					spec[category] = []any{map[string]any{"image": host + "/app:latest"}}
					resource := map[string]any{"spec": spec}
					candidate := policy.DeepCopy()
					if category == "custom" {
						candidate.Spec.ImageExtractors = []policiesv1beta1.ImageExtractor{{Name: category, Expression: "object.spec.custom.map(item, item.image)"}}
						candidate.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "images.custom.all(image, getImageData(image) != null)"}}
					}
					images := &proofImageContext{}
					compiled, errs := NewCompiler(images, nil, nil, imageverifycache.DisabledImageVerifyCache()).Compile(candidate, nil, nil)
					require.Empty(t, errs)
					_, err := compiled.Evaluate(context.Background(), images, nil, resource, nil, false, nil, libs.NewFakeContextProvider())
					require.ErrorContains(t, err, "connection blocked to network host")
					require.Empty(t, images.images, "author-populated image data must never satisfy a workload lookup")
				})
			}
		})
	}
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

type scopeOldObjectAttributes struct {
	admission.Attributes
	old runtime.Object
}

func (attributes scopeOldObjectAttributes) GetOldObject() runtime.Object { return attributes.old }

type metadataScopeContext struct {
	libs.Context
	lookups []string
}

func (provider *metadataScopeContext) GetImageData(image string, _ []remote.Option) (map[string]any, error) {
	provider.lookups = append(provider.lookups, image)
	return map[string]any{}, nil
}

type imageExtractorEvaluationCounter struct {
	cel.Program
	calls int
}

func (extractor *imageExtractorEvaluationCounter) ContextEval(ctx context.Context, variables any) (ref.Val, *cel.EvalDetails, error) {
	extractor.calls++
	return extractor.Program.ContextEval(ctx, variables)
}

func TestResourceImageFetchUsesIsolatedProvider(test *testing.T) {
	const image = "93.184.216.34/workload:tag"
	policy := ivpol.DeepCopy()
	policy.Spec.MatchImageReferences[0].Glob = "*"
	policy.Spec.ValidationConfigurations.Required = ptr.To(false)
	policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "image", Expression: "getImageData(object.foo.bar) != null"}}
	policy.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "getImageData(images.bar[0]) != null"}}
	author := &proofImageContext{}
	workload := &proofImageContext{}
	factoryCalls := 0
	images := imageverify.WithResourceImageContextFactory(author, func() (imagedataloader.ImageContext, error) {
		factoryCalls++
		return workload, nil
	})
	compiled, errs := NewCompiler(images, nil, nil, imageverifycache.DisabledImageVerifyCache()).Compile(policy, nil, nil)
	require.Empty(test, errs)
	policyImpl := compiled.(*compiledPolicy)
	extractor := policyImpl.imageExtractors["bar"]
	counter := &imageExtractorEvaluationCounter{Program: extractor.Program}
	extractor.Program = counter
	policyImpl.imageExtractors["bar"] = extractor
	result, err := compiled.Evaluate(context.Background(), images, nil, obj(image), nil, false, nil, libs.NewFakeContextProvider())
	require.NoError(test, err)
	require.NotNil(test, result)
	require.True(test, result.Result)
	require.Equal(test, 1, counter.calls)
	require.Equal(test, 1, factoryCalls)
	require.Empty(test, author.images)
	require.Equal(test, []string{image, image}, workload.images)
}

func TestResourceImageLookupsGuardSelectedFields(test *testing.T) {
	for _, testCase := range []struct {
		source, field, function, phase string
	}{
		{"object", "containers", "getImageData", "validation"},
		{"object", "initContainers", "image.GetMetadata", "match"},
		{"object", "ephemeralContainers", "image.GetMetadata", "validation"},
		{"oldObject", "containers", "image.GetMetadata", "validation"},
		{"oldObject", "initContainers", "getImageData", "match"},
		{"oldObject", "ephemeralContainers", "getImageData", "validation"},
		{"object", "artifact", "getImageData", "validation"},
		{"oldObject", "artifact", "image.GetMetadata", "validation"},
		{"object", "artifact", "getImageData", "mutation match"},
		{"oldObject", "artifact", "image.GetMetadata", "mutation match"},
		{"object", "containers", "getImageData", "variable"},
		{"oldObject", "initContainers", "image.GetMetadata", "variable"},
		{"object", "artifact", "getImageData", "canonical match"},
		{"object", "artifact", "image.GetMetadata", "canonical match"},
	} {
		test.Run(testCase.source+"/"+testCase.field+"/"+testCase.function+"/"+testCase.phase, func(test *testing.T) {
			request, attributes, pod := buildRequestMapHoistRequestAndAttr(test, admissionv1.Update)
			old := pod.DeepCopy()
			resource := &pod
			if testCase.source == "oldObject" {
				resource = old
			}
			const workloadImage = "169.254.169.254/workload"
			lookup := testCase.source + ".spec." + testCase.field
			policy := &policiesv1beta1.ImageValidatingPolicy{Spec: policiesv1beta1.ImageValidatingPolicySpec{
				ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{Required: ptr.To(false), VerifyDigest: ptr.To(false)},
				Validations:              []admissionregistrationv1.Validation{{Expression: "true"}},
			}}
			if testCase.field == "artifact" {
				require.NoError(test, unstructured.SetNestedField(resource.Object, workloadImage, "spec", testCase.field))
				policy.Spec.ImageExtractors = []policiesv1beta1.ImageExtractor{{Name: "custom", Expression: "[" + lookup + "]"}}
				if testCase.phase == "validation" {
					lookup = "images.custom[0]"
				}
			} else {
				require.NoError(test, unstructured.SetNestedSlice(resource.Object, []any{map[string]any{"name": "workload", "image": workloadImage}}, "spec", testCase.field))
				lookup += "[0].image"
			}
			if testCase.phase == "canonical match" {
				lookup = strconv.Quote(workloadImage + ":latest")
			}
			lookup = testCase.function + "(" + lookup + ")"
			if testCase.phase == "validation" {
				policy.Spec.Validations[0].Expression = lookup + " != null"
			} else if testCase.phase == "variable" {
				policy.Spec.Variables = []admissionregistrationv1.Variable{{Name: "lookup", Expression: lookup + " != null"}}
				policy.Spec.Validations[0].Expression = "variables.lookup"
			} else {
				policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "image", Expression: lookup + " == null"}}
			}
			provider := &metadataScopeContext{Context: libs.NewFakeContextProvider()}
			images := &proofImageContext{}
			compiled, errs := NewCompiler(images, nil, nil, imageverifycache.DisabledImageVerifyCache()).Compile(policy, nil, nil)
			require.Empty(test, errs)
			compiled.(*compiledPolicy).imageContext = provider
			attributes = scopeOldObjectAttributes{Attributes: attributes, old: old}
			var err error
			if testCase.phase == "mutation match" {
				patches, mutationErr := compiled.MutateDigest(context.Background(), images, attributes, request, nil, pod, nil, config.NewDefaultConfiguration(false))
				require.Empty(test, patches)
				err = mutationErr
			} else {
				_, err = compiled.Evaluate(context.Background(), images, attributes, request, nil, true, nil, provider)
			}
			require.ErrorContains(test, err, "connection blocked to network host")
			require.Empty(test, images.images)
			require.Empty(test, provider.lookups)
		})
	}
}

func TestResourceDigestMutationGuardsAllContainerKinds(test *testing.T) {
	for _, field := range []string{"containers", "initContainers", "ephemeralContainers"} {
		test.Run(field, func(test *testing.T) {
			compiled, errs := NewCompiler(nil, nil, nil, imageverifycache.DisabledImageVerifyCache()).Compile(&policiesv1beta1.ImageValidatingPolicy{}, nil, nil)
			require.Empty(test, errs)
			request, attributes, pod := buildRequestMapHoistRequestAndAttr(test, admissionv1.Create)
			require.NoError(test, unstructured.SetNestedSlice(pod.Object, []any{map[string]any{"name": "main", "image": "169.254.169.254/image:tag"}}, "spec", field))
			images := &proofImageContext{}
			patches, err := compiled.MutateDigest(context.Background(), images, attributes, request, nil, pod, nil, config.NewDefaultConfiguration(false))
			require.ErrorContains(test, err, "connection blocked to network host")
			require.Empty(test, patches)
			require.Empty(test, images.images)
		})
	}
}

func TestPolicyImageLiteralIgnoresUnrelatedResourceStrings(test *testing.T) {
	const policyImage = "169.254.169.254/author:tag"
	for _, function := range []string{"getImageData", "image.GetMetadata"} {
		for _, phase := range []string{"evaluation", "mutation match"} {
			test.Run(function+"/"+phase, func(test *testing.T) {
				lookup := function + "(" + strconv.Quote(policyImage) + ")"
				policy := &policiesv1beta1.ImageValidatingPolicy{Spec: policiesv1beta1.ImageValidatingPolicySpec{
					ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{Required: ptr.To(false), VerifyDigest: ptr.To(false)},
					ImageExtractors:          []policiesv1beta1.ImageExtractor{{Name: "custom", Expression: "[" + strconv.Quote(policyImage) + "]"}},
					MatchConditions:          []admissionregistrationv1.MatchCondition{{Name: "image", Expression: lookup + " != null"}},
					Validations:              []admissionregistrationv1.Validation{{Expression: lookup + " != null"}, {Expression: function + "(images.custom[0]) != null"}},
				}}
				if phase == "mutation match" {
					policy.Spec.MatchConditions[0].Expression = lookup + " == null"
				}
				provider := &metadataScopeContext{Context: libs.NewFakeContextProvider()}
				images := &proofImageContext{}
				compiled, errs := NewCompiler(images, nil, nil, imageverifycache.DisabledImageVerifyCache()).Compile(policy, nil, nil)
				require.Empty(test, errs)
				compiled.(*compiledPolicy).imageContext = provider
				request, attributes, pod := buildRequestMapHoistRequestAndAttr(test, admissionv1.Update)
				for _, path := range [][]string{{"metadata", "annotations", "description"}, {"metadata", "labels", "description"}, {"spec", "description"}} {
					require.NoError(test, unstructured.SetNestedField(pod.Object, policyImage, path...))
				}
				require.NoError(test, unstructured.SetNestedSlice(pod.Object, []any{map[string]any{"name": "workload", "image": "169.254.169.254/workload:tag"}}, "spec", "containers"))
				old := pod.DeepCopy()
				attributes = scopeOldObjectAttributes{Attributes: attributes, old: old}
				var err error
				request.Object.Raw, err = pod.MarshalJSON()
				require.NoError(test, err)
				request.OldObject.Raw, err = old.MarshalJSON()
				require.NoError(test, err)
				if phase == "mutation match" {
					patches, err := compiled.MutateDigest(context.Background(), images, attributes, request, nil, pod, nil, config.NewDefaultConfiguration(false))
					require.NoError(test, err)
					require.Empty(test, patches)
				} else {
					result, err := compiled.Evaluate(context.Background(), images, attributes, request, nil, true, nil, provider)
					require.NoError(test, err)
					require.True(test, result.Result)
				}
				if function == "getImageData" {
					require.NotEmpty(test, images.images)
					for _, image := range images.images {
						require.Equal(test, policyImage, image)
					}
					require.Empty(test, provider.lookups)
				} else {
					require.Equal(test, []string{policyImage}, provider.lookups)
					require.Empty(test, images.images)
				}
			})
		}
	}
}

func TestResourceImageReferencesUseSelectedPaths(test *testing.T) {
	test.Parallel()
	data := map[string]any{
		engine.ObjectKey: map[string]any{
			"metadata": map[string]string{"name": "169.254.169.254/author:tag"},
			"spec": map[string]any{
				"containers": []map[string]string{{"image": "169.254.169.254/container:tag", "name": "169.254.169.254/author:tag"}},
				"unrelated":  "169.254.169.254/author:tag",
				"artifacts":  []map[string]string{{"image": "169.254.169.254/custom:tag", "description": "169.254.169.254/author:tag"}},
			},
			"foo": map[string]string{"bar": "169.254.169.254/typed:tag"},
		},
		engine.OldObjectKey: map[string]any{"spec": map[string]any{
			"ephemeralContainers": []map[string]string{{"image": "169.254.169.254/old:tag"}},
		}},
	}
	images := resourceImageReferences(data, [][]string{
		{engine.ObjectKey, "spec", "artifacts", "*", "image"},
		{engine.ObjectKey, "foo", "bar"},
	})
	require.ElementsMatch(test, []string{
		"169.254.169.254/container:tag",
		"169.254.169.254/custom:tag",
		"169.254.169.254/typed:tag",
		"169.254.169.254/old:tag",
	}, images)
}

func TestResourceImagePathsResolveVariableAliases(test *testing.T) {
	test.Parallel()
	env, err := cel.NewEnv(cel.Variable(engine.ObjectKey, cel.DynType), cel.Variable(engine.VariablesKey, cel.DynType))
	require.NoError(test, err)
	paths := compileResourceImagePaths(env, map[string]engine.ImageExtractor{
		"custom": {ResourcePaths: [][]string{{engine.VariablesKey, "alias"}, {engine.VariablesKey, "literal"}}},
	}, []admissionregistrationv1.Variable{
		{Name: "image", Expression: "object.spec.artifact"},
		{Name: "alias", Expression: "variables.image"},
		{Name: "literal", Expression: "'169.254.169.254/author:tag'"},
	})
	require.Equal(test, [][]string{{engine.ObjectKey, "spec", "artifact"}}, paths)
}
