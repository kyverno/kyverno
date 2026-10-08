#!/usr/bin/env python3
"""Render registry egress settings and check them with real controller parsers.

Requires Helm, PyYAML, and binaries built from the same checkout. Appending --help
exits during argument parsing, before Kubernetes clients or controllers start.
"""

import argparse
import json
from pathlib import Path
import subprocess
import tempfile

import yaml


CONTROLLERS = {
    "admission-controller": ("kyverno", "admissionController"),
    "background-controller": ("background-controller", "backgroundController"),
    "reports-controller": ("reports-controller", "reportsController"),
    "cleanup-controller": ("cleanup-controller", "cleanupController"),
}
ALLOWLIST = ["registry.corp.example", "10.20.30.0/24", "fd12:3456:789a::/64"]


def check(condition, message):
    if not condition:
        raise AssertionError(message)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", type=Path, required=True)
    parser.add_argument("--helm", type=Path)
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    helm = str((args.helm or root / ".tools/helm").resolve())
    binaries = {role: args.bin_dir.resolve() / values[0]
                for role, values in CONTROLLERS.items()}
    for binary in binaries.values():
        check(binary.is_file(), f"Build the controller binary first: {binary}")

    def render(values):
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json") as values_file:
            json.dump(values, values_file)
            values_file.flush()
            return subprocess.run(
                [helm, "template", "kyverno", str(root / "charts/kyverno"),
                 "--namespace", "kyverno", "--kube-version", "1.32.0",
                 "--values", values_file.name],
                capture_output=True, text=True, check=False, timeout=60)

    cases = [("audit defaults", {}, {})]
    config = {"privateRegistryEgressMode": "enforce",
              "privateRegistryAllowlist": ALLOWLIST}
    enforced = {role: config for role in CONTROLLERS if role != "cleanup-controller"}
    cases.append(("enforce globally", {"features": {"registryClient": config}}, enforced))
    for role, (_, section) in CONTROLLERS.items():
        if role != "cleanup-controller":
            cases.append((f"{role} override",
                          {section: {"featuresOverride": {"registryClient": config}}},
                          {role: config}))

    for name, values, expected in cases:
        result = render(values)
        check(result.returncode == 0, f"{name}: Helm failed: {result.stderr}")
        seen = set()
        for document in yaml.safe_load_all(result.stdout):
            if not document or document.get("kind") != "Deployment":
                continue
            role = document["metadata"].get("labels", {}).get("app.kubernetes.io/component")
            if role not in CONTROLLERS:
                continue
            seen.add(role)
            containers = document["spec"]["template"]["spec"]["containers"]
            controller = next(container for container in containers
                              if container["name"] in ("kyverno", "controller"))
            rendered_args = controller["args"]
            egress_args = [arg for arg in rendered_args if arg.startswith("--privateRegistry")]
            expected_args = []
            if role != "cleanup-controller":
                settings = expected.get(role, {})
                mode = settings.get("privateRegistryEgressMode", "audit")
                expected_args.append("--privateRegistryEgressMode=" + mode)
                if settings.get("privateRegistryAllowlist"):
                    allowed = ",".join(settings["privateRegistryAllowlist"])
                    expected_args.append("--privateRegistryAllowlist=" + allowed)
            check(sorted(egress_args) == sorted(expected_args),
                  f"{name}: unexpected {role} registry flags: {egress_args}")
            parsed = subprocess.run([str(binaries[role]), *rendered_args, "--help"],
                                    capture_output=True, text=True, check=False, timeout=30)
            check(parsed.returncode == 0,
                  f"{name}: {role} rejected rendered arguments:\n{parsed.stderr}\n{parsed.stdout}")
        missing = set(CONTROLLERS) - seen
        check(not missing, f"{name}: missing controller deployments: {missing}")
        print(f"PASS: {name}: all four controller parsers accept their rendered arguments")

    scopes = [("global", "features")]
    scopes += [(role, section) for role, (_, section) in CONTROLLERS.items()
               if role != "cleanup-controller"]
    for name, section in scopes:
        for invalid in ([""], ["  "], ["registry.corp.example", ""], [42], [None]):
            registry = {"registryClient": {"privateRegistryAllowlist": invalid}}
            values = {section: registry if section == "features"
                      else {"featuresOverride": registry}}
            result = render(values)
            rejected = (result.returncode != 0 and
                        "must be a non-empty string" in result.stderr)
            check(rejected, f"{name}: Helm must reject allowlist {invalid!r}, "
                            f"got {result.returncode}: {result.stderr}")
        print(f"PASS: {name}: Helm rejects empty and non-string allowlist entries")


if __name__ == "__main__":
    main()
