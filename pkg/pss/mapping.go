package pss

import "k8s.io/pod-security-admission/policy"

type restrictedField struct {
	path          string
	allowedValues []interface{}
}

type pssCheckResult struct {
	id               string
	checkResult      policy.CheckResult
	restrictedFields []restrictedField
}

// Translate PSS control to CheckResult.ID so that we can use PSS control in Kyverno policy
// For PSS controls see: https://kubernetes.io/docs/concepts/security/pod-security-standards/
// For CheckResult.ID see: https://github.com/kubernetes/pod-security-admission/tree/master/policy
var PSS_controls_to_check_id = map[string][]string{
	// Controls with 2 different controls for each level
	// container-level control
	"Capabilities": {
		"capabilities_baseline",
		"capabilities_restricted",
	},
	// Container and Pod-level control
	"Seccomp": {
		"seccompProfile_baseline",
		"seccompProfile_restricted",
	},

	// === Baseline
	// Container-level controls
	"Privileged Containers": {
		"privileged",
	},
	"Host Ports": {
		"hostPorts",
	},
	"/proc Mount Type": {
		"procMount",
	},

	// Container and pod-level controls
	"HostProcess": {
		"windowsHostProcess",
	},
	"SELinux": {
		"seLinuxOptions",
	},

	// Pod-level controls
	"Host Namespaces": {
		"hostNamespaces",
	},
	"HostPath Volumes": {
		"hostPathVolumes",
	},
	"Sysctls": {
		"sysctls",
	},

	// Metadata-level control
	"AppArmor": {
		"appArmorProfile",
	},

	// === Restricted
	"Privilege Escalation": {
		"allowPrivilegeEscalation",
	},
	// Container and pod-level controls
	"Running as Non-root": {
		"runAsNonRoot",
	},
	"Running as Non-root user": {
		"runAsUser",
	},

	// Pod-level controls
	"Volume Types": {
		"restrictedVolumes",
	},
}

