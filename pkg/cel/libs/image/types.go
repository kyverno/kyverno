package image

import (
	"fmt"
	"reflect"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/go-containerregistry/pkg/name"
)

var ImageType = types.NewOpaqueType("kyverno.image")

type Image struct {
	Reference name.Reference
}

func (v Image) ConvertToNative(typeDesc reflect.Type) (any, error) {
	if reflect.TypeOf(v.Reference).AssignableTo(typeDesc) {
		return v.Reference, nil
	}
	if reflect.TypeOf("").AssignableTo(typeDesc) {
		return v.Reference.String(), nil
	}
	return nil, fmt.Errorf("type conversion error from 'Image' to '%v'", typeDesc)
}

func (v Image) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case ImageType:
		return v
	default:
		return types.NewErr("type conversion error from '%s' to '%s'", ImageType, typeVal)
	}
}

func (v Image) Equal(other ref.Val) ref.Val {
	img, ok := other.(Image)
	if !ok {
		return types.MaybeNoSuchOverloadErr(other)
	}
	return types.Bool(reflect.DeepEqual(v.Reference, img.Reference))
}

func (v Image) Type() ref.Type {
	return ImageType
}

func (v Image) Value() any {
	return v.Reference
}
