# Controllers

This document contains the list of controllers in this repository.

Most controllers code can be found under `pkg/controllers` except for some legacy controllers.

| Name                             | Leader             | Description                                                   |
|----------------------------------|--------------------|---------------------------------------------------------------|
| `certmanager-controller`         | :heavy_check_mark: | Manages TLS certificates                                      |
| `config-controller`              |                    | Watches config map and reloads config on changes              |
| `openapi-controller`             |                    | Polls discovery API and maintains APIs schemas                |
| [`policycache-controller`]       |                    | Maintains an up to date policy cache                          |
| `webhook-controller`             | :heavy_check_mark: | Configures webhooks                                           |
| `admission-report-controller`    | :heavy_check_mark: | Cleans up admission reports                                   |
| `aggregate-report-controller`    | :heavy_check_mark: | Aggregates reports                                            |
| `background-scan-controller`     | :heavy_check_mark: | Manages background scans reports                              |
| `resource-report-controller`     | :heavy_check_mark: | Watches resources that participate in reports                 |
| `cleanup-controller`             | :heavy_check_mark: | Reconciles cleanup policies and associated cron jobs          |
| `policy-controller`              | :heavy_check_mark: | Manages mutation of existing resources                        |
| `update-request-controller`      |                    | Manages generate policy and its generated resources lifecycle |

[`policycache-controller`]: ./policycache.md

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

The Background Controller handles generate and mutate rules only for existing resources. It has no relationship to the background report scanning which occurs in the Reports Controller.

#### UpdateRequest Controller

This controller reconciles the UpdateRequests created by the Admission Controller and the Policy Controller into their final form, whether that is a generated resource or a mutated existing resource.

#### Policy Controller

The policy controller processes all adds, deletes, and updates to all installed policies, and creates UpdateRequests upon policy events.

### Reports Controller

The report controller is responsible for creation of policy reports from both admission requests and background scans and requires leader election. It track resources that need to be processed in the background and generates background scan reports (when policy/resource change). It also aggregates these and the intermediary admission reports into the final policy report resources `PolicyReport` and `ClusterPolicyReport`.

#### Background Scan Controller

This component performs all the background scans in a cluster when the designated interval elapses and creates the intermediary resources `BackgroundScanReport` and `ClusterBackgroundScanReport`.

#### AdmissionReport Aggregator

This component takes the synchronously-generated AdmissionReport resources from the Admission Controller and aggregates them into a second intermediary AdmissionReport resource on a per-resource basis.

#### Policy Report Aggregator

This component aggregates both the background and admission intermediary reports into the final resources `PolicyReport` and `ClusterPolicyReport`.

### Cleanup Controller

The Cleanup Controller performs all the cleanup (deletion) tasks as a result of a `CleanupPolicy` or `ClusterCleanupPolicy`. It is the only other controller aside from the Admission Controller which uses and manages webhooks.

#### Webhook Server

This component validates `CleanupPolicy` and `ClusterCleanupPolicy` resources and serves as the endpoint for CronJobs to invoke the cleanup handler.

#### Webhook Controller

The webhook controller updates the webhook used by the cleanup controller when the Secret changes. This component also uses leader election.

#### Cleanup Handler

The cleanup handler deletes resources which match cleanup policies in response to being invoked by a CronJob.

#### CronJob Controller

In the cleanup process, the CronJob controller reconciles cleanup policies and existing CronJobs from installed cleanup policies. It requires leader election.

#### Cert Renewer

The certificate renewer controller is responsible for monitoring the Secrets used in the cleanup controller's webhooks. This controller uses leader election.
