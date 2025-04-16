package resource

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
)

type impl struct {
	types.Adapter
}

func (c *impl) list_resources_string_string_string(args ...ref.Val) ref.Val {
	if self, err := utils.ConvertToNative[Context](args[0]); err != nil {
		return types.WrapErr(err)
	} else if apiVersion, err := utils.ConvertToNative[string](args[1]); err != nil {
		return types.WrapErr(err)
	} else if resource, err := utils.ConvertToNative[string](args[2]); err != nil {
		return types.WrapErr(err)
	} else if namespace, err := utils.ConvertToNative[string](args[3]); err != nil {
		return types.WrapErr(err)
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
	if self, err := utils.ConvertToNative[Context](args[0]); err != nil {
		return types.WrapErr(err)
	} else if apiVersion, err := utils.ConvertToNative[string](args[1]); err != nil {
		return types.WrapErr(err)
	} else if resource, err := utils.ConvertToNative[string](args[2]); err != nil {
		return types.WrapErr(err)
	} else if namespace, err := utils.ConvertToNative[string](args[3]); err != nil {
		return types.WrapErr(err)
	} else if name, err := utils.ConvertToNative[string](args[4]); err != nil {
		return types.WrapErr(err)
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
	if self, err := utils.ConvertToNative[Context](args[0]); err != nil {
		return types.WrapErr(err)
	} else if apiVersion, err := utils.ConvertToNative[string](args[1]); err != nil {
		return types.WrapErr(err)
	} else if resource, err := utils.ConvertToNative[string](args[2]); err != nil {
		return types.WrapErr(err)
	} else if namespace, err := utils.ConvertToNative[string](args[3]); err != nil {
		return types.WrapErr(err)
	} else if data, err := utils.ConvertToNative[map[string]any](args[4]); err != nil {
		return types.WrapErr(err)
	} else {
		unpacked, err := UnpackData(data)
		if err != nil {
			return types.NewErr("failed to unpack the provided data: %v", err)
		}

		res, err := self.PostResource(apiVersion, resource, namespace, unpacked)
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to create resource: %v", err)
		}
		return c.NativeToValue(res.UnstructuredContent())
	}
}

func (c *impl) post_resource_string_string_map(args ...ref.Val) ref.Val {
	if self, err := utils.ConvertToNative[Context](args[0]); err != nil {
		return types.WrapErr(err)
	} else if apiVersion, err := utils.ConvertToNative[string](args[1]); err != nil {
		return types.WrapErr(err)
	} else if resource, err := utils.ConvertToNative[string](args[2]); err != nil {
		return types.WrapErr(err)
	} else if data, err := utils.ConvertToNative[map[string]any](args[3]); err != nil {
		return types.WrapErr(err)
	} else {
		unpacked, err := UnpackData(data)
		if err != nil {
			return types.NewErr("failed to unpack the provided data: %v", err)
		}

		res, err := self.PostResource(apiVersion, resource, "", unpacked)
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to create resource: %v", err)
		}
		return c.NativeToValue(res.UnstructuredContent())
	}
}
