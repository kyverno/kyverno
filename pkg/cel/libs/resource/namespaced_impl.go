package resource

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type namespacedImpl struct {
	namespace string
	types.Adapter
}

func (c *namespacedImpl) list_resources_string_string(args ...ref.Val) ref.Val {
	if len(args) != 3 {
		return types.NewErr("expected 3 arguments, got %d", len(args))
	}
	if self, err := utils.GetArg[Context](args, 0); err != nil {
		return err
	} else if apiVersion, err := utils.GetArg[string](args, 1); err != nil {
		return err
	} else if resource, err := utils.GetArg[string](args, 2); err != nil {
		return err
	} else {
		list, err := self.ListResources(apiVersion, resource, c.namespace, nil)
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to list resource: %v", err)
		}
		return c.NativeToValue(list.UnstructuredContent())
	}
}

func (c *namespacedImpl) list_resources_string_string_map(args ...ref.Val) ref.Val {
	if len(args) != 4 {
		return types.NewErr("expected 4 arguments, got %d", len(args))
	}
	if self, err := utils.GetArg[Context](args, 0); err != nil {
		return err
	} else if apiVersion, err := utils.GetArg[string](args, 1); err != nil {
		return err
	} else if resource, err := utils.GetArg[string](args, 2); err != nil {
		return err
	} else if labels, err := utils.GetArg[map[string]string](args, 3); err != nil {
		return err
	} else {
		list, err := self.ListResources(apiVersion, resource, c.namespace, labels)
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to list resource: %v", err)
		}
		return c.NativeToValue(list.UnstructuredContent())
	}
}

func (c *namespacedImpl) list_resources_gvr(args ...ref.Val) ref.Val {
	if len(args) != 2 {
		return types.NewErr("expected 2 arguments, got %d", len(args))
	}
	if gvr, err := utils.GetArg[*schema.GroupVersionResource](args, 1); err != nil {
		return err
	} else {
		return c.list_resources_string_string(args[0], types.String(gvr.GroupVersion().String()), types.String(gvr.Resource))
	}
}

func (c *namespacedImpl) list_resources_gvr_map(args ...ref.Val) ref.Val {
	if len(args) != 3 {
		return types.NewErr("expected 3 arguments, got %d", len(args))
	}
	if gvr, err := utils.GetArg[*schema.GroupVersionResource](args, 1); err != nil {
		return err
	} else {
		return c.list_resources_string_string_map(args[0], types.String(gvr.GroupVersion().String()), types.String(gvr.Resource), args[2])
	}
}

func (c *namespacedImpl) get_resource_string_string_string(args ...ref.Val) ref.Val {
	if len(args) != 4 {
		return types.NewErr("expected 4 arguments, got %d", len(args))
	}
	if self, err := utils.GetArg[Context](args, 0); err != nil {
		return err
	} else if apiVersion, err := utils.GetArg[string](args, 1); err != nil {
		return err
	} else if resource, err := utils.GetArg[string](args, 2); err != nil {
		return err
	} else if name, err := utils.GetArg[string](args, 3); err != nil {
		return err
	} else {
		res, err := self.GetResource(apiVersion, resource, c.namespace, name)
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to get resource: %v", err)
		}
		return c.NativeToValue(res.UnstructuredContent())
	}
}

func (c *namespacedImpl) get_resources_gvr_string(args ...ref.Val) ref.Val {
	if len(args) != 3 {
		return types.NewErr("expected 3 arguments, got %d", len(args))
	}
	if gvr, err := utils.GetArg[*schema.GroupVersionResource](args, 1); err != nil {
		return err
	} else {
		return c.get_resource_string_string_string(args[0], types.String(gvr.GroupVersion().String()), types.String(gvr.Resource), args[2])
	}
}

func (c *namespacedImpl) post_resource_string_string_map(args ...ref.Val) ref.Val {
	if len(args) != 4 {
		return types.NewErr("expected 4 arguments, got %d", len(args))
	}
	if self, err := utils.GetArg[Context](args, 0); err != nil {
		return err
	} else if apiVersion, err := utils.GetArg[string](args, 1); err != nil {
		return err
	} else if resource, err := utils.GetArg[string](args, 2); err != nil {
		return err
	} else if data, err := utils.GetArg[*structpb.Struct](args, 3); err != nil {
		return err
	} else {
		res, err := self.PostResource(apiVersion, resource, c.namespace, data.AsMap())
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to create resource: %v", err)
		}
		return c.NativeToValue(res.UnstructuredContent())
	}
}

func (c *namespacedImpl) convert_to_gvr_string_string(args ...ref.Val) ref.Val {
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
