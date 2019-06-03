package main

import (
	"fmt"
	"go/build"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const (
	pkgGocheck      = "github.com/go-check/check"
	pkgGopkgGocheck = "gopkg.in/check.v1"
)

var allTestingTPkgs = append(
	allTestifyPks,
	pkgGocheck,
	pkgGopkgGocheck,
)

func newFakeImporter() (*fakeImporter, error) {
	tmpDir, err := ioutil.TempDir("", "gty-migrate-from-testify")
	err = errors.Wrapf(err, "failed to create temporary directory")
	return &fakeImporter{tmpDir: tmpDir}, err
}

type fakeImporter struct {
	tmpDir string
}

func (f *fakeImporter) Cleanup() error {
	return os.RemoveAll(f.tmpDir)
}

func (f *fakeImporter) Import(
	ctx *build.Context,
	path string,
	dir string,
	mode build.ImportMode,
) (*build.Package, error) {
	pkg, err := ctx.Import(path, dir, mode)
	if err == nil {
		return pkg, err
	}

	for _, pkgName := range allTestingTPkgs {
		if pkgName == path {
			return importStubPackage(f.tmpDir, pkgName)
		}
	}

	return pkg, err
}

func importStubPackage(tmpDir string, importPath string) (*build.Package, error) {
	pkgName := strings.TrimSuffix(path.Base(importPath), ".v1")
	pkgFilePath := filepath.Join(tmpDir, pkgName)

	if err := os.MkdirAll(pkgFilePath, 0755); err != nil {
		return nil, errors.Wrapf(err, "failed to create stub package directory")
	}

	const filename = "fixture.go"
	if err := writeStubFile(filepath.Join(pkgFilePath, filename), pkgName); err != nil {
		return nil, errors.Wrapf(err, "failed to write stub file")
	}

	return &build.Package{
		Dir:        pkgFilePath,
		Name:       pkgName,
		ImportPath: importPath,
		GoFiles:    []string{filename},
	}, nil
}

func writeStubFile(path string, pkgName string) error {
	content := []byte(fmt.Sprintf(stubFixtureContent, pkgName))
	return ioutil.WriteFile(path, content, 0644)
}

const stubFixtureContent = `
package %s

type TestingT interface {
	Errorf(format string, args ...interface{})
	FailNow()
}

func New(t TestingT) *Assertions {
	return &Assertions{}
}

type Assertions struct {}

type C struct{}

`
