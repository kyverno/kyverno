package sigstoreguard

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/stretchr/testify/require"
)

func TestHistoricalFulcioRootsRemainAvailable(t *testing.T) {
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "historical-fulcio"},
		NotBefore: now.Add(-48 * time.Hour), NotAfter: now.Add(-24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, pub, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	tr, err := root.NewTrustedRoot(root.TrustedRootMediaType01, []root.CertificateAuthority{
		&root.FulcioCertificateAuthority{Root: cert, ValidityPeriodStart: template.NotBefore, ValidityPeriodEnd: template.NotAfter},
	}, nil, nil, nil)
	require.NoError(t, err)
	pool, _, err := fulcioRootsFromTrustedRoot(tr, false)
	require.NoError(t, err)
	_, err = cert.Verify(x509.VerifyOptions{Roots: pool, CurrentTime: now.Add(-36 * time.Hour)})
	require.NoError(t, err, "CPOL verifies historical certificates at signing time")
	_, _, err = FulcioRootsFromTrustedRoot(tr)
	require.Error(t, err, "IVPOL's existing active-only trust selection is unchanged")
}

func TestFulcioHonorsVerificationCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := FulcioRootsWithContext(ctx)
	require.ErrorIs(t, err, context.Canceled)
}
