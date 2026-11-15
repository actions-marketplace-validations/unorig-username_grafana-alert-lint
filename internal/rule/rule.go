// Package rule reads Grafana-managed alert rules from file provisioning
// YAML, provisioning API JSON and Terraform into one model.
package rule

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v4"
)

// unknownTag marks a node whose value is only known at apply time
// (a Terraform reference). Checks skip unknown values.
const unknownTag = "!unknown"

// Value is a scalar field read from a rule file.
type Value struct {
	S       string
	Line    int
	Set     bool // the field is present
	Unknown bool // present, but not statically known
}

// Known reports whether the field is present with a statically known value.
func (v Value) Known() bool { return v.Set && !v.Unknown }

// Rule is one Grafana-managed alert rule.
type Rule struct {
	File  string
	Line  int
	Title string
	UID   string

	Condition    Value
	For          Value
	NoDataState  Value
	ExecErrState Value
	Labels       Map
	Annotations  Map
	Notification *Notification

	Data []Query
	// DataUnknown is set when some data entries cannot be read
	// (Terraform dynamic blocks or references).
	DataUnknown bool
}

// Map is a labels or annotations map.
type Map struct {
	Line    int
	Set     bool
	Unknown bool
	Values  map[string]Value
}

// Notification is a rule's notification_settings block.
type Notification struct {
	Line     int
	Unknown  bool
	Receiver Value // receiver, or contact_point in Terraform
	Policy   Value // policy, Terraform only
}

// Query is one entry of a rule's data list.
type Query struct {
	Line          int
	RefID         Value
	DatasourceUID Value
	ModelUnknown  bool

	// Datasource query fields.
	Expr           Value
	DatasourceType string
	Instant, Range *bool

	// Server-side expression fields.
	Type       Value
	Expression Value
	Reducer    string
	Classic    []Classic
}

// Classic is one condition of a classic_conditions expression.
type Classic struct {
	RefID   string
	Reducer string
}

// IsExpression reports whether q is a Grafana server-side expression.
func (q *Query) IsExpression() bool {
	return q.DatasourceUID.Known() && (q.DatasourceUID.S == "__expr__" || q.DatasourceUID.S == "-100")
}

// IsPrometheus reports whether q looks like a Prometheus query: it has an
// expr and is neither a server-side expression nor a known non-Prometheus
// datasource such as Loki.
func (q *Query) IsPrometheus() bool {
	if q.IsExpression() || q.ModelUnknown || !q.Expr.Set {
		return false
	}
	return q.DatasourceType == "" || strings.Contains(q.DatasourceType, "prometheus")
}

var mathRef = regexp.MustCompile(`\$\{([^}]+)\}|\$([A-Za-z0-9_]+)`)

// Refs returns the refIds a server-side expression reads. ok is false when
// they cannot be determined.
func (q *Query) Refs() (refs []string, ok bool) {
	if q.ModelUnknown || !q.Type.Known() {
		return nil, false
	}
	switch q.Type.S {
	case "math":
		if !q.Expression.Known() {
			return nil, false
		}
		for _, m := range mathRef.FindAllStringSubmatch(q.Expression.S, -1) {
			refs = append(refs, m[1]+m[2])
		}
		return refs, true
	case "reduce", "threshold", "resample":
		if !q.Expression.Known() {
			return nil, false
		}
		return []string{strings.Trim(q.Expression.S, "${} ")}, true
	case "classic_conditions":
		for _, c := range q.Classic {
			refs = append(refs, c.RefID)
		}
		return refs, true
	}
	return nil, false
}

// LoadFile reads the alert rules in one file. The format is picked by
// extension, then by content. A file with no Grafana alert rules returns
// no rules and no error.
func LoadFile(path string) ([]Rule, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".tf" || (ext != ".yaml" && ext != ".yml" && ext != ".json" && bytes.Contains(src, []byte(`"grafana_rule_group"`))) {
		return parseTerraform(path, src)
	}
	return parseYAML(path, src)
}

// parseYAML reads provisioning YAML, export JSON and API JSON. JSON is
// parsed as YAML so every node carries its line.
func parseYAML(file string, src []byte) ([]Rule, error) {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var rules []Rule
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			return rules, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		if len(doc.Content) == 0 {
			continue
		}
		root := doc.Content[0]
		var candidates []*yaml.Node
		switch root.Kind {
		case yaml.MappingNode:
			// File provisioning and export: groups[].rules[]. A single rule
			// from GET /api/v1/provisioning/alert-rules/{uid} is a mapping too.
			for _, g := range seq(get(root, "groups")) {
				candidates = append(candidates, seq(get(g, "rules"))...)
			}
			candidates = append(candidates, root)
		case yaml.SequenceNode:
			// GET /api/v1/provisioning/alert-rules returns an array of rules.
			candidates = root.Content
		}
		for _, n := range candidates {
			if isAlertRule(n) {
				rules = append(rules, fromNode(file, n))
			}
		}
	}
}

