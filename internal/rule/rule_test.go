package rule

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const testdata = "../../testdata/"

func load(t *testing.T, path string) []Rule {
	t.Helper()
	rules, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%s): %v", path, err)
	}
	return rules
}

func write(t *testing.T, name, src string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadFormats(t *testing.T) {
	tests := []struct {
		path   string
		titles []string
	}{
		{"good/checkout.yaml", []string{"Checkout availability SLO burn", "Checkout p99 latency high", "Checkout target missing"}},
		{"good/api-rules.json", []string{"API 5xx ratio high", "API pod near memory limit"}},
		{"good/payments.tf", []string{"Payments availability SLO burn", "Payments settlement stalled"}},
		// The dynamic "rule" block is skipped.
		{"terraform/unknown.tf", []string{`"${each.key} error ratio"`, "Checkout error ratio", "Checkout latency"}},
		{"terraform/heredoc.tf", []string{"API traffic dropped"}},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			var titles []string
			for _, r := range load(t, testdata+tt.path) {
				titles = append(titles, r.Title)
			}
			if !slices.Equal(titles, tt.titles) {
				t.Errorf("titles = %q, want %q", titles, tt.titles)
			}
		})
	}
}

// The same rules in three formats must load to the same model, apart from lines.
func TestFormatsAgree(t *testing.T) {
	base := testdata + "bad/GAL009/"
	yamlRules := load(t, base+"provisioning.yaml")
	for _, f := range []string{"api.json", "terraform.tf"} {
		rules := load(t, base+f)
		if len(rules) != len(yamlRules) {
			t.Fatalf("%s: %d rules, want %d", f, len(rules), len(yamlRules))
		}
		for i, r := range rules {
			y := yamlRules[i]
			if r.Title != y.Title || r.UID != y.UID || r.Condition.S != y.Condition.S || r.For.S != y.For.S ||
				r.NoDataState.S != y.NoDataState.S || len(r.Data) != len(y.Data) {
				t.Errorf("%s rule %d differs from YAML: %+v vs %+v", f, i, r, y)
			}
			for j, q := range r.Data {
				yq := y.Data[j]
				if q.RefID.S != yq.RefID.S || q.Expr.S != yq.Expr.S || q.Type.S != yq.Type.S || q.Reducer != yq.Reducer ||
					!slices.Equal(q.Classic, yq.Classic) || !eqBool(q.Instant, yq.Instant) || !eqBool(q.Range, yq.Range) {
					t.Errorf("%s rule %d query %d differs: %+v vs %+v", f, i, j, q, yq)
				}
			}
		}
	}
}

func eqBool(a, b *bool) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }

func TestTerraformUnknowns(t *testing.T) {
	rules := load(t, testdata+"terraform/unknown.tf")
	r := rules[0]
	for name, v := range map[string]Value{
		"condition": r.Condition, "for": r.For, "no_data_state": r.NoDataState, "exec_err_state": r.ExecErrState,
		"contact_point": r.Notification.Receiver, "expr": r.Data[0].Expr, "datasource_uid": r.Data[0].DatasourceUID,
	} {
		if !v.Set || !v.Unknown {
			t.Errorf("%s = %+v, want set and unknown", name, v)
		}
	}
	if !r.Labels.Unknown || !r.Annotations.Unknown || !r.DataUnknown {
		t.Errorf("labels, annotations and data should be unknown: %+v %+v %v", r.Labels, r.Annotations, r.DataUnknown)
	}
	if r.Data[0].Instant != nil || r.Data[0].Range != nil {
		t.Error("instant/range from variables should be nil")
	}
	if r.Data[1].Reducer != "" || !r.Data[1].Expression.Known() {
		t.Errorf("reduce query: %+v", r.Data[1])
	}

	r = rules[1]
	if r.Labels.Unknown || !r.Labels.Values["team"].Unknown || r.Labels.Values["severity"].S != "page" {
		t.Errorf("labels with one unknown value: %+v", r.Labels)
	}
	if !r.Annotations.Values["summary"].Unknown || r.Annotations.Values["runbook_url"].Line != 77 {
		t.Errorf("annotations: %+v", r.Annotations)
	}
	if r.Data[0].Expr.Line != 88 || r.Data[1].Expr.Unknown != true || r.Data[2].Expression.Unknown != true {
		t.Errorf("data: %+v", r.Data)
	}

	r = rules[2]
	if !r.Data[0].ModelUnknown || !r.Data[1].ModelUnknown || r.Data[2].ModelUnknown {
		t.Errorf("model from a local or file() should be unknown: %+v", r.Data)
	}
}

