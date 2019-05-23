package mergo

import (
	"testing"
)

type DstStructIssue84 struct {
	A int
	B int
	C int
}

type DstNestedStructIssue84 struct {
	A struct {
		A int
		B int
		C int
	}
	B int
	C int
}

func TestIssue84MergeMapWithNilValueToStructWithOverride(t *testing.T) {
	p1 := DstStructIssue84{
		A: 0, B: 1, C: 2,
	}
	p2 := map[string]interface{}{
		"A": 3, "B": 4, "C": 0,
	}
	if err := Map(&p1, p2, WithOverride); err != nil {
		t.Fatalf("Error during the merge: %v", err)
	}
	if p1.C != 0 {
		t.Error("C field should become '0'")
	}
}

func TestIssue84MergeMapWithoutKeyExistsToStructWithOverride(t *testing.T) {
	p1 := DstStructIssue84{
		A: 0, B: 1, C: 2,
	}
	p2 := map[string]interface{}{
		"A": 3, "B": 4,
	}
	if err := Map(&p1, p2, WithOverride); err != nil {
		t.Fatalf("Error during the merge: %v", err)
	}
	if p1.C != 2 {
		t.Error("C field should be '2'")
	}
}

func TestIssue84MergeNestedMapWithNilValueToStructWithOverride(t *testing.T) {
	p1 := DstNestedStructIssue84{
		A: struct {
			A int
			B int
			C int
		}{A: 1, B: 2, C: 0},
		B: 0,
		C: 2,
	}
	p2 := map[string]interface{}{
		"A": map[string]interface{}{
			"A": 0, "B": 0, "C": 5,
		}, "B": 4, "C": 0,
	}
	if err := Map(&p1, p2, WithOverride); err != nil {
		t.Fatalf("Error during the merge: %v", err)
	}
	if p1.B != 4 {
		t.Error("A.C field should become '4'")
	}

	if p1.A.C != 5 {
		t.Error("A.C field should become '5'")
	}

	if p1.A.B != 0 || p1.A.A != 0 {
		t.Error("A.A and A.B field should become '0'")
	}
}
