package cosign

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/oci"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	sgbundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

var timestampKeys = []string{
	"scanFinishedOn",
	"scanTime",
	"scanStartedOn",
	"buildFinishedOn",
	"buildStartedOn",
	"finishedOn",
	"startedOn",
	"timestamp",
	"Timestamp",
	"CreatedAt",
	"created",
	"time",
}

// extractStatementTimestamp extracts a trusted timestamp from signed in-toto statement fields.
// Unsigned OCI manifest annotations are intentionally excluded to prevent mutable registry metadata
// from manipulating attestation evaluation order.
func extractStatementTimestamp(statement map[string]any) (time.Time, bool) {
	if predicate, ok := statement["predicate"].(map[string]any); ok {
		if t, ok := extractFromPredicate(predicate); ok {
			return t, true
		}
	} else {
		if t, ok := extractFromPredicate(statement); ok {
			return t, true
		}
	}

	for _, k := range []string{"created", "timestamp", "Timestamp", "time"} {
		if t, ok := parseTimeValue(statement[k]); ok {
			return t, true
		}
	}

	return time.Time{}, false
}

func extractFromPredicate(predicate map[string]any) (time.Time, bool) {
	if meta, ok := predicate["metadata"].(map[string]any); ok {
		for _, k := range timestampKeys {
			if t, ok := parseTimeValue(meta[k]); ok {
				return t, true
			}
		}
	}
	if runDetails, ok := predicate["runDetails"].(map[string]any); ok {
		if meta, ok := runDetails["metadata"].(map[string]any); ok {
			for _, k := range timestampKeys {
				if t, ok := parseTimeValue(meta[k]); ok {
					return t, true
				}
			}
		}
	}
	if creationInfo, ok := predicate["creationInfo"].(map[string]any); ok {
		if t, ok := parseTimeValue(creationInfo["created"]); ok {
			return t, true
		}
	}
	for _, k := range timestampKeys {
		if t, ok := parseTimeValue(predicate[k]); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseTimeValue(val any) (time.Time, bool) {
	s, ok := val.(string)
	if !ok || s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

type verifiedCandidate struct {
	payload       []byte
	predicateType string
	statementTS   time.Time
	hasStatement  bool
	bundleTS      time.Time
	hasBundle     bool
}

// selectNewestAttestation orders multiple verified attestations chronologically and selects
// the newest one. If timestamps are missing or incomplete across multiple candidates,
// it returns an error to fail closed.
func selectNewestAttestation(candidates []verifiedCandidate, predicateType string) (verifiedCandidate, error) {
	if len(candidates) == 0 {
		return verifiedCandidate{}, fmt.Errorf("no candidates to select from")
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	allHaveStatement := true
	allHaveBundle := true
	for _, c := range candidates {
		if !c.hasStatement {
			allHaveStatement = false
		}
		if !c.hasBundle {
			allHaveBundle = false
		}
	}

	if allHaveStatement {
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].statementTS.Equal(candidates[j].statementTS) {
				return bytes.Compare(candidates[i].payload, candidates[j].payload) > 0
			}
			return candidates[i].statementTS.After(candidates[j].statementTS)
		})
		return candidates[0], nil
	}

	if allHaveBundle {
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].bundleTS.Equal(candidates[j].bundleTS) {
				return bytes.Compare(candidates[i].payload, candidates[j].payload) > 0
			}
			return candidates[i].bundleTS.After(candidates[j].bundleTS)
		})
		return candidates[0], nil
	}

	allHaveAny := true
	for _, c := range candidates {
		if !c.hasStatement && !c.hasBundle {
			allHaveAny = false
			break
		}
	}
	if !allHaveAny {
		return verifiedCandidate{}, fmt.Errorf("multiple attestations found for predicate type %s but unable to determine newest: missing or incomplete timestamps", predicateType)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		ti := candidates[i].statementTS
		if !candidates[i].hasStatement {
			ti = candidates[i].bundleTS
		}
		tj := candidates[j].statementTS
		if !candidates[j].hasStatement {
			tj = candidates[j].bundleTS
		}
		if ti.Equal(tj) {
			return bytes.Compare(candidates[i].payload, candidates[j].payload) > 0
		}
		return ti.After(tj)
	})
	return candidates[0], nil
}

