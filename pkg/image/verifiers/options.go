package verifiers

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
