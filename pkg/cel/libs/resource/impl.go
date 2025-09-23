package resource

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type impl struct {
	types.Adapter
}

func (c *impl) list_resources_string_string_string(args ...ref.Val) ref.Val {
	if len(args) != 4 {
		return types.NewErr("expected 4 arguments, got %d", len(args))
	}
	if self, err := utils.GetArg[Context](args, 0); err != nil {
		return err
	} else if apiVersion, err := utils.GetArg[string](args, 1); err != nil {
		return err
	} else if resource, err := utils.GetArg[string](args, 2); err != nil {
		return err
	} else if namespace, err := utils.GetArg[string](args, 3); err != nil {
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

func (c *impl) list_resources_gvr_string(args ...ref.Val) ref.Val {
	if len(args) != 3 {
		return types.NewErr("expected 3 arguments, got %d", len(args))
	}
	if gvr, err := utils.GetArg[*schema.GroupVersionResource](args, 1); err != nil {
		return err
	} else {
		return c.post_resource_string_string_string_map(args[0], types.String(gvr.GroupVersion().String()), types.String(gvr.Resource), args[2])
	}
}

func (c *impl) get_resource_string_string_string_string(args ...ref.Val) ref.Val {
	if len(args) != 5 {
		return types.NewErr("expected 5 arguments, got %d", len(args))
	}
	if self, err := utils.GetArg[Context](args, 0); err != nil {
		return err
	} else if apiVersion, err := utils.GetArg[string](args, 1); err != nil {
		return err
	} else if resource, err := utils.GetArg[string](args, 2); err != nil {
		return err
	} else if namespace, err := utils.GetArg[string](args, 3); err != nil {
		return err
	} else if name, err := utils.GetArg[string](args, 4); err != nil {
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

func (c *impl) get_resources_gvr_string_string(args ...ref.Val) ref.Val {
	if len(args) != 4 {
		return types.NewErr("expected 4 arguments, got %d", len(args))
	}
	if gvr, err := utils.GetArg[*schema.GroupVersionResource](args, 1); err != nil {
		return err
	} else {
		return c.get_resource_string_string_string_string(args[0], types.String(gvr.GroupVersion().String()), types.String(gvr.Resource), args[2], args[3])
	}
}

func (c *impl) post_resource_string_string_string_map(args ...ref.Val) ref.Val {
	if len(args) != 5 {
		return types.NewErr("expected 5 arguments, got %d", len(args))
	}
	if self, err := utils.GetArg[Context](args, 0); err != nil {
		return err
	} else if apiVersion, err := utils.GetArg[string](args, 1); err != nil {
		return err
	} else if resource, err := utils.GetArg[string](args, 2); err != nil {
		return err
	} else if namespace, err := utils.GetArg[string](args, 3); err != nil {
		return err
	} else if data, err := utils.GetArg[*structpb.Struct](args, 4); err != nil {
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

func (c *impl) convert_to_gvr_string_string(args ...ref.Val) ref.Val {
	if len(args) != 3 {
		return types.NewErr("expected 2 arguments, got %d", len(args))
	}
	if self, err := utils.GetArg[Context](args, 0); err != nil {
		return err
	} else if apiVersion, err := utils.GetArg[string](args, 1); err != nil {
		return err
	} else if resource, err := utils.GetArg[string](args, 2); err != nil {
		return err
	} else {
		gvr, err := self.ToGVR(apiVersion, resource)
		if err != nil {
			return types.NewErr("failed to map kind to resource: %v", err)
		}
		return c.NativeToValue(gvr)
	}
}
