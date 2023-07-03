package registryclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	"golang.org/x/oauth2"
	googauth "golang.org/x/oauth2/google"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

var GetGcloudCmd = func() *exec.Cmd {
	return exec.Command("gcloud", "config", "config-helper", "--force-auth-refresh", "--format=json(credential)")
}

func gcrAuthenticator(r authn.Resource) (authn.Authenticator, error) {

	if !isGoogle(r.RegistryStr()) {
		return authn.Anonymous, nil
	}

	auth_env, err := NewEnvAuthenticator()
	if err == nil && auth_env != authn.Anonymous {
		return auth_env, nil
	}

	auth_gc, err := NewGcloudAuthenticator()
	if err == nil && auth_gc != authn.Anonymous {
		return auth_gc, nil
	}

	logs.Debug.Println("Failed to get any Google credentials, falling back to Anonymous")
	return authn.Anonymous, nil
}

// NewEnvAuthenticator returns an authn.Authenticator that generates access
// tokens from the environment we're running in.
func NewEnvAuthenticator() (authn.Authenticator, error) {
	ts, err := googauth.DefaultTokenSource(context.Background(), cloudPlatformScope)
	if err != nil {
		return nil, err
	}

	token, err := ts.Token()
	if err != nil {
		return nil, err
	}

	return &tokenSourceAuth{oauth2.ReuseTokenSource(token, ts)}, nil
}

// NewGcloudAuthenticator returns an oauth2.TokenSource that generates access
// tokens by shelling out to the gcloud sdk.
func NewGcloudAuthenticator() (authn.Authenticator, error) {
	if _, err := exec.LookPath("gcloud"); err != nil {
		logs.Warn.Println("gcloud binary not found")
		return authn.Anonymous, nil
	}

	ts := gcloudSource{GetGcloudCmd}

	// Attempt to fetch a token to ensure gcloud is installed and we can run it.
	token, err := ts.Token()
	if err != nil {
		return nil, err
	}

	return &tokenSourceAuth{oauth2.ReuseTokenSource(token, ts)}, nil
}

// tokenSourceAuth turns an oauth2.TokenSource into an authn.Authenticator.
type tokenSourceAuth struct {
	oauth2.TokenSource
}

func (tsa *tokenSourceAuth) Authorization() (*authn.AuthConfig, error) {
	token, err := tsa.Token()
	if err != nil {
		return nil, err
	}

	return &authn.AuthConfig{
		Username: "_token",
		Password: token.AccessToken,
	}, nil
}

// google cloud credentials
type gcloudOutput struct {
	Credential struct {
		AccessToken string `json:"access_token"`
		TokenExpiry string `json:"token_expiry"`
	} `json:"credential"`
}

type gcloudSource struct {
	exec func() *exec.Cmd
}

// Token implements oauath2.TokenSource.
func (gs gcloudSource) Token() (*oauth2.Token, error) {
	cmd := gs.exec()
	var out bytes.Buffer
	cmd.Stdout = &out

	cmd.Stderr = logs.Warn.Writer()

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("error executing `gcloud config config-helper`: %w", err)
	}

	creds := gcloudOutput{}
	if err := json.Unmarshal(out.Bytes(), &creds); err != nil {
		return nil, fmt.Errorf("failed to parse `gcloud config config-helper` output: %w", err)
	}

	expiry, err := time.Parse(time.RFC3339, creds.Credential.TokenExpiry)
	if err != nil {
		return nil, fmt.Errorf("failed to parse gcloud token expiry: %w", err)
	}

	token := oauth2.Token{
		AccessToken: creds.Credential.AccessToken,
		Expiry:      expiry,
	}

	return &token, nil
}

func isGoogle(host string) bool {
	return host == "gcr.io" ||
		strings.HasSuffix(host, ".gcr.io") ||
		strings.HasSuffix(host, ".pkg.dev") ||
		strings.HasSuffix(host, ".google.com")
}
