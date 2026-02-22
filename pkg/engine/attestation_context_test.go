package engine

import (
	"context"
	"encoding/json"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/cosign"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"gotest.tools/assert"
)

func Test_AttestationPersistence(t *testing.T) {
	policyRaw := `{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "attestation-persistence"
  },
  "spec": {
    "rules": [
      {
        "name": "verify-image",
        "match": {
          "resources": {
            "kinds": ["Pod"]
          }
        },
        "verifyImages": [
          {
            "imageReferences": ["ghcr.io/kyverno/test-verify-image:*"],
            "attestations": [
              {
                "name": "myAttestation",
                "type": "https://example.com/CodeReview/v1",
                "attestors": [
                  {
                    "entries": [
                      {
                        "keys": {
                          "publicKeys": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM\n5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==\n-----END PUBLIC KEY-----",
                          "rekor": { "url": "https://rekor.sigstore.dev", "ignoreTlog": true },
                          "ctlog": { "ignoreSCT": true }
                        }
                      }
                    ]
                  }
                ]
              }
            ]
          }
        ]
      },
      {
        "name": "validate-attestation",
        "match": {
          "resources": {
            "kinds": ["Pod"]
          }
        },
        "validate": {
          "message": "Attestation data FOUND!",
          "deny": {
            "conditions": [
              {
                "key": "{{ myAttestation.author }}",
                "operator": "Equals",
                "value": "mailto:alice@example.com"
              }
            ]
          }
        }
      }
    ]
  }
}`

	resourceRaw := `{
  "apiVersion": "v1",
  "kind": "Pod",
  "metadata": {"name": "test"},
  "spec": {
    "containers": [
      {
        "name": "test",
        "image": "ghcr.io/kyverno/test-verify-image:signed"
      }
    ]
  }
}`

	attestationPayloadsRepro := [][]byte{
		[]byte(`{"payloadType":"https://example.com/CodeReview/v1","payload":"eyJfdHlwZSI6Imh0dHBzOi8vaW4tdG90by5pby9TdGF0ZW1lbnQvdjAuMSIsInByZWRpY2F0ZVR5cGUiOiJodHRwczovL2V4YW1wbGUuY29tL0NvZGVSZXZpZXcvdjEiLCJzdWJqZWN0IjpbeyJuYW1lIjoiZ2hjci5pby9qaW1idWd3YWRpYS9wYXVzZTIiLCJkaWdlc3QiOnsic2hhMjU2IjoiYjMxYmZiNGQwMjEzZjI1NGQzNjFlMDA3OWRlYWFlYmVmYTRmODJiYTdhYTc2ZWY4MmU5MGI0OTM1YWQ1YjEwNSJ9fV0sInByZWRpY2F0ZSI6eyJhdXRob3IiOiJtYWlsdG86YWxpY2VAZXhhbXBsZS5jb20iLCJyZXBvIjp7ImJyYW5jaCI6Im1haW4iLCJ0eXBlIjoiZ2l0IiwidXJpIjoiaHR0cHM6Ly9naXRodWIuY29tL2V4YW1wbGUvbXktcHJvamVjdCJ9LCJyZXZpZXdlcnMiOlsibWFpbHRvOmJvYkBleGFtcGxlLmNvbSJdfX0=","signatures":[{"keyid":"","sig":"MEYCIQCrEr+vgPDmNCrqGDE/4z9iMLmCXMXcDlGKtSoiuMTSFgIhAN2riBaGk4accWzVl7ypi1XTRxyrPYHst8DesugPXgOf"}]}`),
	}

	err := cosign.SetMock("ghcr.io/kyverno/test-verify-image:signed", attestationPayloadsRepro)
	assert.NilError(t, err)
	defer cosign.ClearMock()

	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)

	var cpol kyvernov1.ClusterPolicy
	err = json.Unmarshal([]byte(policyRaw), &cpol)
	assert.NilError(t, err)

	resourceUnstructured, err := kubeutils.BytesToUnstructured([]byte(resourceRaw))
	assert.NilError(t, err)

	eng := NewEngine(
		cfg,
		jp,
		nil,
		factories.DefaultRegistryClientFactory(adapters.RegistryClient(registryclient.NewOrDie()), nil),
		imageverifycache.DisabledImageVerifyCache(),
		factories.DefaultContextLoaderFactory(nil),
		nil,
		nil,
	)

	policyContext, err := policycontext.NewPolicyContext(
		jp,
		*resourceUnstructured,
		kyvernov1.Create,
		nil,
		cfg,
	)
	assert.NilError(t, err)
	policyContext = policyContext.WithPolicy(&cpol).WithNewResource(*resourceUnstructured)

	// 1. Verify images
	er, _ := eng.VerifyAndPatchImages(context.TODO(), policyContext)
	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusPass)

	val, err := policyContext.JSONContext().Query("myAttestation")
	assert.NilError(t, err)
	assert.Assert(t, val != nil, "myAttestation should be in context")
	t.Logf("myAttestation in context: %v", val)

	// 2. Validate attestation data is available in rule 2
	er2 := eng.Validate(context.TODO(), policyContext)

	foundRule2 := false
	for _, rule := range er2.PolicyResponse.Rules {
		if rule.Name() == "validate-attestation" {
			foundRule2 = true
			if rule.Status() != engineapi.RuleStatusFail {
				t.Errorf("Expected Rule 2 to FAIL because data was found, but got status %v", rule.Status())
			}
		}
	}
	assert.Assert(t, foundRule2, "Rule 2 was not executed")
}
