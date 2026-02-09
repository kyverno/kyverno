package generator

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"google.golang.org/protobuf/types/known/structpb"
)

type impl struct {
	types.Adapter
}

func (c *impl) apply_generator_string_list(args ...ref.Val) ref.Val {
	if self, err := utils.GetArg[Context](args, 0); err != nil {
		return err
	} else if namespace, err := utils.GetArg[string](args, 1); err != nil {
		return err
	} else if dataList, err := utils.GetArg[[]*structpb.Struct](args, 2); err != nil {
		return err
	} else {
		var resources []map[string]any
		for _, data := range dataList {
			resources = append(resources, data.AsMap())
		}
		if err := self.GenerateResources(namespace, resources); err != nil {
			return types.NewErr("failed to generate resources: %v", err)
		}
		return types.True
	}
}
