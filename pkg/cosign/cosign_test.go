package cosign

import (
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/pkg/images"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/cosign/bundle"
	"github.com/sigstore/cosign/v2/pkg/oci"
	"gotest.tools/assert"
)

const cosignPayload = `{
  "critical": {
 	   "identity": {
 	     "docker-reference": "registry-v2.nirmata.io/pause"
 	    },
   	"image": {
 	     "docker-manifest-digest": "sha256:4a1c4b21597c1b4415bdbecb28a3296c6b5e23ca4f9feeb599860a1dac6a0108"
 	    },
 	    "type": "cosign container image signature"
    },
    "optional": {
		"foo": "bar",
		"bar": "baz"
	}
}`

const keylessPayload = `{
	"critical": {
		"identity": {
			"docker-reference": "ghcr.io/kyverno/test-verify-image"
		},
		"image": {
			"docker-manifest-digest": "sha256:ee53528c4e3c723945cf870d73702b76135955a218dd7497bf344aa73ebb4227"
		},
		"type": "cosign container image signature"
	},
	"optional": {
		"Bundle": {
			"SignedEntryTimestamp": "--TIME-STAMP--",
			"Payload": {
				"integratedTime": 1689234389,
				"logIndex": 27432442,
				"logID": "--LOG-ID--"
			}
		},
		"Issuer": "https://accounts.google.com",
		"Subject": "kyverno@nirmata.com"
	}
}`

const globalRekorPubKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE2G2Y+2tabdTV5BcGiBIx0a9fAFwr
kBbmLSGtks4L3qX6yYY0zufBnhC8Ur/iy55GhWP/9A/bY2LhC30M9+RYtw==
-----END PUBLIC KEY-----
`

const wrongPubKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEoiR2ouEAp4JS/JIgkCVYCxpp/dMe
4Mkc/92O8rbWs6xIAcIEju7+Z2yecpQH6RbztEVCZbBZhEVfMdRgWKOrrQ==
-----END PUBLIC KEY-----`

func TestCosignPayload(t *testing.T) {
	image := "registry-v2.nirmata.io/pause"
	signedPayloads := cosign.SignedPayload{Payload: []byte(cosignPayload)}
	ociSig, err := getSignature(signedPayloads)
	assert.NilError(t, err)
	p, err := extractPayload([]oci.Signature{ociSig})
	assert.NilError(t, err)
	a := map[string]string{"foo": "bar"}
	err = checkAnnotations(p, a)
	assert.NilError(t, err)
	d, err := extractDigest(image, p)
	assert.NilError(t, err)
	assert.Equal(t, d, "sha256:4a1c4b21597c1b4415bdbecb28a3296c6b5e23ca4f9feeb599860a1dac6a0108")

	image2 := "ghcr.io/kyverno/test-verify-image"
	signedPayloads2 := cosign.SignedPayload{Payload: []byte(keylessPayload)}
	ociSig, err = getSignature(signedPayloads2)
	assert.NilError(t, err)
	signatures2 := []oci.Signature{ociSig}

	p2, err := extractPayload(signatures2)
	assert.NilError(t, err)

	d2, err := extractDigest(image2, p2)
	assert.NilError(t, err)
	assert.Equal(t, d2, "sha256:ee53528c4e3c723945cf870d73702b76135955a218dd7497bf344aa73ebb4227")
}

func TestCosignInvalidSignatureAlgorithm(t *testing.T) {
	opts := images.Options{
		ImageRef:           "ghcr.io/jimbugwadia/pause2",
		Client:             nil,
		FetchAttestations:  false,
		Key:                globalRekorPubKey,
		SignatureAlgorithm: "sha1",
	}

	rc, err := registryclient.New()
	assert.NilError(t, err)
	opts.Client = rc

	verifier := &cosignVerifier{}
	_, err = verifier.VerifySignature(context.TODO(), opts)
	assert.ErrorContains(t, err, "invalid signature algorithm provided sha1")
}

