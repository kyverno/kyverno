package anchor

import "testing"

func TestRemoveAnchorsFromPath(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{{
		str:  "/path/(to)/X(anchors)",
		want: "/path/to/anchors",
	}, {
		str:  "path/(to)/X(anchors)",
		want: "path/to/anchors",
	}, {
		str:  "../(to)/X(anchors)",
		want: "../to/anchors",
	}, {
		str:  "/path/(to)/X(anchors)",
		want: "/path/to/anchors",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RemoveAnchorsFromPath(tt.str); got != tt.want {
				t.Errorf("RemoveAnchorsFromPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
