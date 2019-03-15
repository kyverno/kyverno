package kubeclient

import (
	"errors"
	"fmt"
	"time"

	"github.com/nirmata/kube-policy/utils"

	certificates "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Issues TLS certificate for webhook server using given PEM private key
// Returns signed and approved TLS certificate in PEM format
func (kc *KubeClient) GenerateTlsPemPair(props utils.TlsCertificateProps) (*utils.TlsPemPair, error) {
	privateKey, err := utils.TlsGeneratePrivateKey()
	if err != nil {
		return nil, err
	}

	certRequest, err := utils.TlsCertificateGenerateRequest(privateKey, props)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to create certificate request: %v", err))
	}

	certRequest, err = kc.submitAndApproveCertificateRequest(certRequest)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to submit and approve certificate request: %v", err))
	}

	tlsCert, err := kc.fetchCertificateFromRequest(certRequest, 10)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to fetch certificate from request: %v", err))
	}

	return &utils.TlsPemPair{
		Certificate: tlsCert,
		PrivateKey:  utils.TlsPrivateKeyToPem(privateKey),
	}, nil
}

// Submits and approves certificate request, returns request which need to be fetched
func (kc *KubeClient) submitAndApproveCertificateRequest(req *certificates.CertificateSigningRequest) (*certificates.CertificateSigningRequest, error) {
	certClient := kc.client.CertificatesV1beta1().CertificateSigningRequests()

	csrList, err := certClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to list existing certificate requests: %v", err))
	}

	for _, csr := range csrList.Items {
		if csr.ObjectMeta.Name == req.ObjectMeta.Name {
			err := certClient.Delete(csr.ObjectMeta.Name, defaultDeleteOptions())
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Unable to delete existing certificate request: %v", err))
			}
			kc.logger.Printf("Old certificate request is deleted")
			break
		}
	}

	res, err := certClient.Create(req)
	if err != nil {
		return nil, err
	}
	kc.logger.Printf("Certificate request %s is created", req.ObjectMeta.Name)

	res.Status.Conditions = append(res.Status.Conditions, certificates.CertificateSigningRequestCondition{
		Type:    certificates.CertificateApproved,
		Reason:  "NKP-Approve",
		Message: "This CSR was approved by Nirmata kube-policy controller",
	})
	res, err = certClient.UpdateApproval(res)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to approve certificate request: %v", err))
	}
	kc.logger.Printf("Certificate request %s is approved", res.ObjectMeta.Name)

	return res, nil
}

const certificateFetchWaitInterval time.Duration = 200 * time.Millisecond

// Fetches certificate from given request. Tries to obtain certificate for maxWaitSeconds
func (kc *KubeClient) fetchCertificateFromRequest(req *certificates.CertificateSigningRequest, maxWaitSeconds uint8) ([]byte, error) {
	// TODO: react of SIGINT and SIGTERM
	timeStart := time.Now()
	certClient := kc.client.CertificatesV1beta1().CertificateSigningRequests()
	for time.Now().Sub(timeStart) < time.Duration(maxWaitSeconds)*time.Second {
		r, err := certClient.Get(req.ObjectMeta.Name, defaultGetOptions())
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
func (kc *KubeClient) ReadTlsPair(props utils.TlsCertificateProps) *utils.TlsPemPair {
	name := generateSecretName(props)
	secret, err := kc.client.CoreV1().Secrets(props.Namespace).Get(name, defaultGetOptions())
	if err != nil {
		kc.logger.Printf("Unable to get secret %s/%s: %s", props.Namespace, name, err)
		return nil
	}

	pemPair := utils.TlsPemPair{
		Certificate: secret.Data[certificateField],
		PrivateKey:  secret.Data[privateKeyField],
	}
	if len(pemPair.Certificate) == 0 {
		kc.logger.Printf("TLS Certificate not found in secret %s/%s", props.Namespace, name)
		return nil
	}
	if len(pemPair.PrivateKey) == 0 {
		kc.logger.Printf("TLS PrivateKey not found in secret %s/%s", props.Namespace, name)
		return nil
	}
	return &pemPair
}

// Writes the pair of TLS certificate and key to the specified secret.
// Updates existing secret or creates new one.
func (kc *KubeClient) WriteTlsPair(props utils.TlsCertificateProps, pemPair *utils.TlsPemPair) error {
	name := generateSecretName(props)
	secret, err := kc.client.CoreV1().Secrets(props.Namespace).Get(name, defaultGetOptions())

	if err == nil { // Update existing secret
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		secret.Data[certificateField] = pemPair.Certificate
		secret.Data[privateKeyField] = pemPair.PrivateKey

		secret, err = kc.client.CoreV1().Secrets(props.Namespace).Update(secret)
		if err == nil {
			kc.logger.Printf("Secret %s is updated", name)
		}

	} else { // Create new secret
		secret = &v1.Secret{
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

		secret, err = kc.client.CoreV1().Secrets(props.Namespace).Create(secret)
		if err == nil {
			kc.logger.Printf("Secret %s is created", name)
		}
	}
	return err
}

func generateSecretName(props utils.TlsCertificateProps) string {
	return utils.GenerateInClusterServiceName(props) + ".kube-policy-tls-pair"
}
