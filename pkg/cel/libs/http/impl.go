package http

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"google.golang.org/protobuf/types/known/structpb"
)

type impl struct {
	types.Adapter
}

func (c *impl) get_request_with_client_string(args ...ref.Val) ref.Val {
	if len(args) != 3 {
		return types.NewErr("expected 3 arguments, got %d", len(args))
	}
	if request, err := utils.GetArg[Context](args, 0); err != nil {
		return err
	} else if url, err := utils.GetArg[string](args, 1); err != nil {
		return err
	} else if header, err := utils.GetArg[map[string]string](args, 2); err != nil {
		return err
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
	if len(args) != 4 {
		return types.NewErr("expected 4 arguments, got %d", len(args))
	}
	if request, err := utils.GetArg[Context](args, 0); err != nil {
		return err
	} else if url, err := utils.GetArg[string](args, 1); err != nil {
		return err
	} else if data, err := utils.GetArg[*structpb.Value](args, 2); err != nil {
		return err
	} else if header, err := utils.GetArg[map[string]string](args, 3); err != nil {
		return err
	} else {
		data, err := request.Post(url, data, header)
		if err != nil {
			return types.NewErr("request failed: %v", err)
		}
		return c.NativeToValue(data)
	}
}

func (c *impl) post_request_string(args ...ref.Val) ref.Val {
	return c.post_request_string_with_client(args[0], args[1], args[2], c.NativeToValue(make(map[string]string, 0)))
}

func (c *impl) post_request_with_headers_string(args ...ref.Val) ref.Val {
	return c.post_request_string_with_client(args...)
}

func (c *impl) http_client_string(request, caBundle ref.Val) ref.Val {
	if request, err := utils.ConvertToNative[Context](request); err != nil {
		return types.NewErr("invalid arg %d: %v", 0, err)
	} else if caBundle, err := utils.ConvertToNative[string](caBundle); err != nil {
		return types.NewErr("invalid arg %d: %v", 1, err)
	} else {
		caRequest, err := request.Client(caBundle)
		if err != nil {
			return types.NewErr("request failed: %v", err)
		}
		return c.NativeToValue(caRequest)
	}
}
