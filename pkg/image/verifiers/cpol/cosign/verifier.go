package cosign

import (
	"context"
	"crypto"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/image/verifiers"
	"github.com/kyverno/kyverno/pkg/tracing"
	"github.com/sigstore/cosign/v3/pkg/oci"
	"go.opentelemetry.io/otel/trace"
)

var signatureAlgorithmMap = map[string]crypto.Hash{
	"":       crypto.SHA256,
	"sha224": crypto.SHA224,
	"sha256": crypto.SHA256,
	"sha384": crypto.SHA384,
	"sha512": crypto.SHA512,
}

type verifier struct{}

func NewVerifier() verifiers.ImageVerifier {
	return &verifier{}
}

func (v *verifier) VerifySignature(ctx context.Context, opts verifiers.Options) (*verifiers.Response, error) {
	if opts.SigstoreBundle {
		results, err := verifyBundleAndFetchAttestations(ctx, opts)
		if err != nil {
			return nil, err
		}

		if len(results) == 0 {
			return nil, fmt.Errorf("sigstore bundle verification failed: no matching signatures found")
		}

		return &verifiers.Response{Digest: results[0].Desc.Digest.String()}, nil
	}

	ref, err := resourceReference(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve image %s: %w", opts.ImageRef, err)
	}

	// verify image signatures is a wrapper around a cosign function of the same name
	signatures, bundleVerified, err := tracing.ChildSpan3(
		ctx,
		"",
		"VERIFY IMG SIGS",
		func(ctx context.Context, span trace.Span) ([]oci.Signature, bool, error) {
			cosignOpts, err := buildCosignOptions(ctx, opts)
			if err != nil {
				return nil, false, err
			}
			return client.VerifyImageSignatures(ctx, ref, cosignOpts)
		},
	)
	if err != nil {
		logger.Info("image verification failed", "error", err.Error())
		return nil, err
	}

	logger.V(3).Info("verified image", "count", len(signatures), "bundleVerified", bundleVerified)
	payload, err := extractPayload(signatures)
	if err != nil {
		return nil, err
	}

	if err := matchSignatures(signatures, opts.Subject, opts.SubjectRegExp, opts.Issuer, opts.IssuerRegExp, opts.AdditionalExtensions); err != nil {
		return nil, err
	}

	err = checkAnnotations(payload, opts.Annotations)
	if err != nil {
		return nil, err
	}

	var digest string
	if opts.Type == "" {
		digest, err = extractDigest(opts.ImageRef, payload)
		if err != nil {
			return nil, err
		}
	}

	return &verifiers.Response{Digest: digest}, nil
}

func (v *verifier) FetchAttestations(ctx context.Context, opts verifiers.Options) (*verifiers.Response, error) {
	if opts.SigstoreBundle {
		results, err := verifyBundleAndFetchAttestations(ctx, opts)
		if err != nil {
			return nil, err
		}

		if len(results) == 0 {
			return nil, fmt.Errorf("sigstore bundle verification failed: no matching signatures found")
		}

		statements, err := decodeStatementsFromBundles(results)
		if err != nil {
			return nil, err
		}
		return &verifiers.Response{Digest: results[0].Desc.Digest.String(), Statements: statements}, nil
	}
	cosignOpts, err := buildCosignOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	signatures, bundleVerified, err := tracing.ChildSpan3(
		ctx,
		"",
		"VERIFY IMG ATTESTATIONS",
		func(ctx context.Context, span trace.Span) (checkedAttestations []oci.Signature, bundleVerified bool, err error) {
			ref, err := resourceReference(ctx, opts)
			if err != nil {
				return nil, false, fmt.Errorf("failed to parse image: %w", err)
			}
			return client.VerifyImageAttestations(ctx, ref, cosignOpts)
		},
	)
	if err != nil {
		msg := err.Error()
		logger.Info("failed to fetch attestations", "error", msg)
		if strings.Contains(msg, "MANIFEST_UNKNOWN: manifest unknown") {
			return nil, fmt.Errorf("not found")
		}

		return nil, err
	}

	payload, err := extractPayload(signatures)
	if err != nil {
		return nil, err
	}

	for _, signature := range signatures {
		match, predicateType, err := matchType(signature, opts.Type)
		if err != nil {
			return nil, err
		}

		if !match {
			logger.V(4).Info("type doesn't match, continue", "expected", opts.Type, "received", predicateType)
			continue
		}

		if err := matchSignatures([]oci.Signature{signature}, opts.Subject, opts.SubjectRegExp, opts.Issuer, opts.IssuerRegExp, opts.AdditionalExtensions); err != nil {
			return nil, err
		}
	}

	err = checkAnnotations(payload, opts.Annotations)
	if err != nil {
		return nil, err
	}

	logger.V(3).Info("verified images", "signatures", len(signatures), "bundleVerified", bundleVerified)
	inTotoStatements, digest, err := decodeStatements(signatures)
	if err != nil {
		return nil, err
	}

	return &verifiers.Response{Digest: digest, Statements: inTotoStatements}, nil
}

func resourceReference(ctx context.Context, opts verifiers.Options) (name.Reference, error) {
	ref, err := name.ParseReference(opts.ImageRef, opts.Client.NameOptions()...)
	if err != nil || opts.Repository == "" {
		return ref, err
	}
	repository, err := name.NewRepository(opts.Repository, opts.Client.NameOptions()...)
	if err != nil {
		return nil, err
	}
	if digest, pinned := ref.(name.Digest); pinned {
		if scoped, ok := opts.Client.(interface {
			ValidateImage(context.Context, string) error
		}); ok {
			if err := scoped.ValidateImage(ctx, opts.ImageRef); err != nil {
				return nil, err
			}
		}
		return repository.Digest(digest.DigestStr()), nil
	}
	options, _, err := opts.Client.Options(ctx)
	if err != nil {
		return nil, err
	}
	descriptor, err := gcrremote.Get(ref, options...)
	if err != nil {
		return nil, err
	}
	return repository.Digest(descriptor.Digest.String()), nil
}
