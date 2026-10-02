#!/usr/bin/env python3
"""Exercise the KEP-6060 receiver against one explicitly selected Kind cluster."""

import argparse
import base64
import http.client
import json
import os
import socket
import ssl
import subprocess
import time


NAMESPACE = "kyverno-webhook-auth-proof"
SERVICE_ACCOUNT = "webhook-auth-proof"


def kubectl(kubeconfig, *args, input_data=None):
    env = dict(os.environ, KUBECONFIG=kubeconfig)
    result = subprocess.run(
        ["kubectl", *args], input=input_data, text=True, capture_output=True,
        check=False, env=env,
    )
    if result.returncode:
        raise RuntimeError(f"kubectl failed: {result.stderr.strip()}")
    return result.stdout


def find_webhook(kubeconfig, kind, path, service_fragment):
    configs = json.loads(kubectl(kubeconfig, "get", kind, "-o", "json"))["items"]
    for config in configs:
        for hook in config["webhooks"]:
            client = hook["clientConfig"]
            service = client.get("service", {})
            if service.get("path") == path and service_fragment in service.get("name", ""):
                port = service.get("port", 443)
                host = f"{service['name']}.{service['namespace']}.svc"
                audience = f"https://{host}:{port}{path}"
                return config, client, service, host, audience
    raise RuntimeError(f"no {kind} webhook for {service_fragment}{path}")


def token_request(kubeconfig, kind, config, audience, group="*"):
    object_kind = ("ValidatingWebhookConfiguration" if kind.startswith("validating")
                   else "MutatingWebhookConfiguration")
    body = {
        "apiVersion": "authentication.k8s.io/v1",
        "kind": "TokenRequest",
        "spec": {
            "audiences": [audience],
            "expirationSeconds": 600,
            "boundObjectRef": {
                "apiVersion": "admissionregistration.k8s.io/v1",
                "kind": object_kind,
                "name": config["metadata"]["name"],
                "uid": config["metadata"]["uid"],
            },
            "attestations": {"admissionReviewAPIGroups": [group]},
        },
    }
    endpoint = f"/api/v1/namespaces/{NAMESPACE}/serviceaccounts/{SERVICE_ACCOUNT}/token"
    output = kubectl(
        kubeconfig, f"--as=system:serviceaccount:{NAMESPACE}:{SERVICE_ACCOUNT}",
        "create", "--raw", endpoint, "-f", "-", input_data=json.dumps(body),
    )
    return json.loads(output)["status"]["token"]


def available_port():
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return listener.getsockname()[1]


class ForwardedConnection(http.client.HTTPSConnection):
    def __init__(self, host, local_port, context):
        super().__init__(host, port=443, timeout=10, context=context)
        self.local_port = local_port

    def connect(self):
        plain = socket.create_connection(("127.0.0.1", self.local_port), 10)
        self.sock = self._context.wrap_socket(plain, server_hostname=self.host)


def review(group="apps", label=False):
    labels = {"app": "webhook-auth-proof"}
    if label:
        labels["webhook-auth-proof"] = "true"
    obj = {
        "apiVersion": "apps/v1", "kind": "Deployment",
        "metadata": {"name": "webhook-auth-proof", "namespace": NAMESPACE, "labels": labels},
        "spec": {
            "replicas": 1, "selector": {"matchLabels": {"app": "webhook-auth-proof"}},
            "template": {"metadata": {"labels": {"app": "webhook-auth-proof"}},
                         "spec": {"containers": [{"name": "web", "image": "nginx:1.27"}]}},
        },
    }
    return {
        "apiVersion": "admission.k8s.io/v1", "kind": "AdmissionReview",
        "request": {
            "uid": "webhook-auth-proof-uid", "kind": {"group": group, "version": "v1", "kind": "Deployment"},
            "resource": {"group": group, "version": "v1", "resource": "deployments"},
            "namespace": NAMESPACE, "name": "webhook-auth-proof", "operation": "CREATE",
            "userInfo": {"username": "webhook-auth-proof"}, "object": obj,
        },
    }


def request(host, port, ca_bundle, method, path, body=None, token=None):
    context = ssl.create_default_context(cadata=base64.b64decode(ca_bundle).decode())
    conn = ForwardedConnection(host, port, context)
    headers = {"Content-Type": "application/json"}
    if token is not None:
        headers["Authorization"] = f"Bearer {token}"
    conn.request(method, path, body=json.dumps(body) if body is not None else None, headers=headers)
    response = conn.getresponse()
    payload = response.read()
    status = response.status
    conn.close()
    if status != 200:
        raise RuntimeError(f"webhook returned HTTP {status} for {path}")
    return json.loads(payload) if body is not None else payload


def port_forward(kubeconfig, service, target_port):
    port = available_port()
    process = subprocess.Popen(
        ["kubectl", "--kubeconfig", kubeconfig, "-n", "kyverno", "port-forward",
         f"svc/{service}", f"{port}:{target_port}"],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )
    for _ in range(60):
        if process.poll() is not None:
            raise RuntimeError(f"port-forward for {service} exited")
        try:
            with socket.create_connection(("127.0.0.1", port), 0.2):
                return process, port
        except OSError:
            time.sleep(0.2)
    process.terminate()
    raise RuntimeError(f"port-forward for {service} did not start")


