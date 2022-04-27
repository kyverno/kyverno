package engine

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/cosign"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"gotest.tools/assert"
)

var testPolicyGood = `{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "attest"
  },
  "spec": {
    "rules": [
      {
        "name": "attest",
        "match": {
          "resources": {
            "kinds": [
              "Pod"
            ]
          }
        },
        "verifyImages": [
          {
            "image": "*",
            "key": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEHMmDjK65krAyDaGaeyWNzgvIu155JI50B2vezCw8+3CVeE0lJTL5dbL3OP98Za0oAEBJcOxky8Riy/XcmfKZbw==\n-----END PUBLIC KEY-----",
            "attestations": [
              {
                "predicateType": "https://example.com/CodeReview/v1",
                "conditions": [
                  {
                    "all": [
                      {
                        "key": "{{ repo.uri }}",
                        "operator": "Equals",
                        "value": "https://github.com/example/my-project"
                      }, 
                      {
                        "key": "{{ repo.branch }}",
                        "operator": "Equals",
                        "value": "main"
                      }
                    ]
                  }
                ]
              }
            ]
          }
        ]
      }
    ]
  }
}`

var testPolicyBad = `{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "attest"
  },
  "spec": {
    "rules": [
      {
        "name": "attest",
        "match": {
          "resources": {
            "kinds": [
              "Pod"
            ]
          }
        },
        "verifyImages": [
          {
            "image": "*",
            "key": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEHMmDjK65krAyDaGaeyWNzgvIu155JI50B2vezCw8+3CVeE0lJTL5dbL3OP98Za0oAEBJcOxky8Riy/XcmfKZbw==\n-----END PUBLIC KEY-----",
            "attestations": [
              {
                "predicateType": "https://example.com/CodeReview/v1",
                "conditions": [
                  {
                    "all": [
                      {
                        "key": "{{ repo.uri }}",
                        "operator": "Equals",
                        "value": "https://github.com/example/my-project"
                      }, 
                      {
                        "key": "{{ repo.branch }}",
                        "operator": "Equals",
                        "value": "prod"
                      }
                    ]
                  }
                ]
              }
            ]
          }
        ]
      }
    ]
  }
}`

var testResource = `{
  "apiVersion": "v1",
  "kind": "Pod",
  "metadata": {
     "name": "test",
     "annotations": {}
  },
  "spec": {
    "containers": [
      {
        "name": "pause2",
        "image": "ghcr.io/jimbugwadia/pause2"
      }
    ]
  }
}`

