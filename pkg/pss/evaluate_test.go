package pss

import (
	"fmt"
	"testing"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/pod-security-admission/api"
	utilpointer "k8s.io/utils/pointer"
)

// Baseline
// Host Process
// https://github.com/kubernetes/pod-security-admission/blob/master/policy/check_windowsHostProcess_test.go
func Test_EvaluateHostProcess(t *testing.T) {
	fmt.Println("===========")
	podSecurityRule := newHostProcessRule()

	lv := api.LevelVersion{
		Level:   podSecurityRule.Level,
		Version: podSecurityRule.Version,
	}

	podMeta := &metav1.ObjectMeta{
		Name:      "test",
		Namespace: "test-namespace",
	}

	podSpec := newHostProcessPodSpec()

	res := EvaluatePSS(lv, podMeta, podSpec)
	assert.True(t, len(res) == 1, res)

	allowed, err := ExemptProfile(podSecurityRule, podSpec, nil)
	assert.NoError(t, err)
	assert.True(t, allowed)
	fmt.Println("===========")
}

func newHostProcessRule() *v1.PodSecurity {
	return &v1.PodSecurity{
		Level:   api.LevelRestricted,
		Version: api.LatestVersion(),
		Exclude: []*v1.PodSecurityStandard{
			{
				// spec.containers[*].securityContext.windowsOptions.hostProcess
				RestrictedField: "containers[*].securityContext.windowsOptions.hostProcess",
				Images:          []string{"ghcr.io/example/nginx:1.2.3"},
				Values:          []string{"true"},
			},
		},
	}
}

func newHostProcessPodSpec() *corev1.PodSpec {
	fakeTrue := true
	fakeFalse := false

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
					},
					WindowsOptions: &corev1.WindowsSecurityContextOptions{
						HostProcess: utilpointer.Bool(true),
					},
				},
			},
		},
	}
	return podSepc
}

// Host Namespaces
// https://github.com/kubernetes/pod-security-admission/blob/master/policy/check_hostNamespaces_test.go
func Test_EvaluateHostNamespaces(t *testing.T) {
	fmt.Println("===========")
	podSecurityRule := newHostNamespacesRule()

	lv := api.LevelVersion{
		Level:   podSecurityRule.Level,
		Version: podSecurityRule.Version,
	}

	podMeta := &metav1.ObjectMeta{
		Name:      "test",
		Namespace: "test-namespace",
	}

	podSpec := newHostNamespacesPodSpec()

	res := EvaluatePSS(lv, podMeta, podSpec)
	assert.True(t, len(res) == 1, res)

	allowed, err := ExemptProfile(podSecurityRule, podSpec, nil)
	assert.NoError(t, err)
	assert.True(t, allowed)
	fmt.Println("===========")
}

func newHostNamespacesRule() *v1.PodSecurity {
	return &v1.PodSecurity{
		Level:   api.LevelRestricted,
		Version: api.LatestVersion(),
		Exclude: []*v1.PodSecurityStandard{
			{
				// spec.hostNetwork
				RestrictedField: "hostNetwork",
				Images:          []string{"ghcr.io/example/nginx:1.2.3"},
				Values:          []string{"true"},
			},
		},
	}
}

