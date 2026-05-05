package common

import (
	"errors"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
)

type errorFile struct {
	billy.File
}

func (errorFile) Read([]byte) (int, error) {
	return 0, errors.New("read error")
}

type errorFS struct {
	billy.Filesystem
}

func (fs errorFS) Open(filename string) (billy.File, error) {
	file, err := fs.Filesystem.Open(filename)
	if err != nil {
		return nil, err
	}
	return errorFile{File: file}, nil
}

func TestReadResourceBytes_ReadError(t *testing.T) {
	baseFS := memfs.New()
	file, err := baseFS.Create("resource.yaml")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := file.Write([]byte("apiVersion: v1\nkind: ConfigMap\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	fs := errorFS{Filesystem: baseFS}
	_, err = readResourceBytes(fs, "resource.yaml")
	if err == nil {
		t.Fatalf("expected readResourceBytes to return an error")
	}
	if errors.Is(err, errOpenResourceFile) {
		t.Fatalf("expected a read error, got open error: %v", err)
	}
}
