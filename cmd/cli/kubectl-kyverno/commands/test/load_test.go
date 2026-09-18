package test

import (
	"strings"
	"testing"
)

// A git URL with a host but no path (e.g. "http://github.com") must not panic
// loadTest with a slice-bounds error on gitURL.Path[1:]; it should return the
// invalid-URL-path error instead.
func Test_loadTest_URLWithoutPath(t *testing.T) {
	_, err := loadTest("http://github.com", "kyverno-test.yaml", "")
	if err == nil || !strings.Contains(err.Error(), "invalid URL path") {
		t.Fatalf("expected an invalid URL path error, got: %v", err)
	}
}
