#!/usr/bin/env python3
"""Trusted configuration and entry point for the shared PR automation."""

import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
from urllib.parse import quote
import uuid


KEYS = {
    "fallback_reviewers", "exempt_labels", "close_days", "warning_days",
    "backlog_days", "milestone_prefix", "labels_file",
}
ACTION = Path(__file__).resolve().parent


def days(value, name):
    if isinstance(value, bool) or not re.fullmatch(r"[1-9][0-9]*", str(value)):
        raise ValueError(f"{name} must be a positive whole number")
    value = int(value)
    if value > 3650:
        raise ValueError(f"{name} must not exceed 3650 days")
    return value


def validate_config(config):
    if not isinstance(config, dict) or set(config) != KEYS:
        raise ValueError(f"config must contain exactly: {', '.join(sorted(KEYS))}")
    for key in ("close_days", "warning_days", "backlog_days"):
        if type(config[key]) is not int:
            raise ValueError(f"{key} must be a JSON integer")
        days(config[key], key)
    if config["warning_days"] > config["close_days"]:
        raise ValueError("warning_days must not exceed close_days")
    if not isinstance(config["fallback_reviewers"], str) or not re.fullmatch(
        r"@[A-Za-z0-9-]+/[A-Za-z0-9_.-]+", config["fallback_reviewers"]
    ):
        raise ValueError("fallback_reviewers must be an @organization/team")
    labels = config["exempt_labels"]
    if not isinstance(labels, list) or any(
        not isinstance(label, str) or not label or len(label) > 50
        or any(ord(c) < 32 for c in label) for label in labels
    ) or len(set(labels)) != len(labels):
        raise ValueError("exempt_labels must be an array of unique label names")
    for key in ("milestone_prefix", "labels_file"):
        if not isinstance(config[key], str) or len(config[key]) > 256 or any(
            ord(c) < 32 for c in config[key]
        ):
            raise ValueError(f"{key} must be a single-line string (at most 256 characters)")
    return config


def trusted_file(workspace, name):
    path = (workspace / name).resolve()
    if Path(name).is_absolute() or not path.is_relative_to(workspace) or not path.is_file():
        raise ValueError(f"configuration file must be inside the trusted checkout: {name}")
    return path


def yaml_file(path):
    return json.loads(subprocess.check_output(
        ["ruby", "-rjson", "-ryaml", "-e",
         "puts JSON.generate(YAML.safe_load(File.read(ARGV[0]), aliases: false))", str(path)],
        text=True, timeout=60
    ))


def render_labels(raw):
    if not isinstance(raw, dict) or not raw:
        raise ValueError("labels_file must be a nonempty mapping of label metadata")
    rules, metadata = {}, []
    for name, spec in raw.items():
        if not isinstance(name, str) or not name or len(name) > 50 or not isinstance(spec, dict):
            raise ValueError("each label must have a name and metadata mapping")
        color = spec.get("color")
        description = spec.get("description", "")
        if not isinstance(color, str) or not re.fullmatch(r"[0-9a-fA-F]{6}", color):
            raise ValueError(f"label {name}: color must be a quoted six-digit hex string")
        if not isinstance(description, str) or len(description) > 100:
            raise ValueError(f"label {name}: description must be a string of at most 100 characters")
        if "rules" in spec and not isinstance(spec["rules"], list):
            raise ValueError(f"label {name}: rules must be an actions/labeler rule array")
        if spec.get("rules"):
            validate_rules(spec["rules"])
        metadata.append({"name": name, "color": color, "description": description})
        if spec.get("rules"):
            rules[name] = spec["rules"]
    return rules, metadata


