#!/usr/bin/env bash
#
# verify-background-controller-rbac.sh
#
# Asserts the background controller's ClusterRole holds every permission the
# chart itself aggregates into the Kubernetes user-facing roles (admin, edit
# and view).
#
# Why this exists: a generate rule that creates a RoleBinding to `admin`
# (the usual namespace-onboarding pattern) is subject to the Kubernetes RBAC
# escalation check. The background controller can only create that binding
# if it holds every permission `admin` grants, or has `bind` on it. Binding
# `admin` has always needed user-supplied RBAC for the built-in Kubernetes
# resources, but the permissions Kyverno itself aggregates into `admin` are
# the chart's responsibility: when #16239 added policies.kyverno.io write
# verbs to the admin-aggregated role without granting them to the background
# controller, every such generate rule started failing on upgrade with
# "attempting to grant RBAC permissions not currently held", and nothing
# caught it until a post-merge chainsaw test two months later.
#
# `view` is aggregated into `edit`, and `edit` into `admin`, so rules
# labelled for any of the three end up in `admin` and are all checked.
#
# It renders the chart with `helm template` and checks that every
# (apiGroup, resource, verb) granted by a chart-rendered ClusterRole labelled
# rbac.authorization.k8s.io/aggregate-to-{admin,edit,view} is covered by the
# union of the ClusterRoles the background controller's aggregationRule
# selects, or that the background controller holds `bind` on clusterroles.
#
# This needs no cluster: `helm template` renders locally. It needs python3
# with PyYAML.
#
# Usage: scripts/verify-background-controller-rbac.sh

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHART_DIR="${ROOT_DIR}/charts/kyverno"
RELEASE_NAME="background-controller-rbac-verify"
# Honor a pinned helm binary passed in by the Makefile, falling back to
# whatever "helm" resolves to on PATH when run standalone.
HELM="${HELM:-helm}"
# `helm template` has no cluster to ask, so pin the version the way the
# Makefile's other `helm template --kube-version $(KUBE_VERSION)` calls do.
KUBE_VERSION="${KUBE_VERSION:-v1.25.0}"
PYTHON="${PYTHON:-python3}"

log() { echo "[verify-background-controller-rbac] $*"; }
fail() { echo "[verify-background-controller-rbac] FAIL: $*" >&2; exit 1; }

"${PYTHON}" -c 'import yaml' 2>/dev/null || fail "${PYTHON} cannot import yaml; install PyYAML (pip install pyyaml)"

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

# Each entry is a set of extra `helm template` flags. The background
# controller's grant depends on which user-facing roles are aggregated, so
# check the default (admin and view) and view only.
VARIANTS=(
  ""
  "--set rbac.roles.aggregate.admin=false"
)

for variant in "${VARIANTS[@]}"; do
  RENDER="${WORK_DIR}/render.yaml"
  log "rendering ${CHART_DIR} with --kube-version ${KUBE_VERSION} ${variant:-(default values)}"
  # shellcheck disable=SC2086 # variant is a list of flags
  "${HELM}" template "${RELEASE_NAME}" "${CHART_DIR}" \
    --kube-version "${KUBE_VERSION}" ${variant} \
    > "${RENDER}"
  "${PYTHON}" - "${RENDER}" <<'EOF'
import sys

import yaml

PREFIX = "[verify-background-controller-rbac]"
AGGREGATE_LABELS = [
    "rbac.authorization.k8s.io/aggregate-to-admin",
    "rbac.authorization.k8s.io/aggregate-to-edit",
    "rbac.authorization.k8s.io/aggregate-to-view",
]
BACKGROUND_SELECTOR = "rbac.kyverno.io/aggregate-to-background-controller"


def fail(msg):
    print(f"{PREFIX} FAIL: {msg}", file=sys.stderr)
    sys.exit(1)


with open(sys.argv[1]) as f:
    docs = [d for d in yaml.safe_load_all(f) if d]
