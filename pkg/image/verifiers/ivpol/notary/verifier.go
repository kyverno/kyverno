package notary

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/go-logr/logr"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/kyverno/sdk/extensions/imagedataloader"
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

// VerifyAttestationSignature records the first listed referrer of referrerType
// that verifies on image and returns it.
func (v *Verifier) VerifyAttestationSignature(
	ctx context.Context,
	image *imagedataloader.ImageData,
	referrerType, certsData, tsaCertsData string,
) ([]gcrv1.Descriptor, error) {
	logger := v.log.WithValues("image", image.Image, "digest", image.Digest) // TODO: use attestor and attestation names
	logger.V(2).Info("verifying notary image signature", "image", image.Image)
	vInfo, err := getVerificationInfo(image, certsData, tsaCertsData)
	if err != nil {
		err := errors.Wrapf(err, "failed to setup notation verification data")
		logger.Error(err, "image verification failed")
		return nil, err
	}
	referrers, err := image.FetchReferrers(referrerType)
	if err != nil {
		err := errors.Wrapf(err, "failed to fetch referrers")
		logger.Error(err, "image attestation verification failed")
		return nil, err
	}
	// The verifications share vInfo: notation's verifier, its revocation
	// validators included, isn't modified after verifier.New, and the
	// repository keeps no state of its own.
	found, errs := referrerPool.firstVerified(ctx, len(referrers), func(ctx context.Context, i int) error {
		return v.verifyReferrer(ctx, logger, image, vInfo, referrers[i])
	})
	if found >= 0 {
		r := referrers[found]
		image.AddVerifiedReferrer(r)
		return []gcrv1.Descriptor{r}, nil
	}

	if len(errs) == 0 {
		return nil, fmt.Errorf("attestation verification failed, no attestations found for type: %s", referrerType)
	}
	err = multierr.Combine(errs...)
	logger.Error(err, "attestation verification failed")
	return nil, err
}

// verifyReferrer verifies one referrer's signature. It runs concurrently with
// the verifications of the other referrers, so it logs a failure only at V(4):
// another referrer may verify.
func (v *Verifier) verifyReferrer(ctx context.Context, logger logr.Logger, image *imagedataloader.ImageData, vInfo *verificationInfo, r gcrv1.Descriptor) error {
	reference := image.WithDigest(r.Digest.String())
	logger = logger.WithValues("attestation ref", reference)

	logger.V(2).Info("verifying attestation")
	opts := notation.VerifyOptions{
		ArtifactReference:    reference,
		MaxSignatureAttempts: 10,
	}

	_, outcomes, err := notation.Verify(notationlog.WithLogger(ctx, NotaryLoggerAdapter(v.log.WithName("Notary Verifier Debug"))), vInfo.Verifier,
		vInfo.Repo, opts)
	if err != nil {
		err := errors.Wrapf(err, "failed to verify attestation %s, digest %s", r.ArtifactType, r.Digest)
		logger.V(4).Info("attestation verification failed", "error", err)
		return err
	}

	if err := checkVerificationOutcomes(outcomes); err != nil {
		err := errors.Wrapf(err, "notation failed to verify attestation signatures")
		logger.V(4).Info("attestation verification failed", "error", err)
		return err
	}
	return nil
}

// referrerPool is shared by every Notary attestation verification in the
// process, so many policies verifying the same image don't multiply the load
// on its registry.
var referrerPool = &verifyPool{extra: make(chan struct{}, 4), hedgeDelay: time.Second}

// verifyPool runs verifications alongside the one each caller runs on its own.
type verifyPool struct {
	// extra holds a token for each verification running alongside a caller's
	// own, across every caller sharing the pool.
	extra chan struct{}
	// hedgeDelay is how long the first index runs alone before others may
	// start alongside it.
	hedgeDelay time.Duration
}

// verifyPanic is a panic recovered on a verification goroutine, raised again
// from the caller with the stack of the goroutine that panicked.
type verifyPanic struct {
	value any
	stack []byte
}

// String gives the engine's recover, which formats the panic with %v, both
// the panic and the stack.
func (p *verifyPanic) String() string {
	return fmt.Sprintf("%v\n\nverification goroutine stack:\n%s", p.value, p.stack)
}

// firstVerified calls verify for the indexes 0 to n-1 in index order. It
// returns the lowest index for which verify returned nil, or -1 if there is
// none, along with the errors for the indexes below it (for every index, if
// none verified): the same result as calling verify in order until it first
// returns nil.
//
// Index 0 runs alone until it fails or p.hedgeDelay passes, so a first index
// that verifies costs nothing extra. After that, indexes start as soon as the
// caller's own lane or a token from p.extra is free. Once an index verifies,
// calls for higher indexes are cancelled through their contexts, and those not
// started yet are skipped.
//
// A panic in verify is recovered on its goroutine, which would otherwise take
// the process down, and raised again here as a *verifyPanic if calling verify
// in order would have reached it.
func (p *verifyPool) firstVerified(ctx context.Context, n int, verify func(ctx context.Context, i int) error) (int, []error) {
	ctxs := make([]context.Context, n)
	cancels := make([]context.CancelFunc, n)
	for i := range n {
		ctxs[i], cancels[i] = context.WithCancel(ctx)
	}
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()

	// own is the caller's lane. It needs no token from p.extra, so every caller
	// keeps making progress like the old sequential loop did.
	own := make(chan struct{}, 1)
	own <- struct{}{}
	// mu keeps an index from checking whether it was cancelled while a verified
	// index is partway through cancelling the ones after it: a cancelled call
	// can free its lane before the higher indexes are cancelled.
	var mu sync.Mutex
	errs := make([]error, n)
	panics := make([]*verifyPanic, n)
	var wg sync.WaitGroup
	for i := range n {
		release := p.acquire(own, i == 1)
		// Cancelled if an earlier index has verified.
		mu.Lock()
		errs[i] = ctxs[i].Err()
		mu.Unlock()
		if errs[i] != nil {
			release()
			continue
		}
		wg.Go(func() {
			defer release()
			defer func() {
				if r := recover(); r != nil {
					panics[i] = &verifyPanic{value: r, stack: debug.Stack()}
				}
			}()
			if errs[i] = verify(ctxs[i], i); errs[i] == nil {
				mu.Lock()
				for _, cancel := range cancels[i+1:] {
					cancel()
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	for i := range n {
		if panics[i] != nil {
			panic(panics[i])
		}
		if errs[i] == nil {
			return i, errs[:i]
		}
	}
	return -1, errs
}

// acquire waits for a lane to verify on: the caller's own if it is free, else
// whichever of it or a token from p.extra frees first. With hedge set, it
// waits up to p.hedgeDelay for the caller's own lane alone before that.
func (p *verifyPool) acquire(own chan struct{}, hedge bool) (release func()) {
	releaseOwn := func() { own <- struct{}{} }
	select {
	case <-own:
		return releaseOwn
	default:
	}
	if hedge {
		select {
		case <-own:
			return releaseOwn
		case <-time.After(p.hedgeDelay):
		}
	}
	select {
	case <-own:
		return releaseOwn
	case p.extra <- struct{}{}:
		return func() { <-p.extra }
	}
}
