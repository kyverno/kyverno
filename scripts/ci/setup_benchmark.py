#!/usr/bin/env python3
"""Record setup overhead and compare full-suite benchmark arms."""

import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import time


ARMS = ("baseline", "no-cleanup", "no-build-cache", "minimal-setup")


def snapshot():
    return {
        "monotonic": time.monotonic(),
        "free_bytes": shutil.disk_usage(".").free,
        "temp_free_bytes": shutil.disk_usage(os.environ.get("TMPDIR", "/tmp")).free,
    }


def mark(directory, name):
    directory.mkdir(parents=True, exist_ok=True)
    data = snapshot()
    data.update({
        "checkout": subprocess.check_output(
            ["git", "rev-parse", "HEAD"], text=True).strip(),
        "cpu_count": os.cpu_count(),
        "image_os": os.environ.get("ImageOS"),
        "image_version": os.environ.get("ImageVersion"),
    })
    (directory / (name + ".json")).write_text(json.dumps(data, indent=2) + "\n")


def run(directory, command):
    started = snapshot()
    minimum = min(started["free_bytes"], started["temp_free_bytes"])
    with subprocess.Popen(command) as process:
        while True:
            current = snapshot()
            minimum = min(minimum, current["free_bytes"], current["temp_free_bytes"])
            try:
                result = process.wait(timeout=1)
                break
            except subprocess.TimeoutExpired:
                continue
    ended = snapshot()
    minimum = min(minimum, ended["free_bytes"], ended["temp_free_bytes"])
    data = {
        "seconds": ended["monotonic"] - started["monotonic"],
        "exit_code": result,
        "sampled_min_free_bytes": minimum,
        "command": command,
    }
    (directory / "test.json").write_text(json.dumps(data, indent=2) + "\n")
    return result if result >= 0 else 128 - result


def coverage(path):
    lines = path.read_text().splitlines()
    if not lines or lines[0] != "mode: atomic":
        raise ValueError("expected atomic coverage: " + str(path))
    blocks = {}
    for line in lines[1:]:
        match = re.fullmatch(r"(.+:\d+\.\d+,\d+\.\d+) (\d+) (\d+)", line)
        if not match:
            raise ValueError("invalid coverage block: " + line)
        location, statements, count = match.groups()
        if location in blocks:
            raise ValueError("duplicate coverage block: " + location)
        blocks[location] = (int(statements), int(count))
    if not blocks:
        raise ValueError("empty coverage profile")
    return blocks


def report(root):
    def read(directory, name):
        return json.loads((directory / (name + ".json")).read_text())

    rows = []
    baseline = coverage(root / "setup-baseline" / "coverage.out")
    identities = {key: value[0] for key, value in baseline.items()}
    checkout = read(root / "setup-baseline", "start")["checkout"]
    for arm in ARMS:
        directory = root / ("setup-" + arm)
        start, cleaned, ready = [read(directory, name)
                                 for name in ("start", "cleaned", "ready")]
        test = read(directory, "test")
        if test["exit_code"] != 0:
            raise ValueError("full test suite failed in " + arm)
        if any(record["checkout"] != checkout for record in (start, cleaned, ready)):
            raise ValueError("benchmark checkouts differ")
        blocks = coverage(directory / "coverage.out")
        if {key: value[0] for key, value in blocks.items()} != identities:
            raise ValueError("coverage block inventory differs in " + arm)
        cleanup = cleaned["monotonic"] - start["monotonic"]
        setup = ready["monotonic"] - cleaned["monotonic"]
        if cleanup < 0 or setup < 0 or test["seconds"] < 0:
            raise ValueError("invalid benchmark timing")
        statements = sum(n for n, count in blocks.values())
        covered = sum(n for n, count in blocks.values() if count > 0)
        rows.append({
            "arm": arm,
            "cleanup_seconds": cleanup,
            "go_setup_seconds": setup,
            "test_seconds": test["seconds"],
            "measured_seconds": cleanup + setup + test["seconds"],
            "start_free_gib": start["free_bytes"] / 2**30,
            "ready_free_gib": ready["free_bytes"] / 2**30,
            "sampled_test_min_free_gib": test["sampled_min_free_bytes"] / 2**30,
            "covered_statements": covered,
            "total_statements": statements,
            "coverage_blocks": len(blocks),
        })
    return {"checkout": checkout, "arms": rows}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="mode", required=True)
    marker = commands.add_parser("mark")
    marker.add_argument("directory", type=Path)
    marker.add_argument("name", choices=("start", "cleaned", "ready"))
    runner = commands.add_parser("run")
    runner.add_argument("directory", type=Path)
    runner.add_argument("command", nargs=argparse.REMAINDER)
    summary = commands.add_parser("report")
    summary.add_argument("root", type=Path)
    args = parser.parse_args()
    if args.mode == "mark":
        mark(args.directory, args.name)
    elif args.mode == "run":
        command = args.command
        if command and command[0] == "--":
            command = command[1:]
        if not command:
            parser.error("run requires a command")
        return run(args.directory, command)
    else:
        result = report(args.root)
        (args.root / "comparison.json").write_text(json.dumps(result, indent=2) + "\n")
        print("| Arm | Cleanup | Go setup | Tests | Measured total | Min free during tests | Covered statements |")
        print("|---|---:|---:|---:|---:|---:|---:|")
        for row in result["arms"]:
            print("| {arm} | {cleanup_seconds:.1f}s | {go_setup_seconds:.1f}s | "
                  "{test_seconds:.1f}s | {measured_seconds:.1f}s | "
                  "{sampled_test_min_free_gib:.1f} GiB | "
                  "{covered_statements}/{total_statements} |".format(**row))
        print("\nAll arms have identical coverage block identities and statement counts. "
              "Covered totals may vary with test nondeterminism; compare before promotion. "
              "Disk minima are sampled once per second during tests, not a capacity guarantee. "
              "Totals exclude checkout, queue, artifacts and reporting. "
              "One run is not a p90 estimate; repeat on broad and cold-cache changes.")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        print("Setup benchmark failed: " + str(error), file=sys.stderr)
        sys.exit(1)
