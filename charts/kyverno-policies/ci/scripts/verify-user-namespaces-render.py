#!/usr/bin/env python3
"""
Regression + shape check for the podSecurityUserNamespaces toggle added to
disallow-proc-mount.yaml / disallow-proc-mount.cel.yaml.

This does not replace `ct lint` / `helm template` (the chart's actual CI
gate) -- it's a targeted, dependency-free check that:

  1. With podSecurityUserNamespaces=false (the default), the CEL template
     renders byte-identical to its pre-change form on main, i.e. no
     behavior change for existing users.
  2. The ClusterPolicy template still contains the original, byte-for-byte
     unmodified procMount pattern block (just re-wrapped under anyPattern),
     and the new hostUsers:false alternative is correctly gated behind the
     podSecurityUserNamespaces flag, evaluated ahead of the original.

Run: python3 charts/kyverno-policies/ci/scripts/verify-user-namespaces-render.py
"""
import subprocess
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[4]
CEL_FILE = REPO_ROOT / "charts/kyverno-policies/templates/baseline/disallow-proc-mount.cel.yaml"
YAML_FILE = REPO_ROOT / "charts/kyverno-policies/templates/baseline/disallow-proc-mount.yaml"

ORIGINAL_PATTERN_BLOCK = (
    '            =(ephemeralContainers):\n'
    '              - =(securityContext):\n'
    '                  =(procMount): "Default"\n'
    '            =(initContainers):\n'
    '              - =(securityContext):\n'
    '                  =(procMount): "Default"\n'
    '            containers:\n'
    '              - =(securityContext):\n'
    '                  =(procMount): "Default"'
)

HOST_USERS_GATE = (
    '{{- if .Values.podSecurityUserNamespaces }}\n'
    '        - spec:\n'
    '            hostUsers: false\n'
    '        {{- end }}'
)

CEL_FLAG_BLOCK = (
    '{{- if .Values.podSecurityUserNamespaces }}\n'
    "        (object.spec.?hostUsers.orValue(true) == false) ||\n"
    '        {{- end }}'
)


def check_cel_flag_false_is_noop():
    # This file uses CRLF line endings in the repo (unlike most others) --
    # normalize to LF before comparing so a line-ending artifact can't hide
    # (or fake) a real content difference.
    new_content = CEL_FILE.read_text(encoding="utf-8").replace("\r\n", "\n")
    main_orig = subprocess.run(
        ["git", "show", "main:charts/kyverno-policies/templates/baseline/disallow-proc-mount.cel.yaml"],
        capture_output=True, text=True, cwd=REPO_ROOT,
    ).stdout.replace("\r\n", "\n")

    assert CEL_FLAG_BLOCK in new_content, "expected hostUsers CEL block not found verbatim in disallow-proc-mount.cel.yaml"
    flag_false_rendered = new_content.replace("        " + CEL_FLAG_BLOCK + "\n", "", 1)
    assert flag_false_rendered == main_orig, "CEL file with the flag block stripped must equal main exactly (no-op check failed)"
    print("OK: disallow-proc-mount.cel.yaml with podSecurityUserNamespaces=false is a no-op vs main")


def check_clusterpolicy_pattern_untouched_and_gated():
    content = YAML_FILE.read_text(encoding="utf-8")
    assert "anyPattern:" in content, "expected pattern -> anyPattern conversion not found"
    assert "pattern:\n          spec:" not in content, "old single-pattern form should no longer be present"
    assert ORIGINAL_PATTERN_BLOCK in content, "original procMount pattern content must remain byte-for-byte unchanged"
    assert HOST_USERS_GATE in content, "hostUsers:false alternative must be gated behind podSecurityUserNamespaces exactly as expected"
    assert content.index(HOST_USERS_GATE) < content.index(ORIGINAL_PATTERN_BLOCK), \
        "hostUsers alternative must come first so it's evaluated before falling back to the original pattern"
    print("OK: disallow-proc-mount.yaml keeps the original pattern content unchanged, with hostUsers:false gated ahead of it")


if __name__ == "__main__":
    try:
        check_cel_flag_false_is_noop()
        check_clusterpolicy_pattern_untouched_and_gated()
    except AssertionError as e:
        print(f"FAIL: {e}", file=sys.stderr)
        sys.exit(1)
    print("All checks passed.")
