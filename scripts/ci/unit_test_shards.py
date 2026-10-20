#!/usr/bin/env python3
"""Run a full Go package inventory in balanced shards and validate their coverage."""

import argparse
import json
import math
from pathlib import Path
import re
import subprocess
import sys


def partition(packages, weights, count):
    if count < 1 or not packages or len(set(packages)) != len(packages):
        raise ValueError("expected a positive shard count and unique nonempty inventory")
    if count > len(packages):
        raise ValueError("shard count exceeds package count")
    if any(not isinstance(w, (int, float)) or not math.isfinite(w) or w <= 0
           for w in weights.values()):
        raise ValueError("package weights must be finite positive numbers")
    shards = [[] for _ in range(count)]
    loads = [0.0] * count
    for package in sorted(packages, key=lambda p: (-weights.get(p, 1), p)):
        index = min(range(count), key=lambda i: (loads[i], i))
        shards[index].append(package)
        loads[index] += weights.get(package, 1)
    return [sorted(shard) for shard in shards]


def run_shard(index, count, output, weights):
    if not 0 <= index < count:
        raise ValueError("shard index must be in [0, count)")
    packages = sorted(subprocess.check_output(
        ["go", "list", "./..."], text=True).splitlines())
    assigned = partition(packages, weights, count)[index]
    output.mkdir(parents=True, exist_ok=False)
    manifest = {
        "index": index,
        "count": count,
        "inventory": packages,
        "packages": assigned,
        "success": False,
    }
    manifest_path = output / "manifest.json"
    manifest_path.write_text(json.dumps(manifest, indent=2) + "\n")
    print("Shard {}/{}: {} of {} packages".format(
        index + 1, count, len(assigned), len(packages)), flush=True)
    command = [
        "go", "test", "-json", "-race", "-covermode=atomic",
        "-coverprofile=" + str(output / "coverage.out"),
    ] + assigned
    completed = set()
    failed = False
    with (output / "events.jsonl").open("w") as events:
        with subprocess.Popen(command, stdout=subprocess.PIPE, text=True) as process:
            for line in process.stdout:
                events.write(line)
                event = json.loads(line)
                if "Output" in event:
                    print(event["Output"], end="", flush=True)
                if event.get("Action") == "fail":
                    failed = True
                if not event.get("Test") and event.get("Action") in ("pass", "skip"):
                    completed.add(event["Package"])
            result = process.wait()
    if result:
        raise subprocess.CalledProcessError(result, command)
    if failed or completed != set(assigned):
        raise ValueError("test events do not confirm every assigned package completed")
    manifest["success"] = True
    manifest_path.write_text(json.dumps(manifest, indent=2) + "\n")


def merge(root, count, output):
    if count < 1:
        raise ValueError("shard count must be positive")
    directories = sorted(root.glob("unit-shard-*"))
    if len(directories) != count:
        raise ValueError("missing or unexpected shard artifacts")
    manifests = [json.loads((d / "manifest.json").read_text()) for d in directories]
    if sorted(m["index"] for m in manifests) != list(range(count)):
        raise ValueError("missing or duplicate shard index")
    inventory = manifests[0]["inventory"]
    if not inventory or inventory != sorted(set(inventory)):
        raise ValueError("invalid package inventory")
    assigned = []
    for manifest in manifests:
        if (manifest["count"] != count or manifest["inventory"] != inventory
                or manifest["success"] is not True or not manifest["packages"]):
            raise ValueError("incomplete shard or inconsistent package inventory")
        assigned.extend(manifest["packages"])
    if sorted(assigned) != inventory:
        raise ValueError("packages must be tested exactly once across all shards")

    blocks = {}
    for directory in directories:
        lines = (directory / "coverage.out").read_text().splitlines()
        if not lines or lines[0] != "mode: atomic":
            raise ValueError("expected an atomic coverage profile in " + str(directory))
        for line in lines[1:]:
            match = re.fullmatch(r"(.+:\d+\.\d+,\d+\.\d+) (\d+) (\d+)", line)
            if not match:
                raise ValueError("malformed coverage block: " + line)
            location, statements, _ = match.groups()
            if location in blocks:
                raise ValueError("unexpected overlapping coverage block: " + location)
            blocks[location] = (int(statements), line)
    if not blocks:
        raise ValueError("merged coverage profile contains no blocks")
    output.write_text("mode: atomic\n" + "\n".join(
        blocks[key][1] for key in sorted(blocks)) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    run = commands.add_parser("run")
    run.add_argument("--index", type=int, required=True)
    run.add_argument("--count", type=int, required=True)
    run.add_argument("--output", type=Path, required=True)
    run.add_argument("--weights", type=Path, required=True)
    aggregate = commands.add_parser("merge")
    aggregate.add_argument("--root", type=Path, required=True)
    aggregate.add_argument("--count", type=int, required=True)
    aggregate.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if args.command == "run":
        run_shard(args.index, args.count, args.output,
                  json.loads(args.weights.read_text()))
    else:
        merge(args.root, args.count, args.output)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        print("Unit-test sharding failed: " + str(error), file=sys.stderr)
        sys.exit(1)
