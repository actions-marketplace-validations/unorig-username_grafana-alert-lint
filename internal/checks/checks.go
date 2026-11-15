// Package checks lints Grafana-managed alert rules.
package checks

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/prometheus/prometheus/promql/parser"

	"github.com/unorig-username/grafana-alert-lint/internal/rule"
)

// Severity of a finding.
type Severity int

// Severities, lowest first.
const (
	Info Severity = iota
	Warning
	Error
)

func (s Severity) String() string {
	return [...]string{"info", "warning", "error"}[s]
}

// MarshalText writes the severity as its name in JSON.
func (s Severity) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// ParseSeverity parses "info", "warning" or "error".
func ParseSeverity(s string) (Severity, bool) {
	for _, v := range []Severity{Info, Warning, Error} {
		if v.String() == s {
			return v, true
		}
	}
	return 0, false
}

// Finding is one problem in one rule.
type Finding struct {
	File     string   `json:"file"`
	Line     int      `json:"line"`
	ID       string   `json:"id"`
	Check    string   `json:"check"`
	Severity Severity `json:"severity"`
	Rule     string   `json:"rule"`
	UID      string   `json:"uid,omitempty"`
	Message  string   `json:"message"`
}

// Check describes one lint check.
type Check struct {
	ID       string
	Name     string
	Severity Severity // the highest severity it reports
	Summary  string
	run      func(*ruleCtx)
}

// Checks lists every check in ID order.
var Checks = []Check{
	{"GAL001", "traffic-guard", Warning, "Ratio or histogram_quantile alert without a traffic guard", trafficGuard},
	{"GAL002", "no-data-state", Warning, "noDataState is Alerting or OK", noDataState},
	{"GAL003", "exec-error-state", Warning, "execErrState is Alerting, OK or unset (defaults to Alerting)", execErrState},
	{"GAL004", "pending-period", Warning, "No pending period (for missing or zero)", pendingPeriod},
	{"GAL005", "rate-window", Warning, "rate-family window shorter than 4 scrape intervals", rateWindow},
	{"GAL006", "histogram-quantile-le", Error, "histogram_quantile over an aggregation that drops le", histogramLe},
	{"GAL007", "annotations", Warning, "Missing summary or runbook_url annotation", annotations},
	{"GAL008", "required-labels", Error, "Missing routing labels or empty notification receiver", requiredLabels},
	{"GAL009", "range-query", Info, "Range query reduced with last; an instant query is cheaper", rangeQuery},
	{"GAL010", "parse-error", Error, "PromQL does not parse or condition matches no data entry", parseError},
}

// Config tunes the checks.
type Config struct {
	ScrapeInterval time.Duration
	RequireLabels  []string
	Disabled       map[string]bool
}

// Run lints rules and returns findings sorted by file, line and ID.
func Run(cfg Config, rules []rule.Rule) []Finding {
	var out []Finding
	for i := range rules {
		c := newCtx(cfg, &rules[i])
		for _, ch := range Checks {
			if !cfg.Disabled[ch.ID] {
				c.check = ch
				ch.run(c)
			}
		}
		out = append(out, c.findings...)
	}
	slices.SortStableFunc(out, func(a, b Finding) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.ID, b.ID), cmp.Compare(a.Message, b.Message))
	})
	return slices.Compact(out)
}

type promQuery struct {
	q    *rule.Query
	expr parser.Expr // nil when the expr is unknown or does not parse
	err  error
}

type ruleCtx struct {
	*rule.Rule
	cfg      Config
	prom     []promQuery
	check    Check
	findings []Finding
}

var promParser = parser.NewParser(parser.Options{EnableExperimentalFunctions: true})

func newCtx(cfg Config, r *rule.Rule) *ruleCtx {
	c := &ruleCtx{Rule: r, cfg: cfg}
	for i := range r.Data {
		q := &r.Data[i]
		if !q.IsPrometheus() {
			continue
		}
		p := promQuery{q: q}
		if q.Expr.Known() {
			p.expr, p.err = promParser.ParseExpr(grafanaMacros(q.Expr.S))
			if p.err != nil {
				p.expr = nil
			}
		}
		c.prom = append(c.prom, p)
	}
	return c
}

func (c *ruleCtx) add(sev Severity, line int, format string, args ...any) {
	if line == 0 {
		line = c.Line
	}
	title := c.Title
	if title == "" {
		title = cmp.Or(c.UID, "untitled")
	}
	c.findings = append(c.findings, Finding{
		File: c.File, Line: line, ID: c.check.ID, Check: c.check.Name, Severity: sev,
		Rule: title, UID: c.UID, Message: fmt.Sprintf(format, args...),
	})
}

// query returns the data entry with the given refId.
func (c *ruleCtx) query(ref string) *rule.Query {
	for i := range c.Data {
		if c.Data[i].RefID.S == ref {
			return &c.Data[i]
		}
	}
	return nil
}

// unknownRefs reports whether some data entries cannot be identified.
func (c *ruleCtx) unknownRefs() bool {
	if c.DataUnknown {
		return true
	}
	for _, q := range c.Data {
		if !q.RefID.Known() {
			return true
		}
	}
	return false
}

// sources returns the datasource queries that ref depends on, following
// server-side expressions. ok is false when the chain cannot be resolved.
func (c *ruleCtx) sources(ref string, seen map[string]bool) (map[string]bool, bool) {
	q := c.query(ref)
	if q == nil {
		return nil, false
	}
	if !q.IsExpression() {
		return map[string]bool{ref: true}, true
	}
	out := map[string]bool{}
	if seen[ref] {
		return out, true
	}
	seen[ref] = true
	refs, ok := q.Refs()
	if !ok {
		return nil, false
	}
	for _, r := range refs {
		sub, ok := c.sources(r, seen)
		if !ok {
			return nil, false
		}
		for k := range sub {
			out[k] = true
		}
	}
	return out, true
}

func equalFold(v rule.Value, s string) bool { return v.Known() && strings.EqualFold(v.S, s) }
