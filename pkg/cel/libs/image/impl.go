package image

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/go-containerregistry/pkg/name"
)

func isImage(arg ref.Val) ref.Val {
	str, ok := arg.Value().(string)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	_, err := name.ParseReference(str)
	return types.Bool(err == nil)
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
	return Image{v}
}

func imageContainsDigest(arg ref.Val) ref.Val {
	v, ok := arg.Value().(name.Reference)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	if digest, ok := v.(name.Digest); ok {
		return types.Bool(len(digest.DigestStr()) != 0)
	}
	return types.False
}

func imageRegistry(arg ref.Val) ref.Val {
	v, ok := arg.Value().(name.Reference)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	return types.String(v.Context().RegistryStr())
}

func imageRepository(arg ref.Val) ref.Val {
	v, ok := arg.Value().(name.Reference)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	return types.String(v.Context().RepositoryStr())
}

func imageIdentifier(arg ref.Val) ref.Val {
	v, ok := arg.Value().(name.Reference)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	return types.String(v.Identifier())
}

func imageTag(arg ref.Val) ref.Val {
	v, ok := arg.Value().(name.Reference)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	var tag string
	if v, ok := v.(name.Tag); ok {
		tag = v.TagStr()
	}
	return types.String(tag)
}

func imageDigest(arg ref.Val) ref.Val {
	v, ok := arg.Value().(name.Reference)
	if !ok {
		return types.MaybeNoSuchOverloadErr(arg)
	}
	var digest string
	if v, ok := v.(name.Digest); ok {
		digest = v.DigestStr()
	}
	return types.String(digest)
}