// isAlertRule reports whether n looks like a Grafana alert rule: a data
// list, a title or condition, and no recording rule definition. The API
// sends "record": null on alert rules. Prometheus rule files (alert/expr),
// ConfigMaps (data is a map) and other JSON with a data list are not ours.
func isAlertRule(n *yaml.Node) bool {
	d, rec := get(n, "data"), get(n, "record")
	return d != nil && (d.Kind == yaml.SequenceNode || d.Kind == yaml.AliasNode) &&
		(get(n, "title") != nil || get(n, "condition") != nil) &&
		(rec == nil || rec.Tag == unknownTag || rec.Tag == "!!null")
}

func fromNode(file string, n *yaml.Node) Rule {
	r := Rule{
		File:         file,
		Line:         n.Line,
		Title:        str(get(n, "title")),
		UID:          str(get(n, "uid")),
		Condition:    val(get(n, "condition")),
		For:          val(get(n, "for")),
		NoDataState:  val(get(n, "noDataState")),
		ExecErrState: val(get(n, "execErrState")),
		Labels:       mapOf(get(n, "labels")),
		Annotations:  mapOf(get(n, "annotations")),
	}
	if ns := get(n, "notification_settings"); ns != nil && ns.Tag != "!!null" {
		r.Notification = &Notification{
			Line:     ns.Line,
			Unknown:  ns.Tag == unknownTag,
			Receiver: val(get(ns, "receiver")),
			Policy:   val(get(ns, "policy")),
		}
	}
	data := get(n, "data")
	if data != nil && data.Kind != yaml.SequenceNode {
		// A reference, an alias (data: *queries) or a merge.
		r.DataUnknown = true
	}
	for _, d := range seq(data) {
		if d.Tag == unknownTag {
			r.DataUnknown = true
			continue
		}
		r.Data = append(r.Data, query(d))
	}
	return r
}

func query(n *yaml.Node) Query {
	q := Query{Line: n.Line, RefID: val(get(n, "refId")), DatasourceUID: val(get(n, "datasourceUid"))}
	m := get(n, "model")
	if m == nil {
		return q
	}
	if m.Tag == unknownTag {
		q.ModelUnknown = true
		return q
	}
	q.Expr = val(get(m, "expr"))
	q.DatasourceType = str(get(get(m, "datasource"), "type"))
	q.Instant = boolp(get(m, "instant"))
	q.Range = boolp(get(m, "range"))
	q.Type = val(get(m, "type"))
	q.Expression = val(get(m, "expression"))
	q.Reducer = str(get(m, "reducer"))
	for _, c := range seq(get(m, "conditions")) {
		params := seq(get(get(c, "query"), "params"))
		if len(params) == 0 {
			continue
		}
		q.Classic = append(q.Classic, Classic{RefID: str(params[0]), Reducer: str(get(get(c, "reducer"), "type"))})
	}
	return q
}

func get(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	merged := false
	for i := 0; i+1 < len(n.Content); i += 2 {
		switch {
		case n.Content[i].Value == key:
			return n.Content[i+1]
		case n.Content[i].Tag == "!!merge":
			merged = true
		}
	}
	if merged {
		// shortcut: a key that may come from a YAML merge (<<: *anchor) is
		// unknown, not resolved. Resolve merges if this misses real problems.
		return scalar(unknownTag, "", n.Line)
	}
	return nil
}

func seq(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

func str(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag == unknownTag || n.Tag == "!!null" {
		return ""
	}
	return n.Value
}

func val(n *yaml.Node) Value {
	switch {
	case n == nil || n.Tag == "!!null":
		return Value{}
	case n.Kind != yaml.ScalarNode || n.Tag == unknownTag:
		// A list or map where a scalar belongs is not ours to judge.
		return Value{Line: n.Line, Set: true, Unknown: true}
	}
	return Value{S: n.Value, Line: n.Line, Set: true}
}

func boolp(n *yaml.Node) *bool {
	if n == nil || n.Tag != "!!bool" {
		return nil
	}
	b := n.Value == "true"
	return &b
}

func mapOf(n *yaml.Node) Map {
	if n == nil || n.Tag == "!!null" {
		return Map{}
	}
	m := Map{Line: n.Line, Set: true, Values: map[string]Value{}}
	if n.Kind != yaml.MappingNode {
		m.Unknown = true
		return m
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		m.Values[n.Content[i].Value] = val(n.Content[i+1])
	}
	return m
}
