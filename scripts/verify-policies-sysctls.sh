#!/usr/bin/env bash
#
# verify-policies-sysctls.sh
#
# Asserts the restrict-sysctls ValidatingPolicy in the kyverno-policies chart
# renders the PSA safe-sysctl allow-list for each Kubernetes version.
#
# The list comes from the kyverno-policies.allowedSysctls helper, which gates
# names on .Capabilities.KubeVersion. The CLI fixture only covers one render,
# so this renders the chart at each gate boundary and compares the list.
#
# This needs no cluster: `helm template` renders locally.
#
# Usage: scripts/verify-policies-sysctls.sh

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHART_DIR="${ROOT_DIR}/charts/kyverno-policies"
TEMPLATE="templates/baseline/restrict-sysctls.cel.yaml"
HELM="${HELM:-helm}"

# PSA sysctlsAllowedV1Dot29. The chart never allows fewer than these.
BASE=(
  kernel.shm_rmid_forced
  net.ipv4.ip_local_port_range
  net.ipv4.tcp_syncookies
  net.ipv4.ping_group_range
  net.ipv4.ip_unprivileged_port_start
  net.ipv4.ip_local_reserved_ports
  net.ipv4.tcp_keepalive_time
  net.ipv4.tcp_fin_timeout
  net.ipv4.tcp_keepalive_intvl
  net.ipv4.tcp_keepalive_probes
)
# Added in PSA sysctlsAllowedV1Dot32.
V1DOT32=(net.ipv4.tcp_rmem net.ipv4.tcp_wmem)
# Added in PSA sysctlsAllowedV1Dot37.
V1DOT37=(net.ipv4.tcp_slow_start_after_idle net.ipv4.tcp_notsent_lowat)

log() { echo "[verify-policies-sysctls] $*"; }
fail() { echo "[verify-policies-sysctls] FAIL: $*" >&2; exit 1; }

# render prints the allowed sysctl names for one Kubernetes version, one per line.
render() {
  "${HELM}" template verify "${CHART_DIR}" \
    --kube-version "$1" \
    --set policyType=ValidatingPolicy \
    --show-only "${TEMPLATE}" |
    grep 'sysctls.all(' |
    grep -o "'[a-z0-9_.]*'" |
    tr -d "'"
}

# check compares the rendered list for a version with the expected names.
check() {
  local version="$1"
  shift
  local want got
  want="$(printf '%s\n' "$@" | sort)"
  got="$(render "${version}" | sort)"
  if [ "${got}" != "${want}" ]; then
    diff <(echo "${want}") <(echo "${got}") >&2 || true
    fail "Kubernetes ${version}: rendered allow-list differs from the PSA list (diff above: < expected, > rendered)"
  fi
  log "  OK   ${version}: $# sysctls"
}

log "rendering ${CHART_DIR}/${TEMPLATE}"
check v1.25.0 "${BASE[@]}"
check v1.31.0 "${BASE[@]}"
check v1.32.0 "${BASE[@]}" "${V1DOT32[@]}"
check v1.36.0 "${BASE[@]}" "${V1DOT32[@]}"
check v1.37.0 "${BASE[@]}" "${V1DOT32[@]}" "${V1DOT37[@]}"
log "restrict-sysctls allow-list matches PSA at every version gate"