func TestYAMLAnchors(t *testing.T) {
	rules := load(t, testdata+"yaml/anchors.yaml")
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(rules))
	}
	m := rules[0]
	if !m.NoDataState.Unknown || !m.ExecErrState.Unknown || !m.For.Unknown || !m.Labels.Unknown || !m.Annotations.Unknown {
		t.Errorf("merged fields should be unknown: %+v", m)
	}
	if !m.Condition.Known() || len(m.Data) != 1 || m.DataUnknown {
		t.Errorf("fields set on the rule itself should stay known: %+v", m)
	}
	if a := rules[1]; !a.DataUnknown || len(a.Data) != 0 || !a.NoDataState.Known() {
		t.Errorf("aliased data should be unknown: %+v", a)
	}
}

func TestHeredocModel(t *testing.T) {
	r := load(t, testdata+"terraform/heredoc.tf")[0]
	if r.Data[0].Expr.Line != 30 || r.Data[0].Instant == nil || !*r.Data[0].Instant {
		t.Errorf("heredoc model: %+v", r.Data[0])
	}
	if r.Data[1].Type.S != "threshold" || r.Data[1].Expression.Line != 42 {
		t.Errorf("quoted string model: %+v", r.Data[1])
	}
}

func TestLoadSkipsOtherFiles(t *testing.T) {
	tests := map[string]string{
		"prometheus.yaml": "groups:\n  - name: api\n    rules:\n      - alert: Down\n        expr: up == 0\n",
		"recording.yaml":  "groups:\n  - name: api\n    rules:\n      - uid: r\n        title: r\n        record: {metric: x, from: A}\n        data: []\n",
		"empty.yaml":      "",
		"contacts.yaml":   "apiVersion: 1\ncontactPoints: []\n",
		"workflow.yml":    "on: push\njobs: {}\n",
		"other.tf":        "resource \"grafana_folder\" \"x\" {\n  title = \"x\"\n}\n",
		"recording.tf":    "resource \"grafana_rule_group\" \"x\" {\n  rule {\n    name = \"r\"\n    record {\n      metric = \"x\"\n      from = \"A\"\n    }\n  }\n}\n",
		"configmap.yaml":  "apiVersion: v1\nkind: ConfigMap\nmetadata: {name: app-config}\ndata:\n  LOG_LEVEL: info\n",
		"response.json":   `{"status": "success", "data": [{"id": 1}]}`,
		"null-data.json":  `{"data": null}`,
		"main.tf.json":    `{"data": {"grafana_data_source": {"x": {"name": "p"}}}}`,
	}
	for name, src := range tests {
		if rules := load(t, write(t, name, src)); len(rules) != 0 {
			t.Errorf("%s: got %d rules, want 0", name, len(rules))
		}
	}
}