def validate_rules(rules):
    glob_keys = {
        "any-glob-to-any-file", "any-glob-to-all-files",
        "all-globs-to-any-file", "all-globs-to-all-files",
    }
    for rule in rules:
        if not isinstance(rule, dict) or not rule:
            raise ValueError("each label rule must be a nonempty matcher mapping")
        for key, value in rule.items():
            if key in {"any", "all"}:
                if not isinstance(value, list) or not value:
                    raise ValueError(f"{key} label rules must be a nonempty array")
                validate_rules(value)
            elif key == "changed-files":
                if not isinstance(value, list) or not value:
                    raise ValueError("changed-files must be a nonempty array")
                for matcher in value:
                    if not isinstance(matcher, dict) or not matcher or not set(matcher) <= glob_keys:
                        raise ValueError("invalid changed-files matcher")
                    for patterns in matcher.values():
                        patterns = [patterns] if isinstance(patterns, str) else patterns
                        if not isinstance(patterns, list) or not patterns or any(
                            not isinstance(pattern, str) or not pattern for pattern in patterns
                        ):
                            raise ValueError("changed-files patterns must be a string or nonempty string array")
            elif key in {"head-branch", "base-branch"}:
                patterns = [value] if isinstance(value, str) else value
                if not isinstance(patterns, list) or not patterns or any(
                    not isinstance(pattern, str) or not pattern for pattern in patterns
                ):
                    raise ValueError(f"{key} must be a string or nonempty string array")
            else:
                raise ValueError(f"unsupported actions/labeler matcher: {key}")


def readiness_program(raw):
    if not isinstance(raw, dict) or not raw:
        raise ValueError("readiness-checks must be a nonempty mapping")
    expressions = []
    for key, check in raw.items():
        if not isinstance(key, str) or not re.fullmatch(r"[a-z_][a-z0-9_]*", key):
            raise ValueError("invalid readiness check identifier")
        if not isinstance(check, dict) or set(check) != {"test", "message"} or any(
            not isinstance(check[k], str) or not check[k].strip() for k in ("test", "message")
        ):
            raise ValueError(f"check {key} requires jq test and message strings")
        expressions.append(
            f'(if ({check["test"]}) then {{id: {json.dumps(key)}, '
            f'message: ({check["message"]})}} else empty end)'
        )
    return ".author.login as $author | [\n" + ",\n".join(expressions) + "\n]\n"


def gh_json(*args):
    return json.loads(subprocess.check_output(["gh", *args], text=True, timeout=120))


def sync_labels(metadata, dry_run, repo):
    pages = gh_json("api", f"repos/{repo}/labels?per_page=100", "--paginate", "--slurp")
    if not isinstance(pages, list) or not pages or any(not isinstance(p, list) for p in pages):
        raise ValueError("incomplete labels response")
    current = {label["name"]: label for page in pages for label in page}
    for label in metadata:
        existing = current.get(label["name"])
        if existing and all(
            (existing.get(k) or "").lower() == label[k].lower() if k == "color"
            else (existing.get(k) or "") == label[k] for k in ("color", "description")
        ):
            continue
        method = "PATCH" if existing else "POST"
        endpoint = f"repos/{repo}/labels"
        if existing:
            endpoint += "/" + quote(label["name"], safe="")
        print(f"{'[dry-run] ' if dry_run else ''}{method} {endpoint}: {json.dumps(label)}")
        if not dry_run:
            subprocess.run(
                ["gh", "api", endpoint, "--method", method, "--input", "-"],
                input=json.dumps(label), text=True, check=True, timeout=120,
            )


def clear_draft(number, dry_run, repo):
    current = gh_json("api", f"repos/{repo}/pulls/{number}")
    if current.get("draft") is not True or current.get("state") != "open":
        return
    if current.get("user", {}).get("login") == "dependabot[bot]":
        return
    labels = current["labels"]
    if not isinstance(labels, list):
        raise ValueError("incomplete draft labels")
    for name in ("ready-for-review", "needs-author-action", "workflow-approval-required"):
        if any(label["name"] == name for label in labels):
            command = ["gh", "pr", "edit", str(number), "--repo", repo, "--remove-label", name]
            if dry_run:
                print("[dry-run] " + " ".join(command))
            else:
                subprocess.run(command, check=True, timeout=120)


