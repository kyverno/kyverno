// Copyright 2013 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bufio"
	"bytes"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/tools/go/packages/packagestest"
	"golang.org/x/tools/internal/testenv"
)

// buildGodoc builds the godoc executable.
// It returns its path, and a cleanup function.
//
// TODO(adonovan): opt: do this at most once, and do the cleanup
// exactly once.  How though?  There's no atexit.
func buildGodoc(t *testing.T) (bin string, cleanup func()) {
	t.Helper()

	if runtime.GOARCH == "arm" {
		t.Skip("skipping test on arm platforms; too slow")
	}
	if runtime.GOOS == "android" {
		t.Skipf("the dependencies are not available on android")
	}
	testenv.NeedsTool(t, "go")

	tmp, err := ioutil.TempDir("", "godoc-regtest-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cleanup == nil { // probably, go build failed.
			os.RemoveAll(tmp)
		}
	}()

	bin = filepath.Join(tmp, "godoc")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Building godoc: %v", err)
	}

	return bin, func() { os.RemoveAll(tmp) }
}

func serverAddress(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		ln, err = net.Listen("tcp6", "[::1]:0")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func waitForServerReady(t *testing.T, cmd *exec.Cmd, addr string) {
	ch := make(chan error, 1)
	go func() { ch <- fmt.Errorf("server exited early: %v", cmd.Wait()) }()
	go waitForServer(t, ch,
		fmt.Sprintf("http://%v/", addr),
		"The Go Programming Language",
		15*time.Second,
		false)
	if err := <-ch; err != nil {
		t.Fatal(err)
	}
}

func waitForSearchReady(t *testing.T, cmd *exec.Cmd, addr string) {
	ch := make(chan error, 1)
	go func() { ch <- fmt.Errorf("server exited early: %v", cmd.Wait()) }()
	go waitForServer(t, ch,
		fmt.Sprintf("http://%v/search?q=FALLTHROUGH", addr),
		"The list of tokens.",
		2*time.Minute,
		false)
	if err := <-ch; err != nil {
		t.Fatal(err)
	}
}

func waitUntilScanComplete(t *testing.T, addr string) {
	ch := make(chan error)
	go waitForServer(t, ch,
		fmt.Sprintf("http://%v/pkg", addr),
		"Scan is not yet complete",
		2*time.Minute,
		// setting reverse as true, which means this waits
		// until the string is not returned in the response anymore
		true,
	)
	if err := <-ch; err != nil {
		t.Fatal(err)
	}
}

const pollInterval = 200 * time.Millisecond

// waitForServer waits for server to meet the required condition.
// It sends a single error value to ch, unless the test has failed.
// The error value is nil if the required condition was met within
// timeout, or non-nil otherwise.
func waitForServer(t *testing.T, ch chan<- error, url, match string, timeout time.Duration, reverse bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)
		if t.Failed() {
			return
		}
		res, err := http.Get(url)
		if err != nil {
			continue
		}
		body, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || res.StatusCode != http.StatusOK {
			continue
		}
		switch {
		case !reverse && bytes.Contains(body, []byte(match)),
			reverse && !bytes.Contains(body, []byte(match)):
			ch <- nil
			return
		}
	}
	ch <- fmt.Errorf("server failed to respond in %v", timeout)
}

// hasTag checks whether a given release tag is contained in the current version
// of the go binary.
func hasTag(t string) bool {
	for _, v := range build.Default.ReleaseTags {
		if t == v {
			return true
		}
	}
	return false
}

func killAndWait(cmd *exec.Cmd) {
	cmd.Process.Kill()
	cmd.Process.Wait()
}

