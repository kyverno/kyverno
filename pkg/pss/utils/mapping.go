package utils

var PSS_baseline_control_names = []string{
	"HostProcess",
	"Host Namespaces",
	"Privileged Containers",
	"Capabilities",
	"HostPath Volumes",
	"Host Ports",
	"AppArmor",
	"SELinux",
	"/proc Mount Type",
	"Seccomp",
	"Sysctls",
}

var PSS_restricted_control_names = []string{
	"Volume Types",
	"Privilege Escalation",
	"Running as Non-root",
	"Running as Non-root user",
	"Seccomp",
	"Capabilities",
}

var PSS_pod_level_control = []string{
	"Host Namespaces",
	"HostPath Volumes",
	"Sysctls",
	"AppArmor",
	"Volume Types",
}

var PSS_container_level_control = []string{
	"Capabilities",
	"Privileged Containers",
	"Host Ports",
	"/proc Mount Type",
	"Privilege Escalation",
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
	// Container-level control
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

var PSS_controls = map[string][]RestrictedField{
	// Control name as key, same as ID field in CheckResult

	// === Baseline
	// Container-level controls
	"privileged": {
		{
			// type:
			// - container-level
			// - pod-container-level
			// - pod level
			Path: "spec.containers[*].securityContext.privileged",
			AllowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			Path: "spec.initContainers[*].securityContext.privileged",
			AllowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			Path: "spec.ephemeralContainers[*].securityContext.privileged",
			AllowedValues: []interface{}{
				false,
				nil,
			},
		},
	},
	"hostPorts": {
		{
			Path: "spec.containers[*].ports[*].hostPort",
			AllowedValues: []interface{}{
				false,
				0,
			},
		},
		{
			Path: "spec.initContainers[*].ports[*].hostPort",
			AllowedValues: []interface{}{
				false,
				0,
			},
		},
		{
			Path: "spec.ephemeralContainers[*].ports[*].hostPort",
			AllowedValues: []interface{}{
				false,
				0,
			},
		},
	},
	"procMount": {
		{
			Path: "spec.containers[*].securityContext.procMount",
			AllowedValues: []interface{}{
				nil,
				"Default",
			},
		},
		{
			Path: "spec.initContainers[*].securityContext.procMount",
			AllowedValues: []interface{}{
				nil,
				"Default",
			},
		},
		{
			Path: "spec.ephemeralContainers[*].securityContext.procMount",
			AllowedValues: []interface{}{
				nil,
				"Default",
			},
		},
	},
	"capabilities_baseline": {
		{
			Path: "spec.containers[*].securityContext.capabilities.add",
			AllowedValues: []interface{}{
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
			Path: "spec.initContainers[*].securityContext.capabilities.add",
			AllowedValues: []interface{}{
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
			Path: "spec.ephemeralContainers[*].securityContext.capabilities.add",
			AllowedValues: []interface{}{
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
			Path: "spec.securityContext.windowsOptions.hostProcess",
			AllowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			Path: "spec.containers[*].securityContext.windowsOptions.hostProcess",
			AllowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			Path: "spec.initContainers[*].securityContext.windowsOptions.hostProcess",
			AllowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			Path: "spec.ephemeralContainers[*].securityContext.windowsOptions.hostProcess",
			AllowedValues: []interface{}{
				false,
				nil,
			},
		},
	},
	"seLinuxOptions": {
		// type
		{
			Path: "spec.securityContext.seLinuxOptions.type",
			AllowedValues: []interface{}{
				"",
				"container_t",
				"container_init_t",
				"container_kvm_t",
			},
		},
		{
			Path: "spec.containers[*].securityContext.seLinuxOptions.type",
			AllowedValues: []interface{}{
				"",
				"container_t",
				"container_init_t",
				"container_kvm_t",
			},
		},
		{
			Path: "spec.initContainers[*].securityContext.seLinuxOptions.type",
			AllowedValues: []interface{}{
				"",
				"container_t",
				"container_init_t",
				"container_kvm_t",
			},
		},
		{
			Path: "spec.ephemeralContainers[*].securityContext.seLinuxOptions.type",
			AllowedValues: []interface{}{
				"",
				"container_t",
				"container_init_t",
				"container_kvm_t",
			},
		},

		// user
		{
			Path: "spec.securityContext.seLinuxOptions.user",
			AllowedValues: []interface{}{
				"",
			},
		},
		{
			Path: "spec.containers[*].securityContext.seLinuxOptions.user",
			AllowedValues: []interface{}{
				"",
			},
		},
		{
			Path: "spec.initContainers[*].securityContext.seLinuxOptions.user",
			AllowedValues: []interface{}{
				"",
			},
		},
		{
			Path: "spec.ephemeralContainers[*].seLinuxOptions.user",
			AllowedValues: []interface{}{
				"",
			},
		},

		// role
		{
			Path: "spec.securityContext.seLinuxOptions.role",
			AllowedValues: []interface{}{
				"",
			},
		},
		{
			Path: "spec.containers[*].securityContext.seLinuxOptions.role",
			AllowedValues: []interface{}{
				"",
			},
		},
		{
			Path: "spec.initContainers[*].securityContext.seLinuxOptions.role",
			AllowedValues: []interface{}{
				"",
			},
		},
		{
			Path: "spec.ephemeralContainers[*].seLinuxOptions.role",
			AllowedValues: []interface{}{
				"",
			},
		},
	},
	"seccompProfile_baseline": {
		{
			Path: "spec.securityContext.seccompProfile.type",
			AllowedValues: []interface{}{
				nil,
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			Path: "spec.containers[*].securityContext.seccompProfile.type",
			AllowedValues: []interface{}{
				nil,
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			Path: "spec.initContainers[*].securityContext.seccompProfile.type",
			AllowedValues: []interface{}{
				nil,
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			Path: "spec.ephemeralContainers[*].securityContext.seccompProfile.type",
			AllowedValues: []interface{}{
				nil,
				"RuntimeDefault",
				"Localhost",
			},
		},
	},
	"seccompProfile_restricted": {
		{
			Path: "spec.securityContext.seccompProfile.type",
			AllowedValues: []interface{}{
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			Path: "spec.containers[*].securityContext.seccompProfile.type",
			AllowedValues: []interface{}{
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			Path: "spec.initContainers[*].securityContext.seccompProfile.type",
			AllowedValues: []interface{}{
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			Path: "spec.ephemeralContainers[*].securityContext.seccompProfile.type",
			AllowedValues: []interface{}{
				"RuntimeDefault",
				"Localhost",
			},
		},
	},

	// Pod-level controls
	"sysctls": {
		{
			Path: "spec.securityContext.sysctls[*].name",
			AllowedValues: []interface{}{
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
			Path: "spec.volumes[*].hostPath",
			AllowedValues: []interface{}{
				nil,
			},
		},
	},
	"hostNamespaces": {
		{
			Path: "spec.hostNetwork",
			AllowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			Path: "spec.hostPID",
			AllowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			Path: "spec.hostIPC",
			AllowedValues: []interface{}{
				false,
				nil,
			},
		},
	},

	// metadata-level controls
	"appArmorProfile": {
		{
			Path: "metadata.annotations",
			AllowedValues: []interface{}{
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
			Path: "spec.volumes[*]",
			AllowedValues: []interface{}{
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
			Path: "spec.containers[*].securityContext.runAsNonRoot",
			AllowedValues: []interface{}{
				true,
				nil,
			},
		},
		{
			Path: "spec.initContainers[*].securityContext.runAsNonRoot",
			AllowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			Path: "spec.ephemeralContainers[*].securityContext.runAsNonRoot",
			AllowedValues: []interface{}{
				false,
				nil,
			},
		},
	},
	"runAsUser": {
		{
			Path: "spec.securityContext.runAsUser",
			AllowedValues: []interface{}{
				"",
				nil,
			},
		},
		{
			Path: "spec.containers[*].securityContext.runAsUser",
			AllowedValues: []interface{}{
				"",
				nil,
			},
		},
		{
			Path: "spec.initContainers[*].securityContext.runAsUser",
			AllowedValues: []interface{}{
				"",
				nil,
			},
		},
		{
			Path: "spec.ephemeralContainers[*].securityContext.runAsUser",
			AllowedValues: []interface{}{
				"",
				nil,
			},
		},
	},
	"allowPrivilegeEscalation": {
		{
			Path: "spec.containers[*].securityContext.allowPrivilegeEscalation",
			AllowedValues: []interface{}{
				false,
			},
		},
		{
			Path: "spec.initContainers[*].securityContext.allowPrivilegeEscalation",
			AllowedValues: []interface{}{
				false,
			},
		},
		{
			Path: "spec.ephemeralContainers[*].securityContext.allowPrivilegeEscalation",
			AllowedValues: []interface{}{
				false,
			},
		},
	},
	"capabilities_restricted": {
		{
			Path: "spec.containers[*].securityContext.capabilities.drop",
			AllowedValues: []interface{}{
				"ALL",
			},
		},
		{
			Path: "spec.initContainers[*].securityContext.capabilities.drop",
			AllowedValues: []interface{}{
				"ALL",
			},
		},
		{
			Path: "spec.ephemeralContainers[*].securityContext.capabilities.drop",
			AllowedValues: []interface{}{
				"ALL",
			},
		},
		{
			Path: "spec.containers[*].securityContext.capabilities.add",
			AllowedValues: []interface{}{
				nil,
				"NET_BIND_SERVICE",
			},
		},
		{
			Path: "spec.initContainers[*].securityContext.capabilities.add",
			AllowedValues: []interface{}{
				nil,
				"NET_BIND_SERVICE",
			},
		},
		{
			Path: "spec.ephemeralContainers[*].securityContext.capabilities.add",
			AllowedValues: []interface{}{
				nil,
				"NET_BIND_SERVICE",
			},
		},
	},
}
