package cosign

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	cosignlib "github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/stretchr/testify/require"
)

func TestSubjectExpressionMustBeEvaluated(t *testing.T) {
	t.Parallel()
	err := subjectExpressionEvaluated(&v1beta1.Cosign{
		Keyless: &v1beta1.Keyless{
			Identities: []v1beta1.Identity{{
				Issuer:            "https://token.actions.githubusercontent.com",
				SubjectExpression: `"^https://github.com/allowed/repo/"`,
			}},
		},
	})
	require.Error(t, err)

	err = subjectExpressionEvaluated(&v1beta1.Cosign{
		Keyless: &v1beta1.Keyless{
			Identities: []v1beta1.Identity{{
				Issuer:            "https://token.actions.githubusercontent.com",
				SubjectExpression: `"^https://github.com/allowed/repo/"`,
				SubjectRegExp:     `^https://github.com/allowed/repo/`,
			}},
		},
	})
	require.NoError(t, err)
}

func TestSubjectRegExpRejectsOtherRepository(t *testing.T) {
	t.Parallel()
	uri, err := url.Parse("https://github.com/other/repo/.github/workflows/build.yml@refs/heads/main")
	require.NoError(t, err)

	key := generateECDSAKey(t)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(9),
		Subject:      pkix.Name{CommonName: "other"},
		URIs:         []*url.URL{uri},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtraExtensions: []pkix.Extension{{
			Id:    asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1},
			Value: []byte("https://token.actions.githubusercontent.com"),
		}},
	}
	cert := createCertificate(t, template, template, &key.PublicKey, key)

	unconstrained := cosignlib.CheckOpts{Identities: []cosignlib.Identity{{
		Issuer: "https://token.actions.githubusercontent.com",
	}}}
	require.NoError(t, cosignlib.CheckCertificatePolicy(cert, &unconstrained))

	pinned := cosignlib.CheckOpts{Identities: []cosignlib.Identity{{
		Issuer:        "https://token.actions.githubusercontent.com",
		SubjectRegExp: `^https://github.com/allowed/repo/`,
	}}}
	require.Error(t, cosignlib.CheckCertificatePolicy(cert, &pinned))
}