func TestURL(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on plan9; fails to start up quickly enough")
	}
	bin, cleanup := buildGodoc(t)
	defer cleanup()

	testcase := func(url string, contents string) func(t *testing.T) {
		return func(t *testing.T) {
			stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)

			args := []string{fmt.Sprintf("-url=%s", url)}
			cmd := exec.Command(bin, args...)
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			cmd.Args[0] = "godoc"

			// Set GOPATH variable to non-existing absolute path
			// and GOPROXY=off to disable module fetches.
			// We cannot just unset GOPATH variable because godoc would default it to ~/go.
			// (We don't want the indexer looking at the local workspace during tests.)
			cmd.Env = append(os.Environ(),
				"GOPATH=/does_not_exist",
				"GOPROXY=off",
				"GO111MODULE=off")

			if err := cmd.Run(); err != nil {
				t.Fatalf("failed to run godoc -url=%q: %s\nstderr:\n%s", url, err, stderr)
			}

			if !strings.Contains(stdout.String(), contents) {
				t.Errorf("did not find substring %q in output of godoc -url=%q:\n%s", contents, url, stdout)
			}
		}
	}

	t.Run("index", testcase("/", "Go is an open source programming language"))
	t.Run("fmt", testcase("/pkg/fmt", "Package fmt implements formatted I/O"))
}

// Basic integration test for godoc HTTP interface.
func TestWeb(t *testing.T) {
	bin, cleanup := buildGodoc(t)
	defer cleanup()
	for _, x := range packagestest.All {
		t.Run(x.Name(), func(t *testing.T) {
			testWeb(t, x, bin, false)
		})
	}
}

// Basic integration test for godoc HTTP interface.
func TestWebIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in -short mode")
	}
	bin, cleanup := buildGodoc(t)
	defer cleanup()
	testWeb(t, packagestest.GOPATH, bin, true)
}

