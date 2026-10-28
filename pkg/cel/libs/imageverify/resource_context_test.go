package imageverify

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type acceptingVerificationCache struct {
	imageverifycache.Client
	calls int
}

func (cache *acceptingVerificationCache) Get(context.Context, metav1.Object, string, string, bool) (bool, error) {
	cache.calls++
	return true, nil
}

func (cache *acceptingVerificationCache) GetWithPayload(context.Context, metav1.Object, string, string, bool) (bool, map[string][]byte, error) {
	cache.calls++
	return true, nil, nil
}

func TestResourceVerificationCacheCannotBypassDestinationCheck(test *testing.T) {
	test.Parallel()
	const blockedImage = "127.0.0.1:5000/app@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	const allowedImage = "198.51.100.10/app@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	for _, expression := range []string{
		`verifyImageSignatures(image, [attestors.notary])`,
		`verifyAttestationSignatures(image, "sbom", [attestors.notary])`,
	} {
		for _, scenario := range []struct {
			name     string
			image    string
			resource string
			blocked  bool
		}{
			{name: "author", image: blockedImage, resource: "127.0.0.1:5000/another:latest"},
			{name: "blocked resource", image: blockedImage, resource: blockedImage, blocked: true},
			{name: "resource alias", image: "127.0.0.1:5000/app:latest", resource: "127.0.0.1:5000/app", blocked: true},
			{name: "allowed resource", image: allowedImage, resource: allowedImage},
		} {
			test.Run(expression+"/"+scenario.name, func(test *testing.T) {
				test.Parallel()
				images := &recordingImageContext{}
				factoryCalls := 0
				author := WithResourceImageContextFactory(images, func() (imagedataloader.ImageContext, error) {
					factoryCalls++
					return nil, errors.New("image loading must remain deferred")
				})
				opts, names := registryclient.ResourceImageOptions()
				scoped, err := NewResourceImageContext(author, []string{scenario.resource}, opts, names)
				require.NoError(test, err)
				cache := &acceptingVerificationCache{}
				env, err := cel.NewEnv(cel.Variable("image", cel.StringType),
					cel.Variable("attestors", cel.MapType(cel.StringType, cel.DynType)),
					Lib(nil, author, ivpol, nil, logr.Discard(), cache, NewImageVerificationResults()))
				require.NoError(test, err)
				ast, issues := env.Compile(expression)
				require.Nil(test, issues)
				program, err := env.Program(ast)
				require.NoError(test, err)
				_, _, err = program.Eval(map[string]any{
					"image":           scenario.image,
					"attestors":       map[string]policiesv1beta1.Attestor{"notary": ivpol.Spec.Attestors[0]},
					RequestContextKey: NewEvaluationContext(context.Background(), scoped),
				})
				if scenario.blocked {
					require.ErrorContains(test, err, "connection blocked to network host")
					require.Zero(test, cache.calls)
				} else {
					require.NoError(test, err)
					require.Equal(test, 1, cache.calls)
				}
				require.Empty(test, images.contexts)
				require.Zero(test, factoryCalls)
			})
		}
	}
}

func TestResourceVerificationLoadsInjectedContext(test *testing.T) {
	test.Parallel()
	const resourceImage = "198.51.100.10/workload:latest"
	for _, expression := range []string{
		`verifyImageSignatures(image, [])`,
		`verifyAttestationSignatures(image, "sbom", [])`,
	} {
		test.Run(expression, func(test *testing.T) {
			test.Parallel()
			native := &recordingImageContext{}
			resource := &recordingImageContext{data: &imagedataloader.ImageData{}}
			factoryCalls := 0
			author := WithResourceImageContextFactory(native, func() (imagedataloader.ImageContext, error) {
				factoryCalls++
				return resource, nil
			})
			options, nameOptions := registryclient.ResourceImageOptions()
			scoped, err := NewResourceImageContext(author, []string{resourceImage}, options, nameOptions)
			require.NoError(test, err)
			env, err := cel.NewEnv(cel.Variable("image", cel.StringType), Lib(nil, author, ivpol, nil, logr.Discard(), nil, NewImageVerificationResults()))
			require.NoError(test, err)
			ast, issues := env.Compile(expression)
			require.Nil(test, issues)
			program, err := env.Program(ast)
			require.NoError(test, err)
			ctx := context.WithValue(context.Background(), requestMarker{}, expression)
			result, _, err := program.Eval(map[string]any{
				"image":           resourceImage,
				RequestContextKey: NewEvaluationContext(ctx, scoped),
			})
			require.NoError(test, err)
			require.Equal(test, int64(0), result.Value())
			require.Empty(test, native.contexts)
			require.Equal(test, []context.Context{ctx}, resource.contexts)
			require.Equal(test, 1, factoryCalls)
		})
	}
}

type factoryImageContext struct {
	recordingImageContext
	options     []remote.Option
	nameOptions []name.Option
}

func (images *factoryImageContext) Get(ctx context.Context, image string, options []remote.Option, nameOptions []name.Option) (*imagedataloader.ImageData, error) {
	images.options = options
	images.nameOptions = nameOptions
	return images.recordingImageContext.Get(ctx, image, options, nameOptions)
}

