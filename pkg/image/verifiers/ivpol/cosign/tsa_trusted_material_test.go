package cosign

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTSALeafCert(t *testing.T, issuerCert *x509.Certificate, issuerKey *rsa.PrivateKey, serial int64) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key := generateECDSAKey(t)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(serial),
		Subject:               pkix.Name{CommonName: "Test TSA Signer"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}
	cert := createCertificate(t, tmpl, issuerCert, &key.PublicKey, issuerKey)
	return cert, key
}

func generateTSAChain(t *testing.T) (leaf, intermediate, rootCert *x509.Certificate) {
	t.Helper()
	rootCert, rootKey := generateRootCA(t)
	intermediate, intermediateKey := generateIntermediateCA(t, rootCert, rootKey)
	leaf, _ = generateTSALeafCert(t, intermediate, intermediateKey, 100)
	return leaf, intermediate, rootCert
}

func emptyPublicTrustedRoot(t *testing.T) *root.TrustedRoot {
	t.Helper()
	tr, err := root.NewTrustedRoot(root.TrustedRootMediaType01, nil, nil, nil, nil)
	require.NoError(t, err)
	return tr
}

func publicTrustedRootWithTSA(t *testing.T) (*root.TrustedRoot, *root.SigstoreTimestampingAuthority) {
	t.Helper()
	leaf, intermediate, rootCert := generateTSAChain(t)
	tsa := &root.SigstoreTimestampingAuthority{
		Root:          rootCert,
		Intermediates: []*x509.Certificate{intermediate},
		Leaf:          leaf,
	}
	tr, err := root.NewTrustedRoot(root.TrustedRootMediaType01, nil, nil, []root.TimestampingAuthority{tsa}, nil)
	require.NoError(t, err)
	return tr, tsa
}

func TestTSAOnlyTrustedMaterial_ExposesConfiguredTSA(t *testing.T) {
	leaf, intermediate, rootCert := generateTSAChain(t)
	tsa := &root.SigstoreTimestampingAuthority{
		Root:          rootCert,
		Intermediates: []*x509.Certificate{intermediate},
		Leaf:          leaf,
	}
	tm := &tsaOnlyTrustedMaterial{tsa: tsa}

	tsAs := tm.TimestampingAuthorities()
	require.Len(t, tsAs, 1)
	assert.Same(t, tsa, tsAs[0])
}

// Other TrustedMaterial methods must return zero values so the wrapper
// doesn't shadow the public-root member when composed in a collection.
func TestTSAOnlyTrustedMaterial_OtherMethodsReturnDefaults(t *testing.T) {
	leaf, intermediate, rootCert := generateTSAChain(t)
	tsa := &root.SigstoreTimestampingAuthority{
		Root:          rootCert,
		Intermediates: []*x509.Certificate{intermediate},
		Leaf:          leaf,
	}
	tm := &tsaOnlyTrustedMaterial{tsa: tsa}

	assert.Empty(t, tm.FulcioCertificateAuthorities())
	assert.Empty(t, tm.RekorLogs())
	assert.Empty(t, tm.CTLogs())
}

func TestMergeTSAIntoTrustedMaterial_NilLeafReturnsPublicRootUnchanged(t *testing.T) {
	publicRoot := emptyPublicTrustedRoot(t)
	got := mergeTSAIntoTrustedMaterial(publicRoot, nil, nil, nil)
	assert.Same(t, publicRoot, got)
}

func TestMergeTSAIntoTrustedMaterial_NoRootsReturnsPublicRootUnchanged(t *testing.T) {
	publicRoot := emptyPublicTrustedRoot(t)
	leaf, intermediate, _ := generateTSAChain(t)
	got := mergeTSAIntoTrustedMaterial(publicRoot, leaf, []*x509.Certificate{intermediate}, nil)
	assert.Same(t, publicRoot, got, "without a root cert the merge is impossible; preserve the public root")
}

func TestMergeTSAIntoTrustedMaterial_AggregatesTSAsFromBothSources(t *testing.T) {
	publicRoot, publicTSA := publicTrustedRootWithTSA(t)
	customLeaf, customInt, customRoot := generateTSAChain(t)

	tm := mergeTSAIntoTrustedMaterial(publicRoot, customLeaf, []*x509.Certificate{customInt}, []*x509.Certificate{customRoot})

	tsAs := tm.TimestampingAuthorities()
	require.Len(t, tsAs, 2)

	foundPublic, foundCustom := false, false
	for _, tsa := range tsAs {
		st, ok := tsa.(*root.SigstoreTimestampingAuthority)
		if !ok {
			continue
		}
		if st == publicTSA {
			foundPublic = true
		} else if st.Leaf != nil && st.Leaf.SerialNumber == customLeaf.SerialNumber {
			foundCustom = true
		}
	}
	assert.True(t, foundPublic)
	assert.True(t, foundCustom)
}

func TestMergeTSAIntoTrustedMaterial_ReturnsCollectionType(t *testing.T) {
	publicRoot := emptyPublicTrustedRoot(t)
	leaf, intermediate, rootCert := generateTSAChain(t)

	tm := mergeTSAIntoTrustedMaterial(publicRoot, leaf, []*x509.Certificate{intermediate}, []*x509.Certificate{rootCert})
	_, ok := tm.(root.TrustedMaterialCollection)
	assert.True(t, ok)
}

// SigstoreTimestampingAuthority has a single Root field; only the first
// element of customTSARoots is used.
func TestMergeTSAIntoTrustedMaterial_OnlyFirstRootIsUsed(t *testing.T) {
	publicRoot := emptyPublicTrustedRoot(t)
	leaf, intermediate, root1 := generateTSAChain(t)
	_, _, root2 := generateTSAChain(t)

	tm := mergeTSAIntoTrustedMaterial(publicRoot, leaf, []*x509.Certificate{intermediate}, []*x509.Certificate{root1, root2})

	tsAs := tm.TimestampingAuthorities()
	require.Len(t, tsAs, 1)
	sta, ok := tsAs[0].(*root.SigstoreTimestampingAuthority)
	require.True(t, ok)
	assert.Same(t, root1, sta.Root)
	assert.NotSame(t, root2, sta.Root)
}
