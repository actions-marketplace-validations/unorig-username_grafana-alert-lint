# grafana-alert-lint

A linter for Grafana-managed alert rules. It reads the three places those
rules usually live in Git:

- Grafana file provisioning YAML, and the JSON or YAML from Grafana's export endpoint
- JSON from `GET /api/v1/provisioning/alert-rules` (one rule or an array)
- Terraform `grafana_rule_group` resources

It parses every Prometheus query with the real PromQL parser, follows the
chain of server-side expressions to the rule's condition, and reports
mistakes that page people at 3am, or keep them from being paged when they
should be.

## Why

Prometheus rule files have a good linter in [pint](https://github.com/cloudflare/pint).
Grafana-managed rules have nothing comparable. They hide PromQL inside
`data[].model.expr`, decide firing in a separate math, reduce or threshold
expression, and carry Grafana-only settings like `noDataState` and
`execErrState` that decide what happens when a scrape breaks or a datasource
times out. Those settings are where a lot of bad pages come from.

## Install

```sh
go install github.com/unorig-username/grafana-alert-lint/cmd/grafana-alert-lint@latest
```

Needs Go 1.27.1 or newer. Or download a binary for Linux, macOS or Windows
from [Releases](https://github.com/unorig-username/grafana-alert-lint/releases);
each release has a `checksums.txt`.

## Usage

```
grafana-alert-lint [flags] PATH...
```

Paths can be files or directories. Directories are searched for `.yaml`,
`.yml`, `.json` and `.tf` files; hidden directories such as `.git` and
`.terraform` are skipped. Files that hold no Grafana alert rules (Prometheus
rule files, workflows, contact points, Kubernetes manifests) are ignored. A
named file or directory with no rules at all gets a note on stderr.

```
Flags:
  -disable value
    	check ID to skip, e.g. GAL009 (repeatable)
  -fail-on string
    	exit 1 on findings at or above this severity: error, warning or info (default "warning")
  -format string
    	output format: text, github, json or sarif (default "text")
  -require-label value
    	label every rule must carry, e.g. team (repeatable, enables GAL008)
  -scrape-interval duration
    	Prometheus scrape interval, for the rate window check (GAL005) (default 1m0s)
  -version
    	print the version and exit
```

Exit status: `0` clean, `1` findings at or above `--fail-on`, `2` usage or I/O error.

A file named on the command line that does not parse is an error (exit 2).
A file found by walking a directory that does not parse is skipped with a
note on stderr, because a tree can hold YAML that is not alert rules (Helm
templates, for one).

### Example

From [`testdata/output/sample.yaml`](testdata/output/sample.yaml):

```console
$ grafana-alert-lint --require-label team testdata/output/sample.yaml
testdata/output/sample.yaml:17: warning GAL001 Checkout: 5xx ratio, all regions: ratio of rates with no traffic guard: at low traffic one failed request is a 100% error rate. Add `and on() sum(rate(...)) > N` or a guard grouped by the ratio's labels, clamp_min the denominator, or combine a request-rate query in the condition.
testdata/output/sample.yaml:30: warning GAL003 Checkout: 5xx ratio, all regions: execErrState is Alerting: a datasource timeout or query error pages as an outage. Use Error or KeepLast.
testdata/output/sample.yaml:34: info GAL007 Checkout: 5xx ratio, all regions: no runbook_url annotation: the responder has to find the runbook while paged. Link one.
testdata/output/sample.yaml:43: error GAL006 Checkout p99 latency: histogram_quantile over sum by (service) drops the le label: Prometheus skips bucket series without le, so the query returns nothing and the alert never fires. Keep le in the grouping.
testdata/output/sample.yaml:61: error GAL008 Checkout p99 latency: missing required label "team": notification policies route on it, so this alert falls through to the default policy.
$ echo $?
1
```

Text output is `path:line: severity ID rule-title: message`. Lines point at
the field that is wrong: the `expr`, the `noDataState`, the `for`. When a
rule field is missing, the line is the start of the rule. When a key is
missing from `labels` or `annotations`, the line is where that map starts.

The other formats:

- `github`: workflow commands (`::warning file=...,line=...,title=GAL001 traffic-guard::...`) that GitHub shows as annotations on the pull request diff.
- `json`: an array of `{file, line, id, check, severity, rule, uid, message}`.
- `sarif`: SARIF 2.1.0, for GitHub code scanning. Each check carries a `helpUri` to its section below.

## Checks

| ID | Name | Severity |
|----|------|----------|
| [GAL001](#gal001-traffic-guard) | traffic-guard | warning |
| [GAL002](#gal002-no-data-state) | no-data-state | warning |
| [GAL003](#gal003-exec-error-state) | exec-error-state | warning |
| [GAL004](#gal004-pending-period) | pending-period | warning |
| [GAL005](#gal005-rate-window) | rate-window | warning |
| [GAL006](#gal006-histogram-quantile-le) | histogram-quantile-le | error |
| [GAL007](#gal007-annotations) | annotations | warning (summary), info (runbook_url) |
| [GAL008](#gal008-required-labels) | required-labels | error (labels), warning (empty receiver) |
| [GAL009](#gal009-range-query) | range-query | info |
| [GAL010](#gal010-parse-error) | parse-error | error |

The checks lean toward silence. A false positive teaches people to ignore
the linter, so when a check cannot tell, it says nothing. Values the linter
cannot read statically (Terraform references, and YAML aliases or merge keys
such as `<<: *defaults`) are unknown, and checks that need them report nothing.

A query counts as Prometheus when its model has an `expr`, its datasource
is not a server-side expression (`__expr__`), and `model.datasource.type`,
if present, names a Prometheus-type datasource.

### GAL001 traffic-guard

Overnight, checkout serves a handful of requests. One times out. The 5xx
ratio for that window is one failure over one request, 100%, and someone is
paged for a single request.

Flags a division where both sides call `rate`, `irate` or `increase`, and
any `histogram_quantile`, unless one of these guards is present:

- an `and` or `unless` whose other side compares a request rate with a
  number, e.g. `... and on() sum(rate(http_requests_total[5m])) > 1`;
- `clamp_min` in the denominator;
- a second query in the rule that the condition reads through math, reduce,
  threshold, resample or classic conditions, e.g. condition `$A > 0.05 && $B > 1`
  where `B` is a request rate.

`and` matches on the full label set. If the ratio is grouped, say
`sum by (route) (...)`, a guard of `and sum(rate(...)) > 1` has no labels,
matches nothing, and the alert can never fire. Use `and on()` for one global
guard, or group the guard by the same labels for one guard per route.

Limits: it does not check that the guard's labels match the ratio's, so the
broken guard above counts as a guard. It does not judge whether the guard's
threshold is sensible, or whether the guard measures the same service. A request rate is a `rate`,
`irate` or `increase` call, or a metric whose name contains `rate` (a
recording rule such as `job:http_requests:rate5m`), so a recording rule
named for an error rate also counts as a guard. When another query in the
rule could be the guard but the condition chain cannot be resolved (unknown
refIds, SQL expressions, Terraform references), it assumes it is.

### GAL002 no-data-state

A relabeling change renames a label and every checkout series disappears.
With `noDataState: Alerting`, every rule on those series pages at once, for
a service that is fine. With `noDataState: OK` the failure is quieter:
checkout dies, its metrics go with it, and the rule reports healthy.

Flags `Alerting` always. Flags `OK` unless a query in the rule can return
nothing while healthy: it filters (a comparison without `bool`, `and`,
`unless`) or calls `absent` or `absent_over_time`. For those rules, no data
is the healthy state and `OK` is right. Recommends `NoData` or `KeepLast`.

Limits: a ratio whose numerator series only exist after the first error
(`code=~"5.."`) also returns nothing on a healthy service, and the check
cannot see that from the query. Give the numerator a zero fallback with the
denominator's labels, `(sum by (route) (rate(errors[5m])) or 0 * sum by (route) (rate(total[5m]))) / sum by (route) (rate(total[5m]))`,
or disable the check for those rules. `or vector(0)` only works for a ratio
with no `by`: its result has no labels, so it matches no grouped denominator
and the ratio stays empty. Rules with non-Prometheus queries are not checked
for `OK`.

### GAL003 exec-error-state

Prometheus restarts during a deploy and queries fail for a short while. With
`execErrState: Alerting`, every rule that reads it pages as if the services
were down. With `OK`, a query that fails on every evaluation (it got too
expensive and times out) reports healthy forever.

Flags `Alerting`, `OK`, and a missing `execErrState`. Missing is flagged
because both the
[file provisioning docs](https://grafana.com/docs/grafana/latest/alerting/set-up/provision-alerting-resources/file-provisioning/)
and the
[Terraform provider docs](https://registry.terraform.io/providers/grafana/grafana/latest/docs/resources/rule_group)
give `Alerting` as the default. Recommends `Error` or `KeepLast`.

### GAL004 pending-period

One slow scrape pushes the p99 over the line for a single evaluation. With
no pending period, that evaluation pages, and the alert has resolved before
anyone opens a laptop.

Flags `for` when it is missing, empty, or zero (`0`, `0s`, `0m`).

Limits: it does not compare `for` with the group's evaluation interval.
Rules that should fire on the first evaluation can use `--disable GAL004`.

### GAL005 rate-window

`rate(x[1m])` with a 60s scrape interval sees one or two samples. Miss one
scrape and the rate is empty: the alert resolves, then fires again on the
next evaluation.

Flags `rate`, `irate`, `increase`, `delta` and `idelta` over a window
shorter than 4 times `--scrape-interval` (default `60s`). Over a subquery,
the window is compared with 4 times the subquery's step instead, since its
samples are steps, not scrapes. `offset` and `@` move the window and do not
change the answer.

Skipped: subqueries without an explicit step (they use the global
evaluation interval, unknown here), duration expressions, and Grafana's
`$__interval`, `$__rate_interval` and `$__range` as a window or a subquery
step, since Grafana fills them in.

Limits: one scrape interval for every query. Targets scraped more slowly
than the flag are not caught.

### GAL006 histogram-quantile-le

Someone adds `by (service)` to a p99 query so the page says which service.
`le` is no longer in the grouping. Prometheus skips bucket series without
an `le` label, the query returns nothing, and the latency alert can never
fire again. Nothing tells you until the incident it should have caught.

Flags `histogram_quantile` over `sum`, `avg`, `min`, `max`, `count`,
`group`, `stddev`, `stdvar` or `quantile` when `by (...)` lacks `le`, or
`without (...)` includes it (`sum(...)` with no grouping drops `le` too).

Skipped: non-aggregated arguments, `topk` and friends (they keep labels),
and aggregations that read no `_bucket` series. Native histograms have no
`le` label, so `histogram_quantile(0.99, sum by (service) (rate(x[5m])))`
over a native histogram is fine and not flagged. The cost: classic buckets
read through a recording rule without `_bucket` in its name are missed.

### GAL007 annotations

The page says "Checkout p99 latency" and nothing else. The first minutes of
the incident go to working out what the rule measures and where the runbook
lives.

Warns when `summary` is missing or empty. Notes (info) when `runbook_url`
is missing or empty.

Limits: it checks that the keys are set, not that templates render or
links resolve.

### GAL008 required-labels

Notification policies route on `team`. A new rule ships without it, falls
through to the default policy, and pages a channel nobody watches.

With `--require-label` (repeatable), every rule must carry those labels
with a non-empty value. Off by default.

Always on: a warning when `notification_settings` is present but its
`receiver` (Terraform: `contact_point`, or `policy`) is empty.

### GAL009 range-query

A rule fetches a whole time range on every evaluation, then a `last`
reducer throws away every point but the newest. An instant query returns
that point directly.

Flags a Prometheus query with `range: true` (or `instant: false` without
`range: false`) when every expression that reads it is a `reduce` or classic
condition with `last`. If anything else reads it (a `mean` or `max`
reducer, math, resample, or an expression the linter cannot read), the
range is needed and the query is not flagged.

### GAL010 parse-error

A hand edit drops a closing parenthesis. Nothing in a typical provisioning
or Terraform pipeline parses PromQL, so the rule fails on every evaluation
from then on.

Flags PromQL that does not parse, a `condition` that names no data entry,
and a rule with no condition.

Limits: a query that does not parse and has a `|` outside its string
literals is assumed to be LogQL from a Loki datasource that did not set
`model.datasource.type`, and is not reported. A `|` inside a regex matcher
such as `code=~"500|502"` does not count.

## Terraform

Rules come from `rule` blocks inside `resource "grafana_rule_group"`. The
linter evaluates attributes statically:

- `model = jsonencode({...})` is evaluated key by key, so one variable in the
  model does not hide the rest. Heredoc and quoted JSON strings work too, with
  line numbers inside the heredoc.
- `jsonencode`, `jsondecode`, `merge`, `format`, `join`, `lower` and `upper`
  are evaluated when their arguments are known.
- Anything that refers to `var.`, `local.`, `data.`, `each.`, another
  resource, or an unsupported function is unknown. Checks that need an
  unknown value report nothing. `labels = merge(local.common, {...})` is
  unknown, so `--require-label` skips that rule rather than guess.
- `dynamic "rule"` blocks are skipped. A `dynamic "data"` block marks the
  rule's data as incomplete, which turns off the checks that need every query.

Locals are not resolved, modules are not followed, and `.tf.json` files are
not read. Neither are rules embedded as a string inside a Kubernetes
ConfigMap (the Grafana Helm chart's sidecar pattern): lint the source file
the ConfigMap is built from. [`testdata/terraform/unknown.tf`](testdata/terraform/unknown.tf)
is the test fixture for all of this.

## In CI

### GitHub Action

```yaml
name: alert-rules
on: pull_request

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7.0.1
      - uses: unorig-username/grafana-alert-lint@v0.1.1
        with:
          path: alerting/ terraform/grafana/
          args: --require-label team --require-label severity
```

Inputs: `path` (space-separated, default `.`), `format` (default `github`,
which annotates the diff), `fail-on` (default `warning`), and `args` for any
other flags.

Pinned to a release tag, the action downloads that release's binary and
checks it against `checksums.txt`, so it takes a few seconds and leaves the
job's Go alone. If you pin to a commit SHA, also set `version` to the
matching tag:

```yaml
      - uses: unorig-username/grafana-alert-lint@<commit-sha> # v0.1.1
        with:
          version: v0.1.1
```

Without a tag or `version` (a branch pin, or a SHA pin with no `version`),
the action builds from source with `actions/setup-go`, which also sets the
job's Go version for later steps.

### Code scanning (SARIF)

```yaml
jobs:
  lint:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      security-events: write
    steps:
      - uses: actions/checkout@v7.0.1
      - uses: actions/setup-go@v7.0.0
        with:
          go-version: stable
      - run: go install github.com/unorig-username/grafana-alert-lint/cmd/grafana-alert-lint@v0.1.1
      # Exit 1 means findings; upload them anyway. Exit 2 still fails the step.
      - run: grafana-alert-lint --format sarif alerting/ > alert-lint.sarif || [ $? -eq 1 ]
      - uses: github/codeql-action/upload-sarif@v4.39.3
        with:
          sarif_file: alert-lint.sarif
```

## How this differs from pint

[pint](https://github.com/cloudflare/pint) is the better tool where it
applies, and it applies to different files.

- pint reads Prometheus rule files: the `groups[].rules[]` with `alert` and
  `expr` that Prometheus, Thanos and Mimir rulers load. It does not read
  Grafana-managed rules, and this tool does not read Prometheus rule files.
- pint can query live Prometheus servers: whether series exist, metric
  types, the real scrape interval, query cost. grafana-alert-lint is static.
  It never connects to anything, so it cannot tell you a metric is
  misspelled or that a label was renamed last week.
- pint has many more checks and years of use behind it. This tool has ten.
- Some checks overlap. pint's `alerts/annotation` and `rule/label` checks
  cover what GAL007 and GAL008 do, with more options. The rate window check
  overlaps too. pint's `promql/rate` reads `scrape_interval`
  from Prometheus and reports a bug under 2x, the minimum for two samples.
  GAL005 takes the interval from a flag and warns under 4x, which leaves
  room for missed scrapes.
- What this tool knows that pint has no reason to: Grafana's `noDataState`
  and `execErrState`, the data and expression chain that leads to the
  condition (so a traffic guard can live in a separate query), and Terraform
  and API JSON as input.

If your alerts can live in Prometheus rule files, use pint. If they live in
Grafana, use this. If you have both, run both.

## License

MIT. See [LICENSE](LICENSE).
