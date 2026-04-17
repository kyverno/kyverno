package trivy

# Ignore vulnerabilities where the affected package is kyverno itself at a
# v0.0.0-<date> pseudo-version. This happens because Trivy scans the kyverno
# binary, finds its embedded Go module info reporting version v0.0.0-<date>
# (main branch dev builds are not tagged), and flags all known kyverno CVEs
# since v0.0.0 is semantically older than any release.
ignore[input.ID] {
	input.PkgName == "kyverno"
	startswith(input.InstalledVersion, "v0.0.0-")
}
