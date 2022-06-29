package pss

import (
	"testing"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/pod-security-admission/api"
)

func Test_EvaluatePSS(t *testing.T) {
	podSecurityRule := newRule()

	lv := api.LevelVersion{
		Level:   podSecurityRule.Level,
		Version: podSecurityRule.Version,
	}

	podMeta := &metav1.ObjectMeta{
		Name:      "test",
		Namespace: "test-namespace",
	}

	podSpec := newPodSpec()

	res := EvaluatePSS(lv, podMeta, podSpec)
	assert.True(t, len(res) == 1, res)

	allowed, err := ExemptProfile(podSecurityRule, podSpec)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func newRule() *v1.PodSecurity {
	return &v1.PodSecurity{
		Level:   api.LevelRestricted,
		Version: api.LatestVersion(),
		Exclude: []*v1.PodSecurityStandard{
			{
				Path:   "containers[*].securityContext.capabilities.add",
				Images: []string{"ghcr.io/example/nginx:1.2.3"},
				Values: []string{"SETGID"},
			},
		},
	}
}

func newPodSpec() *corev1.PodSpec {
	fakeTrue := true
	fakeFalse := false
	// hostPathType := corev1.HostPathDirectory

	podSepc := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "test-container",
				Image: "ghcr.io/example/nginx:1.2.3",
				SecurityContext: &corev1.SecurityContext{
					RunAsNonRoot:             &fakeTrue,
					AllowPrivilegeEscalation: &fakeFalse,
					SeccompProfile:           &corev1.SeccompProfile{Type: "Localhost"},

					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
						Add: []corev1.Capability{
							"SETGID",
							// "SETUID",
						},
					},
				},
			},
		},

		// Volumes: []corev1.Volume{
		// 	{
		// 		Name: "test",
		// 		VolumeSource: corev1.VolumeSource{
		// 			HostPath: &corev1.HostPathVolumeSource{
		// 				Path: "/tmp",
		// 				Type: &hostPathType,
		// 			},
		// 		},
		// 	},
		// },
	}
	return podSepc
}
