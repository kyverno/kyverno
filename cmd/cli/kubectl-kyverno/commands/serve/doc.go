package serve

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#serve`

var description = []string{
	`Run the Kyverno CLI policy evaluation as a local HTTP server (experimental, developer preview).`,
	``,
	`Policies are loaded once at startup from local files or directories; startup fails if any`,
	`policy cannot be loaded. Payloads are then evaluated with POST /v1alpha1/evaluate/{mode},`,
	`where mode is Kubernetes, JSON, HTTP or Envoy, using the same evaluation path as "kyverno apply".`,
	``,
	`Endpoints:`,
	`  POST /v1alpha1/evaluate/{mode}  evaluate a raw JSON/YAML payload`,
	`  GET  /v1alpha1/policies         list the loaded policies and the policy-set digest`,
	`  GET  /healthz, /readyz          health and readiness`,
	``,
	`The v1alpha1 API is experimental and may change in any release without notice. The server`,
	`has no authentication or TLS, never writes to a cluster, evaluates one request at a time, and`,
	`is intended for local development and testing only. It listens on loopback by default.`,
}

var examples = [][]string{
	{
		`# Serve the policies in a directory on the default address (127.0.0.1:8080)`,
		`KYVERNO_EXPERIMENTAL=true kyverno serve --policies ./policies`,
	},
	{
		`# Evaluate a Kubernetes resource`,
		`curl -s -X POST --data-binary @pod.yaml http://127.0.0.1:8080/v1alpha1/evaluate/Kubernetes`,
	},
	{
		`# Serve only Envoy check requests on another port`,
		`KYVERNO_EXPERIMENTAL=true kyverno serve --policies ./envoy-policies --mode Envoy --address 127.0.0.1:9000`,
	},
}