func TestResourceImageScopes(test *testing.T) {
	test.Parallel()

	const resourceImage = "198.51.100.10/workload:latest"
	const blockedImage = "169.254.169.254/workload:latest"
	const policyImage = "169.254.169.254/policy:latest"
	ctx := context.WithValue(context.Background(), requestMarker{}, "resource-factory")
	native := &factoryImageContext{recordingImageContext: recordingImageContext{data: &imagedataloader.ImageData{Digest: "native"}}}
	var resourceContexts []*factoryImageContext
	images := WithResourceImageContextFactory(native, func() (imagedataloader.ImageContext, error) {
		resources := &factoryImageContext{recordingImageContext: recordingImageContext{data: &imagedataloader.ImageData{Digest: "resource"}}}
		resourceContexts = append(resourceContexts, resources)
		return resources, nil
	})
	options, nameOptions := registryclient.ResourceImageOptions()
	nativeOptions := []remote.Option{remote.WithUserAgent("author")}
	nativeNames := []name.Option{name.Insecure}
	empty, err := NewResourceImageContext(images, nil, options, nameOptions)
	require.NoError(test, err)
	require.Same(test, images, empty)
	for evaluation := range 2 {
		for _, image := range []string{resourceImage, blockedImage} {
			data, err := images.Get(ctx, image, nativeOptions, nativeNames)
			require.NoError(test, err)
			require.Same(test, native.data, data)
		}
		scoped, err := NewResourceImageContext(images, []string{resourceImage, blockedImage, "nginx"}, options, nameOptions)
		require.NoError(test, err)
		extended, err := NewResourceImageContext(scoped, []string{resourceImage}, options, nameOptions)
		require.NoError(test, err)
		require.Same(test, scoped, extended)
		require.Len(test, resourceContexts, evaluation)
		_, err = scoped.Get(ctx, blockedImage, nativeOptions, nativeNames)
		require.ErrorIs(test, err, registryclient.ErrRegistryAddressBlocked)
		require.Len(test, resourceContexts, evaluation, "blocked images must not create a resource provider")
		data, err := scoped.Get(ctx, resourceImage, nativeOptions, nativeNames)
		require.NoError(test, err)
		require.Len(test, resourceContexts, evaluation+1)
		resources := resourceContexts[evaluation]
		require.Same(test, resources.data, data)
		require.NotSame(test, native.data, data)
		require.Len(test, resources.options, len(options))
		require.Len(test, resources.nameOptions, len(nameOptions))
		require.Equal(test, []context.Context{ctx}, resources.contexts)
		if evaluation > 0 {
			require.NotSame(test, resourceContexts[evaluation-1].data, data)
		}
		resourceScope := scoped.(*resourceImageContext)
		require.True(test, resourceScope.isResourceImage("docker.io/library/nginx:latest"))
		require.True(test, resourceScope.isResourceImage("nginx:latest"))
		require.False(test, resourceScope.isResourceImage("docker.io/library/busybox:latest"))
		_, err = NewResourceImageContext(scoped, []string{"docker.io/library/busybox:latest"}, options, nameOptions)
		require.NoError(test, err)
		require.True(test, resourceScope.isResourceImage("busybox"))
		data, err = scoped.Get(ctx, policyImage, nativeOptions, nativeNames)
		require.NoError(test, err)
		require.Same(test, native.data, data)
		require.Len(test, native.options, len(nativeOptions))
		require.Len(test, native.nameOptions, len(nativeNames))
		require.NoError(test, scoped.AddImages(ctx, []string{resourceImage}, nativeOptions, nativeNames))
		require.Len(test, resourceContexts, evaluation+1, "resource lookups must reuse the evaluation's provider")
		require.Len(test, resources.contexts, 2)
		require.Len(test, native.contexts, 3*(evaluation+1), "workload images must not reuse author data")
	}
	for _, scenario := range []struct {
		name    string
		images  imagedataloader.ImageContext
		message string
	}{
		{name: "unsupported provider", images: native, message: "does not support resource image isolation"},
		{name: "unconfigured factory", images: WithResourceImageContextFactory(native, nil), message: "resource image context factory is not configured"},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			test.Parallel()
			scoped, err := NewResourceImageContext(scenario.images, []string{resourceImage}, options, nameOptions)
			require.NoError(test, err)
			_, err = scoped.Get(ctx, resourceImage, nativeOptions, nativeNames)
			require.ErrorContains(test, err, scenario.message)
			require.Len(test, native.contexts, 6, "unsupported contexts must not fall back to author or network loading")
		})
	}
}

func TestResourceImageContextFactoryFailureCanRetry(test *testing.T) {
	test.Parallel()

	const resourceImage = "198.51.100.10/workload:latest"
	const blockedImage = "169.254.169.254/workload:latest"
	for testName, factoryError := range map[string]error{"error": errors.New("resource context unavailable"), "nil context": nil} {
		test.Run(testName, func(test *testing.T) {
			test.Parallel()
			factoryCalls := 0
			newResource := func() (imagedataloader.ImageContext, error) { return nil, factoryError }
			images := WithResourceImageContextFactory(&recordingImageContext{}, func() (imagedataloader.ImageContext, error) {
				factoryCalls++
				return newResource()
			})
			options, nameOptions := registryclient.ResourceImageOptions()
			scoped, err := NewResourceImageContext(images, []string{resourceImage, blockedImage}, options, nameOptions)
			require.NoError(test, err)
			ctx := context.Background()
			_, err = scoped.Get(ctx, blockedImage, nil, nil)
			require.ErrorIs(test, err, registryclient.ErrRegistryAddressBlocked)
			require.Zero(test, factoryCalls)
			_, err = scoped.Get(ctx, resourceImage, nil, nil)
			if factoryError != nil {
				require.ErrorIs(test, err, factoryError)
			} else {
				require.ErrorContains(test, err, "resource image context factory returned nil")
			}
			require.Equal(test, 1, factoryCalls)
			require.Nil(test, scoped.(*resourceImageContext).resource)
			newResource = func() (imagedataloader.ImageContext, error) {
				return &recordingImageContext{data: &imagedataloader.ImageData{}}, nil
			}
			for range 2 {
				_, err = scoped.Get(ctx, resourceImage, nil, nil)
				require.NoError(test, err)
			}
			require.Equal(test, 2, factoryCalls)
		})
	}
}
