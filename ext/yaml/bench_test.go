package yaml

import (
	"fmt"
	"strings"
	"testing"
)

var singleDocYAML = []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  namespace: default
data:
  key: value
`)

var multiDocYAML = []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: first
  namespace: default
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: second
  namespace: default
---
apiVersion: v1
kind: Namespace
metadata:
  name: my-namespace
`)

func largeMultiDocYAML(n int) []byte {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteString("---\n")
		}
		fmt.Fprintf(&sb, "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm-%d\n  namespace: default\n", i)
	}
	return []byte(sb.String())
}

func BenchmarkSplitDocuments_Single(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = SplitDocuments(singleDocYAML)
	}
}

func BenchmarkSplitDocuments_Three(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = SplitDocuments(multiDocYAML)
	}
}

func BenchmarkSplitDocuments_Hundred(b *testing.B) {
	data := largeMultiDocYAML(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = SplitDocuments(data)
	}
}

func BenchmarkIsEmptyDocument_Empty(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IsEmptyDocument([]byte("# just a comment\n"))
	}
}

func BenchmarkIsEmptyDocument_NotEmpty(b *testing.B) {
	doc := []byte("apiVersion: v1\nkind: ConfigMap\n")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsEmptyDocument(doc)
	}
}
