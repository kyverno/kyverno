package api

import (
	"context"
	"io"

	"github.com/google/go-containerregistry/pkg/authn"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/pkg/oci/remote"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Resource struct {
	Group        string
	Version      string
	Resource     string
	SubResource  string
	Unstructured unstructured.Unstructured
}

type RawClient interface {
	RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error)
}

type AuthClient interface {
	CanI(ctx context.Context, kind, namespace, verb, subresource, user string) (bool, error)
}

type ResourceClient interface {
	GetResource(ctx context.Context, apiVersion, kind, namespace, name string, subresources ...string) (*unstructured.Unstructured, error)
	GetResources(ctx context.Context, group, version, kind, subresource, namespace, name string) ([]Resource, error)
}

type Client interface {
	RawClient
	AuthClient
	ResourceClient
}

type ImageData struct {
	Image         string
	ResolvedImage string
	Registry      string
	Repository    string
	Identifier    string
	Manifest      []byte
	Config        []byte
}

type ImageDataClient interface {
	ForRef(ctx context.Context, ref string) (*ImageData, error)
	FetchImageDescriptor(context.Context, string) (*gcrremote.Descriptor, error)
}

type KeychainClient interface {
	Keychain() authn.Keychain
	RefreshKeychainPullSecrets(ctx context.Context) error
}

type CosignClient interface {
	BuildRemoteOption(context.Context) remote.Option
}

type RegistryClient interface {
	ImageDataClient
	KeychainClient
	CosignClient
}
