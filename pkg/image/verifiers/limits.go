package verifiers

// payloadLimit bounds the sigstore bundle and attestation reads image validating
// policies make. It is a limit on decompressed bytes, so a legitimate payload that
// compresses well -- an SBOM attestation is the usual case -- can exceed it while
// still being small on the wire. Operators who sign payloads above the default
// raise it with --imageVerifyPayloadLimit; the limit cannot be removed, because
// bounding these reads is what stops a decompression bomb.
var payloadLimit = int64(10 * 1000 * 1000) // 10 MB

// PayloadLimit returns the configured bound for verification payload reads.
func PayloadLimit() int64 {
	return payloadLimit
}

// SetPayloadLimit overrides the payload bound. Call it once at startup, before
// any verifier runs. A non-positive value leaves the default in place.
func SetPayloadLimit(n int64) {
	if n > 0 {
		payloadLimit = n
	}
}
