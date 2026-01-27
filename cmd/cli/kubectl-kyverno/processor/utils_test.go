package processor

import (
	"io"
	"net/http"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	celhttp "github.com/kyverno/kyverno/pkg/cel/libs/http"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
)

func writeFile(t *testing.T, fs billy.Filesystem, path string, contents string) {
	t.Helper()
	f, err := fs.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	_, err = f.Write([]byte(contents))
	if err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestNewContextProvider_LoadsGlobalContextAndHTTPStubs(t *testing.T) {
	fs := memfs.New()
	writeFile(t, fs, "context.yaml", `
apiVersion: cli.kyverno.io/v1alpha1
kind: Context
metadata:
  name: test
spec:
  globalContext:
  - name: corpData
    value:
      foo: bar
  http:
  - method: GET
    url: http://example.test
    status: 200
    body:
      ok: true
`)
	var rm meta.RESTMapper
	cp, err := NewContextProvider(nil, rm, fs, "context.yaml", false, true)
	assert.NoError(t, err)

	got, err := cp.GetGlobalReference("corpData", "")
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"foo": "bar"}, got)

	// verify HTTP stub is used (no network)
	hp, ok := cp.(interface{ HTTPClient() celhttp.ClientInterface })
	assert.True(t, ok)
	req, err := http.NewRequest("GET", "http://example.test", nil)
	assert.NoError(t, err)
	resp, err := hp.HTTPClient().Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(b))
}

func TestNewContextProvider_FileFixturesOverrideInline(t *testing.T) {
	fs := memfs.New()
	writeFile(t, fs, "value.yaml", `
foo: from-file
`)
	writeFile(t, fs, "body.yaml", `
ok: from-file
`)
	writeFile(t, fs, "context.yaml", `
apiVersion: cli.kyverno.io/v1alpha1
kind: Context
metadata:
  name: test
spec:
  globalContext:
  - name: corpData
    value:
      foo: inline
    valueFile: value.yaml
  http:
  - method: GET
    url: http://example.test
    body:
      ok: inline
    bodyFile: body.yaml
`)
	var rm meta.RESTMapper
	cp, err := NewContextProvider(nil, rm, fs, "context.yaml", false, true)
	assert.NoError(t, err)

	got, err := cp.GetGlobalReference("corpData", "")
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"foo": "from-file"}, got)

	hp, ok := cp.(interface{ HTTPClient() celhttp.ClientInterface })
	assert.True(t, ok)
	req, err := http.NewRequest("GET", "http://example.test", nil)
	assert.NoError(t, err)
	resp, err := hp.HTTPClient().Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"ok":"from-file"}`, string(b))
}

