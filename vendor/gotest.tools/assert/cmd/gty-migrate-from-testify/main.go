package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/imports"
)

type options struct {
	pkgs             []string
	dryRun           bool
	debug            bool
	cmpImportName    string
	showLoaderErrors bool
	useAllFiles      bool
}

func main() {
	name := os.Args[0]
	flags, opts := setupFlags(name)
	handleExitError(name, flags.Parse(os.Args[1:]))
	setupLogging(opts)
	opts.pkgs = flags.Args()
	handleExitError(name, run(*opts))
}

func setupLogging(opts *options) {
	log.SetFlags(0)
	enableDebug = opts.debug
}

var enableDebug = false

func debugf(msg string, args ...interface{}) {
	if enableDebug {
		log.Printf("DEBUG: "+msg, args...)
	}
}

func setupFlags(name string) (*pflag.FlagSet, *options) {
	opts := options{}
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.BoolVar(&opts.dryRun, "dry-run", false,
		"don't write changes to file")
	flags.BoolVar(&opts.debug, "debug", false, "enable debug logging")
	flags.StringVar(&opts.cmpImportName, "cmp-pkg-import-alias", "is",
		"import alias to use for the assert/cmp package")
	flags.BoolVar(&opts.showLoaderErrors, "print-loader-errors", false,
		"print errors from loading source")
	flags.BoolVar(&opts.useAllFiles, "ignore-build-tags", false,
		"migrate all files ignoring build tags")
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %s [OPTIONS] PACKAGE [PACKAGE...]

Migrate calls from testify/{assert|require} to gotest.tools/assert.

%s`, name, flags.FlagUsages())
	}
	return flags, &opts
}

func handleExitError(name string, err error) {
	switch {
	case err == nil:
		return
	case err == pflag.ErrHelp:
		os.Exit(0)
	default:
		log.Println(name + ": Error: " + err.Error())
		os.Exit(3)
	}
}

func run(opts options) error {
	program, err := loadProgram(opts)
	if err != nil {
		return errors.Wrapf(err, "failed to load program")
	}

	pkgs := program.InitialPackages()
	debugf("package count: %d", len(pkgs))

	fileset := program.Fset
	for _, pkg := range pkgs {
		for _, astFile := range pkg.Files {
			absFilename := fileset.File(astFile.Pos()).Name()
			filename := relativePath(absFilename)
			importNames := newImportNames(astFile.Imports, opts)
			if !importNames.hasTestifyImports() {
				debugf("skipping file %s, no imports", filename)
				continue
			}

			debugf("migrating %s with imports: %#v", filename, importNames)
			m := migration{
				file:        astFile,
				fileset:     fileset,
				importNames: importNames,
				pkgInfo:     pkg,
			}
			migrateFile(m)
			if opts.dryRun {
				continue
			}

			raw, err := formatFile(m)
			if err != nil {
				return errors.Wrapf(err, "failed to format %s", filename)
			}

			if err := ioutil.WriteFile(absFilename, raw, 0); err != nil {
				return errors.Wrapf(err, "failed to write file %s", filename)
			}
		}
	}

	return nil
}

func loadProgram(opts options) (*loader.Program, error) {
	fakeImporter, err := newFakeImporter()
	if err != nil {
		return nil, err
	}
	defer fakeImporter.Cleanup()

	conf := loader.Config{
		Fset:        token.NewFileSet(),
		ParserMode:  parser.ParseComments,
		Build:       buildContext(opts),
		AllowErrors: true,
		FindPackage: fakeImporter.Import,
	}
	for _, pkg := range opts.pkgs {
		conf.ImportWithTests(pkg)
	}
	if !opts.showLoaderErrors {
		conf.TypeChecker.Error = func(e error) {}
	}
	program, err := conf.Load()
	if opts.showLoaderErrors {
		for p, pkg := range program.AllPackages {
			if len(pkg.Errors) > 0 {
				fmt.Printf("Package %s loaded with some errors:\n", p.Name())
				for _, err := range pkg.Errors {
					fmt.Println("    ", err.Error())
				}
			}
		}
	}
	return program, err
}

func buildContext(opts options) *build.Context {
	c := build.Default
	c.UseAllFiles = opts.useAllFiles
	if val, ok := os.LookupEnv("GOPATH"); ok {
		c.GOPATH = val
	}
	return &c
}

func relativePath(p string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return p
	}
	rel, err := filepath.Rel(cwd, p)
	if err != nil {
		return p
	}
	return rel
}

type importNames struct {
	testifyAssert  string
	testifyRequire string
	assert         string
	cmp            string
}

func (p importNames) hasTestifyImports() bool {
	return p.testifyAssert != "" || p.testifyRequire != ""
}

func (p importNames) matchesTestify(ident *ast.Ident) bool {
	return ident.Name == p.testifyAssert || ident.Name == p.testifyRequire
}

func (p importNames) funcNameFromTestifyName(name string) string {
	switch name {
	case p.testifyAssert:
		return funcNameCheck
	case p.testifyRequire:
		return funcNameAssert
	default:
		panic("unexpected testify import name " + name)
	}
}

func newImportNames(imports []*ast.ImportSpec, opt options) importNames {
	importNames := importNames{
		assert: path.Base(pkgAssert),
		cmp:    path.Base(pkgCmp),
	}
	for _, spec := range imports {
		switch strings.Trim(spec.Path.Value, `"`) {
		case pkgTestifyAssert, pkgGopkgTestifyAssert:
			importNames.testifyAssert = identOrDefault(spec.Name, "assert")
		case pkgTestifyRequire, pkgGopkgTestifyRequire:
			importNames.testifyRequire = identOrDefault(spec.Name, "require")
		default:
			if importedAs(spec, path.Base(pkgAssert)) {
				importNames.assert = "gtyassert"
			}
		}
	}

	if opt.cmpImportName != "" {
		importNames.cmp = opt.cmpImportName
	}
	return importNames
}

func importedAs(spec *ast.ImportSpec, pkg string) bool {
	if path.Base(strings.Trim(spec.Path.Value, `""`)) == pkg {
		return true
	}
	return spec.Name != nil && spec.Name.Name == pkg
}

func identOrDefault(ident *ast.Ident, def string) string {
	if ident != nil {
		return ident.Name
	}
	return def
}

func formatFile(migration migration) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := format.Node(buf, migration.fileset, migration.file)
	if err != nil {
		return nil, err
	}
	filename := migration.fileset.File(migration.file.Pos()).Name()
	return imports.Process(filename, buf.Bytes(), nil)
}
