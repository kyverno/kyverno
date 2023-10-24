package api

import (
	"context"
	"io"

	"github.com/google/go-containerregistry/pkg/authn"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	cosignremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	CanI(ctx context.Context, kind, namespace, verb, subresource, user string) (bool, string, error)
}

type ResourceClient interface {
	GetResource(ctx context.Context, apiVersion, kind, namespace, name string, subresources ...string) (*unstructured.Unstructured, error)
	ListResource(ctx context.Context, apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error)
	GetResources(ctx context.Context, group, version, kind, subresource, namespace, name string) ([]Resource, error)
	GetNamespace(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Namespace, error)
	IsNamespaced(group, version, kind string) (bool, error)
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
}

type RemoteClient interface {
	BuildCosignRemoteOption(context.Context) (cosignremote.Option, error)
	BuildGCRRemoteOption(context.Context) ([]gcrremote.Option, error)
}

type RegistryClient interface {
	ImageDataClient
	KeychainClient
	RemoteClient
}
