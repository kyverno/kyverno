package tls

import (
	"context"
	"crypto/x509"
	"fmt"

	"google.golang.org/grpc/credentials"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func FetchCert(
	ctx context.Context,
	namespace string,
	name string,
	kubeClient kubernetes.Interface,
) (credentials.TransportCredentials, error) {
	secret, err := kubeClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error fetching certificate from secret")
	}

	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(secret.Data["ca.pem"]) {
		return nil, fmt.Errorf("credentials: failed to append certificates")
	}

	transportCreds := credentials.NewClientTLSFromCert(cp, "")
	return transportCreds, nil
}
