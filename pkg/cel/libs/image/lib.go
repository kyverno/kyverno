package image

import (
	"github.com/google/cel-go/cel"
)

const libraryName = "kyverno.image"

func Lib() cel.EnvOption {
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
		// DEPRECATED: alias for backward compatibility — use parseImageReference() instead
		"image": {
			cel.Overload(
				"string_to_image_deprecated",
				[]*cel.Type{cel.StringType},
				ImageType,
				cel.UnaryBinding(stringToImage),
			),
		},
		"parseImageReference": {
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
