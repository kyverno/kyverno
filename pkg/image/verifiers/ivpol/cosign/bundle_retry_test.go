package cosign

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFlakyRegistry starts an in-process registry that, once armed, cuts the
// first `truncations` GET responses for which match returns true in half: the
// headers advertise the full Content-Length, half the body is sent, then the
// connection is dropped. That is the "unexpected EOF" a client sees when a proxy
// or a pod dies mid-download. gets counts the matching GETs made after arm() was
// called.
func newFlakyRegistry(t *testing.T, truncations int64, match func(*http.Request) bool) (name.Repository, *atomic.Int64, func()) {
	t.Helper()
	var (
		armed atomic.Bool
		gets  atomic.Int64
	)
	inner := registry.New(registry.WithReferrersSupport(true))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !armed.Load() || !match(r) {
			inner.ServeHTTP(w, r)
			return
		}
		if gets.Add(1) > truncations {
			inner.ServeHTTP(w, r)
			return
		}
		rec := httptest.NewRecorder()
		inner.ServeHTTP(rec, r)
		body := rec.Body.Bytes()
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(rec.Code)
		_, _ = w.Write(body[:len(body)/2])
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	}))
	t.Cleanup(srv.Close)
	repo, err := name.NewRepository(strings.TrimPrefix(srv.URL, "http://") + "/test/img")
	require.NoError(t, err)
	return repo, &gets, func() { armed.Store(true) }
}

func isBlobGet(r *http.Request) bool {
	return r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/blobs/sha256:")
}

func isReferrersGet(r *http.Request) bool {
	return r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/referrers/")
}

func isReferrerManifestGet(r *http.Request) bool {
	return r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/manifests/sha256:")
}

func isSubjectManifestGet(r *http.Request) bool {
	return r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/manifests/subject")
}

// newFlakyBundleRegistry cuts the first `truncations` blob downloads.
func newFlakyBundleRegistry(t *testing.T, truncations int64) (name.Repository, *atomic.Int64, func()) {
	t.Helper()
	return newFlakyRegistry(t, truncations, isBlobGet)
}

// A bundle blob cut off once must be fetched again, not skipped.
func Test_LoadBundles_RetriesTruncatedBlob(t *testing.T) {
	repo, blobGets, arm := newFlakyBundleRegistry(t, 1)
	subject := pushSubject(t, repo, "subject")
	pushReferrer(t, repo, subject, []byte(validBundle), testBundleMediaType, testBundleMediaType)
	img := fetchTestImage(t, repo.Tag("subject").String())
	v := NewVerifier(nil, logr.Discard())

	arm()
	cOpts, bundles, _, err := v.buildCheckOptsWithBundleDetection(context.TODO(), bundleTestAttestor(), img)
	require.NoError(t, err)

	assert.True(t, cOpts.NewBundleFormat, "a single truncated download must not hide the bundle")
	assert.Len(t, bundles, 1)
	assert.Equal(t, int64(2), blobGets.Load(), "one truncated GET, then one that succeeds")
}

// A blob that is cut off every time must stop after a bounded number of attempts.
func Test_LoadBundles_GivesUpAfterBoundedRetries(t *testing.T) {
	repo, blobGets, arm := newFlakyBundleRegistry(t, 1<<30)
	subject := pushSubject(t, repo, "subject")
	pushReferrer(t, repo, subject, []byte(validBundle), testBundleMediaType, testBundleMediaType)
	img := fetchTestImage(t, repo.Tag("subject").String())
	v := NewVerifier(nil, logr.Discard())

	arm()
	cOpts, bundles, _, err := v.buildCheckOptsWithBundleDetection(context.TODO(), bundleTestAttestor(), img)
	require.NoError(t, err)

	assert.False(t, cOpts.NewBundleFormat)
	assert.Empty(t, bundles)
	assert.Equal(t, int64(3), blobGets.Load(), "must retry, but only up to the attempt limit")
}

// A bundle that downloads fine but does not parse is a permanent failure and
// must be read exactly once.
func Test_LoadBundles_DoesNotRetryUnparsableBundle(t *testing.T) {
	repo, blobGets, arm := newFlakyBundleRegistry(t, 0)
	subject := pushSubject(t, repo, "subject")
	pushReferrer(t, repo, subject, []byte(`{"not":"a bundle"}`), testBundleMediaType, testBundleMediaType)
	img := fetchTestImage(t, repo.Tag("subject").String())
	v := NewVerifier(nil, logr.Discard())

	arm()
	cOpts, _, _, err := v.buildCheckOptsWithBundleDetection(context.TODO(), bundleTestAttestor(), img)
	require.NoError(t, err)

	assert.False(t, cOpts.NewBundleFormat)
	assert.Equal(t, int64(1), blobGets.Load(), "a parse failure is not transient")
}

