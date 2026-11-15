package checks

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/promql/parser"

	"github.com/unorig-username/grafana-alert-lint/internal/rule"
)

// GAL001: a ratio of rates, or histogram_quantile, with no traffic guard.
func trafficGuard(c *ruleCtx) {
	for _, p := range c.prom {
		if p.expr == nil {
			continue
		}
		var msg string
		parser.Inspect(p.expr, func(n parser.Node, path []parser.Node) error {
			switch {
			case msg != "":
				return errStop
			case isRatio(n):
				if !guardedInQuery(n, path) && !anyNode(n.(*parser.BinaryExpr).RHS, isCall("clamp_min")) {
					msg = "ratio of rates with no traffic guard: at low traffic one failed request is a 100% error rate. " +
						"Add `and on() sum(rate(...)) > N` or a guard grouped by the ratio's labels, clamp_min the denominator, or combine a request-rate query in the condition."
				}
			case isCall("histogram_quantile")(n):
				if !guardedInQuery(n, path) {
					msg = "histogram_quantile with no traffic guard: with a handful of requests, one slow request sets the quantile. " +
						"Add `and on() sum(rate(..._count[5m])) > N` or a guard grouped by the quantile's labels, or combine a request-rate query in the condition."
				}
			}
			return nil
		})
		if msg != "" && !c.guardedByCondition(p) {
			c.add(Warning, p.q.Expr.Line, "%s", msg)
		}
	}
}

// guardedInQuery reports whether an and/unless above n has a traffic
// comparison on its other side.
func guardedInQuery(n parser.Node, path []parser.Node) bool {
	child := n
	for i := len(path) - 1; i >= 0; i-- {
		if b, ok := path[i].(*parser.BinaryExpr); ok && (b.Op == parser.LAND || b.Op == parser.LUNLESS) {
			other := parser.Node(b.RHS)
			if child == parser.Node(b.RHS) {
				other = b.LHS
			}
			if isTrafficGuard(other) {
				return true
			}
		}
		child = path[i]
	}
	return false
}

// guardedByCondition reports whether the rule condition may combine p
// with another datasource query that could be a request-rate guard.
func (c *ruleCtx) guardedByCondition(p promQuery) bool {
	if c.unknownRefs() {
		return true
	}
	var others []string
	for i := range c.Data {
		q := &c.Data[i]
		if q == p.q || q.IsExpression() {
			continue
		}
		if e := c.parsed(q); e != nil && (!rateLike(e) || risky(e)) {
			continue
		}
		others = append(others, q.RefID.S)
	}
	if len(others) == 0 {
		return false
	}
	if !c.Condition.Known() {
		return true
	}
	src, ok := c.sources(c.Condition.S, map[string]bool{})
	if !ok || !src[p.q.RefID.S] {
		return true
	}
	return slices.ContainsFunc(others, func(r string) bool { return src[r] })
}

func (c *ruleCtx) parsed(q *rule.Query) parser.Expr {
	for _, p := range c.prom {
		if p.q == q {
			return p.expr
		}
	}
	return nil
}

// GAL002
func noDataState(c *ruleCtx) {
	v := c.NoDataState
	switch {
	case equalFold(v, "Alerting"):
		c.add(Warning, v.Line, "noDataState is Alerting: a broken scrape or renamed metric pages as if the service were down. "+
			"Use NoData or KeepLast, and alert on scrape health separately.")
	case equalFold(v, "OK") && !c.mayBeEmptyWhenHealthy():
		c.add(Warning, v.Line, "noDataState is OK: if the metric disappears because the service or its exporter died, the rule reports healthy and never fires. "+
			"Use NoData or KeepLast.")
	}
}

// mayBeEmptyWhenHealthy reports whether a query can return no series while
// all is well (it filters, or uses absent), which makes noDataState OK a
// deliberate choice.
func (c *ruleCtx) mayBeEmptyWhenHealthy() bool {
	if c.DataUnknown {
		return true
	}
	for i := range c.Data {
		q := &c.Data[i]
		if q.IsExpression() {
			continue
		}
		e := c.parsed(q)
		if e == nil || anyNode(e, filters) {
			return true
		}
	}
	return false
}

func filters(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.BinaryExpr:
		return (n.Op.IsComparisonOperator() && !n.ReturnBool) || n.Op == parser.LAND || n.Op == parser.LUNLESS
	case *parser.Call:
		return n.Func.Name == "absent" || n.Func.Name == "absent_over_time"
	}
	return false
}

