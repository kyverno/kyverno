package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"k8s.io/client-go/rest"
)

// Verifier checks a webhook-bound token against the concrete receiver audience
// and the API group of the AdmissionReview it accompanies.
type Verifier interface {
	Verify(context.Context, string, string, string) error
}

type verifier struct {
	provider *oidc.Provider
}

// NewVerifier uses the configured Kubernetes API server for discovery and keys.
// OIDC supplies signature, issuer, audience, and expiry checks. In particular,
// it allows its standard five-minute nbf skew and does not enforce iat ordering.
func NewVerifier(ctx context.Context, config *rest.Config) (Verifier, error) {
	if config == nil {
		return nil, errors.New("missing Kubernetes REST configuration")
	}
	apiURL, err := url.Parse(config.Host)
	if err != nil || apiURL.Scheme != "https" || apiURL.Host == "" || apiURL.User != nil || apiURL.RawQuery != "" || apiURL.Fragment != "" {
		return nil, errors.New("invalid Kubernetes API server URL")
	}
	clientConfig := rest.CopyConfig(config)
	clientConfig.Timeout = 5 * time.Second
	client, err := rest.HTTPClientFor(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("configure Kubernetes API HTTP client: %w", err)
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	base := strings.TrimRight(apiURL.String(), "/")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/.well-known/openid-configuration", nil)
	if err != nil {
		return nil, fmt.Errorf("create discovery request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("discover Kubernetes token issuer: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Kubernetes issuer discovery returned HTTP %d", response.StatusCode)
	}
	var metadata oidc.ProviderConfig
	body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return nil, errors.New("invalid Kubernetes issuer discovery body")
	}
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("decode Kubernetes issuer discovery: %w", err)
	}
	if metadata.IssuerURL == "" {
		return nil, errors.New("Kubernetes issuer discovery has no issuer")
	}
	metadata.JWKSURL = base + "/openid/v1/jwks"
	return &verifier{provider: metadata.NewProvider(oidc.ClientContext(ctx, client))}, nil
}

func (v *verifier) Verify(ctx context.Context, token, audience, apiGroup string) error {
	if audience == "" {
		return errors.New("missing expected audience")
	}
	idToken, err := v.provider.Verifier(&oidc.Config{ClientID: audience}).Verify(ctx, token)
	if err != nil {
		return err
	}
	var claims struct {
		Kubernetes struct {
			Attestations struct {
				AdmissionReviewAPIGroups []string `json:"admissionReviewAPIGroups"`
			} `json:"attestations"`
		} `json:"kubernetes.io"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return err
	}
	groups := claims.Kubernetes.Attestations.AdmissionReviewAPIGroups
	if len(groups) != 1 || groups[0] == "" || (groups[0] != "*" && groups[0] != apiGroup) {
		return errors.New("token is not attested for the admission API group")
	}
	return nil
}
