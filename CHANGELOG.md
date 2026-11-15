# Changelog

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
