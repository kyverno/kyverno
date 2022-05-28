package tls

import (
	"net/url"

	"k8s.io/client-go/rest"
)

// certificateProps Properties of TLS certificate which should be issued for webhook server
type certificateProps struct {
	apiServerHost string
}

// newCertificateProps creates CertificateProps from a  *rest.Config
func newCertificateProps(configuration *rest.Config) (*certificateProps, error) {
	apiServerURL, err := url.Parse(configuration.Host)
	if err != nil {
		return nil, err
	}
	return &certificateProps{
		apiServerHost: apiServerURL.Hostname(),
	}, nil
}
