package cosign

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/oci/static"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protodsse "github.com/sigstore/protobuf-specs/gen/pb-go/dsse"
	protorekor "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	sgbundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractStatementTimestamp(t *testing.T) {
	t1, err := time.Parse(time.RFC3339, "2026-03-01T12:00:00Z")
	require.NoError(t, err)

	tests := []struct {
		name      string
		statement map[string]any
		wantTime  time.Time
		wantOk    bool
	}{
		{
			name: "predicate metadata scanFinishedOn",
			statement: map[string]any{
				"predicate": map[string]any{
					"metadata": map[string]any{
						"scanFinishedOn": "2026-03-01T12:00:00Z",
					},
				},
			},
			wantTime: t1,
			wantOk:   true,
		},
		{
			name: "predicate metadata scanTime",
			statement: map[string]any{
				"predicate": map[string]any{
					"metadata": map[string]any{
						"scanTime": "2026-03-01T12:00:00Z",
					},
				},
			},
			wantTime: t1,
			wantOk:   true,
		},
		{
			name: "predicate runDetails metadata finishedOn",
			statement: map[string]any{
				"predicate": map[string]any{
					"runDetails": map[string]any{
						"metadata": map[string]any{
							"finishedOn": "2026-03-01T12:00:00Z",
						},
					},
				},
			},
			wantTime: t1,
			wantOk:   true,
		},
		{
			name: "predicate creationInfo created",
			statement: map[string]any{
				"predicate": map[string]any{
					"creationInfo": map[string]any{
						"created": "2026-03-01T12:00:00Z",
					},
				},
			},
			wantTime: t1,
			wantOk:   true,
		},
		{
			name: "direct predicate Timestamp",
			statement: map[string]any{
				"predicate": map[string]any{
					"Timestamp": "2026-03-01T12:00:00Z",
				},
			},
			wantTime: t1,
			wantOk:   true,
		},
		{
			name: "direct predicate scanFinishedOn",
			statement: map[string]any{
				"predicate": map[string]any{
					"scanFinishedOn": "2026-03-01T12:00:00Z",
				},
			},
			wantTime: t1,
			wantOk:   true,
		},
		{
			name: "direct predicate without statement wrapper",
			statement: map[string]any{
				"metadata": map[string]any{
					"scanFinishedOn": "2026-03-01T12:00:00Z",
				},
			},
			wantTime: t1,
			wantOk:   true,
		},
		{
			name: "statement root created",
			statement: map[string]any{
				"created": "2026-03-01T12:00:00Z",
			},
			wantTime: t1,
			wantOk:   true,
		},
		{
			name: "no timestamp present",
			statement: map[string]any{
				"predicate": map[string]any{
					"which": "first",
				},
			},
			wantTime: time.Time{},
			wantOk:   false,
		},
		{
			name: "invalid timestamp format",
			statement: map[string]any{
				"predicate": map[string]any{
					"metadata": map[string]any{
						"scanFinishedOn": "invalid-timestamp",
					},
				},
			},
			wantTime: time.Time{},
			wantOk:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotTime, ok := extractStatementTimestamp(tc.statement)
			assert.Equal(t, tc.wantOk, ok)
			if tc.wantOk {
				assert.True(t, tc.wantTime.Equal(gotTime))
			}
		})
	}
}