// Basic integration test for godoc HTTP interface.
func testWeb(t *testing.T, x packagestest.Exporter, bin string, withIndex bool) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping on plan9; fails to start up quickly enough")
	}

	// Write a fake GOROOT/GOPATH with some third party packages.
	e := packagestest.Export(t, x, []packagestest.Module{
		{
			Name: "godoc.test/repo1",
			Files: map[string]interface{}{
				"a/a.go": `// Package a is a package in godoc.test/repo1.
package a; import _ "godoc.test/repo2/a"; const Name = "repo1a"`,
				"b/b.go": `package b; const Name = "repo1b"`,
			},
		},
		{
			Name: "godoc.test/repo2",
			Files: map[string]interface{}{
				"a/a.go": `package a; const Name = "repo2a"`,
				"b/b.go": `package b; const Name = "repo2b"`,
			},
		},
	})
	defer e.Cleanup()

	// Start the server.
	addr := serverAddress(t)
	args := []string{fmt.Sprintf("-http=%s", addr)}
	if withIndex {
		args = append(args, "-index", "-index_interval=-1s")
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = e.Config.Dir
	cmd.Env = e.Config.Env
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Args[0] = "godoc"

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start godoc: %s", err)
	}
	defer killAndWait(cmd)

	if withIndex {
		waitForSearchReady(t, cmd, addr)
	} else {
		waitForServerReady(t, cmd, addr)
		waitUntilScanComplete(t, addr)
	}

	tests := []struct {
		path        string
		contains    []string // substring
		match       []string // regexp
		notContains []string
		needIndex   bool
		releaseTag  string // optional release tag that must be in go/build.ReleaseTags
	}{
		{
			path:     "/",
			contains: []string{"Go is an open source programming language"},
		},
		{
			path:     "/pkg/fmt/",
			contains: []string{"Package fmt implements formatted I/O"},
		},
		{
			path:     "/src/fmt/",
			contains: []string{"scan_test.go"},
		},
		{
			path:     "/src/fmt/print.go",
			contains: []string{"// Println formats using"},
		},
		{
			path: "/pkg",
			contains: []string{
				"Standard library",
				"Package fmt implements formatted I/O",
				"Third party",
				"Package a is a package in godoc.test/repo1.",
			},
			notContains: []string{
				"internal/syscall",
				"cmd/gc",
			},
		},
		{
			path: "/pkg/?m=all",
			contains: []string{
				"Standard library",
				"Package fmt implements formatted I/O",
				"internal/syscall/?m=all",
			},
			notContains: []string{
				"cmd/gc",
			},
		},
		{
			path: "/search?q=ListenAndServe",
			contains: []string{
				"/src",
			},
			notContains: []string{
				"/pkg/bootstrap",
			},
			needIndex: true,
		},
		{
			path: "/pkg/strings/",
			contains: []string{
				`href="/src/strings/strings.go"`,
			},
		},
		{
			path: "/cmd/compile/internal/amd64/",
			contains: []string{
				`href="/src/cmd/compile/internal/amd64/ssa.go"`,
			},
		},
		{
			path: "/pkg/math/bits/",
			contains: []string{
				`Added in Go 1.9`,
			},
		},
		{
			path: "/pkg/net/",
			contains: []string{
				`// IPv6 scoped addressing zone; added in Go 1.1`,
			},
		},
		{
			path: "/pkg/net/http/httptrace/",
			match: []string{
				`Got1xxResponse.*// Go 1\.11`,
			},
			releaseTag: "go1.11",
		},
		// Verify we don't add version info to a struct field added the same time
		// as the struct itself:
		{
			path: "/pkg/net/http/httptrace/",
			match: []string{
				`(?m)GotFirstResponseByte func\(\)\s*$`,
			},
		},
		// Remove trailing periods before adding semicolons:
		{
			path: "/pkg/database/sql/",
			contains: []string{
				"The number of connections currently in use; added in Go 1.11",
				"The number of idle connections; added in Go 1.11",
			},
			releaseTag: "go1.11",
		},

		// Third party packages.
		{
			path:     "/pkg/godoc.test/repo1/a",
			contains: []string{`const <span id="Name">Name</span> = &#34;repo1a&#34;`},
		},
		{
			path:     "/pkg/godoc.test/repo2/b",
			contains: []string{`const <span id="Name">Name</span> = &#34;repo2b&#34;`},
		},
	}
	for _, test := range tests {
		if test.needIndex && !withIndex {
			continue
		}
		url := fmt.Sprintf("http://%s%s", addr, test.path)
		resp, err := http.Get(url)
		if err != nil {
			t.Errorf("GET %s failed: %s", url, err)
			continue
		}
		body, err := ioutil.ReadAll(resp.Body)
		strBody := string(body)
		resp.Body.Close()
		if err != nil {
			t.Errorf("GET %s: failed to read body: %s (response: %v)", url, err, resp)
		}
		isErr := false
		for _, substr := range test.contains {
			if test.releaseTag != "" && !hasTag(test.releaseTag) {
				continue
			}
			if !bytes.Contains(body, []byte(substr)) {
				t.Errorf("GET %s: wanted substring %q in body", url, substr)
				isErr = true
			}
		}
		for _, re := range test.match {
			if test.releaseTag != "" && !hasTag(test.releaseTag) {
				continue
			}
			if ok, err := regexp.MatchString(re, strBody); !ok || err != nil {
				if err != nil {
					t.Fatalf("Bad regexp %q: %v", re, err)
				}
				t.Errorf("GET %s: wanted to match %s in body", url, re)
				isErr = true
			}
		}
		for _, substr := range test.notContains {
			if bytes.Contains(body, []byte(substr)) {
				t.Errorf("GET %s: didn't want substring %q in body", url, substr)
				isErr = true
			}
		}
		if isErr {
			t.Errorf("GET %s: got:\n%s", url, body)
		}
	}
}

