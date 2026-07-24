package apicall

import (
	"context"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ClientInterface interface {
	RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error)
	GetResource(ctx context.Context, apiVersion, kind, namespace, name string, subresources ...string) (*unstructured.Unstructured, error)
}
