package http

import (
	"fmt"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"google.golang.org/protobuf/types/known/structpb"
)

type impl struct {
	types.Adapter
}

func (c *impl) get_request_with_client_string(args ...ref.Val) ref.Val {
	if request, err := utils.ConvertToNative[Context](args[0]); err != nil {
		return types.WrapErr(err)
	} else if url, err := utils.ConvertToNative[string](args[1]); err != nil {
		return types.WrapErr(err)
	} else if header, err := utils.ConvertToNative[map[string]string](args[2]); err != nil {
		return types.WrapErr(err)
	} else {
		data, err := request.Get(url, header)
		if err != nil {
			return types.NewErr("request failed: %v", err)
		}
		return c.NativeToValue(data)
	}
}

func (c *impl) get_request_string(request, url ref.Val) ref.Val {
	return c.get_request_with_client_string(request, url, c.NativeToValue(make(map[string]string, 0)))
}

func (c *impl) get_request_with_headers_string(args ...ref.Val) ref.Val {
	return c.get_request_with_client_string(args...)
}

func (c *impl) post_request_string_with_client(args ...ref.Val) ref.Val {
	if request, err := utils.ConvertToNative[Context](args[0]); err != nil {
		return types.WrapErr(err)
	} else if url, err := utils.ConvertToNative[string](args[1]); err != nil {
		return types.WrapErr(err)
	} else if data, err := utils.ConvertToNative[*structpb.Value](args[2]); err != nil {
		return types.WrapErr(err)
	} else if header, err := utils.ConvertToNative[map[string]string](args[3]); err != nil {
		return types.WrapErr(err)
	} else {
		data, err := request.Post(url, data, header)
		if err != nil {
			return types.NewErr("request failed: %v", err)
		}
		return c.NativeToValue(data)
	}
}

func (c *impl) http_client_string(request, caBundle ref.Val) ref.Val {
	fmt.Println("http_client_string")
	if request, err := utils.ConvertToNative[Context](request); err != nil {
		fmt.Println("conv request")
		return types.WrapErr(err)
	} else if caBundle, err := utils.ConvertToNative[string](caBundle); err != nil {
		fmt.Println("conv ca bundle")
		return types.WrapErr(err)
	} else {
		fmt.Println("call client")
		caRequest, err := request.Client(caBundle)
		if err != nil {
			return types.NewErr("request failed: %v", err)
		}
		return c.NativeToValue(caRequest)
	}
}

func (c *impl) post_request_string(args ...ref.Val) ref.Val {
	return c.post_request_string_with_client(args[0], args[1], args[2], c.NativeToValue(make(map[string]string, 0)))
}

func (c *impl) post_request_with_headers_string(args ...ref.Val) ref.Val {
	return c.post_request_string_with_client(args...)
}
