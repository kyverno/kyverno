#!/usr/bin/env bash
# Runs one conformance (chainsaw) suite on a local kind cluster, set up the way
# CI sets it up for that suite. `make test-e2e-local` calls it.
#
# Environment:
#
# - SUITE:           the suite to run: a directory under test/conformance/chainsaw
#                    that has a .chainsaw.yaml, such as generating-policies.
# - TEST:            regex for the tests to run, as the chainsaw-tests input of
#                    the CI action. Chainsaw names a test after its directory
#                    relative to the suite, and matches the regex one path
#                    element at a time: clone/sync runs the tests whose first
#                    directory contains "clone" and second one "sync". Empty
#                    runs the whole suite.
# - K8S:             Kubernetes version, such as v1.35.8 or 1.35. It must be in
#                    the CI matrix. Empty uses the newest one CI runs the suite on.
# - USE_CONFIG:      Kyverno configs to install with, instead of the ones CI uses
#                    for the suite.
# - RUN_QUARANTINED: "true" also runs the tests listed in
#                    test/conformance/quarantine.yaml. CI skips them.
# - DRY_RUN:         when not empty, prints the commands and runs nothing.
# - E2E_KIND_NAME:   name of the kind cluster (default kyverno-e2e).
#
# The setup comes from the CI files, so that it follows them when they change:
#
# - .github/workflows/tests-conformance.yaml: the job that runs the suite gives
#   the Kyverno configs, kind config, helm settings, and extra installs.
# - .github/actions/tests/conformance/run/action.yaml: the defaults of the same.
# - .github/workflows/kubernetes-matrix.yaml: the node image of each version.
#
# The steps mirror that action. Keep them in sync when the action changes.
set -euo pipefail

cd "$(dirname "$0")/.."

workflow=.github/workflows/tests-conformance.yaml
matrix=.github/workflows/kubernetes-matrix.yaml
action=.github/actions/tests/conformance/run
chainsaw_action=.github/actions/tools/chainsaw/action.yaml
root=test/conformance/chainsaw
quarantine=test/conformance/quarantine.yaml

KIND=${KIND:-kind}
HELM=${HELM:-helm}
YQ=${YQ:-yq}
CHAINSAW=${CHAINSAW:-chainsaw}
MAKE=${MAKE:-make}
TOOLS_DIR=${TOOLS_DIR:-$PWD/.tools}
name=${E2E_KIND_NAME:-kyverno-e2e}
dry_run=${DRY_RUN:-}
state=$TOOLS_DIR/e2e-local
sep=$(printf '\037')

die() {
  echo "Error: $*" >&2
  exit 1
}

warn() {
  echo "Warning: $*" >&2
}

# Prints a command, and runs it unless this is a dry run.
run() {
  local arg
  {
    printf '+'
    for arg in "$@"; do
      case "$arg" in
        "" | *[!A-Za-z0-9_./:=@,+-]*) printf " '%s'" "$arg" ;;
        *) printf ' %s' "$arg" ;;
      esac
    done
    echo
  } >&2
  [ -n "$dry_run" ] || "$@"
}

suites() {
  find "$root" -name .chainsaw.yaml | sed -e "s|^$root/||" -e 's|/\.chainsaw\.yaml$||' | sort
}

command -v "$YQ" > /dev/null 2>&1 || die "yq is not installed at $YQ. Install it with: make $YQ"