func TestLoadShapes(t *testing.T) {
	// A single rule from GET /api/v1/provisioning/alert-rules/{uid}, numeric for, multi-document YAML.
	single := `{"uid": "x", "title": "single", "condition": "A", "for": 0, "data": [{"refId": "A", "datasourceUid": "-100", "model": {"type": "math", "expression": "1 == 1"}}]}`
	r := load(t, write(t, "single.json", single))
	if len(r) != 1 || r[0].For.S != "0" || !r[0].Data[0].IsExpression() {
		t.Errorf("single rule: %+v", r)
	}
	// The provisioning API writes "record": null and "notification_settings": null on every alert rule.
	api := `[{"uid": "x", "title": "api", "condition": "A", "notification_settings": null, "record": null, "data": []}]`
	if r := load(t, write(t, "api.json", api)); len(r) != 1 || r[0].Notification != nil {
		t.Errorf("API rule with null record: %+v", r)
	}
	multi := "title: a\ndata: []\n---\ntitle: b\ndata: []\n"
	if r := load(t, write(t, "multi.yaml", multi)); len(r) != 2 {
		t.Errorf("multi-document: %d rules", len(r))
	}
	// Unknown extension: sniffed by content.
	tf := "resource \"grafana_rule_group\" \"x\" {\n  rule {\n    name = \"r\"\n  }\n}\n"
	if r := load(t, write(t, "rules.hcl", tf)); len(r) != 1 || r[0].Title != "r" {
		t.Errorf("sniffed Terraform: %+v", r)
	}
	// Scalars where maps belong, and maps where scalars belong, are unknown rather than wrong.
	odd := "title: odd\nlabels: some-string\nfor: {a: b}\nnotification_settings: {policy: p}\ndata: [{refId: A, model: {expr: [x]}}]\n"
	o := load(t, write(t, "odd.yaml", odd))[0]
	if !o.Labels.Unknown || !o.For.Unknown || !o.Data[0].Expr.Unknown || o.Notification.Policy.S != "p" {
		t.Errorf("odd shapes: %+v", o)
	}
}

func TestLoadErrors(t *testing.T) {
	for name, src := range map[string]string{
		"bad.yaml": "groups: [\n",
		"bad.tf":   "resource \"grafana_rule_group\" {\n",
	} {
		if _, err := LoadFile(write(t, name, src)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	if _, err := LoadFile(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Error("missing file: want error")
	}
}

func TestRefs(t *testing.T) {
	v := func(s string) Value { return Value{S: s, Set: true} }
	tests := []struct {
		q    Query
		want []string
		ok   bool
	}{
		{Query{Type: v("math"), Expression: v("$A > 0.05 && ${B 2} > 1")}, []string{"A", "B 2"}, true},
		{Query{Type: v("reduce"), Expression: v("$A")}, []string{"A"}, true},
		{Query{Type: v("threshold"), Expression: v("B")}, []string{"B"}, true},
		{Query{Type: v("classic_conditions"), Classic: []Classic{{RefID: "A"}, {RefID: "C"}}}, []string{"A", "C"}, true},
		{Query{Type: v("math"), Expression: Value{Set: true, Unknown: true}}, nil, false},
		{Query{Type: v("reduce")}, nil, false},
		{Query{Type: v("sql")}, nil, false},
		{Query{ModelUnknown: true}, nil, false},
	}
	for _, tt := range tests {
		got, ok := tt.q.Refs()
		if ok != tt.ok || !slices.Equal(got, tt.want) {
			t.Errorf("Refs(%s %q) = %q, %v; want %q, %v", tt.q.Type.S, tt.q.Expression.S, got, ok, tt.want, tt.ok)
		}
	}
}

func TestIsPrometheus(t *testing.T) {
	v := func(s string) Value { return Value{S: s, Set: true} }
	tests := []struct {
		q    Query
		want bool
	}{
		{Query{DatasourceUID: v("prometheus"), Expr: v("up")}, true},
		{Query{DatasourceUID: Value{Set: true, Unknown: true}, Expr: v("up")}, true},
		{Query{DatasourceUID: v("mimir"), Expr: v("up"), DatasourceType: "grafana-amazonprometheus-datasource"}, true},
		{Query{DatasourceUID: v("loki"), Expr: v("{app=\"x\"}"), DatasourceType: "loki"}, false},
		{Query{DatasourceUID: v("__expr__"), Expr: v("up")}, false},
		{Query{DatasourceUID: v("prometheus")}, false},
		{Query{DatasourceUID: v("prometheus"), ModelUnknown: true}, false},
	}
	for _, tt := range tests {
		if got := tt.q.IsPrometheus(); got != tt.want {
			t.Errorf("IsPrometheus(%+v) = %v, want %v", tt.q, got, tt.want)
		}
	}
}
