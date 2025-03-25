package http

import (
	"net/http"

	"github.com/google/cel-go/common/types"
)

var HTTPType = types.NewOpaqueType("http.HTTP")

type HttpInterface interface {
	Get(url string, headers map[string]string) (map[string]any, error)
	Post(url string, data map[string]any, headers map[string]string) (map[string]any, error)
	Client(caBundle string) (HttpInterface, error)
}

type HTTP struct {
	HttpInterface
}

func NewHTTP() HTTP {
	return HTTP{
		HttpInterface: &httpProvider{
			client: http.DefaultClient,
		},
	}
}
