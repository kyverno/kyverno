package notary

import (
	"context"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/images"
	notationregistry "github.com/notaryproject/notation-go/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type parsedReference struct {
	Repo       notationregistry.Repository
	CraneOpts  crane.Option
	RemoteOpts []gcrremote.Option
	Ref        name.Reference
	Desc       ocispec.Descriptor
}

func parseReferenceCrane(ctx context.Context, ref string, registryClient images.Client) (*parsedReference, error) {
	nameRef, err := name.ParseReference(ref)
	if err != nil {
		return nil, err
	}

	authenticator, err := getAuthenticator(ctx, ref, registryClient)
	if err != nil {
		return nil, err
	}

	craneOpts := crane.WithAuth(*authenticator)
	remoteOpts, err := getRemoteOpts(*authenticator)
	if err != nil {
		return nil, err
	}

	desc, err := crane.Head(ref, craneOpts)
	if err != nil {
		return nil, err
	}

	if !isDigestReference(ref) {
		nameRef, err = name.ParseReference(GetReferenceFromDescriptor(v1ToOciSpecDescriptor(*desc), nameRef))
		if err != nil {
			return nil, err
		}
	}

	repository := NewRepository(craneOpts, remoteOpts, nameRef)
	err = resolveDigestCrane(repository, craneOpts, remoteOpts, nameRef)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve digest")
	}

	return &parsedReference{
		Repo:       repository,
		CraneOpts:  craneOpts,
		RemoteOpts: remoteOpts,
		Ref:        nameRef,
		Desc:       v1ToOciSpecDescriptor(*desc),
	}, nil
}

type imageResource struct {
	ref name.Reference
}

func (ir *imageResource) String() string {
	return ir.ref.Name()
}

func (ir *imageResource) RegistryStr() string {
	return ir.ref.Context().RegistryStr()
}

func getAuthenticator(ctx context.Context, ref string, registryClient images.Client) (*authn.Authenticator, error) {
	parsedRef, err := name.ParseReference(ref)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse registry reference %s", ref)
	}

	if err := registryClient.RefreshKeychainPullSecrets(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to refresh image pull secrets")
	}

	authn, err := registryClient.Keychain().Resolve(&imageResource{parsedRef})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve auth for %s", parsedRef.String())
	}
	return &authn, nil
}

func isDigestReference(reference string) bool {
	parts := strings.SplitN(reference, "/", 2)
	if len(parts) == 1 {
		return false
	}

	index := strings.Index(parts[1], "@")
	return index != -1
}

func getRemoteOpts(authenticator authn.Authenticator) ([]gcrremote.Option, error) {
	remoteOpts := []gcrremote.Option{}
	remoteOpts = append(remoteOpts, gcrremote.WithAuth(authenticator))

	pusher, err := gcrremote.NewPusher(remoteOpts...)
	if err != nil {
		return nil, err
	}
	remoteOpts = append(remoteOpts, gcrremote.Reuse(pusher))

	puller, err := gcrremote.NewPuller(remoteOpts...)
	if err != nil {
		return nil, err
	}
	remoteOpts = append(remoteOpts, gcrremote.Reuse(puller))

	return remoteOpts, nil
}

func resolveDigestCrane(repo notationregistry.Repository, craneOpts crane.Option, remoteOpts []gcrremote.Option, ref name.Reference) error {
	_, err := repo.Resolve(context.Background(), ref.Name())
	if err != nil {
		return err
	}
	return nil
}
