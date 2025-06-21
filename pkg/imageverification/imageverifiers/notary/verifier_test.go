package notary

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
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

	v := Verifier{log: logr.Discard()}
	err = v.VerifyImageSignature(ctx, img, cert, "")
	assert.NoError(t, err)
}

func Test_ImageSignatureVerificationUnsigned(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(ctx, unsignedImage)
	assert.NoError(t, err)

	v := Verifier{log: logr.Discard()}
	err = v.VerifyImageSignature(ctx, img, cert, "")
	assert.ErrorContains(t, err, "make sure the artifact was signed successfully")
}

func Test_ImageAttestationVerificationStandard(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(ctx, image)
	assert.NoError(t, err)

	v := Verifier{log: logr.Discard()}
	err = v.VerifyAttestationSignature(ctx, img, "sbom/cyclone-dx", cert, "")
	assert.NoError(t, err)
}

func Test_ImageAttestationVerificationFailNotFound(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(ctx, image)
	assert.NoError(t, err)

	v := Verifier{log: logr.Discard()}
	err = v.VerifyAttestationSignature(ctx, img, "invalid", cert, "")
	assert.ErrorContains(t, err, "attestation verification failed, no attestations found for type: invalid")
}

func Test_ImageAttestationVerificationFailUntrusted(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(ctx, image)
	assert.NoError(t, err)

	v := Verifier{log: logr.Discard()}
	err = v.VerifyAttestationSignature(ctx, img, "trivy/vulnerability-fail-test", cert, "")
	assert.ErrorContains(t, err, "failed to verify signature with digest sha256:5e52184f10b19c69105e5dd5d3c875753cfd824d3d2f86cd2122e4107bd13d16, the signature's certificate chain does not contain any trusted certificate")
}

func Test_ImageAttestationVerificationFailUnsigned(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(ctx, image)
	assert.NoError(t, err)

	v := Verifier{log: logr.Discard()}
	err = v.VerifyAttestationSignature(ctx, img, "application/vnd.cncf.notary.signature", cert, "")
	assert.ErrorContains(t, err, "make sure the artifact was signed successfully")
}