// GAL003
func execErrState(c *ruleCtx) {
	v := c.ExecErrState
	switch {
	case !v.Set:
		c.add(Warning, 0, "execErrState is not set, and Grafana file provisioning and the Terraform provider default it to Alerting: "+
			"a datasource timeout pages as an outage. Set Error or KeepLast.")
	case equalFold(v, "Alerting"):
		c.add(Warning, v.Line, "execErrState is Alerting: a datasource timeout or query error pages as an outage. Use Error or KeepLast.")
	case equalFold(v, "OK"):
		c.add(Warning, v.Line, "execErrState is OK: a query that fails every time reports healthy, so the rule can never fire. Use Error or KeepLast.")
	}
}

// GAL004
func pendingPeriod(c *ruleCtx) {
	v := c.For
	switch {
	case !v.Set:
		c.add(Warning, 0, "no pending period (for is not set): one bad evaluation pages. Set for to longer than one evaluation interval.")
	case v.Known() && isZeroDuration(v.S):
		c.add(Warning, v.Line, "pending period is %q: one bad evaluation pages. Set for to longer than one evaluation interval.", v.S)
	}
}

func isZeroDuration(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	if d, err := model.ParseDuration(s); err == nil {
		return d == 0
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d == 0
	}
	// API JSON may carry for as a number of nanoseconds.
	n, err := strconv.ParseFloat(s, 64)
	return err == nil && n == 0
}

var rateFamily = map[string]bool{"rate": true, "irate": true, "increase": true, "delta": true, "idelta": true}

// GAL005
func rateWindow(c *ruleCtx) {
	scrape := c.cfg.ScrapeInterval
	for _, p := range c.prom {
		if p.expr == nil {
			continue
		}
		parser.Inspect(p.expr, func(n parser.Node, _ []parser.Node) error {
			call, ok := n.(*parser.Call)
			if !ok || !rateFamily[call.Func.Name] || len(call.Args) != 1 {
				return nil
			}
			switch a := strip(call.Args[0]).(type) {
			case *parser.MatrixSelector:
				// Offsets and @ modifiers move the window, they do not resize it.
				if a.RangeExpr == nil && a.Range != macroRange && a.Range < 4*scrape {
					c.add(Warning, p.q.Expr.Line, "%s: the %s window holds fewer than 4 scrapes at a %s scrape interval, so one missed scrape leaves too few samples and the rule flaps or goes quiet. Use [%s] or longer.",
						call, dur(a.Range), dur(scrape), dur(4*scrape))
				}
			case *parser.SubqueryExpr:
				// A subquery's samples are its steps, not scrapes. Without an
				// explicit step it uses the global evaluation interval, unknown here.
				if a.RangeExpr == nil && a.StepExpr == nil && a.Step > 0 && a.Range != macroRange && a.Step != macroRange && a.Range < 4*a.Step {
					c.add(Warning, p.q.Expr.Line, "%s: the %s subquery range holds fewer than 4 of its %s steps, so the result is noisy or empty. Use [%s:%s] or longer.",
						call, dur(a.Range), dur(a.Step), dur(4*a.Step), dur(a.Step))
				}
			}
			return nil
		})
	}
}

func dur(d time.Duration) string { return model.Duration(d).String() }

// labelDropping aggregations collapse series to their grouping labels.
// topk, bottomk, limitk and count_values keep or add labels.
var labelDropping = map[parser.ItemType]bool{
	parser.SUM: true, parser.AVG: true, parser.MIN: true, parser.MAX: true, parser.COUNT: true,
	parser.GROUP: true, parser.STDDEV: true, parser.STDVAR: true, parser.QUANTILE: true,
}

// GAL006
func histogramLe(c *ruleCtx) {
	for _, p := range c.prom {
		if p.expr == nil {
			continue
		}
		parser.Inspect(p.expr, func(n parser.Node, _ []parser.Node) error {
			call, ok := n.(*parser.Call)
			if !ok || call.Func.Name != "histogram_quantile" || len(call.Args) != 2 {
				return nil
			}
			// Only aggregations over classic _bucket series: native
			// histograms have no le label to keep.
			agg, ok := strip(call.Args[1]).(*parser.AggregateExpr)
			if !ok || !labelDropping[agg.Op] || !anyNode(agg.Expr, isBucketSelector) {
				return nil
			}
			if agg.Without == slices.Contains(agg.Grouping, "le") {
				c.add(Error, p.q.Expr.Line, "histogram_quantile over %s drops the le label: Prometheus skips bucket series without le, so the query returns nothing and the alert never fires. Keep le in the grouping.",
					grouping(agg))
			}
			return nil
		})
	}
}

