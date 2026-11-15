package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/unorig-username/grafana-alert-lint/internal/checks"
)

const repoURL = "https://github.com/unorig-username/grafana-alert-lint"

var formats = map[string]func(io.Writer, []checks.Finding) error{
	"text":   writeText,
	"github": writeGitHub,
	"json":   writeJSON,
	"sarif":  writeSARIF,
}

func writeText(w io.Writer, fs []checks.Finding) error {
	for _, f := range fs {
		if _, err := fmt.Fprintf(w, "%s:%d: %s %s %s: %s\n", f.File, f.Line, f.Severity, f.ID, f.Rule, f.Message); err != nil {
			return err
		}
	}
	return nil
}

// writeGitHub prints workflow commands that GitHub turns into annotations.
func writeGitHub(w io.Writer, fs []checks.Finding) error {
	level := map[checks.Severity]string{checks.Error: "error", checks.Warning: "warning", checks.Info: "notice"}
	prop := strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
	data := strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	for _, f := range fs {
		_, err := fmt.Fprintf(w, "::%s file=%s,line=%d,title=%s::%s\n", level[f.Severity],
			prop.Replace(filepath.ToSlash(f.File)), f.Line, prop.Replace(f.ID+" "+f.Check),
			data.Replace(f.Rule+": "+f.Message))
		if err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(w io.Writer, fs []checks.Finding) error {
	if fs == nil {
		fs = []checks.Finding{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(fs)
}

// writeSARIF writes SARIF 2.1.0 for GitHub code scanning.
func writeSARIF(w io.Writer, fs []checks.Finding) error {
	level := map[checks.Severity]string{checks.Error: "error", checks.Warning: "warning", checks.Info: "note"}
	type msg struct {
		Text string `json:"text"`
	}
	type rule struct {
		ID                   string            `json:"id"`
		Name                 string            `json:"name"`
		ShortDescription     msg               `json:"shortDescription"`
		HelpURI              string            `json:"helpUri"`
		DefaultConfiguration map[string]string `json:"defaultConfiguration"`
	}
	type location struct {
		PhysicalLocation struct {
			ArtifactLocation struct {
				URI string `json:"uri"`
			} `json:"artifactLocation"`
			Region struct {
				StartLine int `json:"startLine"`
			} `json:"region"`
		} `json:"physicalLocation"`
	}
	type result struct {
		RuleID    string     `json:"ruleId"`
		RuleIndex int        `json:"ruleIndex"`
		Level     string     `json:"level"`
		Message   msg        `json:"message"`
		Locations []location `json:"locations"`
	}

	rules := make([]rule, len(checks.Checks))
	index := map[string]int{}
	for i, c := range checks.Checks {
		index[c.ID] = i
		rules[i] = rule{
			ID: c.ID, Name: c.Name, ShortDescription: msg{c.Summary},
			HelpURI:              repoURL + "#" + strings.ToLower(c.ID) + "-" + c.Name,
			DefaultConfiguration: map[string]string{"level": level[c.Severity]},
		}
	}
	results := make([]result, 0, len(fs))
	for _, f := range fs {
		var loc location
		loc.PhysicalLocation.ArtifactLocation.URI = filepath.ToSlash(f.File)
		loc.PhysicalLocation.Region.StartLine = f.Line
		results = append(results, result{
			RuleID: f.ID, RuleIndex: index[f.ID], Level: level[f.Severity],
			Message: msg{f.Rule + ": " + f.Message}, Locations: []location{loc},
		})
	}
	doc := map[string]any{
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
		"version": "2.1.0",
		"runs": []any{map[string]any{
			"tool": map[string]any{"driver": map[string]any{
				"name": "grafana-alert-lint", "version": buildVersion(), "informationUri": repoURL, "rules": rules,
			}},
			"results": results,
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(doc)
}