var payloads = [][]byte{
	[]byte(`{"payloadType":"https://example.com/CodeReview/v1","payload":"eyJfdHlwZSI6Imh0dHBzOi8vaW4tdG90by5pby9TdGF0ZW1lbnQvdjAuMSIsInByZWRpY2F0ZVR5cGUiOiJodHRwczovL2V4YW1wbGUuY29tL0NvZGVSZXZpZXcvdjEiLCJzdWJqZWN0IjpbeyJuYW1lIjoiZ2hjci5pby9qaW1idWd3YWRpYS9wYXVzZTIiLCJkaWdlc3QiOnsic2hhMjU2IjoiYjMxYmZiNGQwMjEzZjI1NGQzNjFlMDA3OWRlYWFlYmVmYTRmODJiYTdhYTc2ZWY4MmU5MGI0OTM1YWQ1YjEwNSJ9fV0sInByZWRpY2F0ZSI6eyJhdXRob3IiOiJtYWlsdG86YWxpY2VAZXhhbXBsZS5jb20iLCJyZXBvIjp7ImJyYW5jaCI6Im1haW4iLCJ0eXBlIjoiZ2l0IiwidXJpIjoiaHR0cHM6Ly9naXRodWIuY29tL2V4YW1wbGUvbXktcHJvamVjdCJ9LCJyZXZpZXdlcnMiOlsibWFpbHRvOmJvYkBleGFtcGxlLmNvbSJdfX0=","signatures":[{"keyid":"","sig":"MEYCIQCrEr+vgPDmNCrqGDE/4z9iMLmCXMXcDlGKtSoiuMTSFgIhAN2riBaGk4accWzVl7ypi1XTRxyrPYHst8DesugPXgOf"}]}`),
	[]byte(`{"payloadType":"cosign.sigstore.dev/attestation/v1","payload":"eyJfdHlwZSI6Imh0dHBzOi8vaW4tdG90by5pby9TdGF0ZW1lbnQvdjAuMSIsInByZWRpY2F0ZVR5cGUiOiJjb3NpZ24uc2lnc3RvcmUuZGV2L2F0dGVzdGF0aW9uL3YxIiwic3ViamVjdCI6W3sibmFtZSI6ImdoY3IuaW8vamltYnVnd2FkaWEvcGF1c2UyIiwiZGlnZXN0Ijp7InNoYTI1NiI6ImIzMWJmYjRkMDIxM2YyNTRkMzYxZTAwNzlkZWFhZWJlZmE0ZjgyYmE3YWE3NmVmODJlOTBiNDkzNWFkNWIxMDUifX1dLCJwcmVkaWNhdGUiOnsiRGF0YSI6ImhlbGxvIVxuIiwiVGltZXN0YW1wIjoiMjAyMS0xMC0wNVQwNToxODoxMVoifX0=","signatures":[{"keyid":"","sig":"MEQCIF5r9lf55rnYNPByZ9v6bortww694UEPvmyBIelIDYbIAiBNTGX4V64Oj6jZVRpkJQRxdzKUPYqC5GZTb4oS6eQ6aQ=="}]}`),
	[]byte(`{"payloadType":"https://example.com/CodeReview/v1","payload":"eyJfdHlwZSI6Imh0dHBzOi8vaW4tdG90by5pby9TdGF0ZW1lbnQvdjAuMSIsInByZWRpY2F0ZVR5cGUiOiJodHRwczovL2V4YW1wbGUuY29tL0NvZGVSZXZpZXcvdjEiLCJzdWJqZWN0IjpbeyJuYW1lIjoiZ2hjci5pby9qaW1idWd3YWRpYS9wYXVzZTIiLCJkaWdlc3QiOnsic2hhMjU2IjoiYjMxYmZiNGQwMjEzZjI1NGQzNjFlMDA3OWRlYWFlYmVmYTRmODJiYTdhYTc2ZWY4MmU5MGI0OTM1YWQ1YjEwNSJ9fV0sInByZWRpY2F0ZSI6eyJhdXRob3IiOiJtYWlsdG86YWxpY2VAZXhhbXBsZS5jb20iLCJyZXBvIjp7ImJyYW5jaCI6Im1haW4iLCJ0eXBlIjoiZ2l0IiwidXJpIjoiaHR0cHM6Ly9naXRodWIuY29tL2V4YW1wbGUvbXktcHJvamVjdCJ9LCJyZXZpZXdlcnMiOlsibWFpbHRvOmJvYkBleGFtcGxlLmNvbSJdfX0=","signatures":[{"keyid":"","sig":"MEUCIEeZbdBEFQzWqiMhB+SJgM6yFppUuQSKrpOIX1mxLDmRAiEA8pXqFq0GVc9LKhPzrnJRZhSruDNiKbiLHG5x7ETFyY8="}]}`),
}

func Test_CosignMockAttest(t *testing.T) {
	policyContext := buildContext(t, testPolicyGood, testResource, "")

	err := cosign.SetMock("ghcr.io/jimbugwadia/pause2:latest", payloads)
	assert.NilError(t, err)

	er := VerifyAndPatchImages(policyContext)
	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status, response.RuleStatusPass)
}

func Test_CosignMockAttest_fail(t *testing.T) {
	policyContext := buildContext(t, testPolicyBad, testResource, "")
	err := cosign.SetMock("ghcr.io/jimbugwadia/pause2:latest", payloads)
	assert.NilError(t, err)

	er := VerifyAndPatchImages(policyContext)
	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status, response.RuleStatusFail)
}

func buildContext(t *testing.T, policy, resource string, oldResource string) *PolicyContext {

	var cpol kyverno.ClusterPolicy
	err := json.Unmarshal([]byte(policy), &cpol)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured([]byte(resource))
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = context.AddResource(ctx, []byte(resource))
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &cpol,
		JSONContext: ctx,
		NewResource: *resourceUnstructured,
	}

	if oldResource != "" {
		oldResourceUnstructured, err := utils.ConvertToUnstructured([]byte(oldResource))
		assert.NilError(t, err)

		err = context.AddOldResource(ctx, []byte(oldResource))
		assert.NilError(t, err)

		policyContext.OldResource = *oldResourceUnstructured
	}

	if err := ctx.AddImageInfos(resourceUnstructured); err != nil {
		t.Errorf("unable to add image info to variables context: %v", err)
		t.Fail()
	}

	return policyContext
}

