package pss

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"

	corev1 "k8s.io/api/core/v1"

	fuzz "github.com/AdamKorcz/go-fuzz-headers-1"
	"golang.org/x/exp/slices"
)

var (
	allowedCapabilities = []corev1.Capability{"AUDIT_WRITE",
		"CHOWN",
		"DAC_OVERRIDE",
		"FOWNER",
		"FSETID",
		"KILL",
		"MKNOD",
		"NET_BIND_SERVICE",
		"SETFCAP",
		"SETGID",
		"SETPCAP",
		"SETUID",
		"SYS_CHROOT"}
	allowedSELinuxTypes = []string{"container_t",
		"container_init_t",
		"container_kvm_t",
		""}
	allowed_sysctls = []string{
		"kernel.shm_rmid_forced",
		"net.ipv4.ip_local_port_range",
		"net.ipv4.ip_unprivileged_port_start",
		"net.ipv4.tcp_syncookies",
		"net.ipv4.ping_group_range",
	}
	baselineV126Policy = []byte(`
		{
			"level": "baseline",
			"version": "v1.26"
		}`)
	baselineLatestPolicy = []byte(`
		{
			"level": "baseline",
			"version": "latest"
		}`)
)

func shouldBlockSELinuxUser(opts *corev1.SELinuxOptions) bool {
	if opts == nil {
		return false
	}

	fieldName := "User"
	value := reflect.ValueOf(opts)
	field := value.Elem().FieldByName(fieldName)

	if field.IsValid() {
		seLinuxUser := opts.User
		if seLinuxUser != "" {
			return true
		}
	}
	return false
}

func shouldBlockSELinuxRole(opts *corev1.SELinuxOptions) bool {
	if opts == nil {
		return false
	}

	fieldName := "Role"
	value := reflect.ValueOf(opts)
	field := value.Elem().FieldByName(fieldName)

	if field.IsValid() {
		seLinuxUser := opts.Role
		if seLinuxUser != "" {
			return true
		}
	}
	return false
}

func shouldAllowBaseline(pod *corev1.Pod) (bool, error) {

	spec := pod.Spec

	if len(spec.Volumes) > 0 {
		volumes := spec.Volumes
		for _, volume := range volumes {
			if volume.HostPath != nil {
				return false, nil
			}
		}
	}

	if len(pod.ObjectMeta.Annotations) > 0 {
		annotations := pod.ObjectMeta.Annotations
		for k, v := range annotations {
			if strings.HasPrefix(k, "container.apparmor.security.beta.kubernetes.io/") {
				if v != "runtime/default" && !strings.HasPrefix(v, "localhost/") {
					return false, nil
				}
			}
		}
	}

	if spec.SecurityContext != nil {
		sc := spec.SecurityContext

		if sc.WindowsOptions != nil {
			if sc.WindowsOptions.HostProcess != nil {
				if *sc.WindowsOptions.HostProcess == true {
					return false, nil
				}
			}
		}

		if shouldBlockContainerSELinux(sc.SELinuxOptions) {
			return false, nil
		}

		if sc.SeccompProfile != nil {
			seccompType := sc.SeccompProfile.Type
			defaultSeccomp := corev1.SeccompProfileTypeRuntimeDefault
			localhostSeccomp := corev1.SeccompProfileTypeLocalhost
			if seccompType != defaultSeccomp && seccompType != localhostSeccomp {
				return false, nil
			}
		}

		fieldName := "Sysctls"
		value := reflect.ValueOf(sc)
		field := value.Elem().FieldByName(fieldName)

		if field.IsValid() {
			for _, sysctl := range sc.Sysctls {
				if !slices.Contains(allowed_sysctls, sysctl.Name) {
					return false, nil
				}
			}
		}
	}

	if pod.Spec.Containers != nil || len(pod.Spec.Containers) != 0 {

		containers := pod.Spec.Containers
		for _, container := range containers {

			if container.SecurityContext != nil {
				if shouldBlockContainerSecurityContext(container.SecurityContext) {
					return false, nil
				}
			}

			fieldName := "Ports"
			value := reflect.ValueOf(container)
			field := value.FieldByName(fieldName)
			if field.IsValid() {
				if shouldBlockContainerPorts(container.Ports) {
					return false, nil
				}
			}
		}
	}

	if pod.Spec.InitContainers != nil || len(pod.Spec.InitContainers) != 0 {

		containers := pod.Spec.InitContainers
		for _, container := range containers {

			if container.SecurityContext != nil {
				if shouldBlockContainerSecurityContext(container.SecurityContext) {
					return false, nil
				}
			}

			fieldName := "Ports"
			value := reflect.ValueOf(container)
			field := value.FieldByName(fieldName)
			if field.IsValid() {
				if shouldBlockContainerPorts(container.Ports) {
					return false, nil
				}
			}
		}
	}

	if pod.Spec.EphemeralContainers != nil || len(pod.Spec.EphemeralContainers) != 0 {
		containers := pod.Spec.EphemeralContainers
		for _, container := range containers {

			if container.SecurityContext != nil {
				if shouldBlockContainerSecurityContext(container.SecurityContext) {
					return false, nil
				}
			}

			fieldName := "Ports"
			value := reflect.ValueOf(container)
			field := value.FieldByName(fieldName)
			if field.IsValid() {
				if shouldBlockContainerPorts(container.Ports) {
					return false, nil
				}
			}
		}
	}

	if spec.SecurityContext != nil {
		fieldName := "HostNetwork"
		value := reflect.ValueOf(spec)
		field := value.FieldByName(fieldName)

		if field.IsValid() {
			if spec.HostNetwork == true {
				return false, nil
			}
		}

		fieldName = "HostPID"
		field = value.FieldByName(fieldName)

		if field.IsValid() {
			if spec.HostPID == true {
				return false, nil
			}
		}

		fieldName = "HostIPC"
		field = value.FieldByName(fieldName)

		if field.IsValid() {
			if spec.HostIPC == true {
				return false, nil
			}
		}
	}
	return true, nil
}