func TestSelectNewestAttestation(t *testing.T) {
	tOld := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	tNew := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)

	t.Run("single candidate returns immediately", func(t *testing.T) {
		candidate := verifiedCandidate{
			payload:       []byte(`{"which":"only"}`),
			predicateType: "custom",
		}
		chosen, err := selectNewestAttestation([]verifiedCandidate{candidate}, "custom")
		require.NoError(t, err)
		assert.Equal(t, candidate.payload, chosen.payload)
	})

	t.Run("zero candidates returns error", func(t *testing.T) {
		_, err := selectNewestAttestation(nil, "custom")
		assert.Error(t, err)
	})

	t.Run("statement timestamp order: newest last returned from registry", func(t *testing.T) {
		oldCand := verifiedCandidate{
			payload:       []byte(`{"name":"old"}`),
			predicateType: "vuln",
			statementTS:   tOld,
			hasStatement:  true,
		}
		newCand := verifiedCandidate{
			payload:       []byte(`{"name":"new"}`),
			predicateType: "vuln",
			statementTS:   tNew,
			hasStatement:  true,
		}

		// Registry returns old then new
		chosen, err := selectNewestAttestation([]verifiedCandidate{oldCand, newCand}, "vuln")
		require.NoError(t, err)
		assert.Equal(t, newCand.payload, chosen.payload)

		// Registry returns new then old
		chosenReverse, err := selectNewestAttestation([]verifiedCandidate{newCand, oldCand}, "vuln")
		require.NoError(t, err)
		assert.Equal(t, newCand.payload, chosenReverse.payload)
	})

	t.Run("bundle timestamp order: arbitrary predicate without statement timestamps", func(t *testing.T) {
		oldCand := verifiedCandidate{
			payload:       []byte(`{"which":"first"}`),
			predicateType: "custom",
			bundleTS:      tOld,
			hasBundle:     true,
		}
		newCand := verifiedCandidate{
			payload:       []byte(`{"which":"second"}`),
			predicateType: "custom",
			bundleTS:      tNew,
			hasBundle:     true,
		}

		chosen, err := selectNewestAttestation([]verifiedCandidate{oldCand, newCand}, "custom")
		require.NoError(t, err)
		assert.Equal(t, newCand.payload, chosen.payload)

		chosenRev, err := selectNewestAttestation([]verifiedCandidate{newCand, oldCand}, "custom")
		require.NoError(t, err)
		assert.Equal(t, newCand.payload, chosenRev.payload)
	})

	t.Run("mixed timestamps: one statement and one bundle timestamp", func(t *testing.T) {
		cand1 := verifiedCandidate{
			payload:       []byte(`{"name":"statement-older"}`),
			predicateType: "custom",
			statementTS:   tOld,
			hasStatement:  true,
		}
		cand2 := verifiedCandidate{
			payload:       []byte(`{"name":"bundle-newer"}`),
			predicateType: "custom",
			bundleTS:      tNew,
			hasBundle:     true,
		}

		chosen, err := selectNewestAttestation([]verifiedCandidate{cand1, cand2}, "custom")
		require.NoError(t, err)
		assert.Equal(t, cand2.payload, chosen.payload)
	})

	t.Run("identical timestamps: deterministic tie-breaking", func(t *testing.T) {
		candA := verifiedCandidate{
			payload:       []byte(`{"id":"aaa"}`),
			predicateType: "custom",
			statementTS:   tNew,
			hasStatement:  true,
		}
		candB := verifiedCandidate{
			payload:       []byte(`{"id":"bbb"}`),
			predicateType: "custom",
			statementTS:   tNew,
			hasStatement:  true,
		}

		// Order in list should not affect the winner
		chosen1, err := selectNewestAttestation([]verifiedCandidate{candA, candB}, "custom")
		require.NoError(t, err)
		chosen2, err := selectNewestAttestation([]verifiedCandidate{candB, candA}, "custom")
		require.NoError(t, err)
		assert.Equal(t, chosen1.payload, chosen2.payload)
	})

	t.Run("multiple candidates without any timestamps fails closed", func(t *testing.T) {
		cand1 := verifiedCandidate{
			payload:       []byte(`{"which":"first"}`),
			predicateType: "custom",
		}
		cand2 := verifiedCandidate{
			payload:       []byte(`{"which":"second"}`),
			predicateType: "custom",
		}

		_, err := selectNewestAttestation([]verifiedCandidate{cand1, cand2}, "custom")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing or incomplete timestamps")
	})

	t.Run("multiple candidates where one lacks timestamps fails closed", func(t *testing.T) {
		cand1 := verifiedCandidate{
			payload:       []byte(`{"which":"first"}`),
			predicateType: "custom",
			statementTS:   tNew,
			hasStatement:  true,
		}
		cand2 := verifiedCandidate{
			payload:       []byte(`{"which":"second"}`),
			predicateType: "custom",
		}

		_, err := selectNewestAttestation([]verifiedCandidate{cand1, cand2}, "custom")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing or incomplete timestamps")
	})
}