var testSampleSingleKeyPolicy = `
{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
        "name": "check-image",
        "annotations": {
            "pod-policies.kyverno.io/autogen-controllers": "none"
        }
    },
    "spec": {
        "validationFailureAction": "enforce",
        "background": false,
        "webhookTimeoutSeconds": 30,
        "failurePolicy": "Fail",
        "rules": [
            {
                "name": "check-signature",
                "match": {
                    "resources": {
                        "kinds": [
                            "Pod"
                        ]
                    }
                },
                "verifyImages": [
                    {
                        "imageReferences": [
                            "ghcr.io/kyverno/test-verify-image:*"
                        ],
                        "attestors": [
                            {
                                "entries": [
                                    {
                                        "staticKey": {
                                            "key": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM\n5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==\n-----END PUBLIC KEY-----"
                                        }
                                    }
                                ]
                            }
                        ]
                    }
                ]
            }
        ]
    }
}
`

var testSampleMultipleKeyPolicy = `
{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
        "name": "check-image",
        "annotations": {
            "pod-policies.kyverno.io/autogen-controllers": "none"
        }
    },
    "spec": {
        "validationFailureAction": "enforce",
        "background": false,
        "webhookTimeoutSeconds": 30,
        "failurePolicy": "Fail",
        "rules": [
            {
                "name": "check-signature",
                "match": {
                    "resources": {
                        "kinds": [
                            "Pod"
                        ]
                    }
                },
                "verifyImages": [
                    {
                        "imageReferences": [
                            "ghcr.io/kyverno/test-verify-image:*"
                        ],
                        "attestors": [
                            {
                                "count": COUNT,
                                "entries": [
                                    {
                                        "staticKey": {
                                            "key": "KEY1"
                                        }
                                    },
                                    {
                                        "staticKey": {
                                            "key": "KEY2"
                                        }
                                    }
                                ]
                            }
                        ]
                    }
                ]
            }
        ]
    }
}
`

var testSampleResource = `{
  "apiVersion": "v1",
  "kind": "Pod",
  "metadata": {"name": "test"},
  "spec": {
    "containers": [
      {
        "name": "pause2",
        "image": "ghcr.io/kyverno/test-verify-image:signed"
      }
    ]
  }
}`

var testVerifyImageKey = `-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==\n-----END PUBLIC KEY-----\n`
var testOtherKey = `-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEyBg8yod24/wIcc5QqlVLtCfL+6Te+nwdPdTvMb1AiZn24zBToHJVZvQdYLgRWAbh0Jd+6JhEwsDmnXRrlV7rfw==\n-----END PUBLIC KEY-----\n`

func Test_SignatureGoodSigned(t *testing.T) {
	policyContext := buildContext(t, testSampleSingleKeyPolicy, testSampleResource, "")
	cosign.ClearMock()
	err := VerifyAndPatchImages(policyContext)
	assert.Equal(t, len(err.PolicyResponse.Rules), 1)
	assert.Equal(t, err.PolicyResponse.Rules[0].Status, response.RuleStatusPass, err.PolicyResponse.Rules[0].Message)
}

func Test_SignatureUnsigned(t *testing.T) {
	cosign.ClearMock()
	unsigned := strings.Replace(testSampleResource, ":signed", ":unsigned", -1)
	policyContext := buildContext(t, testSampleSingleKeyPolicy, unsigned, "")
	err := VerifyAndPatchImages(policyContext)
	assert.Equal(t, len(err.PolicyResponse.Rules), 1)
	assert.Equal(t, err.PolicyResponse.Rules[0].Status, response.RuleStatusFail, err.PolicyResponse.Rules[0].Message)
}

func Test_SignatureWrongKey(t *testing.T) {
	cosign.ClearMock()
	otherKey := strings.Replace(testSampleResource, ":signed", ":signed-by-someone-else", -1)
	policyContext := buildContext(t, testSampleSingleKeyPolicy, otherKey, "")
	err := VerifyAndPatchImages(policyContext)
	assert.Equal(t, len(err.PolicyResponse.Rules), 1)
	assert.Equal(t, err.PolicyResponse.Rules[0].Status, response.RuleStatusFail, err.PolicyResponse.Rules[0].Message)
}

func Test_SignaturesMultiKey(t *testing.T) {
	cosign.ClearMock()
	policy := strings.Replace(testSampleMultipleKeyPolicy, "KEY1", testVerifyImageKey, -1)
	policy = strings.Replace(policy, "KEY2", testVerifyImageKey, -1)
	policy = strings.Replace(policy, "COUNT", "0", -1)
	policyContext := buildContext(t, policy, testSampleResource, "")
	err := VerifyAndPatchImages(policyContext)
	assert.Equal(t, len(err.PolicyResponse.Rules), 1)
	assert.Equal(t, err.PolicyResponse.Rules[0].Status, response.RuleStatusPass, err.PolicyResponse.Rules[0].Message)
}

