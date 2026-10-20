# AGENTS.md — ext/yaml

Splits a multi-document YAML byte stream into individual documents (`SplitDocuments`) and classifies a document as empty/comment-only (`IsEmptyDocument`). `pkg/utils/yaml/loadpolicy.go` uses `SplitDocuments` to load Kyverno policies from multi-document YAML files, so a bug here can silently drop policies with no error surfaced.

## The underlying reader conflates "done" with "broken" — check error before length

`SplitDocuments` reads through `k8s.io/apimachinery/pkg/util/yaml.YAMLReader.Read()`. That reader returns an **empty buffer in two completely different situations**: a clean `io.EOF` (normal end of input) and a real syntax error, e.g. a `---` line followed by anything other than a comment (`--- oops`). Both come back as `(nil, err)` with `err` non-nil in the error case but the buffer empty either way.

Because of this, always check `err != nil && err != io.EOF` **before** the `len(b) == 0` early-return. Checking length first (the original bug, fixed 2026-09) makes a syntax error indistinguishable from EOF — the error gets swallowed, `SplitDocuments` returns `nil` for its own error, and every document already read in that call (including ones parsed successfully before the bad separator) gets discarded along with it.

## A document being read when the error hits is unrecoverable — this is upstream, not fixable here

If a malformed separator appears partway through a document that `YAMLReader` is still accumulating into its internal buffer, that buffer is lost even with the fix above: `YAMLReader.Read()` discovers the syntax error and returns it before ever checking whether its buffer has unread content, so the partially-read document never reaches our code. Only documents from **prior, already-completed** `Read()` calls survive. See `split_test.go`'s `"invalid separator after multiple valid documents"` case — `"first: true"` (a prior completed read) survives, `"second: true"` (being accumulated when `--- oops` hit) does not. Don't try to recover it here; it would need a fix in `k8s.io/apimachinery` itself.