var PSS_controls = map[string][]restrictedField{
	// Control name as key, same as ID field in CheckResult

	// === Baseline
	// Container-level controls
	"privileged": {
		{
			// type:
			// - container-level
			// - pod-container-level
			// - pod level
			path: "spec.containers[*].securityContext.privileged",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.initContainers[*].securityContext.privileged",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.privileged",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
	},
	"hostPorts": {
		{
			path: "spec.containers[*].ports[*].hostPort",
			allowedValues: []interface{}{
				false,
				0,
			},
		},
		{
			path: "spec.initContainers[*].ports[*].hostPort",
			allowedValues: []interface{}{
				false,
				0,
			},
		},
		{
			path: "spec.ephemeralContainers[*].ports[*].hostPort",
			allowedValues: []interface{}{
				false,
				0,
			},
		},
	},
	"procMount": {
		{
			path: "spec.containers[*].securityContext.procMount",
			allowedValues: []interface{}{
				nil,
				"Default",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.procMount",
			allowedValues: []interface{}{
				nil,
				"Default",
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.procMount",
			allowedValues: []interface{}{
				nil,
				"Default",
			},
		},
	},
	"capabilities_baseline": {
		{
			path: "spec.containers[*].securityContext.capabilities.add",
			allowedValues: []interface{}{
				nil,
				"AUDIT_WRITE",
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
				"SYS_CHROOT",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.capabilities.add",
			allowedValues: []interface{}{
				nil,
				"AUDIT_WRITE",
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
				"SYS_CHROOT",
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.capabilities.add",
			allowedValues: []interface{}{
				nil,
				"AUDIT_WRITE",
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
				"SYS_CHROOT",
			},
		},
	},

	// Container and pod-level controls
	"windowsHostProcess": {
		{
			path: "spec.securityContext.windowsOptions.hostProcess",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.containers[*].securityContext.windowsOptions.hostProcess",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.initContainers[*].securityContext.windowsOptions.hostProcess",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.windowsOptions.hostProcess",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
	},
	"seLinuxOptions": {
		// type
		{
			path: "spec.securityContext.seLinuxOptions.type",
			allowedValues: []interface{}{
				"",
				"container_t",
				"container_init_t",
				"container_kvm_t",
			},
		},
		{
			path: "spec.containers[*].securityContext.seLinuxOptions.type",
			allowedValues: []interface{}{
				"",
				"container_t",
				"container_init_t",
				"container_kvm_t",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.seLinuxOptions.type",
			allowedValues: []interface{}{
				"",
				"container_t",
				"container_init_t",
				"container_kvm_t",
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.seLinuxOptions.type",
			allowedValues: []interface{}{
				"",
				"container_t",
				"container_init_t",
				"container_kvm_t",
			},
		},

		// user
		{
			path: "spec.securityContext.seLinuxOptions.user",
			allowedValues: []interface{}{
				"",
			},
		},
		{
			path: "spec.containers[*].securityContext.seLinuxOptions.user",
			allowedValues: []interface{}{
				"",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.seLinuxOptions.user",
			allowedValues: []interface{}{
				"",
			},
		},
		{
			path: "spec.ephemeralContainers[*].seLinuxOptions.user",
			allowedValues: []interface{}{
				"",
			},
		},

		// role
		{
			path: "spec.securityContext.seLinuxOptions.role",
			allowedValues: []interface{}{
				"",
			},
		},
		{
			path: "spec.containers[*].securityContext.seLinuxOptions.role",
			allowedValues: []interface{}{
				"",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.seLinuxOptions.role",
			allowedValues: []interface{}{
				"",
			},
		},
		{
			path: "spec.ephemeralContainers[*].seLinuxOptions.role",
			allowedValues: []interface{}{
				"",
			},
		},
	},
	"seccompProfile_baseline": {
		{
			path: "spec.securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				nil,
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			path: "spec.containers[*].securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				nil,
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				nil,
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				nil,
				"RuntimeDefault",
				"Localhost",
			},
		},
	},
	"seccompProfile_restricted": {
		{
			path: "spec.securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			path: "spec.containers[*].securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				"RuntimeDefault",
				"Localhost",
			},
		},
	},

	// Pod-level controls
	"sysctls": {
		{
			path: "spec.securityContext.sysctls[*].name",
			allowedValues: []interface{}{
				"kernel.shm_rmid_forced",
				"net.ipv4.ip_local_port_range",
				"net.ipv4.tcp_syncookies",
				"net.ipv4.ping_group_range",
				"net.ipv4.ip_unprivileged_port_start",
			},
		},
	},
	"hostPathVolumes": {
		{
			path: "spec.volumes[*].hostPath",
			allowedValues: []interface{}{
				nil,
			},
		},
	},
	"hostNamespaces": {
		{
			path: "spec.hostNetwork",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.hostPID",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.hostIPC",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
	},

	// metadata-level controls
	"appArmorProfile": {
		{
			path: "metadata.annotations",
			allowedValues: []interface{}{
				nil,
				"",
				"runtime/default",
				"localhost/*",
			},
		},
	},

	// === Restricted
	"restrictedVolumes": {
		{
			path: "spec.volumes[*]",
			allowedValues: []interface{}{
				"spec.volumes[*].configMap",
				"spec.volumes[*].downwardAPI",
				"spec.volumes[*].emptyDir",
				"spec.volumes[*].projected",
				"spec.volumes[*].secret",
				"spec.volumes[*].csi",
				"spec.volumes[*].persistentVolumeClaim",
				"spec.volumes[*].ephemeral",
			},
		},
	},
	"runAsNonRoot": {
		{
			path: "spec.containers[*].securityContext.runAsNonRoot",
			allowedValues: []interface{}{
				true,
				nil,
			},
		},
		{
			path: "spec.initContainers[*].securityContext.runAsNonRoot",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.runAsNonRoot",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
	},
	"runAsUser": {
		{
			path: "spec.securityContext.runAsUser",
			allowedValues: []interface{}{
				"",
				nil,
			},
		},
		{
			path: "spec.containers[*].securityContext.runAsUser",
			allowedValues: []interface{}{
				"",
				nil,
			},
		},
		{
			path: "spec.initContainers[*].securityContext.runAsUser",
			allowedValues: []interface{}{
				"",
				nil,
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.runAsUser",
			allowedValues: []interface{}{
				"",
				nil,
			},
		},
	},
	"allowPrivilegeEscalation": {
		{
			path: "spec.containers[*].securityContext.allowPrivilegeEscalation",
			allowedValues: []interface{}{
				false,
			},
		},
		{
			path: "spec.initContainers[*].securityContext.allowPrivilegeEscalation",
			allowedValues: []interface{}{
				false,
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.allowPrivilegeEscalation",
			allowedValues: []interface{}{
				false,
			},
		},
	},
	"capabilities_restricted": {
		{
			path: "spec.containers[*].securityContext.capabilities.drop",
			allowedValues: []interface{}{
				"ALL",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.capabilities.drop",
			allowedValues: []interface{}{
				"ALL",
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.capabilities.drop",
			allowedValues: []interface{}{
				"ALL",
			},
		},
		{
			path: "spec.containers[*].securityContext.capabilities.add",
			allowedValues: []interface{}{
				nil,
				"NET_BIND_SERVICE",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.capabilities.add",
			allowedValues: []interface{}{
				nil,
				"NET_BIND_SERVICE",
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.capabilities.add",
			allowedValues: []interface{}{
				nil,
				"NET_BIND_SERVICE",
			},
		},
	},
}
