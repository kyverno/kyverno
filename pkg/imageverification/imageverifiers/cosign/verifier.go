package cosign

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/pkg/errors"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/policy"
)

type Verifier struct {
	secretInterface imagedataloader.SecretInterface
	log             logr.Logger
}

func NewVerifier(secretInterface imagedataloader.SecretInterface, logger logr.Logger) *Verifier {
	return &Verifier{
		log:             logging.WithName("Notary"),
		secretInterface: secretInterface,
	}
}

func (v *Verifier) VerifyImageSignature(ctx context.Context, image *imagedataloader.ImageData, attestor *policiesv1alpha1.Attestor) error {
	if attestor.Cosign == nil {
		return fmt.Errorf("cosign verifier only supports cosign attestor")
	}

	logger := v.log.WithValues("image", image.Image, "digest", image.Digest, "attestor", attestor.Name)
	logger.V(2).Info("verifying cosign image signature", "image", image.Image)

	cOpts, err := checkOptions(ctx, attestor.Cosign, image.RemoteOpts(), image.NameOpts(), v.secretInterface)
	if err != nil {
		err := errors.Wrapf(err, "failed to build cosign verification opts")
		logger.Error(err, "image verification failed")
		return err
	}
	cOpts.ClaimVerifier = cosign.SimpleClaimVerifier

	sigs, verified, err := cosign.VerifyImageSignatures(ctx, image.NameRef(), cOpts)
	if err != nil {
		err := errors.Wrapf(err, "failed to verify cosign signatures")
		logger.Error(err, "image verification failed")
		return err
	} else if !verified {
		if !(attestor.Cosign.CTLog.InsecureIgnoreTlog || attestor.Cosign.CTLog.InsecureIgnoreSCT) {
			err := fmt.Errorf("transparency log or timestamp verification failed")
			logger.Error(err, "image verification failed")
			return err
		}
	} else if len(sigs) == 0 {
		err := fmt.Errorf("signatures not found")
		logger.Error(err, "image verification failed")
		return err
	}

	if len(attestor.Cosign.Annotations) != 0 {
		for _, sig := range sigs {
			if err := checkSignatureAnnotations(sig, attestor.Cosign.Annotations); err != nil {
				logger.Error(err, "image verification failed")
				return err
			}
		}
	}

	return nil
}

func (v *Verifier) VerifyAttestationSignature(ctx context.Context, image *imagedataloader.ImageData, attestation *policiesv1alpha1.Attestation, attestor *policiesv1alpha1.Attestor) error {
	if attestation.InToto == nil {
		return fmt.Errorf("cosgin verifier only supports intoto referrers as attestations")
	}
	if attestor.Cosign == nil {
		return fmt.Errorf("cosign verifier only supports cosign attestor")
	}

	logger := v.log.WithValues("image", image.Image, "digest", image.Digest, "attestation", attestation.Name, "attestor", attestor.Name)
	logger.V(2).Info("verifying cosign attestation signature", "image", image.Image)

	cOpts, err := checkOptions(ctx, attestor.Cosign, image.RemoteOpts(), image.NameOpts(), v.secretInterface)
	if err != nil {
		err := errors.Wrapf(err, "failed to build cosign verification opts")
		logger.Error(err, "image verification failed")
		return err
	}
	cOpts.ClaimVerifier = cosign.IntotoSubjectClaimVerifier
	sigs, verified, err := cosign.VerifyImageAttestations(ctx, image.NameRef(), cOpts)
	if err != nil {
		err := errors.Wrapf(err, "failed to verify cosign signatures")
		logger.Error(err, "image verification failed")
		return err
	} else if !verified {
		err := fmt.Errorf("cosign bundle verification failed")
		logger.Error(err, "image verification failed")
		return err
	}

	checkedTypes := []string{}
	found := false
	for _, s := range sigs {
		payload, gotType, err := policy.AttestationToPayloadJSON(ctx, attestation.InToto.Type, s)
		if err != nil {
			return fmt.Errorf("converting to consumable policy validation: %w", err)
		}
		checkedTypes = append(checkedTypes, gotType)
		if len(payload) == 0 {
			// This is not the predicate type we're looking for.
			continue
		}

		if err := checkSignatureAnnotations(s, attestor.Cosign.Annotations); err != nil {
			logger.Error(err, "image verification failed")
			return err
		}

		found = true
		image.AddVerifiedIntotoPayloads(gotType, payload)
	}

	if !found {
		err := fmt.Errorf("required predicate type %s not found, found %v", attestation.InToto.Type, checkedTypes)
		logger.Error(err, "image verification failed")
		return err
	}

	return nil
}