// extractAttestationTimestamps extracts both the bundle-verified timestamp (Rekor integrated time or
// RFC 3161 timestamp) and the signed in-toto statement timestamp, if present.
func extractAttestationTimestamps(
	s oci.Signature,
	payload []byte,
	cOpts *cosign.CheckOpts,
	newBundles []*sgbundle.Bundle,
) (bundleTS time.Time, hasBundleTS bool, statementTS time.Time, hasStatementTS bool) {
	// 1. Try bundle timestamp from cosign v3 bundle format
	if cOpts != nil && cOpts.NewBundleFormat && len(newBundles) > 0 {
		if b := findMatchingBundle(s, newBundles); b != nil {
			// Rekor log integrated time
			if entries, err := b.TlogEntries(); err == nil && len(entries) > 0 {
				t := entries[0].IntegratedTime()
				if !t.IsZero() {
					bundleTS = t
					hasBundleTS = true
				}
			}
			// RFC 3161 timestamp if no integrated time
			if !hasBundleTS && cOpts.TrustedMaterial != nil {
				if vts, _, err := verify.VerifySignedTimestamp(b, cOpts.TrustedMaterial); err == nil && len(vts) > 0 && !vts[0].Time.IsZero() {
					bundleTS = vts[0].Time
					hasBundleTS = true
				}
			}
		}
	}

	// Traditional non-bundle signature format
	if !hasBundleTS && s != nil {
		if rb, err := s.Bundle(); err == nil && rb != nil && rb.Payload.IntegratedTime > 0 {
			bundleTS = time.Unix(rb.Payload.IntegratedTime, 0)
			hasBundleTS = true
		} else if rfc, err := s.RFC3161Timestamp(); err == nil && rfc != nil {
			if ts, err := cosign.VerifyRFC3161Timestamp(s, cOpts); err == nil && ts != nil && !ts.Time.IsZero() {
				bundleTS = ts.Time
				hasBundleTS = true
			}
		}
	}

	// 2. Signed in-toto statement timestamp
	if len(payload) > 0 {
		var stmt map[string]any
		if err := json.Unmarshal(payload, &stmt); err == nil {
			statementTS, hasStatementTS = extractStatementTimestamp(stmt)
		}
	}
	if !hasStatementTS && s != nil {
		if rawPayload, err := s.Payload(); err == nil && len(rawPayload) > 0 {
			var env struct {
				Payload string `json:"payload"`
			}
			if err := json.Unmarshal(rawPayload, &env); err == nil && env.Payload != "" {
				if dec, err := base64.StdEncoding.DecodeString(env.Payload); err == nil {
					var stmt map[string]any
					if err := json.Unmarshal(dec, &stmt); err == nil {
						statementTS, hasStatementTS = extractStatementTimestamp(stmt)
					}
				}
			}
		}
	}

	return bundleTS, hasBundleTS, statementTS, hasStatementTS
}

func findMatchingBundle(s oci.Signature, bundles []*sgbundle.Bundle) *sgbundle.Bundle {
	if s == nil {
		return nil
	}
	sPayload, err := s.Payload()
	if err != nil || len(sPayload) == 0 {
		return nil
	}
	var sEnv struct {
		Payload string `json:"payload"`
	}
	var sDecoded []byte
	if err := json.Unmarshal(sPayload, &sEnv); err == nil && sEnv.Payload != "" {
		sDecoded, _ = base64.StdEncoding.DecodeString(sEnv.Payload)
	}

	for _, b := range bundles {
		if b == nil {
			continue
		}
		if dsse, ok := b.Content.(*protobundle.Bundle_DsseEnvelope); ok && dsse.DsseEnvelope != nil {
			dsseBytes, err := json.Marshal(dsse.DsseEnvelope)
			if err == nil && bytes.Equal(sPayload, dsseBytes) {
				return b
			}
			if len(sDecoded) > 0 && bytes.Equal(sDecoded, dsse.DsseEnvelope.Payload) {
				return b
			}
		}
	}
	return nil
}
