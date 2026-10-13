package internal

import (
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	imageutils "github.com/kyverno/kyverno/pkg/utils/image"
	"github.com/stretchr/testify/assert"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_SortStatementsAndGetRawResp_MultipleVersions(t *testing.T) {
	oldScan := map[string]any{
		"type": "cosign.sigstore.dev/attestation/vuln/v1",
		"predicate": map[string]any{
			"metadata": map[string]any{
				"scanTime": "2026-10-01T10:00:00Z",
			},
			"vulnerabilities": []any{"CVE-2026-1234"},
		},
	}
	newScan := map[string]any{
		"type": "cosign.sigstore.dev/attestation/vuln/v1",
		"predicate": map[string]any{
			"metadata": map[string]any{
				"scanTime": "2026-10-12T10:00:00Z",
			},
			"vulnerabilities": []any{},
		},
	}
	sbom := map[string]any{
		"type": "https://spdx.dev/Document",
		"predicate": map[string]any{
			"SPDXVersion": "SPDX-2.3",
		},
	}

	// Registry returns statements in non-chronological order: SBOM, old scan, new scan
	statements := []map[string]any{sbom, oldScan, newScan}

	raw, err := getRawResp(statements, "cosign.sigstore.dev/attestation/vuln/v1")
	assert.NoError(t, err)

	var extracted map[string]any
	err = json.Unmarshal(raw, &extracted)
	assert.NoError(t, err)

	meta := extracted["metadata"].(map[string]any)
	assert.Equal(t, "2026-10-12T10:00:00Z", meta["scanTime"])
	vulns := extracted["vulnerabilities"].([]any)
	assert.Empty(t, vulns)
}

func Test_ExtractStatementTimestamp_Formats(t *testing.T) {
	tests := []struct {
		name      string
		statement map[string]any
		expected  string
	}{
		{
			name: "cosign vuln scanTime",
			statement: map[string]any{
				"predicate": map[string]any{
					"metadata": map[string]any{
						"scanTime": "2026-05-01T12:00:00Z",
					},
				},
			},
			expected: "2026-05-01T12:00:00Z",
		},
		{
			name: "slsa buildStartedOn",
			statement: map[string]any{
				"predicate": map[string]any{
					"metadata": map[string]any{
						"buildStartedOn": "2026-06-01T08:30:00Z",
					},
				},
			},
			expected: "2026-06-01T08:30:00Z",
		},
		{
			name: "trivy CreatedAt",
			statement: map[string]any{
				"predicate": map[string]any{
					"CreatedAt": "2026-07-01T15:20:00Z",
				},
			},
			expected: "2026-07-01T15:20:00Z",
		},
		{
			name: "annotation created",
			statement: map[string]any{
				"annotations": map[string]any{
					"org.opencontainers.image.created": "2026-08-01T10:00:00Z",
				},
			},
			expected: "2026-08-01T10:00:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm, ok := extractStatementTimestamp(tt.statement)
			assert.True(t, ok)
			assert.Equal(t, tt.expected, tm.Format("2006-01-02T15:04:05Z07:00"))
		})
	}
}

func Test_VerifyAttestation_EvaluatesNewestStatement(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	cfg := config.NewDefaultConfiguration(false)
	res := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   map[string]any{"name": "test"},
		},
	}
	policyCtx, err := policycontext.NewPolicyContext(jp, res, kyvernov1.Create, nil, cfg)
	assert.NoError(t, err)

	iv := &imageVerifier{
		logger:        logr.Discard(),
		policyContext: policyCtx,
		rule:          kyvernov1.Rule{Name: "check-vuln"},
	}

	attestation := kyvernov1.Attestation{
		PredicateType: "cosign.sigstore.dev/attestation/vuln/v1",
		Type:          "cosign.sigstore.dev/attestation/vuln/v1",
		Conditions: []kyvernov1.AnyAllConditions{
			{
				AllConditions: []kyvernov1.Condition{
					{
						RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ vulnerabilities }}"`)},
						Operator: kyvernov1.ConditionOperators["Equals"],
						RawValue: &apiextv1.JSON{Raw: []byte(`[]`)},
					},
				},
			},
		},
	}

	oldScan := map[string]any{
		"type": "cosign.sigstore.dev/attestation/vuln/v1",
		"predicate": map[string]any{
			"metadata": map[string]any{
				"scanTime": "2026-10-01T10:00:00Z",
			},
			"vulnerabilities": []any{"CVE-2026-1234"},
		},
	}
	newScan := map[string]any{
		"type": "cosign.sigstore.dev/attestation/vuln/v1",
		"predicate": map[string]any{
			"metadata": map[string]any{
				"scanTime": "2026-10-12T10:00:00Z",
			},
			"vulnerabilities": []any{},
		},
	}

	imgInfo, err := imageutils.GetImageInfo("ghcr.io/org/repo:v1", cfg)
	assert.NoError(t, err)
	imageInfo := apiutils.ImageInfo{ImageInfo: *imgInfo}

	// Case 1: Registry returns old scan first, then new scan.
	// Since new scan has 0 vulnerabilities and newer timestamp, it must pass.
	err = iv.verifyAttestation([]map[string]any{oldScan, newScan}, attestation, imageInfo)
	assert.NoError(t, err)

	// Case 2: Latest scan has vulnerabilities. Verification must fail.
	latestFailingScan := map[string]any{
		"type": "cosign.sigstore.dev/attestation/vuln/v1",
		"predicate": map[string]any{
			"metadata": map[string]any{
				"scanTime": "2026-10-15T10:00:00Z",
			},
			"vulnerabilities": []any{"CVE-2026-9999"},
		},
	}
	err = iv.verifyAttestation([]map[string]any{newScan, latestFailingScan}, attestation, imageInfo)
	assert.Error(t, err)

	// Case 3: Mixed timestamps (one has timestamp, one does not).
	// Because not all statements have timestamps, chronology is uncertain and fallback requires all to pass.
	noTimestampFailingScan := map[string]any{
		"type": "cosign.sigstore.dev/attestation/vuln/v1",
		"predicate": map[string]any{
			"vulnerabilities": []any{"CVE-2026-5555"},
		},
	}
	err = iv.verifyAttestation([]map[string]any{newScan, noTimestampFailingScan}, attestation, imageInfo)
	assert.Error(t, err)

	// Case 4: No timestamps at all, but one fails. Verification must fail.
	noTimestampPassingScan := map[string]any{
		"type": "cosign.sigstore.dev/attestation/vuln/v1",
		"predicate": map[string]any{
			"vulnerabilities": []any{},
		},
	}
	err = iv.verifyAttestation([]map[string]any{noTimestampPassingScan, noTimestampFailingScan}, attestation, imageInfo)
	assert.Error(t, err)

	// Case 5: No timestamps at all, and all pass. Verification must succeed.
	anotherPassingScan := map[string]any{
		"type": "cosign.sigstore.dev/attestation/vuln/v1",
		"predicate": map[string]any{
			"vulnerabilities": []any{},
		},
	}
	err = iv.verifyAttestation([]map[string]any{noTimestampPassingScan, anotherPassingScan}, attestation, imageInfo)
	assert.NoError(t, err)
}