func newHostNamespacesPodSpec() *corev1.PodSpec {
	fakeTrue := true
	fakeFalse := false

	podSepc := &corev1.PodSpec{
		HostNetwork: true,
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
	return podSepc
}

// Privileged Containers
// https://github.com/kubernetes/pod-security-admission/blob/master/policy/check_privileged_test.go
func Test_EvaluatePrivilegedContainers(t *testing.T) {
	fmt.Println("===========")
	podSecurityRule := newPrivilegedContainersRule()

	lv := api.LevelVersion{
		Level:   podSecurityRule.Level,
		Version: podSecurityRule.Version,
	}

	podMeta := &metav1.ObjectMeta{
		Name:      "test",
		Namespace: "test-namespace",
	}

	podSpec := newPrivilegedContainersPodSpec()

	res := EvaluatePSS(lv, podMeta, podSpec)
	assert.True(t, len(res) == 1, res)

	allowed, err := ExemptProfile(podSecurityRule, podSpec, nil)
	assert.NoError(t, err)
	assert.True(t, allowed)
	fmt.Println("===========")
}

func newPrivilegedContainersRule() *v1.PodSecurity {
	return &v1.PodSecurity{
		Level:   api.LevelRestricted,
		Version: api.LatestVersion(),
		Exclude: []*v1.PodSecurityStandard{
			{
				// spec.containers[*].securityContext.privileged
				RestrictedField: "containers[*].securityContext.privileged",
				Images:          []string{"ghcr.io/example/nginx:1.2.3"},
				Values:          []string{"true"},
			},
		},
	}
}

func newPrivilegedContainersPodSpec() *corev1.PodSpec {
	fakeTrue := true
	fakeFalse := false

	podSepc := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "test-container",
				Image: "ghcr.io/example/nginx:1.2.3",
				SecurityContext: &corev1.SecurityContext{
					Privileged:               utilpointer.Bool(true),
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
	return podSepc
}

// HostPath Volumes
// https://github.com/kubernetes/pod-security-admission/blob/master/policy/check_hostPathVolumes_test.go
func Test_EvaluateHostPathVolumes(t *testing.T) {
	fmt.Println("===========")
	podSecurityRule := newHostPathVolumesRule()

	lv := api.LevelVersion{
		Level:   podSecurityRule.Level,
		Version: podSecurityRule.Version,
	}

	podMeta := &metav1.ObjectMeta{
		Name:      "test",
		Namespace: "test-namespace",
	}

	podSpec := newHostPathVolumesPodSpec()

	res := EvaluatePSS(lv, podMeta, podSpec)
	assert.True(t, len(res) == 2, res)

	allowed, err := ExemptProfile(podSecurityRule, podSpec, nil)
	assert.NoError(t, err)
	assert.True(t, allowed)
	fmt.Println("===========")
}

func newHostPathVolumesRule() *v1.PodSecurity {
	return &v1.PodSecurity{
		Level:   api.LevelRestricted,
		Version: api.LatestVersion(),
		Exclude: []*v1.PodSecurityStandard{
			{
				// spec.volumes[*].hostPath
				RestrictedField: "volumes[*]",
				Images:          []string{"ghcr.io/example/nginx:1.2.3"},
				Values:          []string{"hostPath"},
			},
		},
	}
}

func newHostPathVolumesPodSpec() *corev1.PodSpec {
	fakeTrue := true
	fakeFalse := false

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
					},
				},
			},
		},
		Volumes: []corev1.Volume{
			{Name: "a", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{}}},
			{Name: "b", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{}}},
			// {Name: "c", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		},
	}
	return podSepc
}

// Error: panic: interface conversion: interface {} is float64, not string [recovered]
// panic: interface conversion: interface {} is float64, not string

// Host Ports
// https://github.com/kubernetes/pod-security-admission/blob/master/policy/check_hostPorts_test.go
func Test_EvaluateHostPorts(t *testing.T) {
	fmt.Println("===========")
	podSecurityRule := newHostPortsRule()

	lv := api.LevelVersion{
		Level:   podSecurityRule.Level,
		Version: podSecurityRule.Version,
	}

	podMeta := &metav1.ObjectMeta{
		Name:      "test",
		Namespace: "test-namespace",
	}

	podSpec := newHostPortsPodSpec()

	res := EvaluatePSS(lv, podMeta, podSpec)
	assert.True(t, len(res) == 1, res)

	allowed, err := ExemptProfile(podSecurityRule, podSpec, nil)
	assert.NoError(t, err)
	assert.True(t, allowed)
	fmt.Println("===========")
}

func newHostPortsRule() *v1.PodSecurity {
	return &v1.PodSecurity{
		Level:   api.LevelRestricted,
		Version: api.LatestVersion(),
		Exclude: []*v1.PodSecurityStandard{
			{
				// spec.volumes[*].hostPort
				RestrictedField: "containers[*].ports[*].hostPort",
				Images:          []string{"ghcr.io/example/nginx:1.2.3"},
				Values:          []string{"10", "20"},
			},
		},
	}
}

func newHostPortsPodSpec() *corev1.PodSpec {
	fakeTrue := true
	fakeFalse := false

	podSepc := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Ports: []corev1.ContainerPort{
					// {HostPort: 0},
					{HostPort: 10},
					{HostPort: 20},
				},
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
	return podSepc
}

