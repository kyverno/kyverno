package data

import "testing"

func TestDeepEqual(t *testing.T) {
	if got, want := DeepEqual("a", "a"), true; got != want {
		t.Errorf("DeepEqual() = %v, want %v", got, want)
	}
	if got, want := DeepEqual("a", "b"), false; got != want {
		t.Errorf("DeepEqual() = %v, want %v", got, want)
	}
	if got, want := DeepEqual(1, 1), true; got != want {
		t.Errorf("DeepEqual() = %v, want %v", got, want)
	}
	if got, want := DeepEqual(1, 2), false; got != want {
		t.Errorf("DeepEqual() = %v, want %v", got, want)
	}
}