# The suite.
suite=${SUITE:-}
suite=${suite#./}
suite=${suite#"$root"/}
suite=${suite%/}
if [ -z "$suite" ]; then
  die "SUITE is not set.
Usage: make test-e2e-local SUITE=<suite> [TEST=<regex>] [K8S=<version>]
Suites:
$(suites | sed 's/^/  /')"
fi
if [ ! -f "$root/$suite/.chainsaw.yaml" ]; then
  parent=$suite
  while [ -d "$root/$suite" ] && [ "$parent" != "${parent%/*}" ]; do
    parent=${parent%/*}
    if [ -f "$root/$parent/.chainsaw.yaml" ]; then
      die "$suite is inside the suite $parent. Run:
  make test-e2e-local SUITE=$parent TEST=${suite#"$parent"/}"
    fi
  done
  inside=$(suites | grep "^$suite/" || true)
  if [ -n "$inside" ]; then
    die "$root/$suite holds more than one suite. Pick one:
$(echo "$inside" | sed 's/^/  /')"
  fi
  die "$suite is not a suite: $root/$suite has no .chainsaw.yaml.
Suites:
$(suites | sed 's/^/  /')"
fi

# The setups CI uses for the suite: one line for each distinct setup, with the
# job name and the inputs of the conformance action. A job can use matrix values
# in its inputs (rbac runs once for each Kyverno config), so try each value of
# each matrix list. The last line holds the defaults of the action.
fields="tests-path kyverno-configs kind-config install-cert-manager install-kubectl-evict install-openreports explicit-install-settings"
# shellcheck disable=SC2016 # $job and the like are yq variables
setups=$(
  {
    "$YQ" -r '.inputs | to_entries[] | ["default", .key, .value.default // ""] | @tsv' "$action/action.yaml"
    ACTION="./$action" "$YQ" -r '
      .jobs | to_entries[] | .key as $job
      | .value.steps[] | select(.uses == strenv(ACTION)) | .with
      | to_entries[] | ["with", $job, .key, .value] | @tsv
    ' "$workflow"
    "$YQ" -r '
      .jobs | to_entries[] | .key as $job | .value.strategy.matrix // {}
      | to_entries[] | select(.value | tag == "!!seq") | .key as $list
      | .value | to_entries[] | ($list + "#" + (.key | tostring)) as $variant | .value
      | ((select(tag == "!!map") | to_entries[] | ["matrix", $job, $variant, $list + "." + .key, .value]),
         (select(tag != "!!map") | ["matrix", $job, $variant, $list, .]))
      | @tsv
    ' "$workflow"
  } | awk -F '\t' -v suite="$suite" -v fields="$fields" -v sep="$sep" '
    function replace(text, from, to,    at, out) {
      out = ""
      while ((at = index(text, from)) > 0) {
        out = out substr(text, 1, at - 1) to
        text = substr(text, at + length(from))
      }
      return out text
    }
    function setup(job, variant,    f, b, text, line) {
      line = ""
      for (f = 1; f <= count; f++) {
        text = ((job, field[f]) in with) ? with[job, field[f]] : fallback[field[f]]
        for (b = 1; b <= bindings[job, variant]; b++)
          text = replace(text, "${{ matrix." binding[job, variant, b] " }}", value[job, variant, binding[job, variant, b]])
        if (index(text, "${{")) return
        if (f == 1) {
          sub(/^\.\//, "", text)
          sub(/\/$/, "", text)
          if (text != suite) return
        }
        line = line sep text
      }
      if (!(line in printed)) print job line
      printed[line] = 1
    }
    BEGIN { count = split(fields, field, " ") }
    $1 == "default" { fallback[$2] = $3 }
    $1 == "with" && $3 != "" {
      if (!($2 in known)) jobs[++job_count] = $2
      known[$2] = 1
      with[$2, $3] = $4
    }
    $1 == "matrix" {
      if (!(($2, $3) in bindings)) variants[$2, ++variant_count[$2]] = $3
      binding[$2, $3, ++bindings[$2, $3]] = $4
      value[$2, $3, $4] = $5
    }
    END {
      for (j = 1; j <= job_count; j++) {
        setup(jobs[j], "")
        for (v = 1; v <= variant_count[jobs[j]]; v++) setup(jobs[j], variants[jobs[j], v])
      }
      line = "-" sep suite
      for (f = 2; f <= count; f++) line = line sep fallback[field[f]]
      print line
    }
  '
)
found=$(echo "$setups" | grep -c -v "^-$sep" || true)
override=${USE_CONFIG:-}
setup=
if [ -n "$override" ]; then
  setup=$(echo "$setups" | awk -F "$sep" -v configs="$override" '$3 == configs { print; exit }')
fi
[ -n "$setup" ] || setup=$(echo "$setups" | sed -n 1p)
IFS=$sep read -r job _ configs kind_config cert_manager evict openreports settings << EOF
$setup
EOF
if [ "$found" -eq 0 ] && [ -z "$override" ]; then
  die "No job in $workflow runs the suite $suite with the conformance action, so there is no CI setup to copy.
To run it anyway, name the Kyverno configs to install with, for example USE_CONFIG=standard."
fi
[ -z "$override" ] || configs=$override
for config in $(echo "$configs" | tr ',' ' '); do
  [ -f "scripts/config/$config/kyverno.yaml" ] || die "Unknown Kyverno config $config: there is no scripts/config/$config/kyverno.yaml."
done
kind_config=${kind_config#./}
[ -f "$kind_config" ] || die "The kind config $kind_config doesn't exist."

# The Kubernetes version and its node image. The MutatingAdmissionPolicy suites
# run only on the versions that serve their API version; the others on all.
nodes=$("$YQ" -r '.jobs.matrix.steps[] | select(.id == "matrix") | .env.NODES' "$matrix" |
  "$YQ" -p=json -r '.[] | [.version, .image, .mutatingAdmissionPolicy] | join(" ")' -)
[ -n "$nodes" ] || die "Can't read the Kubernetes versions from $matrix."
case $(JOB="$job" "$YQ" -r '.jobs[strenv(JOB)].strategy.matrix["k8s-version"] // ""' "$workflow") in
  *outputs.beta-versions*) api=v1beta1 ;;
  *outputs.stable-versions*) api=v1 ;;
  *) api= ;;
esac
versions=$(echo "$nodes" | awk -v api="$api" 'api == "" || $3 == api { print $1 }')
if [ -z "${K8S:-}" ]; then
  k8s=$(echo "$versions" | sed -n '$p')
else
  k8s=$(echo "$nodes" | awk -v want="v${K8S#v}" '$1 == want || index($1, want ".") == 1 { found = $1 } END { print found }')
  if [ -z "$k8s" ]; then
    die "Kubernetes $K8S is not in the CI matrix ($matrix).
Supported versions: $(echo "$nodes" | awk '{ printf "%s%s", comma, $1; comma = ", " }')"
  fi
  echo "$versions" | grep -x "$k8s" > /dev/null ||
    warn "CI doesn't run $suite on Kubernetes $k8s, only on $(echo "$versions" | awk '{ printf "%s%s", comma, $1; comma = ", " }')."
fi
image=$(echo "$nodes" | awk -v version="$k8s" '$1 == version { print $2 }')

# The quarantined tests of the suite. CI sets spec.skip in their files; exclude
# them by name here, to leave the working tree alone. Chainsaw names a test
# <directory relative to the suite>[<metadata.name>].
exclude=
quarantined=
if [ "${RUN_QUARANTINED:-}" != true ] && [ -f "$quarantine" ]; then
  for path in $("$YQ" -r '(.tests // [])[] | .path // ""' "$quarantine"); do
    path=${path#./}
    path=${path%/}
    case "$path" in
      "$suite") path=. ;;
      "$suite"/*) path=${path#"$suite"/} ;;
      *) continue ;;
    esac
    pattern=$(echo "$path" | sed -e 's/[][\.*^$+?(){}|]/\\&/g' -e 's|/|$/^|g')
    exclude="${exclude:+$exclude|}^chainsaw\$/^$pattern\\["
    quarantined="$quarantined $path"
  done
fi

[ "$job" != - ] || job="none, so the defaults of the conformance action"
echo "Suite:       $suite (CI job: $job)"
echo "Kubernetes:  $k8s ($image)"
echo "Kind:        cluster $name, config $kind_config"
echo "Kyverno:     USE_CONFIG=$configs${settings:+ EXPLICIT_INSTALL_SETTINGS=\"$settings\"}"
echo "Tests:       ^chainsaw\$/${TEST:-}"
[ -z "$quarantined" ] || echo "Quarantined: skipping$quarantined (as CI does; RUN_QUARANTINED=true runs them)"
if [ "$found" -gt 1 ]; then
  echo "CI runs this suite with $found setups (USE_CONFIG=... picks another one):"
  echo "$setups" | grep -v "^-$sep" | awk -F "$sep" '{ print "  " $3 ($8 == "" ? "" : " with " $8) }'
fi
echo

if [ -z "$dry_run" ]; then
  command -v kubectl > /dev/null 2>&1 || die "kubectl is not on your PATH."
  docker info > /dev/null 2>&1 || die "Docker isn't running: kind and the image build need it."
  mkdir -p "$state"

  # CI pins these versions. Another version mostly works, but can behave differently.
  ci_kind=$("$YQ" -r '.jobs.matrix.steps[] | select(.id == "matrix") | .env.KIND_VERSION' "$matrix")
  ci_chainsaw=$("$YQ" -r '[.runs.steps[] | .with.release // ""] | map(select(. != "")) | .[0]' "$chainsaw_action")
  my_kind=$("$KIND" version 2> /dev/null | awk '{ print $2 }' || true)
  my_chainsaw=$("$CHAINSAW" version 2> /dev/null | awk '$1 == "Version:" { print "v" $2 }' || true)
  [ "$my_kind" = "$ci_kind" ] ||
    warn "CI uses kind $ci_kind, but $KIND is ${my_kind:-another version}. The node images are built for the kind version of CI: an older kind can't load images in them."
  [ "$my_chainsaw" = "$ci_chainsaw" ] ||
    warn "CI uses chainsaw $ci_chainsaw, but $CHAINSAW is ${my_chainsaw:-another version}."
fi

# Everything below talks to the cluster through this file, so the current
# context of your own kubeconfig is neither used nor changed.
export KUBECONFIG="$state/kubeconfig"
export PATH="$TOOLS_DIR:$PATH"

# The cluster. Reuse it when it was created for the same setup, else recreate it:
# CI installs every suite on a new cluster.
wanted="name=$name image=$image kind-config=$kind_config use-config=$configs settings=$settings cert-manager=$cert_manager openreports=$openreports"
reuse=
clusters=$("$KIND" get clusters 2> /dev/null || true)
if echo "$clusters" | grep -x "$name" > /dev/null; then
  if [ "$(cat "$state/cluster" 2> /dev/null)" = "$wanted" ]; then
    echo "Reusing the kind cluster $name. For a new one, run make test-e2e-local-clean first."
    reuse=true
    run "$KIND" export kubeconfig --name "$name" --kubeconfig "$KUBECONFIG"
  else
    echo "The kind cluster $name has another setup: recreating it."
    run "$KIND" delete cluster --name "$name"
  fi
fi
if [ -z "$reuse" ]; then
  # Without the host ports 80 and 443 of the CI config, which no test uses: they
  # would clash with another kind cluster created from the same config.
  run cp "$kind_config" "$state/kind.yaml"
  run "$YQ" -i 'del(.nodes[].extraPortMappings)' "$state/kind.yaml"
  run "$KIND" create cluster --name "$name" --image "$image" --config "$state/kind.yaml" --kubeconfig "$KUBECONFIG"
  [ -n "$dry_run" ] || echo "$wanted" > "$state/cluster"
fi

footer() {
  echo
  echo "The kind cluster $name is still running. To look at it: export KUBECONFIG=$KUBECONFIG"
  echo "To delete it: make test-e2e-local-clean"
}
[ -n "$dry_run" ] || trap footer EXIT

# Kyverno. CI loads images built by another job; build them from the working tree.
# With the same kind as the cluster: the kind of the other Makefile targets is older.
run "$MAKE" kind-load-all KIND_NAME="$name" KIND="$KIND"
if [ "$openreports" = true ]; then
  run kubectl apply -f https://raw.githubusercontent.com/openreports/reports-api/refs/heads/main/config/install.yaml
fi
if [ "$cert_manager" = true ]; then
  run "$HELM" upgrade --install cert-manager cert-manager --repo https://charts.jetstack.io \
    --namespace cert-manager --create-namespace --set crds.enabled=true --wait --timeout 300s
fi
run "$MAKE" kind-install-kyverno USE_CONFIG="$configs" EXPLICIT_INSTALL_SETTINGS="$settings"
run kubectl apply --server-side --force-conflicts -R -f ./config/crds
if [ "$evict" = true ] && ! command -v kubectl-evict > /dev/null 2>&1; then
  run env GOBIN="$TOOLS_DIR" go install github.com/ueokande/kubectl-evict@latest
fi
if [ -n "$reuse" ]; then
  # The image tag is the commit, so helm changes nothing when only the working
  # tree changed. Restart Kyverno to run the images that were just loaded.
  run kubectl --namespace kyverno rollout restart deployment
  run kubectl --namespace kyverno rollout status deployment --timeout=180s
else
  run kubectl wait --namespace kyverno --for=condition=ready pod --selector '!job-name' --timeout=60s
fi

# The tests.
set -- test --config .chainsaw.yaml \
  --report-format JSON --report-path "$state" --report-name results \
  --include-test-regex "^chainsaw\$/${TEST:-}"
[ -z "$exclude" ] || set -- "$@" --exclude-test-regex "$exclude"
run rm -f "$state/results.json"
run cd "$root/$suite"
status=0
run "$CHAINSAW" "$@" || status=$?
[ -z "$dry_run" ] || exit 0
if [ "$status" -eq 0 ] && [ "$("$YQ" -p=json -o=json '.tests | length' "$state/results.json")" = 0 ]; then
  echo "Error: no test ran. TEST=${TEST:-} matches no test of $suite that isn't quarantined." >&2
  status=1
fi
exit "$status"
