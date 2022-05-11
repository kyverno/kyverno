package tls

import (
	"net/url"

	"k8s.io/client-go/rest"
)

// CertificateProps Properties of TLS certificate which should be issued for webhook server
type CertificateProps struct {
	APIServerHost string
}

// NewCertificateProps creates CertificateProps from a  *rest.Config
func NewCertificateProps(configuration *rest.Config) (*CertificateProps, error) {
	apiServerURL, err := url.Parse(configuration.Host)
	if err != nil {
		return nil, err
	}
	return &CertificateProps{
		APIServerHost: apiServerURL.Hostname(),
	}, nil
}
