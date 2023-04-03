package images

import (
	"context"

	"github.com/kyverno/kyverno/pkg/registryclient"
)

type ImageVerifier interface {
	// VerifySignature verifies that the image has the expected signatures
	VerifySignature(ctx context.Context, opts Options) (*Response, error)
	// FetchAttestations retrieves signed attestations and decodes them into in-toto statements
	// https://github.com/in-toto/attestation/blob/main/spec/README.md#statement
	FetchAttestations(ctx context.Context, opts Options) (*Response, error)
}

type Options struct {
	ImageRef             string
	RegistryClient       registryclient.Client
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
	RekorURL             string
	SignatureAlgorithm   string
	PredicateType        string
	Identities           string
}

type Response struct {
	Digest     string
	Statements []map[string]interface{}
}
