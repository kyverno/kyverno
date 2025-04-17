package image

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/go-containerregistry/pkg/name"
)

const libraryName = "kyverno.image"

func ImageLib() cel.EnvOption {
	return cel.Lib(imageLib)
}

var imageLib = &imageLibType{}

type imageLibType struct{}

func (*imageLibType) LibraryName() string {
	return libraryName
}

func (*imageLibType) Types() []*cel.Type {
	return []*cel.Type{ImageType}
}

func (*imageLibType) declarations() map[string][]cel.FunctionOpt {
	return map[string][]cel.FunctionOpt{
		"image": {
			cel.Overload(
				"string_to_image",
				[]*cel.Type{cel.StringType},
				ImageType,
				cel.UnaryBinding(stringToImage),
			),
		},
		"isImage": {
			cel.Overload(
				"is_image_string",
				[]*cel.Type{cel.StringType},
				cel.BoolType,
				cel.UnaryBinding(isImage),
			),
		},
		"containsDigest": {
			cel.MemberOverload(
				"image_contains_digest",
				[]*cel.Type{ImageType},
				cel.BoolType,
				cel.UnaryBinding(imageContainsDigest),
			),
		},
		"registry": {
			cel.MemberOverload(
				"image_registry",
				[]*cel.Type{ImageType},
				cel.StringType,
				cel.UnaryBinding(imageRegistry),
			),
		},
		"repository": {
			cel.MemberOverload(
				"image_repository",
				[]*cel.Type{ImageType},
				cel.StringType,
				cel.UnaryBinding(imageRepository),
			),
		},
		"identifier": {
			cel.MemberOverload(
				"image_identifier",
				[]*cel.Type{ImageType},
				cel.StringType,
				cel.UnaryBinding(imageIdentifier),
			),
		},
		"tag": {
			cel.MemberOverload(
				"image_tag",
				[]*cel.Type{ImageType},
				cel.StringType,
				cel.UnaryBinding(imageTag),
			),
		},
		"digest": {
			cel.MemberOverload(
				"image_digest",
				[]*cel.Type{ImageType},
				cel.StringType,
				cel.UnaryBinding(imageDigest),
			),
		},
	}
}

func (i *imageLibType) CompileOptions() []cel.EnvOption {
	imageLibraryDecls := i.declarations()
	options := make([]cel.EnvOption, 0, len(imageLibraryDecls))
	for name, overloads := range imageLibraryDecls {
		options = append(options, cel.Function(name, overloads...))
	}
	return options
}

func (*imageLibType) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

func isImage(arg ref.Val) ref.Val {
	str, ok := arg.Value().(string)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}

	_, err := name.ParseReference(str)
	if err != nil {
		return types.Bool(false)
	}

	return types.Bool(true)
}

func stringToImage(arg ref.Val) ref.Val {
	str, ok := arg.Value().(string)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}

	v, err := name.ParseReference(str)
	if err != nil {
		return types.WrapErr(err)
	}

	return Image{ImageReference: ConvertToImageRef(v)}
}

func imageContainsDigest(arg ref.Val) ref.Val {
	v, ok := arg.Value().(ImageReference)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	return types.Bool(len(v.Digest) != 0)
}

func imageRegistry(arg ref.Val) ref.Val {
	v, ok := arg.Value().(ImageReference)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	return types.String(v.Registry)
}

func imageRepository(arg ref.Val) ref.Val {
	v, ok := arg.Value().(ImageReference)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	return types.String(v.Repository)
}

func imageIdentifier(arg ref.Val) ref.Val {
	v, ok := arg.Value().(ImageReference)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	return types.String(v.Identifier)
}

func imageTag(arg ref.Val) ref.Val {
	v, ok := arg.Value().(ImageReference)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	return types.String(v.Tag)
}

func imageDigest(arg ref.Val) ref.Val {
	v, ok := arg.Value().(ImageReference)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	return types.String(v.Digest)
}

func ConvertToImageRef(ref name.Reference) ImageReference {
	var img ImageReference
	img.Image = ref.String()
	img.Registry = ref.Context().RegistryStr()
	img.Repository = ref.Context().RepositoryStr()
	img.Identifier = ref.Identifier()

	if _, ok := ref.(name.Tag); ok {
		img.Tag = ref.Identifier()
	} else {
		img.Digest = ref.Identifier()
	}

	return img
}
