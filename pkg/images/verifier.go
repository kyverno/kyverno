package images

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
)

type ImageVerifier interface {
	// VerifySignature verifies that the image has the expected signatures
	VerifySignature(ctx context.Context, opts Options) (*Response, error)
	// FetchAttestations retrieves signed attestations and decodes them into in-toto statements
	// https://github.com/in-toto/attestation/blob/main/spec/README.md#statement
	FetchAttestations(ctx context.Context, opts Options) (*Response, error)
}

type Client interface {
	Keychain() authn.Keychain
	Options(context.Context) ([]gcrremote.Option, error)
	NameOptions() []name.Option
}

type Options struct {
	SigstoreBundle       bool
	ImageRef             string
	Client               Client
	FetchAttestations    bool
	Key                  string
	Cert                 string
	CertChain            string
	Roots                string
	Subject              string
	SubjectRegExp        string
	Issuer               string
	IssuerRegExp         string
	AdditionalExtensions map[string]string
	Annotations          map[string]string
	Repository           string
	CosignOCI11          bool
	IgnoreTlog           bool
	RekorURL             string
	RekorPubKey          string
	IgnoreSCT            bool
	TSACertChain         string
	CTLogsPubKey         string
	SignatureAlgorithm   string
	PredicateType        string
	Type                 string
	Identities           string
}

type Response struct {
	Digest     string
	Statements []map[string]interface{}
}
