package verifiers

import (
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// maxReferrersIndexSize bounds the referrers index body before it is unmarshalled.
// A legitimate index of the 50 entries the verifiers allow is a few tens of
// kilobytes, so the headroom here is deliberate. go-containerregistry already fetched and buffered the
// body by the time the index is handed to us, so this cannot stop the download --
// what it stops is the unmarshal, which is where the cost actually lands: a 34 MB
// index expands to roughly 400 MB of Go structs.
var maxReferrersIndexSize = int64(1000 * 1000) // 1 MB

// ReferrersIndex is the part of v1.ImageIndex the bound needs: the buffered body
// and the unmarshal it guards.
type ReferrersIndex interface {
	RawManifest() ([]byte, error)
	IndexManifest() (*v1.IndexManifest, error)
}

// ReferrersIndexManifest unmarshals a referrers index only after its body fits the limit.
func ReferrersIndexManifest(index ReferrersIndex) (*v1.IndexManifest, error) {
	raw, err := index.RawManifest()
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maxReferrersIndexSize {
		return nil, fmt.Errorf("referrers index size %d exceeds %d", len(raw), maxReferrersIndexSize)
	}
	return index.IndexManifest()
}
