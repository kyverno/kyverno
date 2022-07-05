package pss

import (
	"fmt"
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

	allowed, err := ExemptProfile(podSecurityRule, podSpec, nil)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func newRule() *v1.PodSecurity {
	return &v1.PodSecurity{
		Level:   api.LevelRestricted,
		Version: api.LatestVersion(),
		Exclude: []*v1.PodSecurityStandard{
			{
				RestrictedField: "containers[*].securityContext.capabilities.add",
				Images:          []string{"ghcr.io/example/nginx:1.2.3"},
				Values:          []string{"SETGID"},
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

// Volume Type
func newVolumeTypePodSpec() *corev1.PodSpec {
	fakeTrue := true
	fakeFalse := false

	podSpec := &corev1.PodSpec{
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
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "cephfs",
						MountPath: "/mnt/cephfs",
					},
					{
						Name:      "hostPath",
						MountPath: "/mnt/hostPath",
					},
				},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "cephfs",
				VolumeSource: corev1.VolumeSource{
					CephFS: &corev1.CephFSVolumeSource{},
				},
			},
			{
				Name: "hostPath",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{},
				},
			},
		},
	}
	return podSpec
}

func newVolumeTypeRule() *v1.PodSecurity {
	return &v1.PodSecurity{
		Level:   api.LevelRestricted,
		Version: api.LatestVersion(),
		Exclude: []*v1.PodSecurityStandard{
			{
				RestrictedField: "volumes[*]",
				Images:          []string{"ghcr.io/example/nginx:1.2.3"},
				Values:          []string{"cephfs", "hostPath"},
			},
		},
	}
}

func Test_EvaluateVolumeType(t *testing.T) {
	podSecurityRule := newVolumeTypeRule()

	lv := api.LevelVersion{
		Level:   podSecurityRule.Level,
		Version: podSecurityRule.Version,
	}

	podMeta := &metav1.ObjectMeta{
		Name:      "test",
		Namespace: "test-namespace",
	}

	podSpec := newVolumeTypePodSpec()

	res := EvaluatePSS(lv, podMeta, podSpec)
	fmt.Println("res: ", res)
	assert.True(t, len(res) == 2, res)

	allowed, err := ExemptProfile(podSecurityRule, podSpec, nil)

	fmt.Println("allowed: ", allowed)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

// App Armor
func newAppArmorPodSpec() *corev1.PodSpec {
	fakeTrue := true
	fakeFalse := false

	podSpec := &corev1.PodSpec{
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
					},
				},
			},
		},
	}
	return podSpec
}

func newAppArmorPodObjectMeta() *metav1.ObjectMeta {
	objectMeta := &metav1.ObjectMeta{
		Name:      "test",
		Namespace: "test-namespace",
		Annotations: map[string]string{
			`container.apparmor.security.beta.kubernetes.io/`:  `bogus`,
			`container.apparmor.security.beta.kubernetes.io/a`: ``,
			`container.apparmor.security.beta.kubernetes.io/b`: `runtime/default`,
			`container.apparmor.security.beta.kubernetes.io/c`: `localhost/`,
			`container.apparmor.security.beta.kubernetes.io/d`: `localhost/foo`,
			"container.apparmor.security.beta.kubernetes.io/e": "unconfined",
			`container.apparmor.security.beta.kubernetes.io/f`: `unknown`,
		},
	}
	return objectMeta
}

func newAppArmorRule() *v1.PodSecurity {
	return &v1.PodSecurity{
		Level:   api.LevelRestricted,
		Version: api.LatestVersion(),
		Exclude: []*v1.PodSecurityStandard{
			{
				RestrictedField: "metadata.annotations[\"container.apparmor.security.beta.kubernetes.io/*\"]",
				Images:          []string{"ghcr.io/example/nginx:1.2.3"},
				Values:          []string{},
			},
		},
	}
}

func Test_EvaluateAppArmor(t *testing.T) {
	podSecurityRule := newAppArmorRule()

	lv := api.LevelVersion{
		Level:   podSecurityRule.Level,
		Version: podSecurityRule.Version,
	}

	podObjectMeta := newAppArmorPodObjectMeta()
	podSpec := newAppArmorPodSpec()

	res := EvaluatePSS(lv, podObjectMeta, podSpec)
	fmt.Println("res: ", res)
	assert.True(t, len(res) == 1, res)

	// allowed, err := ExemptProfile(podSecurityRule, podSpec, podObjectMeta)

	// fmt.Println("allowed: ", allowed)
	// assert.NoError(t, err)
	// assert.True(t, allowed)
}
