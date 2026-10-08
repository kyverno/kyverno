package sigstoreguard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/fetcher"
)

// Operator file mirrors are confined to their configured directory, including
// symlinks. Policy mirrors never use this fetcher.
func localFetcher(ctx context.Context, mirror *url.URL) fetcher.Fetcher {
	return localFetchFunc(func(rawURL string, maxLength int64) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, err
		}
		if u.Scheme != "file" || u.Host != mirror.Host || (u.Host != "" && u.Host != "localhost") {
			return nil, fmt.Errorf("local TUF target must remain within the configured file mirror")
		}
		relative, err := filepath.Rel(mirror.Path, u.Path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("local TUF target escapes the configured file mirror")
		}
		directory, err := os.OpenRoot(mirror.Path)
		if err != nil {
			return nil, err
		}
		defer directory.Close()
		file, err := directory.Open(relative)
		if errors.Is(err, os.ErrNotExist) {
			return nil, &metadata.ErrDownloadHTTP{StatusCode: http.StatusNotFound, URL: rawURL}
		}
		if err != nil {
			return nil, err
		}
		defer file.Close()
		if maxLength < 0 || maxLength == int64(^uint64(0)>>1) {
			return nil, fmt.Errorf("invalid TUF target length limit")
		}
		data, err := io.ReadAll(io.LimitReader(file, maxLength+1))
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if int64(len(data)) > maxLength {
			return nil, &metadata.ErrDownloadLengthMismatch{Msg: "local TUF target exceeds its length limit"}
		}
		return data, nil
	})
}

type localFetchFunc func(string, int64) ([]byte, error)

func (f localFetchFunc) DownloadFile(path string, maxLength int64, _ time.Duration) ([]byte, error) {
	return f(path, maxLength)
}
