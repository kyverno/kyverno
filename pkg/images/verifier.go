package images

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v2/pkg/oci/remote"
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
	BuildCosignRemoteOption(context.Context) (remote.Option, error)
	BuildGCRRemoteOption(context.Context) ([]gcrremote.Option, error)
}

type Options struct {
	ImageRef             string
	Client               Client
	FetchAttestations    bool
	Key                  string
	Cert                 string
	CertChain            string
	Roots                string
	Subject              string
	Issuer               string
	AdditionalExtensions map[string]string
	Annotations          map[string]string
	Repository           string
	IgnoreTlog           bool
	RekorURL             string
	RekorPubKey          string
	IgnoreSCT            bool
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
