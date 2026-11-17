# AGENTS.md — cmd/cli/kubectl-kyverno/commands/create/role

This package implements the `kyverno create cluster-role` (and `kubectl-kyverno create cluster-role`) CLI command, which generates Kubernetes `ClusterRole` manifests aggregating permissions to specified Kyverno controllers.

## Command Contract and Flag Semantics

1. **Flag Types (`StringSliceVar`)**:
   - `--api-groups`, `--controllers`, `--resources`, and `--verbs` use `StringSliceVar` so users can pass both repeated flags (e.g. `--resources=pods --resources=services`) and comma-delimited strings (e.g. `--resources=pods,services`, `--api-groups=apps,batch`).
   - Flag types must remain `StringSliceVar` (generating `strings` in CLI reference documentation) rather than `StringArrayVar`.

2. **Validation and Trimming**:
   - All slice entries in `opts.Controllers`, `opts.ResourceTypes`, and `verbs` are trimmed of leading and trailing whitespace.
   - Empty entries in controllers, resources, or verbs (including those resulting from trailing commas such as `--controllers=background-controller,`) must be rejected with descriptive errors (`invalid controller provided`, `invalid resource provided`, `invalid verb provided`) to prevent generating invalid labels (`rbac.kyverno.io/aggregate-to-`) or empty RBAC rule items.
   - For `--api-groups`, empty string `""` represents the Kubernetes core API group (e.g. pods, services) and is valid; presence is checked via `cmd.Flags().Changed("api-groups")`.

3. **Verb Expansion**:
   - Verbs are case-folded to lowercase.
   - When the `all` verb is passed (e.g. `--verbs=all` or `--verbs=patch,all`), it expands to standard Kubernetes verbs: `create`, `get`, `update`, `delete`, `list`, `watch`.
   - The wildcard verb `*` is a Kubernetes RBAC wildcard verb and is preserved literally rather than expanded.
   - Any explicitly specified non-standard or additional verbs (such as `patch` or `*`) are preserved in the expanded list when `all` is supplied.

4. **Manifest Template Contract (`aggregated-role.yaml`)**:
   - `apiGroups`, `resources`, and `verbs` in the generated `ClusterRole` must render as discrete YAML array entries (`apiGroups: [""]`, `resources: ["deployments", "statefulsets"]`), never as a single comma-joined quoted string.
   - Controller aggregation labels follow `rbac.kyverno.io/aggregate-to-<controller>: "true"`.
