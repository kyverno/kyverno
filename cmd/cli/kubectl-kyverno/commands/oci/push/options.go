package push

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/kyverno/kyverno/pkg/config"
	policyutils "github.com/kyverno/kyverno/pkg/utils/policy"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
)

type options struct {
	imageRef string
}

func (o options) validate(policy string) error {
	if o.imageRef == "" {
		return errors.New("image is required")
	}
	if policy == "" {
		return errors.New("policy is required")
	}
	return nil
}

func (o options) execute(ctx context.Context, dir string, keychain authn.Keychain) error {
	policies, _, err := policy.Load(nil, "", dir)
	if err != nil {
		return fmt.Errorf("unable to read policy file or directory %s (%w)", dir, err)
	}
	for _, policy := range policies {
		if _, err := policyvalidation.Validate(policy, nil, nil, true, config.KyvernoUserName(config.KyvernoServiceAccountName())); err != nil {
			return fmt.Errorf("validating policy %s: %v", policy.GetName(), err)
		}
	}
	img := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
	img = mutate.ConfigMediaType(img, internal.PolicyConfigMediaType)
	ref, err := name.ParseReference(o.imageRef)
	if err != nil {
		return fmt.Errorf("parsing image reference: %v", err)
	}
	for _, policy := range policies {
		if policy.IsNamespaced() {
			fmt.Fprintf(os.Stderr, "Adding policy [%s]\n", policy.GetName())
		} else {
			fmt.Fprintf(os.Stderr, "Adding cluster policy [%s]\n", policy.GetName())
		}
		policyBytes, err := policyutils.ToYaml(policy)
		if err != nil {
			return fmt.Errorf("converting policy to yaml: %v", err)
		}
		policyLayer := static.NewLayer(policyBytes, internal.PolicyLayerMediaType)
		img, err = mutate.Append(img, mutate.Addendum{
			Layer:       policyLayer,
			Annotations: internal.Annotations(policy),
		})
		if err != nil {
			return fmt.Errorf("mutating image: %v", err)
		}
	}
	fmt.Fprintf(os.Stderr, "Uploading [%s]...\n", ref.Name())
	if err = remote.Write(ref, img, remote.WithContext(ctx), remote.WithAuthFromKeychain(keychain)); err != nil {
		return fmt.Errorf("writing image: %v", err)
	}
	fmt.Fprintf(os.Stderr, "Done.")
	return nil
}
