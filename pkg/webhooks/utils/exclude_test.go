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
		want: false,
	}, {
		name: "PolicyReport",
		args: args{"PolicyReport"},
		want: false,
	}, {
		name: "AdmissionReport",
		args: args{"AdmissionReport"},
		want: true,
	}, {
		name: "BackgroundScanReport",
		args: args{"BackgroundScanReport"},
		want: true,
	}, {
		name: "ClusterAdmissionReport",
		args: args{"ClusterAdmissionReport"},
		want: true,
	}, {
		name: "ClusterBackgroundScanReport",
		args: args{"ClusterBackgroundScanReport"},
		want: true,
	}, {
		name: "EphemeralReport",
		args: args{"EphemeralReport"},
		want: true,
	}, {
		name: "ClusterEphemeralReport",
		args: args{"ClusterEphemeralReport"},
		want: true,
	}, {
		name: "UpdateRequest",
		args: args{"UpdateRequest"},
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

func TestExcludeKyvernoResource(t *testing.T) {
	type args struct {
		group string
		kind  string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "Policy - kyverno.io",
		args: args{"kyverno.io", "Policy"},
		want: false,
	}, {
		name: "ClusterPolicy - kyverno.io",
		args: args{"kyverno.io", "ClusterPolicy"},
		want: false,
	}, {
		name: "ClusterPolicyReport - wgpolicyk8s.io",
		args: args{"wgpolicyk8s.io", "ClusterPolicyReport"},
		want: false,
	}, {
		name: "PolicyReport - wgpolicyk8s.io",
		args: args{"wgpolicyk8s.io", "PolicyReport"},
		want: false,
	}, {
		name: "AdmissionReport - reports.kyverno.io",
		args: args{"reports.kyverno.io", "AdmissionReport"},
		want: true,
	}, {
		name: "AdmissionReport - kyverno.io",
		args: args{"kyverno.io", "AdmissionReport"},
		want: true,
	}, {
		name: "BackgroundScanReport - reports.kyverno.io",
		args: args{"reports.kyverno.io", "BackgroundScanReport"},
		want: true,
	}, {
		name: "BackgroundScanReport - kyverno.io",
		args: args{"kyverno.io", "BackgroundScanReport"},
		want: true,
	}, {
		name: "ClusterAdmissionReport - reports.kyverno.io",
		args: args{"reports.kyverno.io", "ClusterAdmissionReport"},
		want: true,
	}, {
		name: "ClusterAdmissionReport - kyverno.io",
		args: args{"kyverno.io", "ClusterAdmissionReport"},
		want: true,
	}, {
		name: "ClusterBackgroundScanReport - reports.kyverno.io",
		args: args{"reports.kyverno.io", "ClusterBackgroundScanReport"},
		want: true,
	}, {
		name: "ClusterBackgroundScanReport - kyverno.io",
		args: args{"kyverno.io", "ClusterBackgroundScanReport"},
		want: true,
	}, {
		name: "EphemeralReport - reports.kyverno.io",
		args: args{"reports.kyverno.io", "EphemeralReport"},
		want: true,
	}, {
		name: "ClusterEphemeralReport - reports.kyverno.io",
		args: args{"reports.kyverno.io", "ClusterEphemeralReport"},
		want: true,
	}, {
		name: "UpdateRequest - kyverno.io",
		args: args{"kyverno.io", "UpdateRequest"},
		want: true,
	}, {
		name: "EphemeralReport - foreign group",
		args: args{"example.com", "EphemeralReport"},
		want: false,
	}, {
		name: "ClusterEphemeralReport - foreign group",
		args: args{"example.com", "ClusterEphemeralReport"},
		want: false,
	}, {
		name: "UpdateRequest - foreign group",
		args: args{"example.com", "UpdateRequest"},
		want: false,
	}, {
		name: "AdmissionReport - foreign group",
		args: args{"example.com", "AdmissionReport"},
		want: false,
	}, {
		name: "Pod - core",
		args: args{"", "Pod"},
		want: false,
	}, {
		name: "Job - batch",
		args: args{"batch", "Job"},
		want: false,
	}, {
		name: "Deployment - apps",
		args: args{"apps", "Deployment"},
		want: false,
	}, {
		name: "empty",
		args: args{"", ""},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExcludeKyvernoResource(tt.args.group, tt.args.kind)
			assert.Equal(t, tt.want, got)
		})
	}
}
