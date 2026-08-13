package loaders

import (
	gocontext "context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"gotest.tools/assert"
)

// Minimal RegistryClient: only ForRef is ever called on the path this test
// exercises. The other three methods exist to satisfy the interface.
type stubRegistryClient struct{}

func (stubRegistryClient) ForRef(_ gocontext.Context, ref string) (*engineapi.ImageData, error) {
	return &engineapi.ImageData{
		Image:  ref,
		Config: []byte(`{}`),
		// Manifest is unmarshalled unconditionally in fetchImageDataMap.
		Manifest: []byte(`{}`),
	}, nil
}

func (stubRegistryClient) FetchImageDescriptor(gocontext.Context, string) (*gcrremote.Descriptor, error) {
	panic("not used by this test")
}

func (stubRegistryClient) Keychain() authn.Keychain {
	panic("not used by this test")
}

func (stubRegistryClient) Options(gocontext.Context) ([]gcrremote.Option, error) {
	panic("not used by this test")
}

func (stubRegistryClient) NameOptions() []name.Option {
	panic("not used by this test")
}

type stubRegistryClientFactory struct{}

func (stubRegistryClientFactory) GetClient(gocontext.Context, *kyvernov1.ImageRegistryCredentials, string, []string) (engineapi.RegistryClient, error) {
	return stubRegistryClient{}, nil
}

// The JMESPath field, like the image Reference above it, is substituted
// before use. A JMESPath template that resolves to a non-string (a number,
// bool, map, or slice) used to panic on the unguarded assertion instead of
// returning an error. A working registry client stub is needed so the
// unfixed code actually reaches that assertion, rather than panicking
// earlier on a nil rclientFactory.
func Test_ImageDataLoader_WithNonStringJMESPathSubstitution(t *testing.T) {
	entry := kyvernov1.ContextEntry{
		Name: "testimg",
		ImageRegistry: &kyvernov1.ImageRegistry{
			Reference: "ghcr.io/kyverno/kyverno:latest",
			JMESPath:  "{{ pathVar }}",
		},
	}
	ctx := enginecontext.NewContext(jp)
	assert.NilError(t, ctx.AddContextEntry("pathVar", []byte(`123`)))

	ldr := &imageDataLoader{
		ctx:            gocontext.TODO(),
		logger:         logr.Discard(),
		entry:          entry,
		enginectx:      ctx,
		jp:             jp,
		rclientFactory: stubRegistryClientFactory{},
	}

	_, err := ldr.fetchImageData()
	assert.ErrorContains(t, err, "invalid JMESPath")
	assert.ErrorContains(t, err, "testimg")
}
