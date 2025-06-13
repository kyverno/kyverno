package notary

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/notaryproject/notation-go"
	notationlog "github.com/notaryproject/notation-go/log"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

func NewVerifier(logger logr.Logger) *Verifier {
	return &Verifier{
		log: logger.WithName("Notary"),
	}
}

type Verifier struct {
	log logr.Logger
}

func (v *Verifier) VerifyImageSignature(ctx context.Context, image *imagedataloader.ImageData, certsData, tsaCertsData string) error {
	logger := v.log.WithValues("image", image.Image, "digest", image.Digest)
	logger.V(2).Info("verifying notary image signature", "image", image.Image)

	vInfo, err := getVerificationInfo(image, certsData, tsaCertsData)
	if err != nil {
		err := errors.Wrapf(err, "failed to setup notation verification data")
		logger.Error(err, "image verification failed")
		return err
	}

	opts := notation.VerifyOptions{
		ArtifactReference:    image.WithDigest(image.Digest),
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

func (v *Verifier) VerifyAttestationSignature(
	ctx context.Context,
	image *imagedataloader.ImageData,
	referrerType, certsData, tsaCertsData string,
) error {
	logger := v.log.WithValues("image", image.Image, "digest", image.Digest) // TODO: use attestor and attestation names
	logger.V(2).Info("verifying notary image signature", "image", image.Image)
	vInfo, err := getVerificationInfo(image, certsData, tsaCertsData)
	if err != nil {
		err := errors.Wrapf(err, "failed to setup notation verification data")
		logger.Error(err, "image verification failed")
		return err
	}
	referrers, err := image.FetchReferrers(referrerType)
	if err != nil {
		err := errors.Wrapf(err, "failed to fetch referrers")
		logger.Error(err, "image attestation verification failed")
		return err
	}
	var errs []error
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
			err := errors.Wrapf(err, "failed to verify attestation %s, digest %s", r.ArtifactType, r.Digest)
			logger.Error(err, "attestation verification failed")
			errs = append(errs, err)
			continue
		}

		if err := checkVerificationOutcomes(outcomes); err != nil {
			err := errors.Wrapf(err, "notation failed to verify attesattion signatures")
			logger.Error(err, "attesatation verification failed")
			errs = append(errs, err)
			continue
		}

		image.AddVerifiedReferrer(r)
		return nil
	}

	if len(errs) == 0 {
		return fmt.Errorf("attestation verification failed, no attestations found for type: %s", referrerType)
	}
	return multierr.Combine(errs...)
}
