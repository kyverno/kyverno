package resource

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"google.golang.org/protobuf/types/known/structpb"
)

type impl struct {
	types.Adapter
}

func (c *impl) list_resources_string_string_string(args ...ref.Val) ref.Val {
	if len(args) != 4 {
		return types.NewErr("expected 4 arguments, got %d", len(args))
	}
	if self, err := getArg[Context](args, 0); err != nil {
		return err
	} else if apiVersion, err := getArg[string](args, 1); err != nil {
		return err
	} else if resource, err := getArg[string](args, 2); err != nil {
		return err
	} else if namespace, err := getArg[string](args, 3); err != nil {
		return err
	} else {
		list, err := self.ListResources(apiVersion, resource, namespace)
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to list resource: %v", err)
		}
		return c.NativeToValue(list.UnstructuredContent())
	}
}

func (c *impl) get_resource_string_string_string_string(args ...ref.Val) ref.Val {
	if len(args) != 5 {
		return types.NewErr("expected 5 arguments, got %d", len(args))
	}
	if self, err := getArg[Context](args, 0); err != nil {
		return err
	} else if apiVersion, err := getArg[string](args, 1); err != nil {
		return err
	} else if resource, err := getArg[string](args, 2); err != nil {
		return err
	} else if namespace, err := getArg[string](args, 3); err != nil {
		return err
	} else if name, err := getArg[string](args, 4); err != nil {
		return err
	} else {
		res, err := self.GetResource(apiVersion, resource, namespace, name)
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to get resource: %v", err)
		}
		return c.NativeToValue(res.UnstructuredContent())
	}
}

func (c *impl) post_resource_string_string_string_map(args ...ref.Val) ref.Val {
	if len(args) != 5 {
		return types.NewErr("expected 5 arguments, got %d", len(args))
	}
	if self, err := getArg[Context](args, 0); err != nil {
		return err
	} else if apiVersion, err := getArg[string](args, 1); err != nil {
		return err
	} else if resource, err := getArg[string](args, 2); err != nil {
		return err
	} else if namespace, err := getArg[string](args, 3); err != nil {
		return err
	} else if data, err := getArg[*structpb.Struct](args, 4); err != nil {
		return err
	} else {
		res, err := self.PostResource(apiVersion, resource, namespace, data.AsMap())
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to create resource: %v", err)
		}
		return c.NativeToValue(res.UnstructuredContent())
	}
}

func (c *impl) post_resource_string_string_map(args ...ref.Val) ref.Val {
	if len(args) != 4 {
		return types.NewErr("expected 4 arguments, got %d", len(args))
	}
	return c.post_resource_string_string_string_map(args[0], args[1], args[2], types.String(""), args[3])
}
