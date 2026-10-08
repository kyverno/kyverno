package sigstoreguard

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/sigstore/cosign/v3/pkg/blob"
	sigs "github.com/sigstore/cosign/v3/pkg/signature"
	rekorclient "github.com/sigstore/rekor/pkg/client"
	"github.com/sigstore/rekor/pkg/generated/client"
	"github.com/sigstore/rekor/pkg/util"
	"github.com/sigstore/sigstore/pkg/signature"
)

// NewRekorClient injects the process egress policy into every Rekor request,
// including retries and redirects. Rekor's convenience constructor does not
// expose its HTTP transport, so construct its generated client directly.
func NewRekorClient(serverURL string) (*client.Rekor, error) {
	return newRekorClient(serverURL, registryclient.EgressHTTPClient())
}

func newRekorClient(serverURL string, httpClient *http.Client) (*client.Rekor, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, err
	}
	if u.Path == "" {
		u.Path = client.DefaultBasePath
	}
	retries := retryablehttp.NewClient()
	retries.HTTPClient = httpClient
	retries.RetryMax = rekorclient.DefaultRetryCount
	retries.Logger = nil
	retries.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		if errors.Is(err, egress.ErrAddressBlocked) {
			return false, err
		}
		return retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	}
	rt := httptransport.NewWithClient(u.Host, u.Path, []string{u.Scheme}, retries.StandardClient())
	rt.Consumers["application/json"] = runtime.JSONConsumer()
	rt.Consumers["application/x-pem-file"] = runtime.TextConsumer()
	rt.Producers["application/json"] = runtime.JSONProducer()
	formats := strfmt.NewFormats()
	formats.Add("signedCheckpoint", &util.SignedNote{}, util.SignedCheckpointValidator)
	return client.New(rt, formats), nil
}

// LoadFileOrURL preserves Cosign's file and environment references while using
// a guarded client for URLs instead of blob.LoadFileOrURL's http.Get.
func LoadFileOrURL(ctx context.Context, ref string) ([]byte, error) {
	return loadFileOrURL(ctx, ref, registryclient.EgressHTTPClient())
}

func loadFileOrURL(ctx context.Context, ref string, client *http.Client) ([]byte, error) {
	if !isHTTPURL(ref) {
		return blob.LoadFileOrURL(ref)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ref, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("loading URL %s: server returned HTTP %d", ref, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// PublicKeyFromKeyRefWithHashAlgo protects HTTP key references without changing
// the handling of Kubernetes, KMS, PKCS11, or local references.
func PublicKeyFromKeyRefWithHashAlgo(ctx context.Context, ref string, hash crypto.Hash) (signature.Verifier, error) {
	if !isHTTPURL(ref) {
		return sigs.PublicKeyFromKeyRefWithHashAlgo(ctx, ref, hash)
	}
	key, err := LoadFileOrURL(ctx, ref)
	if err != nil {
		return nil, err
	}
	return sigs.LoadPublicKeyRaw(key, hash)
}

func isHTTPURL(ref string) bool {
	scheme, _, found := strings.Cut(ref, ":")
	return found && (strings.EqualFold(scheme, "http") || strings.EqualFold(scheme, "https"))
}
