package notary

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/images"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"gotest.tools/assert"
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
)

func TestExtractStatements(t *testing.T) {
	imageRef := "jimnotarytest.azurecr.io/jim/net-monitor:v1"
	ref, err := name.ParseReference(imageRef)
	assert.NilError(t, err)
	repoDesc, err := remote.Head(ref)
	assert.NilError(t, err)
	referrers, err := remote.Referrers(ref.Context().Digest(repoDesc.Digest.String()))
	assert.NilError(t, err)
	referrersDescs, err := referrers.IndexManifest()
	assert.NilError(t, err)

	for _, referrer := range referrersDescs.Manifests {
		if referrer.ArtifactType == "application/vnd.cncf.notary.signature" {
			statements, err := extractStatements(context.Background(), ref, referrer, nil)
			assert.NilError(t, err)
			assert.Assert(t, len(statements) == 1)
			assert.Assert(t, statements[0]["type"] == referrer.ArtifactType)
			assert.Assert(t, statements[0]["mediaType"] == string(referrer.MediaType))
		}
	}
}

func TestNotaryImageVerification(t *testing.T) {
	opts := images.Options{
		ImageRef: "ghcr.io/kyverno/test-verify-image:signed",
		Cert:     cert,
	}

	rc, err := registryclient.New()
	assert.NilError(t, err)
	opts.Client = rc

	verifier := &notaryVerifier{}
	_, err = verifier.VerifySignature(context.TODO(), opts)
	assert.NilError(t, err)
}
