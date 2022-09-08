package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExcludeKyvernoResources(t *testing.T) {
	type args struct {
		kind string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "Policy",
		args: args{"Policy"},
		want: false,
	}, {
		name: "ClusterPolicy",
		args: args{"ClusterPolicy"},
		want: false,
	}, {
		name: "ClusterPolicyReport",
		args: args{"ClusterPolicyReport"},
		want: true,
	}, {
		name: "PolicyReport",
		args: args{"PolicyReport"},
		want: true,
	}, {
		name: "ReportChangeRequest",
		args: args{"ReportChangeRequest"},
		want: true,
	}, {
		name: "GenerateRequest",
		args: args{"GenerateRequest"},
		want: true,
	}, {
		name: "ClusterReportChangeRequest",
		args: args{"ClusterReportChangeRequest"},
		want: true,
	}, {
		name: "Pod",
		args: args{"Pod"},
		want: false,
	}, {
		name: "Job",
		args: args{"Job"},
		want: false,
	}, {
		name: "Deployment",
		args: args{"Deployment"},
		want: false,
	}, {
		name: "empty",
		args: args{""},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExcludeKyvernoResources(tt.args.kind)
			assert.Equal(t, tt.want, got)
		})
	}
}
