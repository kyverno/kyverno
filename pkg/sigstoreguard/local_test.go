package sigstoreguard

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestLocalTUFFetcherConfinement(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "root.json"), []byte("root"), 0o600))
	mirror := &url.URL{Scheme: "file", Path: dir}
	fetch := localFetcher(context.Background(), mirror)
	data, err := fetch.DownloadFile(mirror.String()+"/root.json", 4, 0)
	require.NoError(t, err)
	require.Equal(t, []byte("root"), data)
	_, err = fetch.DownloadFile(mirror.String()+"/root.json", 3, 0)
	var lengthError *metadata.ErrDownloadLengthMismatch
	require.ErrorAs(t, err, &lengthError)
	_, err = fetch.DownloadFile(mirror.String()+"/../secret", 100, 0)
	require.ErrorContains(t, err, "escapes")
	_, err = fetch.DownloadFile("http://169.254.169.254/root.json", 100, 0)
	require.ErrorContains(t, err, "configured file mirror")
	outside := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "escape")))
	_, err = fetch.DownloadFile(mirror.String()+"/escape", 100, 0)
	require.Error(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = localFetcher(ctx, mirror).DownloadFile(mirror.String()+"/root.json", 100, 0)
	require.ErrorIs(t, err, context.Canceled)
}
