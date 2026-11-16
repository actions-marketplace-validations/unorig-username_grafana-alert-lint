# Changelog

## 0.1.1 (2026-11-15)

- Release binaries for Linux, macOS and Windows on amd64 and arm64, with
  SHA-256 checksums.
- The GitHub Action downloads the release binary when it's pinned to a tag,
  instead of building from source. It now takes seconds instead of about a
  minute, and no longer changes the job's Go version.
- New action input `version`, for workflows that pin the action to a commit
  SHA.

## 0.1.0 (2026-11-15)

First release.

- Ten checks, GAL001 to GAL010: traffic guards, noDataState, execErrState,
  pending period, rate windows, histogram_quantile and `le`, annotations,
  required labels and receivers, range queries reduced with `last`, and
  PromQL or condition errors.
- Inputs: Grafana file provisioning YAML, export JSON, provisioning API JSON
  (one rule or an array), and Terraform `grafana_rule_group` resources with
  unknown references skipped instead of guessed.
- Output: text, GitHub annotations, JSON and SARIF 2.1.0.
- Composite GitHub Action.
