package rule

import (
	"bytes"
	"strconv"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
	"go.yaml.in/yaml/v4"
)

// tfFuncs are the Terraform functions we can evaluate statically. Any other
// function makes its attribute unknown.
var tfFuncs = map[string]function.Function{
	"jsonencode": stdlib.JSONEncodeFunc,
	"jsondecode": stdlib.JSONDecodeFunc,
	"merge":      stdlib.MergeFunc,
	"format":     stdlib.FormatFunc,
	"join":       stdlib.JoinFunc,
	"lower":      stdlib.LowerFunc,
	"upper":      stdlib.UpperFunc,
}

// tfAttrs maps grafana_rule_group rule attributes to provisioning keys.
var tfAttrs = map[string]string{
	"name":           "title",
	"uid":            "uid",
	"condition":      "condition",
	"for":            "for",
	"no_data_state":  "noDataState",
	"exec_err_state": "execErrState",
	"labels":         "labels",
	"annotations":    "annotations",
}

// parseTerraform turns each rule block of a grafana_rule_group resource
// into the provisioning YAML shape, so one extractor serves every format.
func parseTerraform(file string, src []byte) ([]Rule, error) {
	f, diags := hclsyntax.ParseConfig(src, file, hcl.InitialPos)
	if diags.HasErrors() {
		return nil, diags
	}
	var rules []Rule
	for _, b := range f.Body.(*hclsyntax.Body).Blocks {
		if b.Type != "resource" || len(b.Labels) != 2 || b.Labels[0] != "grafana_rule_group" {
			continue
		}
		// dynamic "rule" blocks are skipped: their content depends on for_each.
		for _, rb := range b.Body.Blocks {
			if rb.Type == "rule" && !hasBlock(rb.Body, "record") {
				rules = append(rules, fromNode(file, tfRule(src, rb)))
			}
		}
	}
	return rules, nil
}

func tfRule(src []byte, b *hclsyntax.Block) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode, Line: b.DefRange().Start.Line}
	for name, key := range tfAttrs {
		a, ok := b.Body.Attributes[name]
		if !ok {
			continue
		}
		v := ctyNode(eval(a.Expr), line(a.Expr), keyLines(a.Expr))
		if name == "name" && v.Tag == unknownTag {
			// Show the source text, e.g. "${var.service} errors", as the title.
			r := a.Expr.Range()
			v = scalar("!!str", string(src[r.Start.Byte:r.End.Byte]), r.Start.Line)
		}
		add(n, key, v)
	}
	data := &yaml.Node{Kind: yaml.SequenceNode, Line: n.Line}
	add(n, "data", data)
	for _, c := range b.Body.Blocks {
		switch {
		case c.Type == "data":
			data.Content = append(data.Content, tfData(src, c))
		case c.Type == "dynamic" && len(c.Labels) == 1 && c.Labels[0] == "data":
			data.Content = append(data.Content, scalar(unknownTag, "", c.DefRange().Start.Line))
		case c.Type == "notification_settings":
			ns := &yaml.Node{Kind: yaml.MappingNode, Line: c.DefRange().Start.Line}
			tfAttr(ns, c.Body, "contact_point", "receiver")
			tfAttr(ns, c.Body, "policy", "policy")
			add(n, "notification_settings", ns)
		case c.Type == "dynamic" && len(c.Labels) == 1 && c.Labels[0] == "notification_settings":
			add(n, "notification_settings", scalar(unknownTag, "", c.DefRange().Start.Line))
		}
	}
	return n
}

func tfData(src []byte, b *hclsyntax.Block) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode, Line: b.DefRange().Start.Line}
	tfAttr(n, b.Body, "ref_id", "refId")
	tfAttr(n, b.Body, "datasource_uid", "datasourceUid")
	if a, ok := b.Body.Attributes["model"]; ok {
		add(n, "model", tfModel(src, a))
	}
	return n
}

