# Kyverno Helm Chart Changelog

All notable changes to the kyverno Helm chart are documented here.
Version headings correspond to GitHub release tags of the form `kyverno-chart-<version>`.

## [Unreleased]

## [3.7.1] - 2026-01-28

### Fixed
- Ensure `spec.template.metadata` isn't null

### Removed
- Remove the `delete` permission for policyexceptions in the admission controller

### Changed
- Enable the flag `--generateValidatingAdmissionPolicy` by default in the admission controller
- Enable the flag `--validatingAdmissionPolicyReports` by default in the reports controller
