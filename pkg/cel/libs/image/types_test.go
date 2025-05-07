package image_test

import (
	"reflect"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kyverno/kyverno/pkg/cel/libs/image"
	"github.com/stretchr/testify/assert"
)

func TestImage_ConvertToNative(t *testing.T) {
	tests := []struct {
		name      string
		reference name.Reference
		typeDesc  reflect.Type
		want      any
		wantErr   bool
	}{{
		name:      "string",
		reference: name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:latest"),
		typeDesc:  reflect.TypeFor[string](),
		want:      "registry.k8s.io/kube-apiserver-arm64:latest",
		wantErr:   false,
	}, {
		name:      "reference",
		reference: name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:latest"),
		typeDesc:  reflect.TypeFor[name.Reference](),
		want:      name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:latest"),
		wantErr:   false,
	}, {
		name:      "bool",
		reference: name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:latest"),
		typeDesc:  reflect.TypeFor[bool](),
		want:      nil,
		wantErr:   true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := image.Image{
				Reference: tt.reference,
			}
			got, err := v.ConvertToNative(tt.typeDesc)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestImage_ConvertToType(t *testing.T) {
	tests := []struct {
		name      string
		reference name.Reference
		typeVal   ref.Type
		want      ref.Val
	}{{
		name:      "valid",
		reference: name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:latest"),
		typeVal:   image.ImageType,
		want:      image.Image{Reference: name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:latest")},
	}, {
		name:      "invalid",
		reference: name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:latest"),
		typeVal:   cel.StringType,
		want:      types.NewErr("type conversion error from 'kyverno.image' to 'string'"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := image.Image{
				Reference: tt.reference,
			}
			got := v.ConvertToType(tt.typeVal)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestImage_Equal(t *testing.T) {
	tests := []struct {
		name      string
		reference name.Reference
		other     ref.Val
		want      ref.Val
	}{{
		name:      "not an image",
		reference: name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:latest"),
		other:     types.String("foo"),
		want:      types.MaybeNoSuchOverloadErr(types.String("foo")),
	}, {
		name:      "same image",
		reference: name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:latest"),
		other:     image.Image{Reference: name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:latest")},
		want:      types.True,
	}, {
		name:      "different image",
		reference: name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:not-latest"),
		other:     image.Image{Reference: name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:latest")},
		want:      types.False,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := image.Image{
				Reference: tt.reference,
			}
			got := v.Equal(tt.other)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestImage_Type(t *testing.T) {
	v := image.Image{
		Reference: name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:latest"),
	}
	assert.Equal(t, image.ImageType, v.Type())
}

func TestImage_Value(t *testing.T) {
	v := image.Image{
		Reference: name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:latest"),
	}
	assert.Equal(t, name.MustParseReference("registry.k8s.io/kube-apiserver-arm64:latest"), v.Value())
}