roles = [d for d in docs if d.get("kind") == "ClusterRole"]


def labels(role):
    return (role.get("metadata") or {}).get("labels") or {}


def selected_by(selectors, role):
    lbls = labels(role)
    for sel in selectors:
        match = sel.get("matchLabels") or {}
        if match and all(lbls.get(k) == v for k, v in match.items()):
            return True
    return False


def matches(granted, wanted):
    return "*" in granted or wanted in granted


def covers(rules, group, resource, verb):
    for rule in rules:
        # A rule limited to resourceNames does not cover an unrestricted one.
        if rule.get("resourceNames"):
            continue
        if (
            matches(rule.get("apiGroups") or [], group)
            and matches(rule.get("resources") or [], resource)
            and matches(rule.get("verbs") or [], verb)
        ):
            return True
    return False


# The background controller's top-level ClusterRole is the one whose
# aggregationRule selects the aggregate-to-background-controller label.
background = [
    r
    for r in roles
    if any(
        BACKGROUND_SELECTOR in (s.get("matchLabels") or {})
        for s in ((r.get("aggregationRule") or {}).get("clusterRoleSelectors") or [])
    )
]
if len(background) != 1:
    fail(
        f"expected exactly one ClusterRole aggregating {BACKGROUND_SELECTOR}, found {len(background)}; "
        "did the background controller's clusterrole.yaml change shape?"
    )
background = background[0]
selectors = background["aggregationRule"]["clusterRoleSelectors"]
held = [
    rule
    for r in roles
    if r is not background and selected_by(selectors, r)
    for rule in (r.get("rules") or [])
]
print(f"{PREFIX} {background['metadata']['name']} aggregates {len(held)} rules")

def can_bind(rules, role_name):
    for rule in rules:
        names = rule.get("resourceNames") or []
        if names and role_name not in names:
            continue
        if (
            matches(rule.get("apiGroups") or [], "rbac.authorization.k8s.io")
            and matches(rule.get("resources") or [], "clusterroles")
            and matches(rule.get("verbs") or [], "bind")
        ):
            return True
    return False


# Holding bind on a role satisfies the escalation check for that role on its
# own, whether the grant is unrestricted or limited by resourceNames. Only
# skip the check when every user-facing role the chart aggregates into is
# bindable.
if all(can_bind(held, name) for name in ("admin", "edit", "view")):
    print(f"{PREFIX} background controller holds bind on admin, edit and view; nothing else to check")
    sys.exit(0)

aggregated = [r for r in roles if any(labels(r).get(l) == "true" for l in AGGREGATE_LABELS)]
if not aggregated:
    fail(
        "the chart render contained no ClusterRole labelled aggregate-to-admin/edit/view; "
        "is rbac.roles.aggregate still enabled by default, or did the label change?"
    )

missing = []
for role in aggregated:
    name = role["metadata"]["name"]
    for rule in role.get("rules") or []:
        if rule.get("nonResourceURLs"):
            fail(f"{name} aggregates nonResourceURLs, which this check does not understand")
        for group in rule.get("apiGroups") or []:
            for resource in rule.get("resources") or []:
                verbs = [v for v in rule.get("verbs") or [] if not covers(held, group, resource, v)]
                if verbs:
                    missing.append((name, group or "core", resource, verbs))
    print(f"{PREFIX}   checked {name}")

if missing:
    for name, group, resource, verbs in missing:
        print(f"{PREFIX}   missing: {resource}.{group} [{', '.join(verbs)}] (from {name})", file=sys.stderr)
    fail(
        f"the background controller lacks permissions on {len(missing)} resources (listed above) that the chart aggregates "
        "into the admin/edit/view roles. Generate rules creating a RoleBinding to admin would fail with "
        "'attempting to grant RBAC permissions not currently held'. Grant them in "
        "charts/kyverno/templates/background-controller/clusterrole.yaml."
    )

print(f"{PREFIX} background controller covers everything Kyverno aggregates into admin/edit/view")
EOF
done
