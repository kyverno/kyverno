# Controllers

This document contains the list on controllers in this repository.

Most controllers code can be found under `pkg/controllers` except for some legacy controllers.

| Name                             | Leader             | Description                                           |
|----------------------------------|--------------------|-------------------------------------------------------|
| `certmanager-controller`         | :white_check_mark: | Manages TLS certificates                              |
| `config-controller`              |                    | Watches config map and reloads config on changes      |
| `openapi-controller`             |                    | Polls discovery API and maintains APIs schemas        |
| `policycache-controller`         |                    | Maintains an up to date policy cache                  |
| `webhook-controller`             | :white_check_mark: | Configures webhooks                                   |
| `admission-report-controller`    | :white_check_mark: | Cleans up admission reports                           |
| `aggregate-report-controller`    | :white_check_mark: | Aggregates reports                                    |
| `background-scan-controller`     | :white_check_mark: | Manages background scans reports                      |
| `resource-report-controller`     | :white_check_mark: | Watches resources that participate in reports         |
| `cleanup-controller`             | :white_check_mark: | Reconciles cleanup policies and associated cron jobs  |
| `policy-controller`              | :white_check_mark: | Manages mutation of existing resources                |

