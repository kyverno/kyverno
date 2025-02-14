package notary

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/imagedataloader"
	"github.com/stretchr/testify/assert"
)

var (
	cert = `-----BEGIN CERTIFICATE-----
MIIDTTCCAjWgAwIBAgIJAPI+zAzn4s0xMA0GCSqGSIb3DQEBCwUAMEwxCzAJBgNV
BAYTAlVTMQswCQYDVQQIDAJXQTEQMA4GA1UEBwwHU2VhdHRsZTEPMA0GA1UECgwG
Tm90YXJ5MQ0wCwYDVQQDDAR0ZXN0MB4XDTIzMDUyMjIxMTUxOFoXDTMzMDUxOTIx
MTUxOFowTDELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAldBMRAwDgYDVQQHDAdTZWF0
dGxlMQ8wDQYDVQQKDAZOb3RhcnkxDTALBgNVBAMMBHRlc3QwggEiMA0GCSqGSIb3
DQEBAQUAA4IBDwAwggEKAoIBAQDNhTwv+QMk7jEHufFfIFlBjn2NiJaYPgL4eBS+
b+o37ve5Zn9nzRppV6kGsa161r9s2KkLXmJrojNy6vo9a6g6RtZ3F6xKiWLUmbAL
hVTCfYw/2n7xNlVMjyyUpE+7e193PF8HfQrfDFxe2JnX5LHtGe+X9vdvo2l41R6m
Iia04DvpMdG4+da2tKPzXIuLUz/FDb6IODO3+qsqQLwEKmmUee+KX+3yw8I6G1y0
Vp0mnHfsfutlHeG8gazCDlzEsuD4QJ9BKeRf2Vrb0ywqNLkGCbcCWF2H5Q80Iq/f
ETVO9z88R7WheVdEjUB8UrY7ZMLdADM14IPhY2Y+tLaSzEVZAgMBAAGjMjAwMAkG
A1UdEwQCMAAwDgYDVR0PAQH/BAQDAgeAMBMGA1UdJQQMMAoGCCsGAQUFBwMDMA0G
CSqGSIb3DQEBCwUAA4IBAQBX7x4Ucre8AIUmXZ5PUK/zUBVOrZZzR1YE8w86J4X9
kYeTtlijf9i2LTZMfGuG0dEVFN4ae3CCpBst+ilhIndnoxTyzP+sNy4RCRQ2Y/k8
Zq235KIh7uucq96PL0qsF9s2RpTKXxyOGdtp9+HO0Ty5txJE2txtLDUIVPK5WNDF
ByCEQNhtHgN6V20b8KU2oLBZ9vyB8V010dQz0NRTDLhkcvJig00535/LUylECYAJ
5/jn6XKt6UYCQJbVNzBg/YPGc1RF4xdsGVDBben/JXpeGEmkdmXPILTKd9tZ5TC0
uOKpF5rWAruB5PCIrquamOejpXV9aQA/K2JQDuc0mcKz
-----END CERTIFICATE-----`
	unsignedImage = "ghcr.io/kyverno/test-verify-image:unsigned"
)

func Test_ImageSignatureVerificationStandard(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(ctx, image)
	assert.NoError(t, err)

	attestor := &v1alpha1.Attestor{
		Name: "test",
		Notary: &v1alpha1.Notary{
			Certs: cert,
		},
	}

	v := notaryVerifier{log: logr.Discard()}
	err = v.VerifyImageSignature(ctx, img, attestor)
	assert.NoError(t, err)
}

func Test_ImageSignatureVerificationUnsigned(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(ctx, unsignedImage)
	assert.NoError(t, err)

	attestor := &v1alpha1.Attestor{
		Name: "test",
		Notary: &v1alpha1.Notary{
			Certs: cert,
		},
	}

	v := notaryVerifier{log: logr.Discard()}
	err = v.VerifyImageSignature(ctx, img, attestor)
	assert.ErrorContains(t, err, "make sure the artifact was signed successfully")
}

func Test_ImageAttestationVerificationStandard(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(ctx, image)
	assert.NoError(t, err)

	attestor := &v1alpha1.Attestor{
		Name: "test",
		Notary: &v1alpha1.Notary{
			Certs: cert,
		},
	}

	attestation := &v1alpha1.Attestation{
		Name: "attestation",
		Referrer: &v1alpha1.Referrer{
			Type: "sbom/cyclone-dx",
		},
	}

	v := notaryVerifier{log: logr.Discard()}
	err = v.VerifyAttestationSignature(ctx, img, attestation, attestor)
	assert.NoError(t, err)
}

func Test_ImageAttestationVerificationFailNotFound(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(ctx, image)
	assert.NoError(t, err)

	attestor := &v1alpha1.Attestor{
		Name: "test",
		Notary: &v1alpha1.Notary{
			Certs: cert,
		},
	}

	attestation := &v1alpha1.Attestation{
		Name: "attestation",
		Referrer: &v1alpha1.Referrer{
			Type: "invalid",
		},
	}

	v := notaryVerifier{log: logr.Discard()}
	err = v.VerifyAttestationSignature(ctx, img, attestation, attestor)
	assert.ErrorContains(t, err, "attestation verification failed, no attestations found for type: invalid")
}

func Test_ImageAttestationVerificationFailUntrusted(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(ctx, image)
	assert.NoError(t, err)

	attestor := &v1alpha1.Attestor{
		Name: "test",
		Notary: &v1alpha1.Notary{
			Certs: cert,
		},
	}

	attestation := &v1alpha1.Attestation{
		Name: "attestation",
		Referrer: &v1alpha1.Referrer{
			Type: "trivy/vulnerability-fail-test",
		},
	}

	v := notaryVerifier{log: logr.Discard()}
	err = v.VerifyAttestationSignature(ctx, img, attestation, attestor)
	assert.ErrorContains(t, err, "failed to verify signature with digest sha256:5e52184f10b19c69105e5dd5d3c875753cfd824d3d2f86cd2122e4107bd13d16, signature is not produced by a trusted signer")
}

func Test_ImageAttestationVerificationFailUnsigned(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(ctx, image)
	assert.NoError(t, err)

	attestor := &v1alpha1.Attestor{
		Name: "test",
		Notary: &v1alpha1.Notary{
			Certs: cert,
		},
	}

	attestation := &v1alpha1.Attestation{
		Name: "attestation",
		Referrer: &v1alpha1.Referrer{
			Type: "application/vnd.cncf.notary.signature",
		},
	}
	v := notaryVerifier{log: logr.Discard()}
	err = v.VerifyAttestationSignature(ctx, img, attestation, attestor)
	assert.ErrorContains(t, err, "make sure the artifact was signed successfully")
}