def main():
    workspace = Path(os.environ.get("GITHUB_WORKSPACE", ".")).resolve()
    mode = os.environ["PR_MODE"]
    if mode not in {"labels", "readiness", "stale", "sync-labels", "validate"}:
        raise ValueError("mode must be labels, readiness, stale, sync-labels, or validate")
    dry_run = os.environ.get("DRY_RUN", "true")
    if dry_run not in {"true", "false"}:
        raise ValueError("dry-run must be true or false")
    config = validate_config(json.loads(trusted_file(
        workspace, os.environ.get("PR_CONFIG", ".github/pr-management.json")
    ).read_text()))
    stale_days = days(os.environ.get("STALE_DAYS", "7"), "stale-days")
    nudge_days = days(os.environ.get("NUDGE_DAYS", "7"), "review-nudge-days")
    if stale_days > config["close_days"]:
        raise ValueError("stale-days must not exceed close_days")
    env = dict(os.environ, DRY_RUN=dry_run, STALE_DAYS=str(stale_days), NUDGE_DAYS=str(nudge_days),
               CLOSE_DAYS=str(config["close_days"]), WARNING_DAYS=str(config["warning_days"]),
               BACKLOG_DAYS=str(config["backlog_days"]), FALLBACK_REVIEWERS=config["fallback_reviewers"],
               EXEMPT_LABELS=json.dumps(config["exempt_labels"]), MILESTONE_PREFIX=config["milestone_prefix"])
    if mode == "validate":
        render_labels(yaml_file(trusted_file(workspace, config["labels_file"])))
        program = readiness_program(yaml_file(trusted_file(
            workspace, os.environ.get("READINESS_CHECKS", ".github/pr-readiness-checks.yml")
        )))
        # A jq module can contain definitions, not a top-level program. Import without
        # invoking the function, so even expression-delimiter injection cannot run it.
        work = workspace / f".pr-management-validate-{uuid.uuid4().hex}"
        work.mkdir()
        try:
            (work / "checks.jq").write_text(f"def readiness: ({program});\n")
            subprocess.run(["jq", "-L", str(work), "-n", 'import "checks" as checks; null'],
                           check=True, stdout=subprocess.DEVNULL, timeout=60)
        finally:
            shutil.rmtree(work)
        print("Valid PR management config, label metadata/rules, and readiness jq.")
        return
    if mode in {"labels", "sync-labels"}:
        rules, metadata = render_labels(yaml_file(trusted_file(workspace, config["labels_file"])))
        if mode == "sync-labels":
            sync_labels(metadata, dry_run == "true", os.environ["GITHUB_REPOSITORY"])
            return
        generated = workspace / f".pr-management-labeler-{uuid.uuid4().hex}.json"
        generated.write_text(json.dumps(rules, indent=2))
        with open(os.environ["GITHUB_OUTPUT"], "a") as output:
            output.write(f"configuration-path={generated}\n")
        if dry_run == "true":
            print("[dry-run] File labeling rules (no labeler or merge-conflict mutations):")
            print(json.dumps(rules, indent=2))
        return
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    pr = event.get("pull_request", {})
    if os.environ["GITHUB_EVENT_NAME"] == "pull_request_target" and pr.get("draft") is True:
        number = pr.get("number")
        if type(number) is not int or number < 1:
            raise ValueError("invalid draft PR number")
        clear_draft(number, dry_run == "true", os.environ["GITHUB_REPOSITORY"])
        return
    env.update(EVENT_NAME=os.environ["GITHUB_EVENT_NAME"], PR_NUMBER=str(pr.get("number", "")),
               IS_DRAFT=str(pr.get("draft", False)).lower(),
               PR_AUTHOR=pr.get("user", {}).get("login", ""))
    work = workspace / f".pr-management-work-{uuid.uuid4().hex}"
    work.mkdir()
    try:
        if mode == "readiness":
            program = readiness_program(yaml_file(trusted_file(
                workspace, os.environ.get("READINESS_CHECKS", ".github/pr-readiness-checks.yml")
            )))
            (work / "readiness-checks.jq").write_text(program)
        subprocess.run(["bash", str(ACTION / f"{mode}.sh")], cwd=work, env=env, check=True)
    finally:
        shutil.rmtree(work)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, OSError, subprocess.SubprocessError) as error:
        print(f"::error::{error}", file=sys.stderr)
        sys.exit(1)