// tfModel evaluates a data block's model. jsonencode({...}) is evaluated
// object by object, so one unknown field does not hide the others.
func tfModel(src []byte, a *hclsyntax.Attribute) *yaml.Node {
	if call, ok := a.Expr.(*hclsyntax.FunctionCallExpr); ok && call.Name == "jsonencode" && len(call.Args) == 1 {
		return ctyNode(eval(call.Args[0]), line(a.Expr), keyLines(call.Args[0]))
	}
	v := eval(a.Expr)
	if !v.IsKnown() || v.IsNull() || v.Type() != cty.String {
		return scalar(unknownTag, "", line(a.Expr))
	}
	var doc yaml.Node
	if yaml.Unmarshal([]byte(v.AsString()), &doc) != nil || len(doc.Content) == 0 {
		return scalar(unknownTag, "", line(a.Expr))
	}
	// A heredoc's first line is the one after <<EOT; a quoted string starts
	// on the attribute's own line.
	r := a.Expr.Range()
	shift := r.Start.Line - 1
	if bytes.HasPrefix(src[r.Start.Byte:], []byte("<<")) {
		shift++
	}
	shiftLines(doc.Content[0], shift)
	return doc.Content[0]
}

func tfAttr(n *yaml.Node, body *hclsyntax.Body, name, key string) {
	if a, ok := body.Attributes[name]; ok {
		add(n, key, ctyNode(eval(a.Expr), line(a.Expr), nil))
	}
}

// eval evaluates an expression with every variable (var, local, data,
// resource references) unknown. Errors make the whole value unknown.
func eval(e hclsyntax.Expression) cty.Value {
	ctx := &hcl.EvalContext{Variables: map[string]cty.Value{}, Functions: tfFuncs}
	for _, t := range e.Variables() {
		ctx.Variables[t.RootName()] = cty.DynamicVal
	}
	v, diags := e.Value(ctx)
	if diags.HasErrors() {
		return cty.DynamicVal
	}
	return v
}

// ctyNode converts a cty value to a YAML node. Unknown values become
// unknownTag nodes. keys gives source lines for top-level object keys.
func ctyNode(v cty.Value, ln int, keys map[string]int) *yaml.Node {
	if !v.IsKnown() {
		return scalar(unknownTag, "", ln)
	}
	if v.IsNull() {
		return scalar("!!null", "null", ln)
	}
	t := v.Type()
	switch {
	case t == cty.String:
		return scalar("!!str", v.AsString(), ln)
	case t == cty.Number:
		return scalar("!!float", v.AsBigFloat().Text('f', -1), ln)
	case t == cty.Bool:
		return scalar("!!bool", strconv.FormatBool(v.True()), ln)
	case t.IsObjectType() || t.IsMapType():
		n := &yaml.Node{Kind: yaml.MappingNode, Line: ln}
		for it := v.ElementIterator(); it.Next(); {
			k, ev := it.Element()
			kl := ln
			if l, ok := keys[k.AsString()]; ok {
				kl = l
			}
			add(n, k.AsString(), ctyNode(ev, kl, nil))
		}
		return n
	case t.IsListType() || t.IsTupleType() || t.IsSetType():
		n := &yaml.Node{Kind: yaml.SequenceNode, Line: ln}
		for it := v.ElementIterator(); it.Next(); {
			_, ev := it.Element()
			n.Content = append(n.Content, ctyNode(ev, ln, nil))
		}
		return n
	}
	return scalar(unknownTag, "", ln)
}

// keyLines returns the line of each key of an object constructor.
func keyLines(e hclsyntax.Expression) map[string]int {
	oc, ok := e.(*hclsyntax.ObjectConsExpr)
	if !ok {
		return nil
	}
	lines := map[string]int{}
	for _, item := range oc.Items {
		if k, diags := item.KeyExpr.Value(nil); !diags.HasErrors() && k.IsKnown() && k.Type() == cty.String {
			lines[k.AsString()] = item.KeyExpr.Range().Start.Line
		}
	}
	return lines
}

func hasBlock(body *hclsyntax.Body, typ string) bool {
	for _, b := range body.Blocks {
		if b.Type == typ {
			return true
		}
	}
	return false
}

func line(e hclsyntax.Expression) int { return e.Range().Start.Line }

func scalar(tag, value string, ln int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value, Line: ln}
}

func add(n *yaml.Node, key string, v *yaml.Node) {
	n.Content = append(n.Content, scalar("!!str", key, v.Line), v)
}

func shiftLines(n *yaml.Node, by int) {
	n.Line += by
	for _, c := range n.Content {
		shiftLines(c, by)
	}
}
