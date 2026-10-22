# Controllers

This document contains the list of controllers in this repository.

Most controllers code can be found under `pkg/controllers` except for some legacy controllers.

| Name                             | Leader             | Description                                                   |
|----------------------------------|--------------------|---------------------------------------------------------------|
| `certmanager-controller`         | :heavy_check_mark: | Manages TLS certificates                                      |
| `config-controller`              |                    | Watches config map and reloads config on changes              |
| `openapi-controller`             |                    | Polls discovery API and maintains APIs schemas                |
| `webhook-controller`             | :heavy_check_mark: | Configures webhooks                                           |
| `admission-report-controller`    | :heavy_check_mark: | Cleans up admission reports                                   |
| `aggregate-report-controller`    | :heavy_check_mark: | Aggregates reports                                            |
| `background-scan-controller`     | :heavy_check_mark: | Manages background scans reports                              |
| `resource-report-controller`     | :heavy_check_mark: | Watches resources that participate in reports                 |
| `cleanup-controller`             | :heavy_check_mark: | Runs CEL `DeletingPolicy`/`NamespacedDeletingPolicy` and TTL cleanup; no longer reconciles legacy `CleanupPolicy`/`ClusterCleanupPolicy` |
| `policy-controller`              | :heavy_check_mark: | Reconciles CEL `GeneratingPolicy`/`NamespacedGeneratingPolicy`/`MutatingPolicy`/`NamespacedMutatingPolicy` and creates `UpdateRequest`s for them; no longer reconciles legacy `kyverno.io/v1` policies |
| `update-request-controller`      |                    | Processes `UpdateRequest`s for CEL `gpol`/`mpol`, and deletes (rather than processes) any legacy `generate`/`mutateExisting` `UpdateRequest` left over from an older release |

## Controller Internals

The internal processes/functions of each controller are explained below.

### Admission Controller

The Admission Controller is where the "heart" of Kyverno's processing logic for most policy types is found. It is a required component of any Kyverno installation.

#### Webhook Server

The webhook server is where Kyverno receives and processes AdmissionReview requests which are sent in response to policies and their matching resources. For mutate and validate rules in `Enforce` mode, Kyverno returns the decision along with the admission response; it is a synchronous request-response process. However, for generate and validate rules in `Audit` mode, Kyverno pushes these requests to a queue and returns the response immediately, then starts processing the data asynchronously. The queue in the validate audit handler is used to generate policy reports.

The webhook server also performs validation for all policies except cleanup policies.

#### Webhook Controller

The webhook controller is responsible for configuring the various webhooks. These webhooks, by default managed dynamically, instruct the Kubernetes API server which resources to send to Kyverno. The webhook controller uses leader election as it maintains an internal webhook timestamp to monitor the webhook status. The controller also recreates the webhook configurations if any are missing.

#### Cert Renewer

The certificate renewer controller is responsible for monitoring the Secrets Kyverno uses for its webhooks. This controller uses leader election.

#### Exceptions Controller

Policy Exceptions are processed in this component so that matching resources of installed policies which also match a Policy Exception are handled properly.

#### UpdateRequest Generator

The UpdateRequest is an intermediary resource used by the Background Controller in handling of generate and mutate-existing rules. UpdateRequests are synchronously generated inside this component and then asynchronously processed by the Background Controller.

#### AdmissionReport Generator

An AdmissionReport is an intermediary resource used by the Reports Controller to create and reconcile the final policy reports. They are created in this component in response to an incoming AdmissionReview request which matches an installed policy.

### Background Controller

The Background Controller handles CEL `GeneratingPolicy`/`NamespacedGeneratingPolicy` (`gpol`) generate rules and
CEL `MutatingPolicy`/`NamespacedMutatingPolicy` (`mpol`) mutate-existing rules only. It has no relationship to the
background report scanning which occurs in the Reports Controller. Legacy `kyverno.io/v1` `generate`/
`mutateExisting` execution was removed from this binary; see `pkg/background/AGENTS.md` for the detail on how a
leftover legacy `UpdateRequest` is deleted instead of processed.

#### UpdateRequest Controller

This controller reconciles `UpdateRequest`s created by the Admission Controller and the Policy Controller into
their final form, whether that is a generated resource or a mutated existing resource, for the two CEL request
types. Any `UpdateRequest` of a legacy `generate`/`mutateExisting` type is deleted on sync instead of processed.

#### Policy Controller

The policy controller processes all adds, deletes, and updates to installed CEL policies (`gpol`/`ngpol`/`mpol`/
`nmpol`), and creates `UpdateRequest`s upon those policy events.

### Reports Controller

The report controller is responsible for creation of policy reports from both admission requests and background scans and requires leader election. It tracks resources that need to be processed in the background and generates background scan reports (when policy/resource change). It also aggregates these and the intermediary admission reports into the final policy report resources `PolicyReport` and `ClusterPolicyReport`.

#### Background Scan Controller

This component performs all the background scans in a cluster when the designated interval elapses and creates the intermediary resources `BackgroundScanReport` and `ClusterBackgroundScanReport`.

#### AdmissionReport Aggregator

This component takes the synchronously-generated AdmissionReport resources from the Admission Controller and aggregates them into a second intermediary AdmissionReport resource on a per-resource basis.

#### Policy Report Aggregator

This component aggregates both the background and admission intermediary reports into the final resources `PolicyReport` and `ClusterPolicyReport`.

### Cleanup Controller

The Cleanup Controller runs CEL `DeletingPolicy`/`NamespacedDeletingPolicy` cleanup and TTL-annotated resource deletion. It no longer reconciles the legacy `CleanupPolicy`/`ClusterCleanupPolicy` CronJob-based deletion flow, which was removed along with the rest of legacy policy execution. It still uses and manages webhooks, so its webhook validation rejects any new `CleanupPolicy`/`ClusterCleanupPolicy` write unconditionally.

#### Webhook Server

This component serves the webhook route that unconditionally denies writes to the legacy `CleanupPolicy` and `ClusterCleanupPolicy` kinds. See [pkg/deprecations/AGENTS.md](../../pkg/deprecations/AGENTS.md) for the denial mechanism.

#### Webhook Controller

The webhook controller updates the webhook used by the cleanup controller when the Secret changes. This component also uses leader election.

#### Deleting Policy Controller

This component (`pkg/controllers/deleting`) reconciles CEL `DeletingPolicy`/`NamespacedDeletingPolicy` resources and performs the scheduled resource deletion they describe. It requires leader election.

#### TTL Controller

This component deletes TTL-annotated expired resources using a metadata-only client. It requires leader election.

#### Cert Renewer

The certificate renewer controller is responsible for monitoring the Secrets used in the cleanup controller's webhooks. This controller uses leader election.
