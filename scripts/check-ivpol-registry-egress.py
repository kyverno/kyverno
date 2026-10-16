#!/usr/bin/env python3
"""Run the checked-in registry egress proof through a standalone Kyverno CLI.

Both unsafe image addresses must be rejected by the request guard. The unmatched
Pod must remain allowed or skipped. No reachable registry or cluster is needed.
"""

import argparse
from pathlib import Path
import subprocess


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cli", type=Path, required=True,
                        help="CLI binary built from this checkout")
    args = parser.parse_args()
    cli = args.cli.resolve()
    root = Path(__file__).resolve().parent.parent
    fixture = root / "test/conformance/chainsaw/image-validating-policies/standard/registry-egress-hard-block"
    for resource, host in (("metadata-pod.yaml", "169.254.169.254"),
                           ("loopback-pod.yaml", "127.0.0.1"),
                           ("unmatched-pod.yaml", None)):
        result = subprocess.run(
            [str(cli), "apply", str(fixture / "policy.yaml"),
             "--resource", str(fixture / resource), "--policy-report"],
            capture_output=True, text=True, check=False, timeout=60)
        output = result.stdout + result.stderr
        if host:
            expected = f'connection blocked to network host "{host}"'
            # YAML reports may fold the message across lines.
            if result.returncode == 0 or expected not in " ".join(output.split()):
                raise AssertionError(f"{resource}: expected guarded rejection, "
                                     f"got exit {result.returncode}:\n{output}")
        elif result.returncode != 0:
            raise AssertionError(f"{resource}: unmatched Pod was rejected:\n{output}")
        print(f"PASS: {resource}: " + ("request guard rejected image" if host else "unmatched Pod allowed or skipped"))


if __name__ == "__main__":
    main()
