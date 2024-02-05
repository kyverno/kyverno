package apicall

import (
	"context"
	"io"
)

type ClientInterface interface {
	RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error)
}
