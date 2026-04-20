package generate

import (
	"errors"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

func TestNewGenerateResponse(t *testing.T) {
	data := map[string]interface{}{"key": "value"}
	target := kyvernov1.ResourceSpec{Kind: "ConfigMap", Name: "test"}
	testErr := errors.New("test error")

	resp := newGenerateResponse(data, Create, target, testErr)

	if resp.GetAction() != Create {
		t.Errorf("GetAction() = %v, want %v", resp.GetAction(), Create)
	}
	if resp.GetTarget().Kind != "ConfigMap" {
		t.Errorf("GetTarget().Kind = %v, want ConfigMap", resp.GetTarget().Kind)
	}
	if resp.GetError() != testErr {
		t.Errorf("GetError() = %v, want %v", resp.GetError(), testErr)
	}
	if resp.GetData()["key"] != "value" {
		t.Errorf("GetData()[key] = %v, want value", resp.GetData()["key"])
	}
}

func TestNewSkipGenerateResponse(t *testing.T) {
	target := kyvernov1.ResourceSpec{Kind: "Secret", Name: "skip-test"}
	resp := newSkipGenerateResponse(nil, target, nil)

	if resp.GetAction() != Skip {
		t.Errorf("GetAction() = %v, want %v", resp.GetAction(), Skip)
	}
}

func TestNewUpdateGenerateResponse(t *testing.T) {
	target := kyvernov1.ResourceSpec{Kind: "ConfigMap", Name: "update-test"}
	resp := newUpdateGenerateResponse(nil, target, nil)

	if resp.GetAction() != Update {
		t.Errorf("GetAction() = %v, want %v", resp.GetAction(), Update)
	}
}

func TestNewCreateGenerateResponse(t *testing.T) {
	target := kyvernov1.ResourceSpec{Kind: "ConfigMap", Name: "create-test"}
	resp := newCreateGenerateResponse(nil, target, nil)

	if resp.GetAction() != Create {
		t.Errorf("GetAction() = %v, want %v", resp.GetAction(), Create)
	}
}

func TestResourceModeConstants(t *testing.T) {
	tests := []struct {
		mode resourceMode
		want string
	}{
		{Skip, "SKIP"},
		{Create, "CREATE"},
		{Update, "UPDATE"},
	}
	for _, tt := range tests {
		if string(tt.mode) != tt.want {
			t.Errorf("resourceMode = %v, want %v", tt.mode, tt.want)
		}
	}
}
