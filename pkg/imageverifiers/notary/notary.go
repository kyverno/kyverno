package notary

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/imagedataloader"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/notaryproject/notation-go"
	notationlog "github.com/notaryproject/notation-go/log"
	"github.com/pkg/errors"
)

func NewVerifier() *notaryVerifier {
	return &notaryVerifier{
		log: logging.WithName("Notary"),
	}
}

type notaryVerifier struct {
	log logr.Logger
}

func (v *notaryVerifier) VerifyImageSignature(ctx context.Context, image *imagedataloader.ImageData, attestor Notary) error {
	logger := v.log.WithValues("image", image.Image, "digest", image.Digest)
	logger.V(2).Info("verifying notary image signature", "image", image.Image)

	vInfo, err := getVerificationInfo(image, attestor)
	if err != nil {
		err := errors.Wrapf(err, "failed to setup notation verification data")
		logger.Error(err, "image verification failed")
		return err
	}

	opts := notation.VerifyOptions{
		ArtifactReference:    image.Image,
		MaxSignatureAttempts: 10,
	}
	_, outcomes, err := notation.Verify(notationlog.WithLogger(ctx, NotaryLoggerAdapter(v.log.WithName("Notary Verifier Debug"))), vInfo.Verifier,
		vInfo.Repo, opts)
	if err != nil {
		err := errors.Wrapf(err, "failed to verify image %s", image.Image)
		logger.Error(err, "image verification failed")
		return err
	}

	if err := checkVerificationOutcomes(outcomes); err != nil {
		err := errors.Wrapf(err, "notation failed to verify signatures")
		logger.Error(err, "image verification failed")
		return err
	}

	return nil
}

func (v *notaryVerifier) VerifyAttestationSignature(ctx context.Context, image *imagedataloader.ImageData, attestation Referrer, attestor Notary) error {
	logger := v.log.WithValues("image", image.Image, "digest", image.Digest, "attestation type", attestation.Type, "attestor", attestor) // TODO: use attestor and attestation names
	logger.V(2).Info("verifying notary image signature", "image", image.Image)

	vInfo, err := getVerificationInfo(image, attestor)
	if err != nil {
		err := errors.Wrapf(err, "failed to setup notation verification data")
		logger.Error(err, "image verification failed")
		return err
	}

	referrers, err := image.FetchRefererrs(attestation.Type)
	if err != nil {
		err := errors.Wrapf(err, "failed to fetch referrers")
		logger.Error(err, "image attestation verification failed")
		return err
	}

	for _, r := range referrers {
		reference := image.WithDigest(r.Digest.String())
		logger := logger.WithValues("attestation ref", reference)

		logger.V(2).Info("verifying attestation")
		opts := notation.VerifyOptions{
			ArtifactReference:    reference,
			MaxSignatureAttempts: 10,
		}

		_, outcomes, err := notation.Verify(notationlog.WithLogger(ctx, NotaryLoggerAdapter(v.log.WithName("Notary Verifier Debug"))), vInfo.Verifier,
			vInfo.Repo, opts)
		if err != nil {
			err := errors.Wrapf(err, "failed to verify attestation %s", image.Image)
			logger.Error(err, "attestation verification failed")
			continue
		}

		if err := checkVerificationOutcomes(outcomes); err != nil {
			err := errors.Wrapf(err, "notation failed to verify attesattion signatures")
			logger.Error(err, "attesatation verification failed")
			continue
		}

		image.AddVerifiedReferrer(r)
		return nil
	}

	return nil
}
