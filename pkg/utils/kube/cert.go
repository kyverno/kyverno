package kube

import (
	"context"
	"crypto/x509"
	"fmt"

	"github.com/kyverno/kyverno/pkg/config"
	"google.golang.org/grpc/credentials"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func FetchCert(
	ctx context.Context,
	certs string,
	kubeClient kubernetes.Interface,
) (credentials.TransportCredentials, error) {
	secret, err := kubeClient.CoreV1().Secrets(config.KyvernoNamespace()).Get(ctx, certs, metav1.GetOptions{})
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