// Several referrers whose blobs are always cut off must not each get a full set
// of retries: the retries spent across the whole load are capped.
func Test_LoadBundles_RetryBudgetIsSharedAcrossReferrers(t *testing.T) {
	const referrers = 4
	repo, blobGets, arm := newFlakyBundleRegistry(t, 1<<30)
	subject := pushSubject(t, repo, "subject")
	for i := 0; i < referrers; i++ {
		pushReferrer(t, repo, subject, []byte(validBundle), testBundleMediaType, fmt.Sprintf("application/vnd.test.referrer.%d", i))
	}
	img := fetchTestImage(t, repo.Tag("subject").String())
	v := NewVerifier(nil, logr.Discard())

	arm()
	cOpts, bundles, _, err := v.buildCheckOptsWithBundleDetection(context.TODO(), bundleTestAttestor(), img)
	require.NoError(t, err)

	assert.False(t, cOpts.NewBundleFormat)
	assert.Empty(t, bundles)
	assert.Equal(t, int64(referrers+maxRetriesPerLoad), blobGets.Load(), "one read per referrer plus the shared retry budget, no more")
}

// assertLoadRetriesOnce cuts the first GET that match selects and expects the
// bundle to be found anyway, after exactly one more such GET.
func assertLoadRetriesOnce(t *testing.T, match func(*http.Request) bool) {
	t.Helper()
	repo, gets, arm := newFlakyRegistry(t, 1, match)
	subject := pushSubject(t, repo, "subject")
	pushReferrer(t, repo, subject, []byte(validBundle), testBundleMediaType, testBundleMediaType)
	img := fetchTestImage(t, repo.Tag("subject").String())
	v := NewVerifier(nil, logr.Discard())

	arm()
	cOpts, bundles, _, err := v.buildCheckOptsWithBundleDetection(context.TODO(), bundleTestAttestor(), img)
	require.NoError(t, err)

	assert.True(t, cOpts.NewBundleFormat, "a single truncated response must not hide the bundle")
	assert.Len(t, bundles, 1)
	assert.Equal(t, int64(2), gets.Load(), "one truncated GET, then one that succeeds")
}

func Test_LoadBundles_RetriesTruncatedSubjectManifest(t *testing.T) {
	assertLoadRetriesOnce(t, isSubjectManifestGet)
}

func Test_LoadBundles_RetriesTruncatedReferrersList(t *testing.T) {
	assertLoadRetriesOnce(t, isReferrersGet)
}

func Test_LoadBundles_RetriesTruncatedReferrerManifest(t *testing.T) {
	assertLoadRetriesOnce(t, isReferrerManifestGet)
}

func newCapturingVerifier(verbosity int) (*Verifier, *strings.Builder) {
	l, sb := captureLogs(verbosity)
	v := NewVerifier(nil, logr.Discard())
	v.log = l
	return v, sb
}

func captureLogs(verbosity int) (logr.Logger, *strings.Builder) {
	var sb strings.Builder
	l := funcr.New(func(prefix, args string) { sb.WriteString(args + "\n") }, funcr.Options{Verbosity: verbosity})
	return l, &sb
}

func Test_logBundleDetectionFailed(t *testing.T) {
	t.Run("transient error is logged at the default level", func(t *testing.T) {
		v, sb := newCapturingVerifier(0)
		v.logBundleDetectionFailed("cannot list referrers", "sha256:abc", io.ErrUnexpectedEOF)
		assert.Contains(t, sb.String(), "bundle detection failed: transient registry read error")
		assert.NotContains(t, sb.String(), "skipping referrer")
	})
	t.Run("other error is only logged at V(2)", func(t *testing.T) {
		quiet, qsb := newCapturingVerifier(0)
		quiet.logBundleDetectionFailed("cannot list referrers", "sha256:abc", errors.New("boom"))
		assert.Empty(t, qsb.String())
		verbose, vsb := newCapturingVerifier(2)
		verbose.logBundleDetectionFailed("cannot list referrers", "sha256:abc", errors.New("boom"))
		assert.Contains(t, vsb.String(), "bundle detection failed: cannot list referrers")
	})
}

func Test_LoadBundles_ImageWideFailureIsNotLoggedAsSkippedReferrer(t *testing.T) {
	v, sb := newCapturingVerifier(2)
	ref, err := name.ParseReference("127.0.0.1:1/unreachable/repo:tag", name.Insecure)
	require.NoError(t, err)
	bundles, hash := v.loadBundles(context.Background(), ref, nil)
	assert.Nil(t, bundles)
	assert.Nil(t, hash)
	assert.Contains(t, sb.String(), "bundle detection failed")
	assert.NotContains(t, sb.String(), "skipping referrer")
}
