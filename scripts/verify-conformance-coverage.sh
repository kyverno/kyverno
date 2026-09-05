#!/usr/bin/env bash
# Check that every chainsaw conformance test is reachable by CI.
# Usage: scripts/verify-conformance-coverage.sh
#
# The conformance action runs `chainsaw test --config .chainsaw.yaml` from the
# suite directory, so a test only runs if a .chainsaw.yaml sits above it and a
# workflow names the suite. Fails, listing offenders, when either is missing.

set -e

CHAINSAW_DIR="test/conformance/chainsaw"
WORKFLOW_DIR=".github"

# Suites deliberately not run by CI. Add a reason with any entry.
EXEMPT_SUITES=()

if [ ! -d "$CHAINSAW_DIR" ]; then
  echo "error: $CHAINSAW_DIR not found; run from the repository root" >&2
  exit 1
fi

suite_roots=$(find "$CHAINSAW_DIR" -name '.chainsaw.yaml' -printf '%h\n' | sed "s|^$CHAINSAW_DIR/||" | sort -u)
test_dirs=$(find "$CHAINSAW_DIR" -name 'chainsaw-test.yaml' -printf '%h\n' | sed "s|^$CHAINSAW_DIR/||" | sort -u)

# Suites named by a tests-path input or by a job that cds into one. Either may
# carry a subdirectory or a matrix expression, so match the first segment.
referenced=$(
  {
    grep -rho -- 'tests-path:[[:space:]]*[^[:space:]]*' "$WORKFLOW_DIR" | sed 's|tests-path:[[:space:]]*||'
    grep -rho -- "$CHAINSAW_DIR/[A-Za-z0-9_.-]*" "$WORKFLOW_DIR" | sed "s|^$CHAINSAW_DIR/||"
  } 2>/dev/null | cut -d/ -f1 | grep -v '^\.$' | sort -u
)

failed=0

for dir in $test_dirs; do
  covered=false
  for root in $suite_roots; do
    case "$dir/" in
      "$root"/*) covered=true; break ;;
    esac
  done
  if [ "$covered" = false ]; then
    echo "error: no .chainsaw.yaml above $CHAINSAW_DIR/$dir, so chainsaw never collects it" >&2
    failed=1
  fi
done

for root in $suite_roots; do
  suite="${root%%/*}"
  exempted=false
  for exempt in "${EXEMPT_SUITES[@]}"; do
    if [ "$suite" = "$exempt" ]; then
      exempted=true
    fi
  done
  if [ "$exempted" = true ]; then
    continue
  fi
  if ! echo "$referenced" | grep -qx -- "$suite"; then
    echo "error: no workflow runs $CHAINSAW_DIR/$root" >&2
    failed=1
  fi
done

for suite in $referenced; do
  if [ ! -d "$CHAINSAW_DIR/$suite" ]; then
    echo "error: workflows reference $CHAINSAW_DIR/$suite, which does not exist" >&2
    failed=1
  fi
done

if [ "$failed" -ne 0 ]; then
  echo "add a .chainsaw.yaml at the suite root, add a job in tests-conformance.yaml, or list the suite in EXEMPT_SUITES" >&2
  exit 1
fi

echo "ok: $(echo "$suite_roots" | wc -l) conformance suites, all reachable and referenced"