func TestCosignKeyless(t *testing.T) {
	opts := images.Options{
		ImageRef:  "ghcr.io/jimbugwadia/pause2",
		Issuer:    "https://github.com/",
		Subject:   "jim",
		RekorURL:  "https://rekor.sigstore.dev",
		IgnoreSCT: true,
	}

	rc, err := registryclient.New()
	assert.NilError(t, err)
	opts.Client = rc

	verifier := &cosignVerifier{}
	_, err = verifier.VerifySignature(context.TODO(), opts)
	assert.ErrorContains(t, err, "subject mismatch: expected jim, received jim@nirmata.com")

	opts.Subject = "jim@nirmata.com"
	_, err = verifier.VerifySignature(context.TODO(), opts)
	assert.ErrorContains(t, err, "issuer mismatch: expected https://github.com/, received https://github.com/login/oauth")

	opts.Issuer = "https://github.com/login/oauth"
	_, err = verifier.VerifySignature(context.TODO(), opts)
	assert.NilError(t, err)
}

func TestRekorPubkeys(t *testing.T) {
	opts := images.Options{
		ImageRef:    "ghcr.io/jimbugwadia/pause2",
		Issuer:      "https://github.com/login/oauth",
		Subject:     "jim@nirmata.com",
		RekorURL:    "--INVALID--", // To avoid using the default rekor url as thats where signature is uploaded
		RekorPubKey: wrongPubKey,
		IgnoreSCT:   true,
	}

	rc, err := registryclient.New()
	assert.NilError(t, err)
	opts.Client = rc

	verifier := &cosignVerifier{}
	_, err = verifier.VerifySignature(context.TODO(), opts)
	assert.ErrorContains(t, err, "rekor log public key not found for payload")

	opts.RekorPubKey = globalRekorPubKey
	_, err = verifier.VerifySignature(context.TODO(), opts)
	assert.NilError(t, err)
}

func TestIgnoreTlogsandIgnoreSCT(t *testing.T) {
	err := SetMock("ghcr.io/kyverno/test-verify-image", [][]byte{[]byte(keylessPayload)})
	defer ClearMock()
	assert.NilError(t, err)

	opts := images.Options{
		ImageRef: "ghcr.io/kyverno/test-verify-image",
	}

	rc, err := registryclient.New()
	assert.NilError(t, err)
	opts.Client = rc

	verifier := &cosignVerifier{}

	opts.RekorPubKey = "--INVALID KEY--"
	_, err = verifier.VerifySignature(context.TODO(), opts)
	// RekorPubKey is checked when ignoreTlog is set to false
	assert.ErrorContains(t, err, "failed to load Rekor public keys: failed to get rekor public keys: PEM decoding failed")

	opts.IgnoreTlog = true
	_, err = verifier.VerifySignature(context.TODO(), opts)
	// RekorPubKey is NOT checked when ignoreTlog is set to true
	assert.NilError(t, err)

	opts.CTLogsPubKey = "--INVALID KEY--"
	_, err = verifier.VerifySignature(context.TODO(), opts)
	// CTLogsPubKey is checked when ignoreSCT is set to false
	assert.ErrorContains(t, err, "failed to load CTLogs public keys: failed to get transparency log public keys: PEM decoding failed")

	opts.IgnoreSCT = true
	_, err = verifier.VerifySignature(context.TODO(), opts)
	// CTLogsPubKey is NOT checked when ignoreSCT is set to true
	assert.NilError(t, err)
}

func TestCTLogsPubkeys(t *testing.T) {
	opts := images.Options{
		ImageRef:     "ghcr.io/vishal-chdhry/cosign-test:v1",
		Issuer:       "https://accounts.google.com",
		Subject:      "vishal.choudhary@nirmata.com",
		RekorPubKey:  globalRekorPubKey,
		CTLogsPubKey: wrongPubKey,
	}

	rc, err := registryclient.New()
	assert.NilError(t, err)
	opts.Client = rc

	verifier := &cosignVerifier{}
	_, err = verifier.VerifySignature(context.TODO(), opts)
	assert.ErrorContains(t, err, "ctfe public key not found for payload.")

	opts.CTLogsPubKey = ""
	_, err = verifier.VerifySignature(context.TODO(), opts)
	assert.NilError(t, err)
}