func grouping(a *parser.AggregateExpr) string {
	s := a.Op.String()
	switch {
	case a.Without:
		s += " without (" + strings.Join(a.Grouping, ", ") + ")"
	case len(a.Grouping) > 0:
		s += " by (" + strings.Join(a.Grouping, ", ") + ")"
	}
	return s
}

func isBucketSelector(n parser.Node) bool {
	vs, ok := n.(*parser.VectorSelector)
	if !ok {
		return false
	}
	if strings.HasSuffix(vs.Name, "_bucket") {
		return true
	}
	for _, m := range vs.LabelMatchers {
		if m.Name == "__name__" && strings.HasSuffix(m.Value, "_bucket") {
			return true
		}
	}
	return false
}

// GAL007
func annotations(c *ruleCtx) {
	a := c.Annotations
	if a.Unknown {
		return
	}
	if missing(a, "summary") {
		c.add(Warning, a.Line, "no summary annotation: the page arrives with only the rule title. Say what is broken and who is affected.")
	}
	if missing(a, "runbook_url") {
		c.add(Info, a.Line, "no runbook_url annotation: the responder has to find the runbook while paged. Link one.")
	}
}

func missing(m rule.Map, key string) bool {
	v, ok := m.Values[key]
	return !ok || blank(v)
}

func blank(v rule.Value) bool { return !v.Set || (v.Known() && strings.TrimSpace(v.S) == "") }

// GAL008
func requiredLabels(c *ruleCtx) {
	if !c.Labels.Unknown {
		for _, l := range c.cfg.RequireLabels {
			if missing(c.Labels, l) {
				c.add(Error, c.Labels.Line, "missing required label %q: notification policies route on it, so this alert falls through to the default policy.", l)
			}
		}
	}
	if n := c.Notification; n != nil && !n.Unknown && blank(n.Receiver) && blank(n.Policy) {
		c.add(Warning, n.Line, "notification_settings has an empty receiver (contact_point in Terraform): the alert has nowhere to go. "+
			"Set it, or drop notification_settings to route by notification policies.")
	}
}

// GAL009
func rangeQuery(c *ruleCtx) {
	for _, p := range c.prom {
		q := p.q
		isRange := (q.Range != nil && *q.Range) || (q.Instant != nil && !*q.Instant && q.Range == nil)
		if !isRange || !q.RefID.Known() || c.unknownRefs() {
			continue
		}
		if by := onlyLastReaders(c.Data, q.RefID.S); by != "" {
			c.add(Info, q.Line, "query %s is a range query and %s keeps only its last value: an instant query returns that value without fetching the whole time range. Set instant: true and range: false.",
				q.RefID.S, by)
		}
	}
}

// onlyLastReaders returns the refId of the first expression that reads ref,
// if every expression that reads it keeps only the last value. Any other
// reader (math, resample, another reducer, or one it cannot read) needs the
// range, and the result is "".
func onlyLastReaders(data []rule.Query, ref string) string {
	by := ""
	for i := range data {
		e := &data[i]
		if !e.IsExpression() {
			if !e.DatasourceUID.Known() {
				return "" // may be an expression we cannot see
			}
			continue
		}
		refs, ok := e.Refs()
		if !ok {
			return ""
		}
		if !slices.Contains(refs, ref) {
			continue
		}
		if !keepsLast(e, ref) {
			return ""
		}
		if by == "" {
			by = e.RefID.S
		}
	}
	return by
}

// keepsLast reports whether e reduces ref with "last" and nothing else.
func keepsLast(e *rule.Query, ref string) bool {
	switch {
	case equalFold(e.Type, "reduce"):
		return e.Reducer == "last"
	case equalFold(e.Type, "classic_conditions"):
		for _, cc := range e.Classic {
			if cc.RefID == ref && cc.Reducer != "last" {
				return false
			}
		}
		return true
	}
	return false
}

