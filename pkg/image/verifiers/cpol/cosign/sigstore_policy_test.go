package cosign

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/digitorus/timestamp"
	"github.com/kyverno/kyverno/pkg/image/verifiers"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	pbundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	common "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	pdsse "github.com/sigstore/protobuf-specs/gen/pb-go/dsse"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

func policyTestCertificate(t *testing.T, parent *x509.Certificate, parentKey *ecdsa.PrivateKey, usage x509.ExtKeyUsage) (*x509.Certificate, *ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: serial.String()},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature,
	}
	if parent == nil {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
		parent, parentKey = template, key
	} else {
		template.EmailAddresses = []string{"signer@example.test"}
		template.ExtKeyUsage = []x509.ExtKeyUsage{usage}
		if usage == x509.ExtKeyUsageTimeStamping {
			eku, err := asn1.Marshal([]asn1.ObjectIdentifier{{1, 3, 6, 1, 5, 5, 7, 3, 8}})
			if err != nil {
				t.Fatal(err)
			}
			template.ExtraExtensions = []pkix.Extension{{Id: asn1.ObjectIdentifier{2, 5, 29, 37}, Critical: true, Value: eku}}
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert, key, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestPolicyBundleTrust(t *testing.T) {
	signCA, signKey, signPEM := policyTestCertificate(t, nil, nil, 0)
	signLeaf, leafKey, leafPEM := policyTestCertificate(t, signCA, signKey, x509.ExtKeyUsageCodeSigning)
	tsaCA, tsaKey, tsaPEM := policyTestCertificate(t, nil, nil, 0)
	tsaLeaf, tsaLeafKey, tsaLeafPEM := policyTestCertificate(t, tsaCA, tsaKey, x509.ExtKeyUsageTimeStamping)
	_, _, unrelatedPEM := policyTestCertificate(t, nil, nil, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opts := verifiers.Options{Roots: signPEM, TSACertChain: tsaLeafPEM + tsaPEM, IgnoreTlog: true, IgnoreSCT: true}

	payload := []byte(`{"_type":"https://in-toto.io/Statement/v0.1","subject":[{"name":"image","digest":{"sha256":"0000000000000000000000000000000000000000000000000000000000000000"}}],"predicateType":"test","predicate":{"status":"active"}}`)
	payloadType := "application/vnd.in-toto+json"
	hash := sha256.Sum256(dsse.PAE(payloadType, payload))
	sig, err := ecdsa.SignASN1(rand.Reader, leafKey, hash[:])
	if err != nil {
		t.Fatal(err)
	}
	imprint := sha256.Sum256(sig)
	ts := &timestamp.Timestamp{
		HashAlgorithm: crypto.SHA256, HashedMessage: imprint[:], Time: time.Now(),
		Policy: asn1.ObjectIdentifier{1, 2, 3, 4}, SerialNumber: big.NewInt(1),
		AddTSACertificate: true,
	}
	timestampDER, err := ts.CreateResponseWithOpts(tsaLeaf, tsaLeafKey, crypto.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	b, err := bundle.NewBundle(&pbundle.Bundle{
		MediaType: "application/vnd.dev.sigstore.bundle.v0.3+json",
		Content: &pbundle.Bundle_DsseEnvelope{DsseEnvelope: &pdsse.Envelope{
			Payload: payload, PayloadType: payloadType, Signatures: []*pdsse.Signature{{Sig: sig}},
		}},
		VerificationMaterial: &pbundle.VerificationMaterial{
			Content: &pbundle.VerificationMaterial_Certificate{Certificate: &common.X509Certificate{RawBytes: signLeaf.Raw}},
			TimestampVerificationData: &pbundle.TimestampVerificationData{
				Rfc3161Timestamps: []*common.RFC3161SignedTimestamp{{SignedTimestamp: timestampDER}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	policy := verify.NewPolicy(verify.WithArtifactDigest("sha256", make([]byte, 32)), verify.WithoutIdentitiesUnsafe())
	for _, tc := range []struct {
		name    string
		signing string
		tsa     string
		valid   bool
	}{
		{"policy chain", signPEM, tsaLeafPEM + tsaPEM, true},
		{"TSA CA only", signPEM, tsaPEM, true},
		{"multi-root signing", signPEM + unrelatedPEM, tsaLeafPEM + tsaPEM, true},
		{"multi-root TSA", signPEM, tsaLeafPEM + tsaPEM + unrelatedPEM, true},
		{"wrong signing CA", unrelatedPEM, tsaLeafPEM + tsaPEM, false},
		{"wrong TSA CA", signPEM, unrelatedPEM, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := opts
			options.Roots, options.TSACertChain = tc.signing, tc.tsa
			trusted, err := getBundleTrustedRoot(ctx, options)
			if err != nil {
				t.Fatal(err)
			}
			if len(trusted.RekorLogs()) != 0 || len(trusted.CTLogs()) != 0 {
				t.Fatal("policy trust must not contain log keys")
			}
			v, err := verify.NewSignedEntityVerifier(trusted, buildVerifyOptions(options)...)
			if err != nil {
				t.Fatal(err)
			}
			_, err = v.Verify(b, policy)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, verification error: %v", tc.valid, err)
			}
			if tc.valid {
				saved := b.VerificationMaterial.TimestampVerificationData
				b.VerificationMaterial.TimestampVerificationData = nil
				_, err = v.Verify(b, policy)
				b.VerificationMaterial.TimestampVerificationData = saved
				if err == nil {
					t.Fatal("missing timestamp accepted with ignoreSCT=true")
				}
				wrongDigest := verify.NewPolicy(verify.WithArtifactDigest("sha256", []byte{1}), verify.WithoutIdentitiesUnsafe())
				if _, err := v.Verify(b, wrongDigest); err == nil {
					t.Fatal("wrong image digest accepted")
				}
			}
		})
	}
	for _, tc := range []struct {
		name string
		opts verifiers.Options
	}{
		{"missing signing roots", verifiers.Options{TSACertChain: tsaPEM}},
		{"missing TSA chain", verifiers.Options{Roots: signPEM}},
		{"missing ignoreTlog", verifiers.Options{Roots: signPEM, TSACertChain: tsaLeafPEM + tsaPEM}},
		{"invalid signing PEM", verifiers.Options{Roots: "invalid", TSACertChain: tsaPEM, IgnoreTlog: true}},
		{"invalid TSA PEM", verifiers.Options{Roots: signPEM, TSACertChain: "invalid", IgnoreTlog: true}},
		{"signing leaf as root", verifiers.Options{Roots: leafPEM, TSACertChain: tsaPEM, IgnoreTlog: true}},
		{"TSA without root", verifiers.Options{Roots: signPEM, TSACertChain: tsaLeafPEM, IgnoreTlog: true}},
		{"multiple TSA leaves", verifiers.Options{Roots: signPEM, TSACertChain: tsaLeafPEM + tsaLeafPEM + tsaPEM, IgnoreTlog: true}},
		{"oversized signing roots", verifiers.Options{Roots: strings.Repeat(signPEM, 2*maxIntermediateCerts+1), TSACertChain: tsaPEM, IgnoreTlog: true}},
		{"oversized TSA chain", verifiers.Options{Roots: signPEM, TSACertChain: strings.Repeat(tsaPEM, 2*maxIntermediateCerts+2), IgnoreTlog: true}},
		{"too many signing roots", verifiers.Options{Roots: strings.Repeat(signPEM, maxRootCerts+1), TSACertChain: tsaPEM, IgnoreTlog: true}},
		{"too many TSA roots", verifiers.Options{Roots: signPEM, TSACertChain: strings.Repeat(tsaPEM, maxRootCerts+1), IgnoreTlog: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := getBundleTrustedRoot(ctx, tc.opts); err == nil {
				t.Fatal("invalid policy trust must fail without TUF fallback")
			}
		})
	}
}

func TestPolicyBundleVerifyOptions(t *testing.T) {
	if got := len(buildVerifyOptions(verifiers.Options{})); got != 2 {
		t.Fatalf("default log and observer requirements changed: %d", got)
	}
	if got := len(buildVerifyOptions(verifiers.Options{IgnoreTlog: true, IgnoreSCT: true, TSACertChain: "configured"})); got != 1 {
		t.Fatalf("expected signed timestamp requirement despite ignoreSCT=true: %d", got)
	}
	if got := len(buildVerifyOptions(verifiers.Options{IgnoreTlog: true, IgnoreSCT: false, TSACertChain: "configured"})); got != 2 {
		t.Fatalf("expected independent signed timestamp and observer requirements: %d", got)
	}
	if got := len(buildVerifyOptions(verifiers.Options{Key: "k", IgnoreTlog: true, IgnoreSCT: false, TSACertChain: "configured"})); got != 1 {
		t.Fatalf("key-based attestor must not require a policy TSA signed timestamp: %d", got)
	}
}
