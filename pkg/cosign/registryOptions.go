package cosign

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"runtime"

	"github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/chrismellard/docker-credential-acr-env/pkg/credhelper"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	alibabaacr "github.com/mozillazg/docker-credential-acr-helper/pkg/credhelper"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	"sigs.k8s.io/release-utils/version"
)

var uaString = fmt.Sprintf("cosign/%s (%s; %s)", version.GetVersionInfo().GitVersion, runtime.GOOS, runtime.GOARCH)

func UserAgent() string {
	return uaString
}

type RegistryOptions struct {
	AllowInsecure      bool
	AllowHTTPRegistry  bool
	KubernetesKeychain bool
	TagPrefix          string
	Keychain           authn.Keychain
}

func (o *RegistryOptions) ClientOpts(ctx context.Context) ([]ociremote.Option, error) {
	opts := []ociremote.Option{ociremote.WithRemoteOptions(o.GetRegistryClientOpts(ctx)...)}
	if o.TagPrefix != "" {
		opts = append(opts, ociremote.WithPrefix(o.TagPrefix))
	}
	targetRepoOverride, err := ociremote.GetEnvTargetRepository()
	if err != nil {
		return nil, err
	}
	if (targetRepoOverride != name.Repository{}) {
		opts = append(opts, ociremote.WithTargetRepository(targetRepoOverride))
	}
	return opts, nil
}

func (o *RegistryOptions) GetRegistryClientOpts(ctx context.Context) []remote.Option {
	opts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithUserAgent(UserAgent()),
	}

	switch {
	case o.Keychain != nil:
		opts = append(opts, remote.WithAuthFromKeychain(o.Keychain))
	case o.KubernetesKeychain:
		kc := authn.NewMultiKeychain(
			authn.DefaultKeychain,
			google.Keychain,
			authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(io.Discard))),
			authn.NewKeychainFromHelper(credhelper.NewACRCredentialsHelper()),
			authn.NewKeychainFromHelper(alibabaacr.NewACRHelper().WithLoggerOut(io.Discard)),
			github.Keychain,
		)
		opts = append(opts, remote.WithAuthFromKeychain(kc))
	default:
		opts = append(opts, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	}

	if o.AllowInsecure {
		opts = append(opts, remote.WithTransport(&http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}})) // #nosec G402
	}
	return opts
}
