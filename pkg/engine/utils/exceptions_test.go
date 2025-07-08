package utils

import "testing"

func TestMatchesResource(t *testing.T) {
    tests := []struct {
        kinds       []string
        reqKind     string
        subresource string
        want        bool
    }{
        // exact subresource match
        {[]string{"Pod/exec"}, "Pod", "exec", true},
        // wrong subresource
        {[]string{"Pod/exec"}, "Pod", "attach", false},
        // base-kind match on any subresource
        {[]string{"Pod"}, "Pod", "exec", true},
        // base-kind match on no subresource
        {[]string{"Pod"}, "Pod", "", true},
        // unrelated kind
        {[]string{"Service"}, "Pod", "exec", false},
    }

    for _, tt := range tests {
        got := matchesResource(tt.kinds, tt.reqKind, tt.subresource)
        if got != tt.want {
            t.Errorf("matchesResource(%v, %q, %q) = %v; want %v",
                tt.kinds, tt.reqKind, tt.subresource, got, tt.want)
        }
    }
}