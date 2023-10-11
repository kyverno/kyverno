package notary

import (
	"context"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/images"
	notationregistry "github.com/notaryproject/notation-go/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type parsedReference struct {
	Repo       notationregistry.Repository
	RemoteOpts []gcrremote.Option
	Ref        name.Reference
	Desc       ocispec.Descriptor
}

func parseReferenceCrane(ctx context.Context, ref string, registryClient images.Client) (*parsedReference, error) {
	nameRef, err := name.ParseReference(ref)
	if err != nil {
		return nil, err
	}

	remoteOpts, err := registryClient.BuildGCRRemoteOption(ctx)
	if err != nil {
		return nil, err
	}

	desc, err := gcrremote.Head(nameRef, remoteOpts...)
	if err != nil {
		return nil, err
	}

	if !isDigestReference(ref) {
		nameRef, err = name.ParseReference(GetReferenceFromDescriptor(v1ToOciSpecDescriptor(*desc), nameRef))
		if err != nil {
			return nil, err
		}
	}

	repository := NewRepository(remoteOpts, nameRef)
	err = resolveDigestCrane(repository, remoteOpts, nameRef)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve digest")
	}

	return &parsedReference{
		Repo:       repository,
		RemoteOpts: remoteOpts,
		Ref:        nameRef,
		Desc:       v1ToOciSpecDescriptor(*desc),
	}, nil
}

func isDigestReference(reference string) bool {
	parts := strings.SplitN(reference, "/", 2)
	if len(parts) == 1 {
		return false
	}

	index := strings.Index(parts[1], "@")
	return index != -1
}

func resolveDigestCrane(repo notationregistry.Repository, remoteOpts []gcrremote.Option, ref name.Reference) error {
	_, err := repo.Resolve(context.Background(), ref.Identifier())
	if err != nil {
		return err
	}
	return nil
}
