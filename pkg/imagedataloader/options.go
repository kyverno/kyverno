package imagedataloader

import (
	"fmt"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
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

func makeDefaultOpts(lister v1.SecretInterface, opts ...Option) ([]crane.Option, error) {
	craneOpts := make([]crane.Option, 0)
	craneOpts = append(craneOpts, makeBaseOptions(opts...)...)
	authOpts, err := makeAuthOptions(lister, opts...)
	if err != nil {
		return nil, err
	}

	craneOpts = append(craneOpts, authOpts...)
	return craneOpts, nil
}

func makeBaseOptions(opts ...Option) []crane.Option {
	craneOpts := make([]crane.Option, 0)
	opt := options{}
	for _, o := range opts {
		o(&opt)
	}

	var transport http.RoundTripper
	if opt.tracing {
		transport = tracing.Transport(transport, otelhttp.WithFilter(tracing.RequestFilterIsInSpan))
	} else {
		transport = DefaultTransport
	}

	craneOpts = append(craneOpts,
		crane.WithTransport(DefaultTransport),
		crane.WithUserAgent(UserAgent),
	)

	return craneOpts
}

func makeAuthOptions(lister v1.SecretInterface, opts ...Option) ([]crane.Option, error) {
	craneOpts := make([]crane.Option, 0)

	opt := options{}
	for _, o := range opts {
		o(&opt)
	}

	if opt.insecure {
		craneOpts = append(craneOpts, crane.Insecure)
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

	craneOpts = append(craneOpts,
		crane.WithAuthFromKeychain(authn.NewMultiKeychain(keychains...)),
	)
	return craneOpts, nil
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
