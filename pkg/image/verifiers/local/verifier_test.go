package local

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/image/verifiers"
)

func TestProviderAddGetHas(t *testing.T) {
	p := NewProvider()
	if p.Has("img", "type") {
		t.Fatal("expected no attestations for empty provider")
	}
	p.Add("img", "type", map[string]any{"bomFormat": "CycloneDX"})
	if !p.Has("img", "type") {
		t.Fatal("expected attestation to exist")
	}
	statements, ok := p.Get("img", "type")
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %v (ok=%v)", statements, ok)
	}
	s := statements[0]
	if s["type"] != "type" || s["predicateType"] != "type" {
		t.Fatalf("unexpected statement type fields: %v", s)
	}
	predicate, ok := s["predicate"].(map[string]any)
	if !ok || predicate["bomFormat"] != "CycloneDX" {
		t.Fatalf("unexpected predicate: %v", s["predicate"])
	}
}

func TestVerifierFetchAttestations(t *testing.T) {
	p := NewProvider()
	p.Add("img", "type", map[string]any{"k": "v"})
	v := NewVerifier(p)

	resp, err := v.FetchAttestations(context.Background(), verifiers.Options{ImageRef: "img", Type: "type"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Statements) != 1 {
		t.Fatalf("expected one statement, got %d", len(resp.Statements))
	}

	if _, err := v.FetchAttestations(context.Background(), verifiers.Options{ImageRef: "img", Type: "other"}); err == nil {
		t.Fatal("expected error for missing attestations")
	}

	if _, err := v.VerifySignature(context.Background(), verifiers.Options{ImageRef: "img"}); err == nil {
		t.Fatal("expected VerifySignature to be unsupported")
	}
}

func TestProviderContext(t *testing.T) {
	if _, ok := ProviderFromContext(context.Background()); ok {
		t.Fatal("expected no provider in empty context")
	}
	p := NewProvider()
	ctx := WithProvider(context.Background(), p)
	got, ok := ProviderFromContext(ctx)
	if !ok || got != p {
		t.Fatalf("expected provider from context, got %v (ok=%v)", got, ok)
	}
}
