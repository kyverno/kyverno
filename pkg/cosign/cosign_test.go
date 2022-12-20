package cosign

import (
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/sigstore/cosign/pkg/cosign/bundle"
	"github.com/sigstore/cosign/pkg/oci"
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

const tektonPayload = `{
  "Critical": {
    "Identity": {
      "docker-reference": "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/nop"
    },
    "Image": {
      "Docker-manifest-digest": "sha256:6a037d5ba27d9c6be32a9038bfe676fb67d2e4145b4f53e9c61fb3e69f06e816"
    },
    "Type": "Tekton container signature"
  },
  "Optional": {
	  "Issuer": "https://github.com/login/oauth",
	  "Subject": "https://github.com/mycompany/demo/.github/workflows/ci.yml@refs/heads/main"
  }
}`

func TestCosignPayload(t *testing.T) {
	image := "registry-v2.nirmata.io/pause"
	signedPayloads := cosign.SignedPayload{Payload: []byte(cosignPayload)}
	p, err := extractPayload([]oci.Signature{&sig{cosignPayload: signedPayloads}})
	assert.NilError(t, err)
	a := map[string]string{"foo": "bar"}
	err = checkAnnotations(p, a)
	assert.NilError(t, err)
	d, err := extractDigest(image, p)
	assert.NilError(t, err)
	assert.Equal(t, d, "sha256:4a1c4b21597c1b4415bdbecb28a3296c6b5e23ca4f9feeb599860a1dac6a0108")

	image2 := "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/nop"
	signedPayloads2 := cosign.SignedPayload{Payload: []byte(tektonPayload)}
	signatures2 := []oci.Signature{&sig{cosignPayload: signedPayloads2}}
	p2, err := extractPayload(signatures2)
	assert.NilError(t, err)

	d2, err := extractDigest(image2, p2)
	assert.NilError(t, err)
	assert.Equal(t, d2, "sha256:6a037d5ba27d9c6be32a9038bfe676fb67d2e4145b4f53e9c61fb3e69f06e816")
}

func TestCosignKeyless(t *testing.T) {
	opts := Options{
		ImageRef: "ghcr.io/jimbugwadia/pause2",
		Issuer:   "https://github.com/",
		Subject:  "jim",
	}

	client, err := registryclient.New()
	assert.NilError(t, err)

	_, err = VerifySignature(context.TODO(), client, opts)
	assert.ErrorContains(t, err, "subject mismatch: expected jim, received jim@nirmata.com")

	opts.Subject = "jim@nirmata.com"
	_, err = VerifySignature(context.TODO(), client, opts)
	assert.ErrorContains(t, err, "issuer mismatch: expected https://github.com/, received https://github.com/login/oauth")

	opts.Issuer = "https://github.com/login/oauth"
	_, err = VerifySignature(context.TODO(), client, opts)
	assert.NilError(t, err)
}

func TestCosignMatchCertificateData(t *testing.T) {
	pem1 := "-----BEGIN CERTIFICATE-----\nMIIDtzCCAzygAwIBAgIUX9MdOHZMlRONmc0Iu3DtiLXLVLYwCgYIKoZIzj0EAwMw\nNzEVMBMGA1UEChMMc2lnc3RvcmUuZGV2MR4wHAYDVQQDExVzaWdzdG9yZS1pbnRl\ncm1lZGlhdGUwHhcNMjIxMDA3MTkyNDI0WhcNMjIxMDA3MTkzNDI0WjAAMFkwEwYH\nKoZIzj0CAQYIKoZIzj0DAQcDQgAE0+a5/FhwY4fREWP++3V4rciGiqWGRgHaiP1z\nSlWihKkU71sBVeTzjdrcN8wXzBAefqh5URBfCeE8pJRfQsVKxKOCAlswggJXMA4G\nA1UdDwEB/wQEAwIHgDATBgNVHSUEDDAKBggrBgEFBQcDAzAdBgNVHQ4EFgQUJy79\nhpkwHtXtLWOvFu/icY56bwgwHwYDVR0jBBgwFoAU39Ppz1YkEZb5qNjpKFWixi4Y\nZD8wbgYDVR0RAQH/BGQwYoZgaHR0cHM6Ly9naXRodWIuY29tL0ppbUJ1Z3dhZGlh\nL2RlbW8tamF2YS10b21jYXQvLmdpdGh1Yi93b3JrZmxvd3MvcHVibGlzaC55YW1s\nQHJlZnMvdGFncy92MC4wLjIyMDkGCisGAQQBg78wAQEEK2h0dHBzOi8vdG9rZW4u\nYWN0aW9ucy5naXRodWJ1c2VyY29udGVudC5jb20wEgYKKwYBBAGDvzABAgQEcHVz\naDA2BgorBgEEAYO/MAEDBChjNzY0NTI4NGZhN2FlYmU1NTQ2MThlZWU4NzliNGQ2\nOTQ3Zjg1NjRlMB8GCisGAQQBg78wAQQEEWJ1aWxkLXNpZ24tYXR0ZXN0MCoGCisG\nAQQBg78wAQUEHEppbUJ1Z3dhZGlhL2RlbW8tamF2YS10b21jYXQwHwYKKwYBBAGD\nvzABBgQRcmVmcy90YWdzL3YwLjAuMjIwgYoGCisGAQQB1nkCBAIEfAR6AHgAdgAI\nYJLwKFL/aEXR0WsnhJxFZxisFj3DONJt5rwiBjZvcgAAAYOz5+pbAAAEAwBHMEUC\nIBb8fwsLBOu+qJkL6UhT4pwGvRVAN2n74BF1BL703rqPAiEAznbfgYJbqA+JIUiQ\nwwLiFOD8pqidSl+HhW8Lhdg3o+wwCgYIKoZIzj0EAwMDaQAwZgIxAJIBIkZBhM+K\nkBIFNeuWBsyVaAcFRallz3C8jvPQCPbec0ZpIsw624dUs8zD3c96AQIxALf875rt\n+oZgwE6hsDazJzoTcBZ1mYVF6bAlwVdtMiC98aApG6T+qaBirxSgu7IGQw==\n-----END CERTIFICATE-----\n"
	cert1, err := loadCert([]byte(pem1))
	assert.NilError(t, err)

	subject1 := "https://github.com/JimBugwadia/demo-java-tomcat/.github/workflows/publish.yaml@refs/tags/*"
	issuer1 := "https://token.actions.githubusercontent.com"
	extensions := map[string]string{
		"githubWorkflowTrigger":    "push",
		"githubWorkflowSha":        "c7645284fa7aebe554618eee879b4d6947f8564e",
		"githubWorkflowName":       "build-sign-attest",
		"githubWorkflowRepository": "JimBugwadia/demo-java-tomcat",
	}

	matchErr := matchCertificateData(cert1, subject1, issuer1, extensions)
	assert.NilError(t, matchErr)

	matchErr = matchCertificateData(cert1, "", issuer1, extensions)
	assert.NilError(t, matchErr)

	matchErr = matchCertificateData(cert1, subject1, issuer1, nil)
	assert.NilError(t, matchErr)

	matchErr = matchCertificateData(cert1, "wrong-subject", issuer1, extensions)
	assert.Error(t, matchErr, "subject mismatch: expected wrong-subject, received https://github.com/JimBugwadia/demo-java-tomcat/.github/workflows/publish.yaml@refs/tags/v0.0.22")

	extensions["githubWorkflowTrigger"] = "pull"
	matchErr = matchCertificateData(cert1, subject1, issuer1, extensions)
	assert.Error(t, matchErr, "extension mismatch: expected pull for key githubWorkflowTrigger, received push")
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
	issuer2 := "https://github.com/login/oauth"

	matchErr := matchSignatures(sigs, subject1, issuer1, extensions)
	assert.NilError(t, matchErr)

	matchErr = matchSignatures(sigs, subject2, issuer2, nil)
	assert.NilError(t, matchErr)

	matchErr = matchSignatures(sigs, subject2, issuer1, nil)
	assert.Error(t, matchErr, "subject mismatch: expected *@nirmata.com, received https://github.com/JimBugwadia/demo-java-tomcat/.github/workflows/publish.yaml@refs/tags/v0.0.22; issuer mismatch: expected https://token.actions.githubusercontent.com, received https://github.com/login/oauth")

	matchErr = matchSignatures(sigs, subject2, issuer2, extensions)
	assert.ErrorContains(t, matchErr, "extension mismatch")
}
