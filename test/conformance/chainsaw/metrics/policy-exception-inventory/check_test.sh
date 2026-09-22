#!/usr/bin/env bash
# Exercise the endpoint proof's replica and retry checks without a cluster.
set -euo pipefail
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
export PROOF_TEST_STATE="$work"
cat > "$work/kubectl" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
attempt=$(cat "$PROOF_TEST_STATE/attempt")
if [[ $1 == -n ]]; then
  attempt=$((attempt + 1))
  printf '%s' "$attempt" > "$PROOF_TEST_STATE/attempt"
  case "$PROOF_TEST_SCENARIO" in
    list-failure) printf 'partial-output'; exit 1 ;;
    list-recovery) if [[ $attempt == 1 ]]; then printf 'partial-output'; exit 1; fi ;;
    one) printf 'pod-a'; exit 0 ;;
    three) printf 'pod-a pod-b pod-c'; exit 0 ;;
    empty) exit 0 ;;
    replica-recovery) if [[ $attempt == 1 ]]; then printf 'pod-a'; exit 0; fi ;;
  esac
  printf 'pod-a pod-b'
  exit 0
fi
printf '%s\n' "$3" >> "$PROOF_TEST_STATE/scraped"
if [[ $PROOF_TEST_SCENARIO == metrics-recovery && $attempt == 1 ]]; then exit 1; fi
printf '# TYPE kyverno_policy_exception_info gauge\n'
printf 'kyverno_policy_exception_info{exception_api_group="policies.kyverno.io",exception_namespace="proof",exception_name="retained-inventory",policy_kind="ValidatingPolicy",policy_name="retained-policy"} 1\n'
if [[ $PROOF_TEST_SCENARIO == stale-second && $3 == *pod-b:* ]]; then exit 0; fi
case "$PROOF_TEST_PHASE" in
  created)
    printf 'kyverno_policy_exception_info{exception_api_group="policies.kyverno.io",exception_namespace="proof",exception_name="cel-inventory",policy_kind="ValidatingPolicy",policy_name="missing-cel-policy"} 1\n'
    printf 'kyverno_policy_exception_info{exception_api_group="policies.kyverno.io",exception_namespace="proof",exception_name="cel-inventory",policy_kind="ImageValidatingPolicy",policy_name="other-missing-policy"} 1\n' ;;
  updated)
    printf 'kyverno_policy_exception_info{exception_api_group="policies.kyverno.io",exception_namespace="proof",exception_name="cel-inventory",policy_kind="ValidatingPolicy",policy_name="replacement-cel"} 1\n' ;;
esac
STUB
printf '#!/usr/bin/env bash\nexit 0\n' > "$work/sleep"
chmod +x "$work/kubectl" "$work/sleep"
export PATH="$work:$PATH"
for scenario in healthy replica-recovery list-recovery metrics-recovery one three empty list-failure stale-second; do
  for phase in created updated deleted; do
    # The deleted phase has only the retained row, so the stale fixture is identical.
    if [[ $scenario == stale-second && $phase == deleted ]]; then continue; fi
    export PROOF_TEST_SCENARIO="$scenario" PROOF_TEST_PHASE="$phase"
    printf '0' > "$work/attempt"
    : > "$work/scraped"
    expected_status=0
    expected_attempts=1
    case "$scenario" in
      *-recovery) expected_attempts=2 ;;
      one|three|empty|list-failure|stale-second) expected_status=1; expected_attempts=60 ;;
    esac
    status=0
    bash "$script_dir/check.sh" "$phase" proof > "$work/output" 2>&1 || status=$?
    if [[ $status != "$expected_status" || $(cat "$work/attempt") != "$expected_attempts" ]]; then
      cat "$work/output"
      printf 'FAIL: %s %s (status=%s, attempts=%s)\n' "$scenario" "$phase" "$status" "$(cat "$work/attempt")" >&2
      exit 1
    fi
    if [[ $expected_status == 0 ]]; then
      grep -q 'pod-a:8000' "$work/scraped"
      grep -q 'pod-b:8000' "$work/scraped"
    fi
    printf 'PASS: %s %s\n' "$scenario" "$phase"
  done
done
