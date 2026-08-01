package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseMakefile(t *testing.T) {
	const fixture = `
#########
# TOOLS #
#########

install-tools: ## Install tools
	@echo installing

undocumented-target:
	@echo no comment, should be skipped

#################
# BUILD (LOCAL) #
#################

build-all: build-kyverno ## Build all binaries

# a plain comment line, not a hash-delimited header, must not change category
still-build-category: ## Still under BUILD (LOCAL)
`

	dir := t.TempDir()
	path := filepath.Join(dir, "Makefile")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	got, err := parseMakefile(path)
	if err != nil {
		t.Fatalf("parseMakefile: %v", err)
	}

	want := []Task{
		{Target: "install-tools", Description: "Install tools", Category: "TOOLS"},
		{Target: "build-all", Description: "Build all binaries", Category: "BUILD (LOCAL)"},
		{Target: "still-build-category", Description: "Still under BUILD (LOCAL)", Category: "BUILD (LOCAL)"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseMakefile() = %#v, want %#v", got, want)
	}
}

func TestParseMakefileMissingFile(t *testing.T) {
	if _, err := parseMakefile(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