// Basic integration test for godoc -analysis=type (via HTTP interface).
func TestTypeAnalysis(t *testing.T) {
	bin, cleanup := buildGodoc(t)
	defer cleanup()
	testTypeAnalysis(t, packagestest.GOPATH, bin)
	// TODO(golang.org/issue/34473): Add support for type, pointer
	// analysis in module mode, then enable its test coverage here.
}
func testTypeAnalysis(t *testing.T, x packagestest.Exporter, bin string) {
	if runtime.GOOS == "plan9" {
		t.Skip("skipping test on plan9 (issue #11974)") // see comment re: Plan 9 below
	}

	// Write a fake GOROOT/GOPATH.
	// TODO(golang.org/issue/34473): This test uses import paths without a dot in first
	// path element. This is not viable in module mode; import paths will need to change.
	e := packagestest.Export(t, x, []packagestest.Module{
		{
			Name: "app",
			Files: map[string]interface{}{
				"main.go": `
package main
import "lib"
func main() { print(lib.V) }
`,
			},
		},
		{
			Name: "lib",
			Files: map[string]interface{}{
				"lib.go": `
package lib
type T struct{}
const C = 3
var V T
func (T) F() int { return C }
`,
			},
		},
	})
	goroot := filepath.Join(e.Temp(), "goroot")
	if err := os.Mkdir(goroot, 0755); err != nil {
		t.Fatalf("os.Mkdir(%q) failed: %v", goroot, err)
	}
	defer e.Cleanup()

	// Start the server.
	addr := serverAddress(t)
	cmd := exec.Command(bin, fmt.Sprintf("-http=%s", addr), "-analysis=type")
	cmd.Dir = e.Config.Dir
	// Point to an empty GOROOT directory to speed things up
	// by not doing type analysis for the entire real GOROOT.
	// TODO(golang.org/issue/34473): This test optimization may not be viable in module mode.
	cmd.Env = append(e.Config.Env, fmt.Sprintf("GOROOT=%s", goroot))
	cmd.Stdout = os.Stderr
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Args[0] = "godoc"
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start godoc: %s", err)
	}
	defer killAndWait(cmd)
	waitForServerReady(t, cmd, addr)

	// Wait for type analysis to complete.
	reader := bufio.NewReader(stderr)
	for {
		s, err := reader.ReadString('\n') // on Plan 9 this fails
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprint(os.Stderr, s)
		if strings.Contains(s, "Type analysis complete.") {
			break
		}
	}
	go io.Copy(os.Stderr, reader)

	t0 := time.Now()

	// Make an HTTP request and check for a regular expression match.
	// The patterns are very crude checks that basic type information
	// has been annotated onto the source view.
tryagain:
	for _, test := range []struct{ url, pattern string }{
		{"/src/lib/lib.go", "L2.*package .*Package docs for lib.*/lib"},
		{"/src/lib/lib.go", "L3.*type .*type info for T.*struct"},
		{"/src/lib/lib.go", "L5.*var V .*type T struct"},
		{"/src/lib/lib.go", "L6.*func .*type T struct.*T.*return .*const C untyped int.*C"},

		{"/src/app/main.go", "L2.*package .*Package docs for app"},
		{"/src/app/main.go", "L3.*import .*Package docs for lib.*lib"},
		{"/src/app/main.go", "L4.*func main.*package lib.*lib.*var lib.V lib.T.*V"},
	} {
		url := fmt.Sprintf("http://%s%s", addr, test.url)
		resp, err := http.Get(url)
		if err != nil {
			t.Errorf("GET %s failed: %s", url, err)
			continue
		}
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Errorf("GET %s: failed to read body: %s (response: %v)", url, err, resp)
			continue
		}

		if !bytes.Contains(body, []byte("Static analysis features")) {
			// Type analysis results usually become available within
			// ~4ms after godoc startup (for this input on my machine).
			if elapsed := time.Since(t0); elapsed > 500*time.Millisecond {
				t.Fatalf("type analysis results still unavailable after %s", elapsed)
			}
			time.Sleep(10 * time.Millisecond)
			goto tryagain
		}

		match, err := regexp.Match(test.pattern, body)
		if err != nil {
			t.Errorf("regexp.Match(%q) failed: %s", test.pattern, err)
			continue
		}
		if !match {
			// This is a really ugly failure message.
			t.Errorf("GET %s: body doesn't match %q, got:\n%s",
				url, test.pattern, string(body))
		}
	}
}
