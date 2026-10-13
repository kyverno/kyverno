package apicall

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// canonicalizeURLPath parses and normalizes a raw Kubernetes API server path
// (as used by APICall.URLPath) into the exact string that will be sent on
// the wire. Namespace-confinement validation must run against this same
// canonical value that is later executed - otherwise validation and
// execution can disagree about which resource a path actually reaches,
// letting query strings, fragments, percent-encoding, or "." / ".."
// segments be used to spoof the apparent target namespace while the real
// request lands somewhere else.
//
// It returns two values: the canonical value to execute (path plus any query) and
// the cleaned path on its own. Namespace validation must use the path-only value -
// the clamp regex looks for a /namespaces/<name>/ segment anywhere in the string it
// is given, so matching it against path+query lets a decoy segment planted in a
// query value satisfy the check while the request routes somewhere else entirely.
func canonicalizeURLPath(raw string) (string, string, error) {
	if raw == "" {
		return "", "", fmt.Errorf("urlPath must not be empty")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse urlPath %q: %w", raw, err)
	}

	if parsed.IsAbs() || parsed.Host != "" || parsed.User != nil {
		return "", "", fmt.Errorf("urlPath %q must be a relative Kubernetes API path, not an absolute URL", raw)
	}

	if parsed.Fragment != "" || parsed.RawFragment != "" {
		return "", "", fmt.Errorf("urlPath %q must not contain a fragment", raw)
	}

	if !strings.HasPrefix(parsed.Path, "/") {
		return "", "", fmt.Errorf("urlPath %q must be an absolute path", raw)
	}

	// path.Clean resolves "." / ".." segments and collapses repeated
	// slashes using the *decoded* path (url.Parse already decoded any
	// percent-encoding), so traversal hidden behind encoding is resolved
	// the same way it will be interpreted once decoded on the wire.
	cleaned := path.Clean(parsed.Path)
	if !strings.HasPrefix(cleaned, "/") {
		// path.Clean always roots a rooted input; this is a defensive
		// guard in case that invariant ever changes.
		return "", "", fmt.Errorf("urlPath %q resolved outside of the API root", raw)
	}

	parsed.Path = cleaned
	parsed.RawPath = ""

	// RequestURI reconstructs "path?query" from the now-canonical Path and
	// the original RawQuery, re-escaping consistently. The fragment was
	// already rejected above, and the query string plays no part in
	// namespace/path validation since it is never used by the API server
	// for routing - it is carried through unchanged, but validation is
	// handed the path alone.
	return parsed.RequestURI(), cleaned, nil
}