def denied_auth(result):
    response = result.get("response") or {}
    return (response.get("allowed") is False
            and (response.get("status") or {}).get("reason") == "Unauthorized")


def assert_policy_output(path, result):
    response = result["response"]
    if path == "/validate/fail":
        message = (response.get("status") or {}).get("message", "")
        if response.get("allowed") is not False or "webhook authentication proof label is required" not in message:
            raise AssertionError("validation policy did not deny the unlabeled Deployment")
    elif path == "/mutate/fail":
        patch = json.loads(base64.b64decode(response.get("patch", "")))
        if not response.get("allowed") or not any(
            part.get("path") == "/metadata/labels/webhook-auth-mutated" and part.get("value") == "true"
            for part in patch
        ):
            raise AssertionError("mutation policy did not return its expected patch")


def exercise(kubeconfig, phase, kind, path, service_fragment, label=False):
    config, client, service, host, audience = find_webhook(kubeconfig, kind, path, service_fragment)
    process, port = port_forward(kubeconfig, service["name"], service.get("port", 443))
    try:
        for probe in ("/health/liveness", "/health/readiness"):
            request(host, port, client["caBundle"], "GET", probe)
        body = review(label=label)
        unauthenticated = request(host, port, client["caBundle"], "POST", path, body)
        if phase == "enabled":
            if not denied_auth(unauthenticated):
                raise AssertionError(f"missing token passed authentication on {path}")
            token = token_request(kubeconfig, kind, config, audience)
            valid = request(host, port, client["caBundle"], "POST", path, body, token)
            if denied_auth(valid) or valid["response"]["uid"] != body["request"]["uid"]:
                raise AssertionError(f"valid token failed authentication on {path}")
            if service_fragment == "kyverno-svc":
                assert_policy_output(path, valid)
            if path == "/validate/fail":
                allowed = request(host, port, client["caBundle"], "POST", path, review(label=True), token)
                if not allowed["response"]["allowed"]:
                    raise AssertionError("labeled Deployment did not pass validation")
                apps_token = token_request(kubeconfig, kind, config, audience, group="apps")
                wrong_group = review(group="rbac.authorization.k8s.io", label=True)
                if not denied_auth(request(host, port, client["caBundle"], "POST", path, wrong_group, apps_token)):
                    raise AssertionError("token for apps group accepted another API group")
            wrong_audience = token_request(
                kubeconfig, kind, config,
                audience.replace(host, f"other.{service['namespace']}.svc", 1),
            )
            if not denied_auth(request(host, port, client["caBundle"], "POST", path, body, wrong_audience)):
                raise AssertionError(f"wrong audience accepted on {path}")
            tampered = token.rsplit(".", 1)[0] + "." + ("A" if token[-1] != "A" else "B") + token[-1]
            if not denied_auth(request(host, port, client["caBundle"], "POST", path, body, tampered)):
                raise AssertionError(f"invalid signature accepted on {path}")
            refreshed = token_request(kubeconfig, kind, config, audience)
            if denied_auth(request(host, port, client["caBundle"], "POST", path, body, refreshed)):
                raise AssertionError(f"reacquired token failed on {path}")
            print(f"{path}: signed token accepted; missing, wrong audience, and tampered tokens rejected; probes passed")
        else:
            if denied_auth(unauthenticated):
                raise AssertionError(f"unauthenticated request rejected while feature is {phase} on {path}")
            if service_fragment == "kyverno-svc":
                assert_policy_output(path, unauthenticated)
            if path == "/validate/fail":
                allowed = request(host, port, client["caBundle"], "POST", path, review(label=True))
                if not allowed["response"]["allowed"]:
                    raise AssertionError("labeled Deployment did not pass validation")
            print(f"{path}: unauthenticated request reached admission; probes passed")
    finally:
        process.terminate()
        process.wait(timeout=5)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--kubeconfig", required=True)
    parser.add_argument("--phase", choices=("baseline", "enabled", "rollback"), required=True)
    args = parser.parse_args()
    expected_context = "kind-kyverno-17399-20261002"
    current = kubectl(args.kubeconfig, "config", "current-context").strip()
    if current != expected_context:
        raise RuntimeError(f"refusing to test context {current!r}; expected {expected_context!r}")
    exercise(args.kubeconfig, args.phase, "validatingwebhookconfigurations", "/validate/fail", "kyverno-svc")
    exercise(args.kubeconfig, args.phase, "mutatingwebhookconfigurations", "/mutate/fail", "kyverno-svc", label=True)
    exercise(args.kubeconfig, args.phase, "validatingwebhookconfigurations", "/validate", "cleanup")
    exercise(args.kubeconfig, args.phase, "validatingwebhookconfigurations", "/verifyttl", "cleanup")


if __name__ == "__main__":
    main()
