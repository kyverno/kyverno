package imagedataloader

import (
	"fmt"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/release-utils/version"
)

var (
	DefaultKeychain  = AnonymousKeychain
	DefaultTransport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			// By default we wrap the transport in retries, so reduce the
			// default dial timeout to 5s to avoid 5x 30s of connection
			// timeouts when doing the "ping" on certain http registries.
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	UserAgent = fmt.Sprintf("Kyverno/%s (%s; %s)", version.GetVersionInfo().GitVersion, runtime.GOOS, runtime.GOARCH)
)

type Option func(*options)

type options struct {
	insecure            bool
	secrets             []string
	credentialProviders []string
	localCredentials    bool
	tracing             bool
}

func WithInsecure(v bool) Option {
	return func(o *options) {
		o.insecure = v
	}
}

func WithTracing(v bool) Option {
	return func(o *options) {
		o.tracing = v
	}
}

func WithPullSecret(secrets []string) Option {
	return func(o *options) {
		o.secrets = secrets
	}
}

func WithCredentialProviders(providers ...string) Option {
	return func(o *options) {
		o.credentialProviders = providers
	}
}

func WithLocalCredentials(v bool) Option {
	return func(o *options) {
		o.localCredentials = v
	}
}

func makeDefaultOpts(lister corev1.SecretInterface, opts ...Option) ([]remote.Option, error) {
	remoteOpts := make([]remote.Option, 0)
	remoteOpts = append(remoteOpts, makeBaseOptions(opts...)...)
	authOpts, err := makeAuthOptions(lister, opts...)
	if err != nil {
		return nil, err
	}

	remoteOpts = append(remoteOpts, authOpts...)
	return remoteOpts, nil
}

func makeBaseOptions(opts ...Option) []remote.Option {
	remoteOpts := make([]remote.Option, 0)
	opt := options{}
	for _, o := range opts {
		o(&opt)
	}

	var transport http.RoundTripper
	transport = DefaultTransport
	if opt.tracing {
		transport = tracing.Transport(DefaultTransport, otelhttp.WithFilter(tracing.RequestFilterIsInSpan))
	}

	remoteOpts = append(remoteOpts,
		remote.WithTransport(transport),
		remote.WithUserAgent(UserAgent),
	)

	return remoteOpts
}

func makeAuthOptions(lister corev1.SecretInterface, opts ...Option) ([]remote.Option, error) {
	remoteOpts := make([]remote.Option, 0)

	opt := options{}
	for _, o := range opts {
		o(&opt)
	}

	keychains := make([]authn.Keychain, 0)
	if len(opt.secrets) > 0 {
		if lister == nil {
			return nil, fmt.Errorf("secret lister is nil, cannot create image pull secrets")
		}
		kc, err := NewAutoRefreshSecretsKeychain(lister, opt.secrets...)
		if err != nil {
			return nil, err
		}
		keychains = append(keychains, kc)
	}

	if len(opt.credentialProviders) > 0 {
		keychains = append(keychains, KeychainsForProviders(opt.credentialProviders...)...)
	}

	if opt.localCredentials {
		keychains = []authn.Keychain{authn.DefaultKeychain} // only in kyverno CLI
	}

	if len(keychains) == 0 {
		keychains = []authn.Keychain{AnonymousKeychain}
	}

	remoteOpts = append(remoteOpts,
		remote.WithAuthFromKeychain(authn.NewMultiKeychain(keychains...)),
	)
	return remoteOpts, nil
}

func nameOptions(opts ...Option) []name.Option {
	nameOpts := make([]name.Option, 0)
	opt := options{}
	for _, o := range opts {
		o(&opt)
	}
	if opt.insecure {
		nameOpts = append(nameOpts, name.Insecure)
	}
	return nameOpts
}