func TestCosignMatchCertificateData(t *testing.T) {
	pem1 := "-----BEGIN CERTIFICATE-----\nMIIDtzCCAzygAwIBAgIUX9MdOHZMlRONmc0Iu3DtiLXLVLYwCgYIKoZIzj0EAwMw\nNzEVMBMGA1UEChMMc2lnc3RvcmUuZGV2MR4wHAYDVQQDExVzaWdzdG9yZS1pbnRl\ncm1lZGlhdGUwHhcNMjIxMDA3MTkyNDI0WhcNMjIxMDA3MTkzNDI0WjAAMFkwEwYH\nKoZIzj0CAQYIKoZIzj0DAQcDQgAE0+a5/FhwY4fREWP++3V4rciGiqWGRgHaiP1z\nSlWihKkU71sBVeTzjdrcN8wXzBAefqh5URBfCeE8pJRfQsVKxKOCAlswggJXMA4G\nA1UdDwEB/wQEAwIHgDATBgNVHSUEDDAKBggrBgEFBQcDAzAdBgNVHQ4EFgQUJy79\nhpkwHtXtLWOvFu/icY56bwgwHwYDVR0jBBgwFoAU39Ppz1YkEZb5qNjpKFWixi4Y\nZD8wbgYDVR0RAQH/BGQwYoZgaHR0cHM6Ly9naXRodWIuY29tL0ppbUJ1Z3dhZGlh\nL2RlbW8tamF2YS10b21jYXQvLmdpdGh1Yi93b3JrZmxvd3MvcHVibGlzaC55YW1s\nQHJlZnMvdGFncy92MC4wLjIyMDkGCisGAQQBg78wAQEEK2h0dHBzOi8vdG9rZW4u\nYWN0aW9ucy5naXRodWJ1c2VyY29udGVudC5jb20wEgYKKwYBBAGDvzABAgQEcHVz\naDA2BgorBgEEAYO/MAEDBChjNzY0NTI4NGZhN2FlYmU1NTQ2MThlZWU4NzliNGQ2\nOTQ3Zjg1NjRlMB8GCisGAQQBg78wAQQEEWJ1aWxkLXNpZ24tYXR0ZXN0MCoGCisG\nAQQBg78wAQUEHEppbUJ1Z3dhZGlhL2RlbW8tamF2YS10b21jYXQwHwYKKwYBBAGD\nvzABBgQRcmVmcy90YWdzL3YwLjAuMjIwgYoGCisGAQQB1nkCBAIEfAR6AHgAdgAI\nYJLwKFL/aEXR0WsnhJxFZxisFj3DONJt5rwiBjZvcgAAAYOz5+pbAAAEAwBHMEUC\nIBb8fwsLBOu+qJkL6UhT4pwGvRVAN2n74BF1BL703rqPAiEAznbfgYJbqA+JIUiQ\nwwLiFOD8pqidSl+HhW8Lhdg3o+wwCgYIKoZIzj0EAwMDaQAwZgIxAJIBIkZBhM+K\nkBIFNeuWBsyVaAcFRallz3C8jvPQCPbec0ZpIsw624dUs8zD3c96AQIxALf875rt\n+oZgwE6hsDazJzoTcBZ1mYVF6bAlwVdtMiC98aApG6T+qaBirxSgu7IGQw==\n-----END CERTIFICATE-----\n"
	cert1, err := loadCert([]byte(pem1))
	assert.NilError(t, err)

	subject1 := "https://github.com/JimBugwadia/demo-java-tomcat/.github/workflows/publish.yaml@refs/tags/*"
	subject1RegExp := `https://github\.com/JimBugwadia/demo-java-tomcat/.+`
	issuer1 := "https://token.actions.githubusercontent.com"
	issuer1RegExp := `https://token\.actions\..+`

	extensions := map[string]string{
		"githubWorkflowTrigger":    "push",
		"githubWorkflowSha":        "c7645284fa7aebe554618eee879b4d6947f8564e",
		"githubWorkflowName":       "build-sign-attest",
		"githubWorkflowRepository": "JimBugwadia/demo-java-tomcat",
	}

	matchErr := matchCertificateData(cert1, subject1, "", issuer1, "", extensions)
	assert.NilError(t, matchErr)

	matchErr = matchCertificateData(cert1, "", "", issuer1, "", extensions)
	assert.NilError(t, matchErr)

	matchErr = matchCertificateData(cert1, subject1, "", issuer1, "", nil)
	assert.NilError(t, matchErr)

	matchErr = matchCertificateData(cert1, "", subject1RegExp, "", issuer1RegExp, nil)
	assert.NilError(t, matchErr)

	matchErr = matchCertificateData(cert1, "", "", "", issuer1RegExp, nil)
	assert.NilError(t, matchErr)

	matchErr = matchCertificateData(cert1, subject1, subject1RegExp, issuer1, issuer1RegExp, nil)
	assert.NilError(t, matchErr)

	matchErr = matchCertificateData(cert1, "", `^wrong-regex$`, issuer1, issuer1RegExp, nil)
	assert.Error(t, matchErr, "subject mismatch: expected ^wrong-regex$, received https://github.com/JimBugwadia/demo-java-tomcat/.github/workflows/publish.yaml@refs/tags/v0.0.22")

	matchErr = matchCertificateData(cert1, "", "", "", `^wrong-regex$`, nil)
	assert.Error(t, matchErr, "issuer mismatch: expected ^wrong-regex$, received https://token.actions.githubusercontent.com")

	matchErr = matchCertificateData(cert1, "wrong-subject", "", issuer1, "", extensions)
	assert.Error(t, matchErr, "subject mismatch: expected wrong-subject, received https://github.com/JimBugwadia/demo-java-tomcat/.github/workflows/publish.yaml@refs/tags/v0.0.22")

	matchErr = matchCertificateData(cert1, "", "*", "", issuer1RegExp, nil)
	assert.Error(t, matchErr, "invalid regexp for subject: * : error parsing regexp: missing argument to repetition operator: `*`")

	matchErr = matchCertificateData(cert1, "", subject1RegExp, "", "?", nil)
	assert.Error(t, matchErr, "invalid regexp for issuer: ? : error parsing regexp: missing argument to repetition operator: `?`")

	extensions["githubWorkflowTrigger"] = "pull"
	matchErr = matchCertificateData(cert1, subject1, "", issuer1, "", extensions)
	assert.Error(t, matchErr, "extension mismatch: expected pull for key githubWorkflowTrigger, received push")
}

