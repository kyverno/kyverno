package internal

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/cosign"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	imageutils "github.com/kyverno/kyverno/pkg/utils/image"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_verifyAttestations_Coverage(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)

	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test",
			},
		},
	}

	pc, err := policycontext.NewPolicyContext(jp, resource, kyvernov1.Create, nil, cfg)
	assert.NoError(t, err)

	rule := kyvernov1.Rule{
		Name: "test-rule",
	}

	ivm := engineapi.ImageVerificationMetadata{}
	rclient := adapters.RegistryClient(registryclient.NewOrDie())
	iv := NewImageVerifier(logr.Discard(), rclient, imageverifycache.DisabledImageVerifyCache(), pc, rule, &ivm)

	iv.policyContext.JSONContext().Checkpoint()

	imageVerify := kyvernov1.ImageVerification{
		Attestations: []kyvernov1.Attestation{
			{
				Name: "myAttestation",
				Type: "https://example.com/CodeReview/v1",
				Attestors: []kyvernov1.AttestorSet{
					{
						Entries: []kyvernov1.Attestor{
							{
								Keys: &kyvernov1.StaticKeyAttestor{
									PublicKeys: "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM\n5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==\n-----END PUBLIC KEY-----",
								},
							},
						},
					},
				},
			},
		},
	}

	imageInfo := apiutils.ImageInfo{
		ImageInfo: imageutils.ImageInfo{
			Registry: "ghcr.io",
			Name:     "test-verify-image",
			Path:     "kyverno/test-verify-image",
			Tag:      "signed",
		},
	}

	payloads := [][]byte{
		[]byte(`{"payloadType":"https://example.com/CodeReview/v1","payload":"eyJfdHlwZSI6Imh0dHBzOi8vaW4tdG90by5pby9TdGF0ZW1lbnQvdjAuMSIsInByZWRpY2F0ZVR5cGUiOiJodHRwczovL2V4YW1wbGUuY29tL0NvZGVSZXZpZXcvdjEiLCJzdWJqZWN0IjpbeyJuYW1lIjoiZ2hjci5pby9qaW1idWd3YWRpYS9wYXVzZTIiLCJkaWdlc3QiOnsic2hhMjU2IjoiYjMxYmZiNGQwMjEzZjI1NGQzNjFlMDA3OWRlYWFlYmVmYTRmODJiYTdhYTc2ZWY4MmU5MGI0OTM1YWQ1YjEwNSJ9fV0sInByZWRpY2F0ZSI6eyJhdXRob3IiOiJtYWlsdG86YWxpY2VAZXhhbXBsZS5jb20iLCJyZXBvIjp7ImJyYW5jaCI6Im1haW4iLCJ0eXBlIjoiZ2l0IiwidXJpIjoiaHR0cHM6Ly9naXRodWIuY29tL2V4YW1wbGUvbXktcHJvamVjdCJ9LCJyZXZpZXdlcnMiOlsibWFpbHRvOmJvYkBleGFtcGxlLmNvbSJdfX0=","signatures":[{"keyid":"","sig":"MEYCIQCrEr+vgPDmNCrqGDE/4z9iMLmCXMXcDlGKtSoiuMTSFgIhAN2riBaGk4accWzVl7ypi1XTRxyrPYHst8DesugPXgOf"}]}`),
	}

	err = cosign.SetMock(imageInfo.String(), payloads)
	assert.NoError(t, err)
	defer cosign.ClearMock()

	resp, _ := iv.verifyAttestations(context.TODO(), imageVerify, imageInfo)
	if resp != nil && resp.Status() != engineapi.RuleStatusPass {
		t.Errorf("Rule failed: %s", resp.Message())
	}
	assert.NotNil(t, resp)
	assert.Equal(t, engineapi.RuleStatusPass, resp.Status())

	// Verify that "myAttestation" was persisted in the context
	val, err := iv.policyContext.JSONContext().Query("myAttestation")
	assert.NoError(t, err)
	assert.NotNil(t, val)
}