func Test_SignaturesMultiKeyFail(t *testing.T) {
	cosign.ClearMock()
	policy := strings.Replace(testSampleMultipleKeyPolicy, "KEY1", testVerifyImageKey, -1)
	policy = strings.Replace(policy, "COUNT", "0", -1)
	policyContext := buildContext(t, policy, testSampleResource, "")
	err := VerifyAndPatchImages(policyContext)
	assert.Equal(t, len(err.PolicyResponse.Rules), 1)
	assert.Equal(t, err.PolicyResponse.Rules[0].Status, response.RuleStatusFail, err.PolicyResponse.Rules[0].Message)
}

func Test_SignaturesMultiKeyOneGoodKey(t *testing.T) {
	cosign.ClearMock()
	policy := strings.Replace(testSampleMultipleKeyPolicy, "KEY1", testVerifyImageKey, -1)
	policy = strings.Replace(policy, "KEY2", testOtherKey, -1)
	policy = strings.Replace(policy, "COUNT", "1", -1)
	policyContext := buildContext(t, policy, testSampleResource, "")
	err := VerifyAndPatchImages(policyContext)
	assert.Equal(t, len(err.PolicyResponse.Rules), 1)
	assert.Equal(t, err.PolicyResponse.Rules[0].Status, response.RuleStatusPass, err.PolicyResponse.Rules[0].Message)
}

func Test_SignaturesMultiKeyZeroGoodKey(t *testing.T) {
	cosign.ClearMock()
	policy := strings.Replace(testSampleMultipleKeyPolicy, "KEY1", testOtherKey, -1)
	policy = strings.Replace(policy, "KEY2", testOtherKey, -1)
	policy = strings.Replace(policy, "COUNT", "1", -1)
	policyContext := buildContext(t, policy, testSampleResource, "")
	err := VerifyAndPatchImages(policyContext)
	assert.Equal(t, len(err.PolicyResponse.Rules), 1)
	assert.Equal(t, err.PolicyResponse.Rules[0].Status, response.RuleStatusFail, err.PolicyResponse.Rules[0].Message)
}

var testNestedAttestorPolicy = `
{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
        "name": "check-image-keyless",
        "annotations": {
            "pod-policies.kyverno.io/autogen-controllers": "none"
        }
    },
    "spec": {
        "validationFailureAction": "enforce",
        "background": false,
        "webhookTimeoutSeconds": 30,
        "failurePolicy": "Fail",
        "rules": [
            {
                "name": "check-image-keyless",
                "match": {
                    "resources": {
                        "kinds": [
                            "Pod"
                        ]
                    }
                },
                "verifyImages": [
                    {
                        "imageReferences": [
                            "ghcr.io/kyverno/test-verify-image:*"
                        ],
                        "attestors": [
                            {
                                "count": COUNT,
                                "entries": [
                                    {
                                        "staticKey": {
                                            "key": "KEY1"
                                        }
                                    },
                                    {
                                        "attestor": {
                                            "entries": [
                                                {
                                                    "staticKey": {
                                                        "key": "KEY2"
                                                    }
                                                }
                                            ]
                                        }
                                    }
                                ]
                            }
                        ]
                    }
                ]
            }
        ]
    }
}
`

func Test_NestedAttestors(t *testing.T) {
	cosign.ClearMock()

	policy := strings.Replace(testNestedAttestorPolicy, "KEY1", testVerifyImageKey, -1)
	policy = strings.Replace(policy, "KEY2", testVerifyImageKey, -1)
	policy = strings.Replace(policy, "COUNT", "0", -1)
	policyContext := buildContext(t, policy, testSampleResource, "")
	err := VerifyAndPatchImages(policyContext)
	assert.Equal(t, len(err.PolicyResponse.Rules), 1)
	assert.Equal(t, err.PolicyResponse.Rules[0].Status, response.RuleStatusPass)

	policy = strings.Replace(testNestedAttestorPolicy, "KEY1", testVerifyImageKey, -1)
	policy = strings.Replace(policy, "KEY2", testOtherKey, -1)
	policy = strings.Replace(policy, "COUNT", "0", -1)
	policyContext = buildContext(t, policy, testSampleResource, "")
	err = VerifyAndPatchImages(policyContext)
	assert.Equal(t, len(err.PolicyResponse.Rules), 1)
	assert.Equal(t, err.PolicyResponse.Rules[0].Status, response.RuleStatusFail)

	policy = strings.Replace(testNestedAttestorPolicy, "KEY1", testVerifyImageKey, -1)
	policy = strings.Replace(policy, "KEY2", testOtherKey, -1)
	policy = strings.Replace(policy, "COUNT", "1", -1)
	policyContext = buildContext(t, policy, testSampleResource, "")
	err = VerifyAndPatchImages(policyContext)
	assert.Equal(t, len(err.PolicyResponse.Rules), 1)
	assert.Equal(t, err.PolicyResponse.Rules[0].Status, response.RuleStatusPass)
}