func TestTSACertChain(t *testing.T) {
	key := `
-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEstG5Xl7UxkQsmLUxdmS85HLgYBFy
c/P/oQ22iazkKm8P0sNlaZiaZC4TSEea3oh2Pim0+wxSubhKoK+7jq9Egg==
-----END PUBLIC KEY-----`

	tsaCertChain := `
-----BEGIN CERTIFICATE-----
MIIH/zCCBeegAwIBAgIJAMHphhYNqOmAMA0GCSqGSIb3DQEBDQUAMIGVMREwDwYD
VQQKEwhGcmVlIFRTQTEQMA4GA1UECxMHUm9vdCBDQTEYMBYGA1UEAxMPd3d3LmZy
ZWV0c2Eub3JnMSIwIAYJKoZIhvcNAQkBFhNidXNpbGV6YXNAZ21haWwuY29tMRIw
EAYDVQQHEwlXdWVyemJ1cmcxDzANBgNVBAgTBkJheWVybjELMAkGA1UEBhMCREUw
HhcNMTYwMzEzMDE1MjEzWhcNNDEwMzA3MDE1MjEzWjCBlTERMA8GA1UEChMIRnJl
ZSBUU0ExEDAOBgNVBAsTB1Jvb3QgQ0ExGDAWBgNVBAMTD3d3dy5mcmVldHNhLm9y
ZzEiMCAGCSqGSIb3DQEJARYTYnVzaWxlemFzQGdtYWlsLmNvbTESMBAGA1UEBxMJ
V3VlcnpidXJnMQ8wDQYDVQQIEwZCYXllcm4xCzAJBgNVBAYTAkRFMIICIjANBgkq
hkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAtgKODjAy8REQ2WTNqUudAnjhlCrpE6ql
mQfNppeTmVvZrH4zutn+NwTaHAGpjSGv4/WRpZ1wZ3BRZ5mPUBZyLgq0YrIfQ5Fx
0s/MRZPzc1r3lKWrMR9sAQx4mN4z11xFEO529L0dFJjPF9MD8Gpd2feWzGyptlel
b+PqT+++fOa2oY0+NaMM7l/xcNHPOaMz0/2olk0i22hbKeVhvokPCqhFhzsuhKsm
q4Of/o+t6dI7sx5h0nPMm4gGSRhfq+z6BTRgCrqQG2FOLoVFgt6iIm/BnNffUr7V
DYd3zZmIwFOj/H3DKHoGik/xK3E82YA2ZulVOFRW/zj4ApjPa5OFbpIkd0pmzxzd
EcL479hSA9dFiyVmSxPtY5ze1P+BE9bMU1PScpRzw8MHFXxyKqW13Qv7LWw4sbk3
SciB7GACbQiVGzgkvXG6y85HOuvWNvC5GLSiyP9GlPB0V68tbxz4JVTRdw/Xn/XT
FNzRBM3cq8lBOAVt/PAX5+uFcv1S9wFE8YjaBfWCP1jdBil+c4e+0tdywT2oJmYB
BF/kEt1wmGwMmHunNEuQNzh1FtJY54hbUfiWi38mASE7xMtMhfj/C4SvapiDN837
gYaPfs8x3KZxbX7C3YAsFnJinlwAUss1fdKar8Q/YVs7H/nU4c4Ixxxz4f67fcVq
M2ITKentbCMCAwEAAaOCAk4wggJKMAwGA1UdEwQFMAMBAf8wDgYDVR0PAQH/BAQD
AgHGMB0GA1UdDgQWBBT6VQ2MNGZRQ0z357OnbJWveuaklzCBygYDVR0jBIHCMIG/
gBT6VQ2MNGZRQ0z357OnbJWveuakl6GBm6SBmDCBlTERMA8GA1UEChMIRnJlZSBU
U0ExEDAOBgNVBAsTB1Jvb3QgQ0ExGDAWBgNVBAMTD3d3dy5mcmVldHNhLm9yZzEi
MCAGCSqGSIb3DQEJARYTYnVzaWxlemFzQGdtYWlsLmNvbTESMBAGA1UEBxMJV3Vl
cnpidXJnMQ8wDQYDVQQIEwZCYXllcm4xCzAJBgNVBAYTAkRFggkAwemGFg2o6YAw
MwYDVR0fBCwwKjAooCagJIYiaHR0cDovL3d3dy5mcmVldHNhLm9yZy9yb290X2Nh
LmNybDCBzwYDVR0gBIHHMIHEMIHBBgorBgEEAYHyJAEBMIGyMDMGCCsGAQUFBwIB
FidodHRwOi8vd3d3LmZyZWV0c2Eub3JnL2ZyZWV0c2FfY3BzLmh0bWwwMgYIKwYB
BQUHAgEWJmh0dHA6Ly93d3cuZnJlZXRzYS5vcmcvZnJlZXRzYV9jcHMucGRmMEcG
CCsGAQUFBwICMDsaOUZyZWVUU0EgdHJ1c3RlZCB0aW1lc3RhbXBpbmcgU29mdHdh
cmUgYXMgYSBTZXJ2aWNlIChTYWFTKTA3BggrBgEFBQcBAQQrMCkwJwYIKwYBBQUH
MAGGG2h0dHA6Ly93d3cuZnJlZXRzYS5vcmc6MjU2MDANBgkqhkiG9w0BAQ0FAAOC
AgEAaK9+v5OFYu9M6ztYC+L69sw1omdyli89lZAfpWMMh9CRmJhM6KBqM/ipwoLt
nxyxGsbCPhcQjuTvzm+ylN6VwTMmIlVyVSLKYZcdSjt/eCUN+41K7sD7GVmxZBAF
ILnBDmTGJmLkrU0KuuIpj8lI/E6Z6NnmuP2+RAQSHsfBQi6sssnXMo4HOW5gtPO7
gDrUpVXID++1P4XndkoKn7Svw5n0zS9fv1hxBcYIHPPQUze2u30bAQt0n0iIyRLz
aWuhtpAtd7ffwEbASgzB7E+NGF4tpV37e8KiA2xiGSRqT5ndu28fgpOY87gD3ArZ
DctZvvTCfHdAS5kEO3gnGGeZEVLDmfEsv8TGJa3AljVa5E40IQDsUXpQLi8G+UC4
1DWZu8EVT4rnYaCw1VX7ShOR1PNCCvjb8S8tfdudd9zhU3gEB0rxdeTy1tVbNLXW
99y90xcwr1ZIDUwM/xQ/noO8FRhm0LoPC73Ef+J4ZBdrvWwauF3zJe33d4ibxEcb
8/pz5WzFkeixYM2nsHhqHsBKw7JPouKNXRnl5IAE1eFmqDyC7G/VT7OF669xM6hb
Ut5G21JE4cNK6NNucS+fzg1JPX0+3VhsYZjj7D5uljRvQXrJ8iHgr/M6j2oLHvTA
I2MLdq2qjZFDOCXsxBxJpbmLGBx9ow6ZerlUxzws2AWv2pk=
-----END CERTIFICATE-----
`
	opts := images.Options{
		ImageRef: "ghcr.io/kyverno/test-verify-image:tsa",
		Key:      key,
	}

	rc, err := registryclient.New()
	assert.NilError(t, err)
	opts.Client = rc

	verifier := &cosignVerifier{}
	_, err = verifier.VerifySignature(context.TODO(), opts)
	assert.ErrorContains(t, err, "unable to verify RFC3161 timestamp bundle: no TSA root certificate(s) provided to verify timestamp")

	opts.TSACertChain = tsaCertChain
	_, err = verifier.VerifySignature(context.TODO(), opts)
	assert.NilError(t, err)
}

