package main

import (
	"go/parser"
	"go/token"
	"testing"

	"golang.org/x/tools/go/loader"
	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

func TestMigrateFileReplacesTestingT(t *testing.T) {
	source := `
package foo

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSomething(t *testing.T) {
	a := assert.TestingT
	b := require.TestingT
	c := require.TestingT(t)
	if a == b {}
}

func do(t require.TestingT) {}
`
	migration := newMigrationFromSource(t, source)
	migrateFile(migration)

	expected := `package foo

import (
	"testing"

	"gotest.tools/assert"
)

func TestSomething(t *testing.T) {
	a := assert.TestingT
	b := assert.TestingT
	c := assert.TestingT(t)
	if a == b {
	}
}

func do(t assert.TestingT) {}
`
	actual, err := formatFile(migration)
	assert.NilError(t, err)
	assert.Assert(t, cmp.Equal(expected, string(actual)))
}

func newMigrationFromSource(t *testing.T, source string) migration {
	fileset := token.NewFileSet()
	nodes, err := parser.ParseFile(
		fileset,
		"foo.go",
		source,
		parser.AllErrors|parser.ParseComments)
	assert.NilError(t, err)

	fakeImporter, err := newFakeImporter()
	assert.NilError(t, err)
	defer fakeImporter.Cleanup()

	opts := options{}
	conf := loader.Config{
		Fset:        fileset,
		ParserMode:  parser.ParseComments,
		Build:       buildContext(opts),
		AllowErrors: true,
		FindPackage: fakeImporter.Import,
	}
	conf.TypeChecker.Error = func(e error) {}
	conf.CreateFromFiles("foo.go", nodes)
	prog, err := conf.Load()
	assert.NilError(t, err)

	pkgInfo := prog.InitialPackages()[0]

	return migration{
		file:        pkgInfo.Files[0],
		fileset:     fileset,
		importNames: newImportNames(nodes.Imports, opts),
		pkgInfo:     pkgInfo,
	}
}

func TestMigrateFileWithNamedCmpPackage(t *testing.T) {
	source := `
package foo

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestSomething(t *testing.T) {
	assert.Equal(t, "a", "b")
}
`
	migration := newMigrationFromSource(t, source)
	migration.importNames.cmp = "is"
	migrateFile(migration)

	expected := `package foo

import (
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestSomething(t *testing.T) {
	assert.Check(t, is.Equal("a", "b"))
}
`
	actual, err := formatFile(migration)
	assert.NilError(t, err)
	assert.Assert(t, cmp.Equal(expected, string(actual)))
}

func TestMigrateFileWithCommentsOnAssert(t *testing.T) {
	source := `
package foo

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestSomething(t *testing.T) {
	// This is going to fail
	assert.Equal(t, "a", "b")
}
`
	migration := newMigrationFromSource(t, source)
	migrateFile(migration)

	expected := `package foo

import (
	"testing"

	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

func TestSomething(t *testing.T) {
	// This is going to fail
	assert.Check(t, cmp.Equal("a", "b"))
}
`
	actual, err := formatFile(migration)
	assert.NilError(t, err)
	assert.Assert(t, cmp.Equal(expected, string(actual)))
}

func TestMigrateFileConvertNilToNilError(t *testing.T) {
	source := `
package foo

import (
	"testing"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/assert"
)

func TestSomething(t *testing.T) {
	var err error
	assert.Nil(t, err)
	require.Nil(t, err)
}
`
	migration := newMigrationFromSource(t, source)
	migrateFile(migration)

	expected := `package foo

import (
	"testing"

	"gotest.tools/assert"
)

func TestSomething(t *testing.T) {
	var err error
	assert.Check(t, err)
	assert.NilError(t, err)
}
`
	actual, err := formatFile(migration)
	assert.NilError(t, err)
	assert.Assert(t, cmp.Equal(expected, string(actual)))
}

func TestMigrateFileConvertAssertNew(t *testing.T) {
	source := `
package foo

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSomething(t *testing.T) {
	is := assert.New(t)
	is.Equal("one", "two")
	is.NotEqual("one", "two")

	assert := require.New(t)
	assert.Equal("one", "two")
	assert.NotEqual("one", "two")
}

func TestOtherName(z *testing.T) {
	is := require.New(z)
	is.Equal("one", "two")
}

`
	migration := newMigrationFromSource(t, source)
	migrateFile(migration)

	expected := `package foo

import (
	"testing"

	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

func TestSomething(t *testing.T) {

	assert.Check(t, cmp.Equal("one", "two"))
	assert.Check(t, "one" != "two")

	assert.Equal(t, "one", "two")
	assert.Assert(t, "one" != "two")
}

func TestOtherName(z *testing.T) {

	assert.Equal(z, "one", "two")
}
`
	actual, err := formatFile(migration)
	assert.NilError(t, err)
	assert.Assert(t, cmp.Equal(expected, string(actual)))
}

func TestMigrateFileWithExtraArgs(t *testing.T) {
	source := `
package foo

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSomething(t *testing.T) {
	var err error
	assert.Error(t, err, "this is a comment")
	assert.Empty(t, nil, "more comment")
	require.Equal(t, []string{}, []string{}, "because")
}
`
	migration := newMigrationFromSource(t, source)
	migration.importNames.cmp = "is"
	migrateFile(migration)

	expected := `package foo

import (
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestSomething(t *testing.T) {
	var err error
	assert.Check(t, is.ErrorContains(err, ""), "this is a comment")
	assert.Check(t, is.Len(nil, 0), "more comment")
	assert.Assert(t, is.DeepEqual([]string{}, []string{}), "because")
}
`
	actual, err := formatFile(migration)
	assert.NilError(t, err)
	assert.Assert(t, cmp.Equal(expected, string(actual)))
}
