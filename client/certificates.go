package client

import (
	"errors"
	"fmt"
	"time"

	tls "github.com/nirmata/kube-policy/pkg/tls"
	certificates "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Issues TLS certificate for webhook server using given PEM private key
// Returns signed and approved TLS certificate in PEM format
func (c *Client) GenerateTlsPemPair(props tls.TlsCertificateProps) (*tls.TlsPemPair, error) {
	privateKey, err := tls.TlsGeneratePrivateKey()
	if err != nil {
		return nil, err
	}

	certRequest, err := tls.TlsCertificateGenerateRequest(privateKey, props)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to create certificate request: %v", err))
	}

	certRequest, err = c.submitAndApproveCertificateRequest(certRequest)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to submit and approve certificate request: %v", err))
	}

	tlsCert, err := c.fetchCertificateFromRequest(certRequest, 10)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to fetch certificate from request: %v", err))
	}

	return &tls.TlsPemPair{
		Certificate: tlsCert,
		PrivateKey:  tls.TlsPrivateKeyToPem(privateKey),
	}, nil
}

// Submits and approves certificate request, returns request which need to be fetched
func (c *Client) submitAndApproveCertificateRequest(req *certificates.CertificateSigningRequest) (*certificates.CertificateSigningRequest, error) {
	//TODO: using the CSR interface from the kubeclient
	certClient, err := c.GetCSRInterface()
	if err != nil {
		return nil, err
	}
	//	certClient := kc.client.CertificatesV1beta1().CertificateSigningRequests()
	csrList, err := c.ListResource("certificatesigningrequests", "")
	//	csrList, err := certClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to list existing certificate requests: %v", err))
	}

	for _, csr := range csrList.Items {
		csr.GetName()
		if csr.GetName() == req.ObjectMeta.Name {
			// Delete
			err := c.DeleteResouce("certificatesigningrequests", "", csr.GetName())
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Unable to delete existing certificate request: %v", err))
			}
			c.logger.Printf("Old certificate request is deleted")
			break
		}
	}

	// Create
	unstrRes, err := c.CreateResource("certificatesigningrequests", "", req)
	//	res, err := certClient.Create(req)
	if err != nil {
		return nil, err
	}
	c.logger.Printf("Certificate request %s is created", unstrRes.GetName())

	res, err := convertToCSR(unstrRes)
	if err != nil {
		return nil, err
	}
	res.Status.Conditions = append(res.Status.Conditions, certificates.CertificateSigningRequestCondition{
		Type:    certificates.CertificateApproved,
		Reason:  "NKP-Approve",
		Message: "This CSR was approved by Nirmata kube-policy controller",
	})
	res, err = certClient.UpdateApproval(res)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to approve certificate request: %v", err))
	}
	c.logger.Printf("Certificate request %s is approved", res.ObjectMeta.Name)

	return res, nil
}

const certificateFetchWaitInterval time.Duration = 200 * time.Millisecond

// Fetches certificate from given request. Tries to obtain certificate for maxWaitSeconds
func (c *Client) fetchCertificateFromRequest(req *certificates.CertificateSigningRequest, maxWaitSeconds uint8) ([]byte, error) {
	// TODO: react of SIGINT and SIGTERM
	timeStart := time.Now()
	c.GetResource("certificatesigningrequests", "", req.ObjectMeta.Name)
	for time.Now().Sub(timeStart) < time.Duration(maxWaitSeconds)*time.Second {
		unstrR, err := c.GetResource("certificatesigningrequests", "", req.ObjectMeta.Name)
		if err != nil {
			return nil, err
		}
		r, err := convertToCSR(unstrR)
		if err != nil {
			return nil, err
		}

		if r.Status.Certificate != nil {
			return r.Status.Certificate, nil
		}

		for _, condition := range r.Status.Conditions {
			if condition.Type == certificates.CertificateDenied {
				return nil, errors.New(condition.String())
			}
		}
	}
	return nil, errors.New(fmt.Sprintf("Cerificate fetch timeout is reached: %d seconds", maxWaitSeconds))
}

const privateKeyField string = "privateKey"
const certificateField string = "certificate"

// Reads the pair of TLS certificate and key from the specified secret.
func (c *Client) ReadTlsPair(props tls.TlsCertificateProps) *tls.TlsPemPair {
	name := generateSecretName(props)
	unstrSecret, err := c.GetResource("secrets", props.Namespace, name)
	if err != nil {
		c.logger.Printf("Unable to get secret %s/%s: %s", props.Namespace, name, err)
		return nil
	}
	secret, err := convertToSecret(unstrSecret)
	if err != nil {
		return nil
	}
	pemPair := tls.TlsPemPair{
		Certificate: secret.Data[certificateField],
		PrivateKey:  secret.Data[privateKeyField],
	}
	if len(pemPair.Certificate) == 0 {
		c.logger.Printf("TLS Certificate not found in secret %s/%s", props.Namespace, name)
		return nil
	}
	if len(pemPair.PrivateKey) == 0 {
		c.logger.Printf("TLS PrivateKey not found in secret %s/%s", props.Namespace, name)
		return nil
	}
	return &pemPair
}

// Writes the pair of TLS certificate and key to the specified secret.
// Updates existing secret or creates new one.
func (c *Client) WriteTlsPair(props tls.TlsCertificateProps, pemPair *tls.TlsPemPair) error {
	name := generateSecretName(props)
	unstrSecret, err := c.GetResource("secrets", props.Namespace, name)
	if err == nil { // Update existing secret
		secret, err := convertToSecret(unstrSecret)
		if err != nil {
			return nil
		}

		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		secret.Data[certificateField] = pemPair.Certificate
		secret.Data[privateKeyField] = pemPair.PrivateKey
		c.UpdateResource("secrets", props.Namespace, secret)
		if err == nil {
			c.logger.Printf("Secret %s is updated", name)
		}

	} else { // Create new secret

		secret := &v1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: props.Namespace,
			},
			Data: map[string][]byte{
				certificateField: pemPair.Certificate,
				privateKeyField:  pemPair.PrivateKey,
			},
		}

		_, err := c.CreateResource("secrets", props.Namespace, secret)
		if err == nil {
			c.logger.Printf("Secret %s is created", name)
		}
	}
	return err
}

func generateSecretName(props tls.TlsCertificateProps) string {
	return tls.GenerateInClusterServiceName(props) + ".kube-policy-tls-pair"
}