func TestCosignOCI11Experimental(t *testing.T) {
	opts := images.Options{
		ImageRef: "ghcr.io/kyverno/test-verify-image:cosign-oci11",
		Key: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEoKYkkX32oSx61B4iwKXa6llAF2dB
IoL3R/9n1SJ7s00Nfkk3z4/Ar6q8el/guUmXi8akEJMxvHnvphorVUz8vQ==
-----END PUBLIC KEY-----
`,
	}

	rc, err := registryclient.New()
	assert.NilError(t, err)
	opts.Client = rc

	verifier := &cosignVerifier{}
	_, err = verifier.VerifySignature(context.TODO(), opts)
	assert.ErrorContains(t, err, "no signatures found")

	opts.CosignOCI11 = true
	_, err = verifier.VerifySignature(context.TODO(), opts)
	assert.NilError(t, err)
}

type testSignature struct {
	cert *x509.Certificate
}

func (ts testSignature) Digest() (v1.Hash, error) {
	return v1.Hash{}, fmt.Errorf("not implemented")
}

func (ts testSignature) DiffID() (v1.Hash, error) {
	return v1.Hash{}, fmt.Errorf("not implemented")
}

func (ts testSignature) Compressed() (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (ts testSignature) Uncompressed() (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (ts testSignature) Size() (int64, error) {
	return 0, fmt.Errorf("not implemented")
}

func (ts testSignature) MediaType() (types.MediaType, error) {
	return "", fmt.Errorf("not implemented")
}

func (ts testSignature) Annotations() (map[string]string, error) {
	return nil, fmt.Errorf("not implemented")
}

func (ts testSignature) Payload() ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (ts testSignature) Signature() ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (ts testSignature) Base64Signature() (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (ts testSignature) Cert() (*x509.Certificate, error) {
	return ts.cert, nil
}

func (ts testSignature) Chain() ([]*x509.Certificate, error) {
	return nil, fmt.Errorf("not implemented")
}

func (ts testSignature) Bundle() (*bundle.RekorBundle, error) {
	return nil, fmt.Errorf("not implemented")
}

func (ts testSignature) RFC3161Timestamp() (*bundle.RFC3161Timestamp, error) {
	return nil, nil
}

func TestCosignMatchSignatures(t *testing.T) {
	pem1 := "-----BEGIN CERTIFICATE-----\nMIIDtzCCAzygAwIBAgIUX9MdOHZMlRONmc0Iu3DtiLXLVLYwCgYIKoZIzj0EAwMw\nNzEVMBMGA1UEChMMc2lnc3RvcmUuZGV2MR4wHAYDVQQDExVzaWdzdG9yZS1pbnRl\ncm1lZGlhdGUwHhcNMjIxMDA3MTkyNDI0WhcNMjIxMDA3MTkzNDI0WjAAMFkwEwYH\nKoZIzj0CAQYIKoZIzj0DAQcDQgAE0+a5/FhwY4fREWP++3V4rciGiqWGRgHaiP1z\nSlWihKkU71sBVeTzjdrcN8wXzBAefqh5URBfCeE8pJRfQsVKxKOCAlswggJXMA4G\nA1UdDwEB/wQEAwIHgDATBgNVHSUEDDAKBggrBgEFBQcDAzAdBgNVHQ4EFgQUJy79\nhpkwHtXtLWOvFu/icY56bwgwHwYDVR0jBBgwFoAU39Ppz1YkEZb5qNjpKFWixi4Y\nZD8wbgYDVR0RAQH/BGQwYoZgaHR0cHM6Ly9naXRodWIuY29tL0ppbUJ1Z3dhZGlh\nL2RlbW8tamF2YS10b21jYXQvLmdpdGh1Yi93b3JrZmxvd3MvcHVibGlzaC55YW1s\nQHJlZnMvdGFncy92MC4wLjIyMDkGCisGAQQBg78wAQEEK2h0dHBzOi8vdG9rZW4u\nYWN0aW9ucy5naXRodWJ1c2VyY29udGVudC5jb20wEgYKKwYBBAGDvzABAgQEcHVz\naDA2BgorBgEEAYO/MAEDBChjNzY0NTI4NGZhN2FlYmU1NTQ2MThlZWU4NzliNGQ2\nOTQ3Zjg1NjRlMB8GCisGAQQBg78wAQQEEWJ1aWxkLXNpZ24tYXR0ZXN0MCoGCisG\nAQQBg78wAQUEHEppbUJ1Z3dhZGlhL2RlbW8tamF2YS10b21jYXQwHwYKKwYBBAGD\nvzABBgQRcmVmcy90YWdzL3YwLjAuMjIwgYoGCisGAQQB1nkCBAIEfAR6AHgAdgAI\nYJLwKFL/aEXR0WsnhJxFZxisFj3DONJt5rwiBjZvcgAAAYOz5+pbAAAEAwBHMEUC\nIBb8fwsLBOu+qJkL6UhT4pwGvRVAN2n74BF1BL703rqPAiEAznbfgYJbqA+JIUiQ\nwwLiFOD8pqidSl+HhW8Lhdg3o+wwCgYIKoZIzj0EAwMDaQAwZgIxAJIBIkZBhM+K\nkBIFNeuWBsyVaAcFRallz3C8jvPQCPbec0ZpIsw624dUs8zD3c96AQIxALf875rt\n+oZgwE6hsDazJzoTcBZ1mYVF6bAlwVdtMiC98aApG6T+qaBirxSgu7IGQw==\n-----END CERTIFICATE-----\n"
	cert1, err := loadCert([]byte(pem1))
	assert.NilError(t, err)

	pem2 := "-----BEGIN CERTIFICATE-----\nMIICnjCCAiSgAwIBAgIUfHC63TD7cn1QEYwI6sJ50PclbMMwCgYIKoZIzj0EAwMw\nNzEVMBMGA1UEChMMc2lnc3RvcmUuZGV2MR4wHAYDVQQDExVzaWdzdG9yZS1pbnRl\ncm1lZGlhdGUwHhcNMjIxMTIxMTczNDIwWhcNMjIxMTIxMTc0NDIwWjAAMFkwEwYH\nKoZIzj0CAQYIKoZIzj0DAQcDQgAEpJeio6iqU9TbHm+WV5KmeinSPWFrMFzoduFN\ntvrjMRAJV6qDX7aHvZRQPtSUxt3PvWwPwZz6Id8XwfHgJwtpp6OCAUMwggE/MA4G\nA1UdDwEB/wQEAwIHgDATBgNVHSUEDDAKBggrBgEFBQcDAzAdBgNVHQ4EFgQU+sL4\nr0BAYZhtjWXdqZd6ktDbl60wHwYDVR0jBBgwFoAU39Ppz1YkEZb5qNjpKFWixi4Y\nZD8wHQYDVR0RAQH/BBMwEYEPamltQG5pcm1hdGEuY29tMCwGCisGAQQBg78wAQEE\nHmh0dHBzOi8vZ2l0aHViLmNvbS9sb2dpbi9vYXV0aDCBigYKKwYBBAHWeQIEAgR8\nBHoAeAB2AN09MGrGxxEyYxkeHJlnNwKiSl643jyt/4eKcoAvKe6OAAABhJtBVS4A\nAAQDAEcwRQIgbWUReMySzQUjZBII8Mdfrw7+MtmcPObrU7lDGNzvc40CIQCSa0xj\nafVdGMlgOPxDvc9gkI2ht6eQN2kmZXkNHe95PTAKBggqhkjOPQQDAwNoADBlAjEA\npQJPNKjRHqsfjhTcrvS1tKodYbz/NKWRJQbacmQaEX3aGZEa/Jczp2IFkcU6eEH/\nAjADp3TpZ56DdgAGCFXDRk3xOcgeDtPeIG6i+fq8Xfik+pIFs+thR7n1ya6LmaXv\nkhw=\n-----END CERTIFICATE-----\n"
	cert2, err := loadCert([]byte(pem2))
	assert.NilError(t, err)

	sigs := []oci.Signature{
		testSignature{cert: cert1},
		testSignature{cert: cert2},
	}

	subject1 := "https://github.com/JimBugwadia/demo-java-tomcat/.github/workflows/publish.yaml@refs/tags/*"
	issuer1 := "https://token.actions.githubusercontent.com"
	extensions := map[string]string{
		"githubWorkflowTrigger":    "push",
		"githubWorkflowSha":        "c7645284fa7aebe554618eee879b4d6947f8564e",
		"githubWorkflowName":       "build-sign-attest",
		"githubWorkflowRepository": "JimBugwadia/demo-java-tomcat",
	}

	subject2 := "*@nirmata.com"
	subject2RegExp := `.+@nirmata\.com`
	issuer2 := "https://github.com/login/oauth"
	issuer2RegExp := `https://github\.com/login/.+`

	matchErr := matchSignatures(sigs, subject1, "", issuer1, "", extensions)
	assert.NilError(t, matchErr)

	matchErr = matchSignatures(sigs, subject2, "", issuer2, "", nil)
	assert.NilError(t, matchErr)

	matchErr = matchSignatures(sigs, "", subject2RegExp, issuer2, "", nil)
	assert.NilError(t, matchErr)

	matchErr = matchSignatures(sigs, "", "", "", issuer2RegExp, nil)
	assert.NilError(t, matchErr)

	matchErr = matchSignatures(sigs, subject2, "", issuer1, "", nil)
	assert.Error(t, matchErr, "subject mismatch: expected *@nirmata.com, received https://github.com/JimBugwadia/demo-java-tomcat/.github/workflows/publish.yaml@refs/tags/v0.0.22; issuer mismatch: expected https://token.actions.githubusercontent.com, received https://github.com/login/oauth")

	matchErr = matchSignatures(sigs, "", subject2RegExp, issuer1, "", nil)
	assert.Error(t, matchErr, `subject mismatch: expected .+@nirmata\.com, received https://github.com/JimBugwadia/demo-java-tomcat/.github/workflows/publish.yaml@refs/tags/v0.0.22; issuer mismatch: expected https://token.actions.githubusercontent.com, received https://github.com/login/oauth`)

	matchErr = matchSignatures(sigs, subject2, "", issuer2, "", extensions)
	assert.ErrorContains(t, matchErr, "extension mismatch")
}