func Test_ExpandKeys(t *testing.T) {
	as := expandStaticKeys(createStaticKeyAttestorSet(""))
	assert.Equal(t, 1, len(as.Entries))

	as = expandStaticKeys(createStaticKeyAttestorSet(testOtherKey))
	assert.Equal(t, 1, len(as.Entries))

	as = expandStaticKeys(createStaticKeyAttestorSet(testOtherKey + testOtherKey + testOtherKey))
	assert.Equal(t, 3, len(as.Entries))
}

func createStaticKeyAttestorSet(s string) kyverno.AttestorSet {
	return kyverno.AttestorSet{
		Entries: []kyverno.Attestor{
			{
				StaticKey: &kyverno.StaticKeyAttestor{
					Keys: s,
				},
			},
		},
	}
}

func Test_ChangedAnnotation(t *testing.T) {
	name := "nginx"
	digest := "sha256:859ab6768a6f26a79bc42b231664111317d095a4f04e4b6fe79ce37b3d199097"
	annotationKey := makeAnnotationKey(name, digest)
	annotationNew := fmt.Sprintf("\"annotations\": {\"%s\": \"%s\"}", annotationKey, "true")
	newResource := strings.ReplaceAll(testResource, "\"annotations\": {}", annotationNew)

	policyContext := buildContext(t, testPolicyGood, testResource, testResource)
	hasChanged := hasImageVerifiedAnnotationChanged(policyContext, name, digest)
	assert.Equal(t, hasChanged, false)

	policyContext = buildContext(t, testPolicyGood, newResource, testResource)
	hasChanged = hasImageVerifiedAnnotationChanged(policyContext, name, digest)
	assert.Equal(t, hasChanged, true)

	annotationOld := fmt.Sprintf("\"annotations\": {\"%s\": \"%s\"}", annotationKey, "false")
	oldResource := strings.ReplaceAll(testResource, "\"annotations\": {}", annotationOld)

	policyContext = buildContext(t, testPolicyGood, newResource, oldResource)
	hasChanged = hasImageVerifiedAnnotationChanged(policyContext, name, digest)
	assert.Equal(t, hasChanged, true)
}

func Test_MarkImageVerified(t *testing.T) {
	imageVerifyRule := &kyverno.ImageVerification{Required: true}
	iv := &imageVerifier{
		logger:        log.Log,
		policyContext: buildContext(t, testPolicyGood, testResource, ""),
		rule:          &kyverno.Rule{VerifyImages: []*kyverno.ImageVerification{imageVerifyRule}},
		resp:          &response.EngineResponse{},
	}

	ruleResp := &response.RuleResponse{Status: response.RuleStatusPass}
	digest := "sha256:859ab6768a6f26a79bc42b231664111317d095a4f04e4b6fe79ce37b3d199097"
	imageInfo := kubeutils.ImageInfo{}
	imageInfo.Name = "nginx"

	iv.markImageVerified(imageVerifyRule, ruleResp, digest, imageInfo)
	assert.Equal(t, len(ruleResp.Patches), 1)

	u := applyPatches(t, ruleResp)
	key := makeAnnotationKey(imageInfo.Name, digest)
	value := u.GetAnnotations()[key]
	assert.Equal(t, value, "true")

	ruleResp.Patches = nil
	imageVerifyRule = &kyverno.ImageVerification{Required: false}
	iv.rule = &kyverno.Rule{VerifyImages: []*kyverno.ImageVerification{imageVerifyRule}}
	iv.markImageVerified(imageVerifyRule, ruleResp, digest, imageInfo)
	assert.Equal(t, len(ruleResp.Patches), 0)
}

func applyPatches(t *testing.T, ruleResp *response.RuleResponse) unstructured.Unstructured {
	patchedResource, err := utils.ApplyPatches([]byte(testResource), ruleResp.Patches)
	assert.NilError(t, err)
	assert.Assert(t, patchedResource != nil)

	u := unstructured.Unstructured{}
	err = u.UnmarshalJSON(patchedResource)
	assert.NilError(t, err)
	return u
}