// SELinux
//https://github.com/kubernetes/pod-security-admission/blob/master/policy/check_seLinuxOptions_test.go
func Test_EvaluateSELinux(t *testing.T) {
	fmt.Println("===========")
	podSecurityRule := newSELinuxRule()

	lv := api.LevelVersion{
		Level:   podSecurityRule.Level,
		Version: podSecurityRule.Version,
	}

	podMeta := &metav1.ObjectMeta{
		Name:      "test",
		Namespace: "test-namespace",
	}

	podSpec := newSELinuxPodSpec()

	res := EvaluatePSS(lv, podMeta, podSpec)
	assert.True(t, len(res) == 1, res)

	allowed, err := ExemptProfile(podSecurityRule, podSpec, nil)
	assert.NoError(t, err)
	assert.True(t, allowed)
	fmt.Println("===========")
}

func newSELinuxRule() *v1.PodSecurity {
	return &v1.PodSecurity{
		Level:   api.LevelRestricted,
		Version: api.LatestVersion(),
		Exclude: []*v1.PodSecurityStandard{
			{
				// spec.containers[*].securityContext.seLinuxOptions.type
				RestrictedField: "containers[*].securityContext.seLinuxOptions.type",
				Images:          []string{"ghcr.io/example/nginx:1.2.3"},
				Values:          []string{"bar"},
			},
		},
	}
}

func newSELinuxPodSpec() *corev1.PodSpec {
	fakeTrue := true
	fakeFalse := false

	podSepc := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "test-container",
				Image: "ghcr.io/example/nginx:1.2.3",
				SecurityContext: &corev1.SecurityContext{
					SELinuxOptions: &corev1.SELinuxOptions{
						// Type: "container_t",
						Type: "bar",
					},
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
	return podSepc
}

// /proc Mount Type
//https://github.com/kubernetes/pod-security-admission/blob/master/policy/check_seLinuxOptions_test.go
func Test_EvaluateProcMountType(t *testing.T) {
	fmt.Println("===========")
	podSecurityRule := newProcMountTypeRule()

	lv := api.LevelVersion{
		Level:   podSecurityRule.Level,
		Version: podSecurityRule.Version,
	}

	podMeta := &metav1.ObjectMeta{
		Name:      "test",
		Namespace: "test-namespace",
	}

	podSpec := newProcMountTypePodSpec()

	res := EvaluatePSS(lv, podMeta, podSpec)
	assert.True(t, len(res) == 1, res)

	allowed, err := ExemptProfile(podSecurityRule, podSpec, nil)
	assert.NoError(t, err)
	assert.True(t, allowed)
	fmt.Println("===========")
}

func newProcMountTypeRule() *v1.PodSecurity {
	return &v1.PodSecurity{
		Level:   api.LevelRestricted,
		Version: api.LatestVersion(),
		Exclude: []*v1.PodSecurityStandard{
			{
				// spec.containers[*].securityContext.procMount
				RestrictedField: "containers[*].securityContext.procMount",
				Images:          []string{"ghcr.io/example/nginx:1.2.3"},
				Values:          []string{"Unmasked"},
			},
		},
	}
}

func newProcMountTypePodSpec() *corev1.PodSpec {
	fakeTrue := true
	fakeFalse := false
	// defaultValue := corev1.DefaultProcMount
	unmaskedValue := corev1.UnmaskedProcMount

	podSepc := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "test-container",
				Image: "ghcr.io/example/nginx:1.2.3",
				SecurityContext: &corev1.SecurityContext{
					// ProcMount:                &defaultValue,
					ProcMount:                &unmaskedValue,
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
	return podSepc
}

// Capabilities
func Test_EvaluatePSS(t *testing.T) {
	fmt.Println("===========")
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
	fmt.Println("===========")
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
	fmt.Println("===========")
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
	fmt.Println("===========")
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
				Values:          []string{""},
			},
		},
	}
}

func Test_EvaluateAppArmor(t *testing.T) {
	fmt.Println("===========")
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

	// JMESPATH problem
	// allowed, err := ExemptProfile(podSecurityRule, podSpec, podObjectMeta)

	// fmt.Println("allowed: ", allowed)
	// assert.NoError(t, err)
	// assert.True(t, allowed)
	// fmt.Println("===========")
}
