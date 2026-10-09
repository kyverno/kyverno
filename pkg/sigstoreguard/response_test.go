package sigstoreguard

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoteTrustMaterialSizeBoundaries(t *testing.T) {
	for _, test := range []struct {
		name      string
		size      int64
		oversized bool
	}{
		{"empty", 0, false},
		{"below_limit", maxRemoteTrustMaterialBytes - 1, false},
		{"exact_limit", maxRemoteTrustMaterialBytes, false},
		{"over_limit", maxRemoteTrustMaterialBytes + 1024, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := &trustResponseBody{remaining: test.size}
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: body, Request: req, ContentLength: -1}, nil
			})}
			data, err := loadFileOrURL(context.Background(), "https://keys.example/root.json", client)
			require.True(t, body.closed)
			if test.oversized {
				require.ErrorContains(t, err, "response exceeds 16777216 bytes")
				require.Nil(t, data)
				require.EqualValues(t, maxRemoteTrustMaterialBytes+1, body.read)
			} else {
				require.NoError(t, err)
				require.Len(t, data, int(test.size))
				require.Equal(t, test.size, body.read)
			}
		})
	}
}

func TestRemoteTrustMaterialPreservesReadErrors(t *testing.T) {
	failure := errors.New("connection interrupted")
	body := &trustResponseBody{remaining: 32, failure: failure}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: body, Request: req}, nil
	})}
	data, err := loadFileOrURL(context.Background(), "https://keys.example/root.json", client)
	require.ErrorIs(t, err, failure)
	require.Nil(t, data)
	require.True(t, body.closed)
	require.EqualValues(t, 32, body.read)
}

func TestRemoteTrustMaterialBoundsRedirectResponse(t *testing.T) {
	body := &trustResponseBody{remaining: maxRemoteTrustMaterialBytes + 1024}
	var destinations []string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		destinations = append(destinations, req.URL.String())
		if req.URL.Path == "/redirect" {
			return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://mirror.example/root.json"}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: body, Request: req, ContentLength: -1}, nil
	})}
	data, err := loadFileOrURL(context.Background(), "https://keys.example/redirect", client)
	require.ErrorContains(t, err, "response exceeds 16777216 bytes")
	require.Nil(t, data)
	require.Equal(t, []string{"https://keys.example/redirect", "https://mirror.example/root.json"}, destinations)
	require.EqualValues(t, maxRemoteTrustMaterialBytes+1, body.read)
	require.True(t, body.closed)
}

func TestRemoteTrustLimitPreservesLocalAndEnvironmentReferences(t *testing.T) {
	contents := strings.Repeat("x", maxRemoteTrustMaterialBytes+1)
	file := filepath.Join(t.TempDir(), "root.pem")
	require.NoError(t, os.WriteFile(file, []byte(contents), 0o600))
	t.Setenv("KYVERNO_TEST_LARGE_TRUST", contents)
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("local trust reference attempted HTTP")
		return nil, errors.New("unexpected HTTP request")
	})}
	for _, ref := range []string{file, "env://KYVERNO_TEST_LARGE_TRUST"} {
		data, err := loadFileOrURL(context.Background(), ref, client)
		require.NoError(t, err)
		require.Equal(t, contents, string(data))
	}
}

type trustResponseBody struct {
	remaining int64
	read      int64
	failure   error
	closed    bool
}

func (b *trustResponseBody) Read(p []byte) (int, error) {
	if b.remaining == 0 {
		if b.failure != nil {
			return 0, b.failure
		}
		return 0, io.EOF
	}
	n := min(int64(len(p)), b.remaining)
	clear(p[:n])
	b.remaining -= n
	b.read += n
	return int(n), nil
}

func (b *trustResponseBody) Close() error {
	b.closed = true
	return nil
}