func shouldBlockContainerSecurityContext(sc *corev1.SecurityContext) bool {
	if sc.WindowsOptions != nil {
		if sc.WindowsOptions.HostProcess != nil {
			if *sc.WindowsOptions.HostProcess == true {
				return true
			}
		}
	}

	if sc.Privileged != nil {
		if *sc.Privileged == true {
			return true
		}
	}

	if sc.Capabilities != nil {
		capabilities := sc.Capabilities

		if shouldBlockBaselineCapabilities(capabilities) {
			return true
		}
	}

	if sc.SELinuxOptions != nil {
		seLinuxOptions := sc.SELinuxOptions
		if shouldBlockContainerSELinux(seLinuxOptions) {
			return true
		}
	}

	if sc.ProcMount != nil {
		if *sc.ProcMount != corev1.DefaultProcMount {
			return true
		}
	}

	if sc.SeccompProfile != nil {
		seccompType := sc.SeccompProfile.Type
		defaultSeccomp := corev1.SeccompProfileTypeRuntimeDefault
		localhostSeccomp := corev1.SeccompProfileTypeLocalhost
		if seccompType != defaultSeccomp && seccompType != localhostSeccomp {
			return true
		}
	}

	return false
}

func shouldBlockContainerSELinux(opts *corev1.SELinuxOptions) bool {
	if opts == nil {
		return false
	}

	fieldName := "Type"
	value := reflect.ValueOf(opts)
	field := value.Elem().FieldByName(fieldName)

	if field.IsValid() {
		seLinuxType := opts.Type
		if !slices.Contains(allowedSELinuxTypes, seLinuxType) {
			return true
		}
	}

	if shouldBlockSELinuxUser(opts) {
		return true
	}

	if shouldBlockSELinuxRole(opts) {
		return true
	}

	return false
}

func shouldBlockContainerPorts(ports []corev1.ContainerPort) bool {
	if len(ports) > 0 {
		for _, port := range ports {

			fieldName := "HostPort"
			value := reflect.ValueOf(port)
			field := value.FieldByName(fieldName)

			if field.IsValid() {
				if port.HostPort != 0 {
					return true
				}
			}
		}
	}
	return false
}

func shouldBlockBaselineCapabilities(capabilities *corev1.Capabilities) bool {
	fieldName := "Add"
	value := reflect.ValueOf(capabilities)
	field := value.Elem().FieldByName(fieldName)

	if field.IsValid() {
		if len(capabilities.Add) > 0 {
			for _, capability := range capabilities.Add {
				if !slices.Contains(allowedCapabilities, capability) {
					return true
				}
			}
		}
	}
	return false
}

func getPod(ff *fuzz.ConsumeFuzzer) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	err := ff.GenerateStruct(pod)
	pod.Kind = "Pod"
	pod.APIVersion = "v1"
	return pod, err
}

var (
	baselineV124Rule, baselineLatestRule kyvernov1.PodSecurity
)

func init() {
	err := json.Unmarshal(baselineV126Policy, &baselineV124Rule)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(baselineLatestPolicy, &baselineLatestRule)
	if err != nil {
		panic(err)
	}
}

func FuzzBaselinePS(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)

		pod, err := getPod(ff)
		if err != nil {
			return
		}

		if len(pod.ObjectMeta.Annotations) > 0 {
			for k, v := range pod.ObjectMeta.Annotations {
				for _, r := range k {
					if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && (r != '-' && r != '/' && r != '_' && r != ',' && r != '.') {
						return
					}
				}
				for _, r := range v {
					if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && (r != '-' && r != '/' && r != '_' && r != ',' && r != '.') {
						return
					}
				}
			}
		}

		var allowPod bool
		allowPod, _ = shouldAllowBaseline(pod)
		if allowPod {
			return
		}

		policyToCheck, err := ff.GetInt()
		if err != nil {
			return
		}

		var rule kyvernov1.PodSecurity

		switch policyToCheck % 2 {
		case 0:
			rule = baselineV124Rule
		case 1:
			rule = baselineLatestRule
		}

		allowed, _, _ := EvaluatePod(&rule, pod)
		if allowPod != allowed {
			pJson, err := json.MarshalIndent(pod, "", "")
			if err != nil {
				panic(err)
			}
			fmt.Println(string(pJson))
			fmt.Println("policyToCheck: ", policyToCheck%2)
			fmt.Println("allowed: ", allowed, "allowPod: ", allowPod)
			panic("They don't correlate")
		}
	})
}
