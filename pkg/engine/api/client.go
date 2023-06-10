package api

import (
	"context"
	"io"

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
	GetResources(group, version, kind, subresource, namespace, name string) ([]Resource, error)
}

type Client interface {
	RawClient
	AuthClient
	ResourceClient
}
