#!/usr/bin/env python3
"""Inspect selected GlobalContextEntry storage records on a disposable kind cluster."""

import argparse
import base64
import copy
import json
import os
from pathlib import Path
import re
import subprocess


def write_report(path, report):
    """Create a private report; refuse existing files and symlinks."""
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as output:
        json.dump(report, output, indent=2)
        output.write("\n")


def preserved_fields(obj):
    result = copy.deepcopy(obj)
    result.pop("apiVersion", None)
    for field in ("resourceVersion", "managedFields"):
        result["metadata"].pop(field, None)
    result.get("status", {}).pop("lastRefreshTime", None)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--kubeconfig", required=True)
    parser.add_argument("--context", required=True)
    parser.add_argument("--name", action="append", required=True)
    parser.add_argument("--expected-version", choices=("v2", "v2beta1"), required=True)
    parser.add_argument("--compare", type=Path)
    parser.add_argument("--output", type=Path, required=True,
                        help="new report path (must not already exist)")
    args = parser.parse_args()
    if not args.context.startswith("kind-"):
        parser.error("use a disposable kind cluster")
    if len(set(args.name)) != len(args.name) or any(
        not re.fullmatch(r"[a-z0-9](?:[-a-z0-9.]*[a-z0-9])?", name)
        for name in args.name
    ):
        parser.error("provide distinct Kubernetes object names")

    def kubectl(*arguments):
        result = subprocess.run(
            ["kubectl", "--kubeconfig", args.kubeconfig, "--context", args.context,
             *arguments],
            check=True, capture_output=True, text=True,
        )
        return json.loads(result.stdout)

    nodes = kubectl("get", "nodes", "-o", "json")["items"]
    if not nodes or not all(
        node.get("spec", {}).get("providerID", "").startswith("kind://")
        for node in nodes
    ):
        raise RuntimeError("the selected cluster did not verify as kind")
    pods = kubectl("get", "pods", "-n", "kube-system", "-l", "component=etcd", "-o", "json")["items"]
    if len(pods) != 1:
        raise RuntimeError("this proof requires a single-control-plane kind cluster")
    pod = pods[0]["metadata"]["name"]
    crd = kubectl("get", "crd", "globalcontextentries.kyverno.io", "-o", "json")
    served = {v["name"] for v in crd["spec"]["versions"] if v["served"]}
    storage = [v["name"] for v in crd["spec"]["versions"] if v["storage"]]
    if not {"v2", "v2beta1", "v2alpha1"} <= served:
        raise AssertionError("a retained API version is no longer served")
    objects = {}
    for name in args.name:
        key = "/registry/kyverno.io/globalcontextentries/" + name
        response = kubectl(
            "exec", "-n", "kube-system", pod, "--", "etcdctl",
            "--endpoints=https://127.0.0.1:2379",
            "--cacert=/etc/kubernetes/pki/etcd/ca.crt",
            "--cert=/etc/kubernetes/pki/etcd/server.crt",
            "--key=/etc/kubernetes/pki/etcd/server.key",
            "get", key, "-w", "json",
        )
        records = response.get("kvs", [])
        if len(records) != 1 or base64.b64decode(records[0]["key"]).decode() != key:
            raise AssertionError("missing or unexpected etcd record for " + name)
        obj = json.loads(base64.b64decode(records[0]["value"]))
        if obj["metadata"]["name"] != name or not obj["metadata"].get("uid"):
            raise AssertionError("invalid stored object identity for " + name)
        objects[name] = obj
    report = {
        "objects": objects,
        "storedVersions": crd["status"]["storedVersions"],
        "storageVersion": storage,
        "servedVersions": sorted(served),
    }
    # Preserve the observation even if the expected version check fails.
    write_report(args.output, report)
    expected = "kyverno.io/" + args.expected_version
    for name, obj in objects.items():
        if obj["apiVersion"] != expected:
            raise AssertionError(name + " is stored as " + obj["apiVersion"] + "; expected " + expected)
    if storage != [args.expected_version] or report["storedVersions"] != [args.expected_version]:
        raise AssertionError("CRD storage declarations have not converged: " + str(report["storedVersions"]))
    if args.compare:
        before = json.loads(args.compare.read_text(encoding="utf-8"))["objects"]
        if set(before) != set(objects):
            raise AssertionError("the before/after fixture names differ")
        for name, obj in before.items():
            if preserved_fields(obj) != preserved_fields(objects[name]):
                raise AssertionError("migration changed retained fields for " + name)
    print("Verified " + str(len(objects)) + " stored objects and CRD markers at " + args.expected_version)


if __name__ == "__main__":
    main()
