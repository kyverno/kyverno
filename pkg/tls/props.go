package tls

import (
	"net/url"

	"github.com/kyverno/kyverno/pkg/config"
	"k8s.io/client-go/rest"
)

// CertificateProps Properties of TLS certificate which should be issued for webhook server
type CertificateProps struct {
	Service       string
	Namespace     string
	APIServerHost string
}

// NewCertificateProps creates CertificateProps from a  *rest.Config
func NewCertificateProps(configuration *rest.Config) (*CertificateProps, error) {
	apiServerURL, err := url.Parse(configuration.Host)
	if err != nil {
		return nil, err
	}
	return &CertificateProps{
		Service:       config.KyvernoServiceName(),
		Namespace:     config.KyvernoNamespace(),
		APIServerHost: apiServerURL.Hostname(),
	}, nil
}

// inClusterServiceName The generated service name should be the common name for TLS certificate
// TODO: could be static
func (props *CertificateProps) inClusterServiceName() string {
	return props.Service + "." + props.Namespace + ".svc"
}

func (props *CertificateProps) GenerateTLSPairSecretName() string {
	return props.inClusterServiceName() + ".kyverno-tls-pair"
}

func (props *CertificateProps) GenerateRootCASecretName() string {
	return props.inClusterServiceName() + ".kyverno-tls-ca"
}