// GAL010
func parseError(c *ruleCtx) {
	for _, p := range c.prom {
		// A | outside string literals is not PromQL: it is a LogQL pipeline
		// from a Loki datasource that did not name its type.
		if p.err != nil && !strings.Contains(quoted.ReplaceAllString(p.q.Expr.S, ""), "|") {
			c.add(Error, p.q.Expr.Line, "PromQL does not parse: %s", strings.ReplaceAll(p.err.Error(), "\n", "; "))
		}
	}
	if c.unknownRefs() {
		return
	}
	cond := c.Condition
	switch {
	case !cond.Set:
		c.add(Error, 0, "rule has no condition: Grafana needs the refId of the query or expression that decides firing.")
	case cond.Known() && c.query(cond.S) == nil:
		var refs []string
		for _, q := range c.Data {
			refs = append(refs, q.RefID.S)
		}
		c.add(Error, cond.Line, "condition %q matches no data entry (refIds: %s), so Grafana cannot evaluate the rule.", cond.S, strings.Join(refs, ", "))
	}
}

// PromQL helpers.

var errStop = errors.New("stop")

// quoted matches PromQL string literals, where | is regex alternation.
var quoted = regexp.MustCompile("\"(?:[^\"\\\\]|\\\\.)*\"|'(?:[^'\\\\]|\\\\.)*'|`[^`]*`")

var (
	macroNum = regexp.MustCompile(`\$\{?__(?:rate_interval|interval|range)_(?:ms|s)\}?`)
	macroDur = regexp.MustCompile(`\$\{?__(?:rate_interval|interval|range)\}?`)
)

// macroRange stands in for Grafana's $__interval, $__rate_interval and
// $__range so the query parses. GAL005 skips windows equal to it.
const macroRange = 7777 * time.Hour

func grafanaMacros(s string) string {
	s = macroNum.ReplaceAllString(s, "1")
	return macroDur.ReplaceAllString(s, "7777h")
}

func strip(e parser.Expr) parser.Expr {
	for {
		switch n := e.(type) {
		case *parser.ParenExpr:
			e = n.Expr
		case *parser.StepInvariantExpr:
			e = n.Expr
		default:
			return e
		}
	}
}

// anyNode reports whether f holds for n or any node below it.
func anyNode(n parser.Node, f func(parser.Node) bool) bool {
	found := false
	parser.Inspect(n, func(n parser.Node, _ []parser.Node) error {
		if n != nil && f(n) {
			found = true
			return errStop
		}
		return nil
	})
	return found
}

func isCall(names ...string) func(parser.Node) bool {
	return func(n parser.Node) bool {
		c, ok := n.(*parser.Call)
		return ok && slices.Contains(names, c.Func.Name)
	}
}

var hasRateCall = isCall("rate", "irate", "increase")

func hasRate(n parser.Node) bool { return anyNode(n, hasRateCall) }

// rateLike: a rate call, or a recording rule named like one (job:requests:rate5m).
func rateLike(n parser.Node) bool {
	return anyNode(n, func(n parser.Node) bool {
		vs, ok := n.(*parser.VectorSelector)
		return hasRateCall(n) || (ok && strings.Contains(vs.Name, "rate"))
	})
}

func isRatio(n parser.Node) bool {
	b, ok := n.(*parser.BinaryExpr)
	return ok && b.Op == parser.DIV && hasRate(b.LHS) && hasRate(b.RHS)
}

func risky(n parser.Node) bool {
	return anyNode(n, func(n parser.Node) bool { return isRatio(n) || isCall("histogram_quantile")(n) })
}

func isNumber(e parser.Expr) bool {
	switch n := strip(e).(type) {
	case *parser.NumberLiteral:
		return true
	case *parser.UnaryExpr:
		return isNumber(n.Expr)
	case *parser.BinaryExpr:
		return isNumber(n.LHS) && isNumber(n.RHS)
	}
	return false
}

// isTrafficGuard reports whether n compares a request-rate expression (not
// itself a ratio or quantile) against a number.
func isTrafficGuard(n parser.Node) bool {
	return anyNode(n, func(n parser.Node) bool {
		b, ok := n.(*parser.BinaryExpr)
		if !ok || !b.Op.IsComparisonOperator() {
			return false
		}
		guard := func(x, num parser.Expr) bool { return isNumber(num) && rateLike(x) && !risky(x) }
		return guard(b.LHS, b.RHS) || guard(b.RHS, b.LHS)
	})
}
