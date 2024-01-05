package pluralize

import "testing"

func TestPluralize(t *testing.T) {
	tests := []struct {
		name     string
		number   int
		singular string
		plural   string
		want     string
	}{{
		name:     "singular",
		number:   1,
		singular: "policy",
		plural:   "policies",
		want:     "policy",
	}, {
		name:     "plural",
		number:   2,
		singular: "policy",
		plural:   "policies",
		want:     "policies",
	}, {
		name:     "zero",
		number:   0,
		singular: "policy",
		plural:   "policies",
		want:     "policies",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Pluralize(tt.number, tt.singular, tt.plural); got != tt.want {
				t.Errorf("Pluralize() = %v, want %v", got, tt.want)
			}
		})
	}
}
