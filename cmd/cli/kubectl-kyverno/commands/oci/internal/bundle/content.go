package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"sort"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
)

// epoch is the fixed mtime, both for tar entries and the gzip header, that makes the content
// layer's digest reproducible across runs of the same writer against the same input
// (bundle-spec.md section 5, "Determinism").
var epoch = time.Unix(0, 0).UTC()

// buildContentLayer archives files (bundle-root-relative path -> original bytes) into the
// deterministic tar+gzip content layer: entries sorted by path, regular files only, mode 0644,
// uid and gid 0, empty uname and gname, mtime the Unix epoch, gzip with a zeroed header
// timestamp and no stored filename.
func buildContentLayer(files map[string][]byte) (v1.Layer, error) {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for _, p := range paths {
		content := files[p]
		hdr := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     p,
			Size:     int64(len(content)),
			Mode:     0o644,
			Uid:      0,
			Gid:      0,
			Uname:    "",
			Gname:    "",
			ModTime:  epoch,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("writing tar header for %s: %w", p, err)
		}
		if _, err := tw.Write(content); err != nil {
			return nil, fmt.Errorf("writing tar content for %s: %w", p, err)
		}
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("closing tar archive: %w", err)
	}

	var gzBuf bytes.Buffer
	// A zero-value gzip.Writer.Header (unset Name, zero ModTime) writes MTIME 0 and no FNAME.
	gw := gzip.NewWriter(&gzBuf)
	if _, err := gw.Write(tarBuf.Bytes()); err != nil {
		return nil, fmt.Errorf("compressing content layer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("closing gzip writer: %w", err)
	}

	return static.NewLayer(gzBuf.Bytes(), types.MediaType(internal.ContentMediaType)), nil
}