func TestExtractAttestationTimestamps(t *testing.T) {
	tStatement := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	tRekor := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

	stmt := map[string]any{
		"_type":         "https://in-toto.io/Statement/v0.1",
		"predicateType": "https://cosign.sigstore.dev/attestation/v1",
		"predicate": map[string]any{
			"metadata": map[string]any{
				"scanFinishedOn": tStatement.Format(time.RFC3339),
			},
		},
	}
	stmtBytes, err := json.Marshal(stmt)
	require.NoError(t, err)

	dsseEnv := map[string]any{
		"payloadType": "application/vnd.in-toto+json",
		"payload":     base64.StdEncoding.EncodeToString(stmtBytes),
	}
	dsseBytes, err := json.Marshal(dsseEnv)
	require.NoError(t, err)

	sig, err := static.NewAttestation(dsseBytes)
	require.NoError(t, err)

	t.Run("extract from payload directly", func(t *testing.T) {
		_, _, sTS, hasS := extractAttestationTimestamps(sig, stmtBytes, nil, nil)
		assert.True(t, hasS)
		assert.True(t, tStatement.Equal(sTS))
	})

	t.Run("extract from signature DSSE when payload is empty", func(t *testing.T) {
		_, _, sTS, hasS := extractAttestationTimestamps(sig, nil, nil, nil)
		assert.True(t, hasS)
		assert.True(t, tStatement.Equal(sTS))
	})

	t.Run("extract from bundle Rekor integrated time", func(t *testing.T) {
		tlogBody := map[string]any{
			"kind":       "hashedrekord",
			"apiVersion": "0.0.1",
			"spec": map[string]any{
				"signature": map[string]any{
					"content": "MEQCIFrwIdVX8n5RM+Fy9fgCmaBc20jmksfL0XL08y1zx3XpAiB95HkXz37kTUzdykwuNStwCc5B9NKHtioD+3GYMuWU/w==",
					"publicKey": map[string]any{
						"content": "LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUZrd0V3WUhLb1pJemowQ0FRWUlLb1pJemowREFRY0RRZ0FFNHlQQ080MStjeGxE\ndENBUndTNDNvQU1YVWs3NApyWGZ5eGhKSldJZ05KbTUyTlppZllHaDNnYzNaakJVOVJhRXJLb0NidGVxdW1IWU9CSnN6RmNIUGFBPT0KLS0tLS1FTkQgUFV\nCTElDIEtFWS0tLS0tCg==",
					},
				},
				"data": map[string]any{
					"hash": map[string]any{
						"algorithm": "sha256",
						"value":     "0c0f699f002f4de2ab43b04a0c930ceb57be35d2c81b31648d88b021713e9477",
					},
				},
			},
		}
		canonicalTlogBody, err := json.Marshal(tlogBody)
		require.NoError(t, err)

		pbBundle := &protobundle.Bundle{
			MediaType: "application/vnd.dev.sigstore.bundle.v0.3+json",
			Content: &protobundle.Bundle_DsseEnvelope{
				DsseEnvelope: &protodsse.Envelope{
					PayloadType: "application/vnd.in-toto+json",
					Payload:     stmtBytes,
				},
			},
			VerificationMaterial: &protobundle.VerificationMaterial{
				Content: &protobundle.VerificationMaterial_PublicKey{
					PublicKey: &protocommon.PublicKeyIdentifier{
						Hint: "test",
					},
				},
				TlogEntries: []*protorekor.TransparencyLogEntry{
					{
						LogIndex: 42,
						LogId: &protocommon.LogId{
							KeyId: []byte("deadbeef"),
						},
						KindVersion: &protorekor.KindVersion{
							Kind:    "hashedrekord",
							Version: "0.0.1",
						},
						IntegratedTime:    tRekor.Unix(),
						CanonicalizedBody: canonicalTlogBody,
						InclusionPromise: &protorekor.InclusionPromise{
							SignedEntryTimestamp: []byte("set"),
						},
						InclusionProof: &protorekor.InclusionProof{
							LogIndex:   42,
							RootHash:   []byte("b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c"),
							Checkpoint: &protorekor.Checkpoint{Envelope: "checkpoint"},
						},
					},
				},
			},
		}

		sgb, err := sgbundle.NewBundle(pbBundle)
		require.NoError(t, err)

		cOpts := &cosign.CheckOpts{
			NewBundleFormat: true,
		}

		bTS, hasB, sTS, hasS := extractAttestationTimestamps(sig, stmtBytes, cOpts, []*sgbundle.Bundle{sgb})
		assert.True(t, hasS)
		assert.True(t, tStatement.Equal(sTS))
		assert.True(t, hasB)
		assert.True(t, tRekor.Equal(bTS))
	})

	t.Run("ignore bundle Rekor integrated time when IgnoreTlog is true", func(t *testing.T) {
		tlogBody := map[string]any{
			"kind":       "hashedrekord",
			"apiVersion": "0.0.1",
			"spec": map[string]any{
				"signature": map[string]any{
					"content": "MEQCIFrwIdVX8n5RM+Fy9fgCmaBc20jmksfL0XL08y1zx3XpAiB95HkXz37kTUzdykwuNStwCc5B9NKHtioD+3GYMuWU/w==",
					"publicKey": map[string]any{
						"content": "LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUZrd0V3WUhLb1pJemowQ0FRWUlLb1pJemowREFRY0RRZ0FFNHlQQ080MStjeGxE\ndENBUndTNDNvQU1YVWs3NApyWGZ5eGhKSldJZ05KbTUyTlppZllHaDNnYzNaakJVOVJhRXJLb0NidGVxdW1IWU9CSnN6RmNIUGFBPT0KLS0tLS1FTkQgUFV\nCTElDIEtFWS0tLS0tCg==",
					},
				},
				"data": map[string]any{
					"hash": map[string]any{
						"algorithm": "sha256",
						"value":     "0c0f699f002f4de2ab43b04a0c930ceb57be35d2c81b31648d88b021713e9477",
					},
				},
			},
		}
		canonicalTlogBody, err := json.Marshal(tlogBody)
		require.NoError(t, err)

		pbBundle := &protobundle.Bundle{
			MediaType: "application/vnd.dev.sigstore.bundle.v0.3+json",
			Content: &protobundle.Bundle_DsseEnvelope{
				DsseEnvelope: &protodsse.Envelope{
					PayloadType: "application/vnd.in-toto+json",
					Payload:     stmtBytes,
				},
			},
			VerificationMaterial: &protobundle.VerificationMaterial{
				Content: &protobundle.VerificationMaterial_PublicKey{
					PublicKey: &protocommon.PublicKeyIdentifier{
						Hint: "test",
					},
				},
				TlogEntries: []*protorekor.TransparencyLogEntry{
					{
						LogIndex: 42,
						LogId: &protocommon.LogId{
							KeyId: []byte("deadbeef"),
						},
						KindVersion: &protorekor.KindVersion{
							Kind:    "hashedrekord",
							Version: "0.0.1",
						},
						IntegratedTime:    tRekor.Unix(),
						CanonicalizedBody: canonicalTlogBody,
						InclusionPromise: &protorekor.InclusionPromise{
							SignedEntryTimestamp: []byte("set"),
						},
						InclusionProof: &protorekor.InclusionProof{
							LogIndex:   42,
							RootHash:   []byte("b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c"),
							Checkpoint: &protorekor.Checkpoint{Envelope: "checkpoint"},
						},
					},
				},
			},
		}

		sgb, err := sgbundle.NewBundle(pbBundle)
		require.NoError(t, err)

		cOpts := &cosign.CheckOpts{
			NewBundleFormat: true,
			IgnoreTlog:      true,
		}

		bTS, hasB, sTS, hasS := extractAttestationTimestamps(sig, stmtBytes, cOpts, []*sgbundle.Bundle{sgb})
		assert.False(t, hasB, "bundle timestamp must not be trusted when IgnoreTlog is true")
		assert.True(t, bTS.IsZero())
		assert.True(t, hasS, "signed statement timestamp must still be extracted")
		assert.True(t, tStatement.Equal(sTS))
	})

	t.Run("bundle does not match signature", func(t *testing.T) {
		tlogBody := map[string]any{
			"kind":       "hashedrekord",
			"apiVersion": "0.0.1",
			"spec": map[string]any{
				"signature": map[string]any{
					"content": "MEQCIFrwIdVX8n5RM+Fy9fgCmaBc20jmksfL0XL08y1zx3XpAiB95HkXz37kTUzdykwuNStwCc5B9NKHtioD+3GYMuWU/w==",
					"publicKey": map[string]any{
						"content": "LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUZrd0V3WUhLb1pJemowQ0FRWUlLb1pJemowREFRY0RRZ0FFNHlQQ080MStjeGxE\ndENBUndTNDNvQU1YVWs3NApyWGZ5eGhKSldJZ05KbTUyTlppZllHaDNnYzNaakJVOVJhRXJLb0NidGVxdW1IWU9CSnN6RmNIUGFBPT0KLS0tLS1FTkQgUFV\nCTElDIEtFWS0tLS0tCg==",
					},
				},
				"data": map[string]any{
					"hash": map[string]any{
						"algorithm": "sha256",
						"value":     "0c0f699f002f4de2ab43b04a0c930ceb57be35d2c81b31648d88b021713e9477",
					},
				},
			},
		}
		canonicalTlogBody, err := json.Marshal(tlogBody)
		require.NoError(t, err)

		mismatchedBundle := &protobundle.Bundle{
			MediaType: "application/vnd.dev.sigstore.bundle.v0.3+json",
			Content: &protobundle.Bundle_DsseEnvelope{
				DsseEnvelope: &protodsse.Envelope{
					PayloadType: "application/vnd.in-toto+json",
					Payload:     []byte("unrelated-payload"),
				},
			},
			VerificationMaterial: &protobundle.VerificationMaterial{
				Content: &protobundle.VerificationMaterial_PublicKey{
					PublicKey: &protocommon.PublicKeyIdentifier{
						Hint: "test",
					},
				},
				TlogEntries: []*protorekor.TransparencyLogEntry{
					{
						LogIndex: 42,
						LogId: &protocommon.LogId{
							KeyId: []byte("deadbeef"),
						},
						KindVersion: &protorekor.KindVersion{
							Kind:    "hashedrekord",
							Version: "0.0.1",
						},
						IntegratedTime:    tRekor.Unix(),
						CanonicalizedBody: canonicalTlogBody,
						InclusionPromise: &protorekor.InclusionPromise{
							SignedEntryTimestamp: []byte("set"),
						},
						InclusionProof: &protorekor.InclusionProof{
							LogIndex:   42,
							RootHash:   []byte("b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c"),
							Checkpoint: &protorekor.Checkpoint{Envelope: "checkpoint"},
						},
					},
				},
			},
		}

		sgb, err := sgbundle.NewBundle(mismatchedBundle)
		require.NoError(t, err)

		cOpts := &cosign.CheckOpts{
			NewBundleFormat: true,
		}

		bTS, hasB, _, _ := extractAttestationTimestamps(sig, nil, cOpts, []*sgbundle.Bundle{sgb})
		assert.False(t, hasB, "hasBundle must be false when bundle does not match signature")
		assert.True(t, bTS.IsZero())
	})

	t.Run("malformed DSSE payload extracts no statement timestamp", func(t *testing.T) {
		malformedSig, err := static.NewAttestation([]byte("{invalid-json"))
		require.NoError(t, err)

		_, hasB, sTS, hasS := extractAttestationTimestamps(malformedSig, nil, nil, nil)
		assert.False(t, hasS, "hasStatement must be false for malformed DSSE envelope")
		assert.False(t, hasB)
		assert.True(t, sTS.IsZero())
	})

	t.Run("valid payload with no timestamps reports neither timestamp", func(t *testing.T) {
		noTimeStmt := map[string]any{
			"_type":         "https://in-toto.io/Statement/v0.1",
			"predicateType": "custom",
			"predicate": map[string]any{
				"key": "value-without-timestamp",
			},
		}
		noTimeBytes, err := json.Marshal(noTimeStmt)
		require.NoError(t, err)

		noTimeDsse := map[string]any{
			"payloadType": "application/vnd.in-toto+json",
			"payload":     base64.StdEncoding.EncodeToString(noTimeBytes),
		}
		noTimeDsseBytes, err := json.Marshal(noTimeDsse)
		require.NoError(t, err)

		sigNoTime, err := static.NewAttestation(noTimeDsseBytes)
		require.NoError(t, err)

		bTS, hasB, sTS, hasS := extractAttestationTimestamps(sigNoTime, noTimeBytes, nil, nil)
		assert.False(t, hasS, "hasStatement must be false when payload has no timestamps")
		assert.False(t, hasB, "hasBundle must be false when no bundle is provided")
		assert.True(t, sTS.IsZero())
		assert.True(t, bTS.IsZero())
	})
}
