// Command grafana-alert-lint lints Grafana-managed alert rules.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/unorig-username/grafana-alert-lint/internal/checks"
	"github.com/unorig-username/grafana-alert-lint/internal/rule"
)

// version is set with -ldflags "-X main.version=..." or read from the module version.
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }

const usage = `Usage: grafana-alert-lint [flags] PATH...

Lints Grafana-managed alert rules in file provisioning YAML, export or
provisioning API JSON, and Terraform grafana_rule_group resources.
Directories are searched for .yaml, .yml, .json and .tf files.

Exit status: 0 clean, 1 findings at or above --fail-on, 2 usage or I/O error.

Flags:
`

func run(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("grafana-alert-lint", flag.ContinueOnError)
	fl.SetOutput(stderr)
	fl.Usage = func() {
		_, _ = fmt.Fprint(stderr, usage)
		fl.PrintDefaults()
	}
	var disable, requireLabels listFlag
	format := fl.String("format", "text", "output format: text, github, json or sarif")
	failOn := fl.String("fail-on", "warning", "exit 1 on findings at or above this severity: error, warning or info")
	scrape := fl.Duration("scrape-interval", time.Minute, "Prometheus scrape interval, for the rate window check (GAL005)")
	fl.Var(&disable, "disable", "check ID to skip, e.g. GAL009 (repeatable)")
	fl.Var(&requireLabels, "require-label", "label every rule must carry, e.g. team (repeatable, enables GAL008)")
	showVersion := fl.Bool("version", false, "print the version and exit")
	if err := fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *showVersion {
		_, _ = fmt.Fprintln(stdout, "grafana-alert-lint", buildVersion())
		return 0
	}
	write, ok := formats[*format]
	if !ok {
		_, _ = fmt.Fprintf(stderr, "unknown --format %q: use text, github, json or sarif\n", *format)
		return 2
	}
	threshold, ok := checks.ParseSeverity(*failOn)
	if !ok {
		_, _ = fmt.Fprintf(stderr, "unknown --fail-on %q: use error, warning or info\n", *failOn)
		return 2
	}
	cfg := checks.Config{ScrapeInterval: *scrape, RequireLabels: requireLabels, Disabled: map[string]bool{}}
	for _, id := range disable {
		cfg.Disabled[strings.ToUpper(id)] = true
	}
	if fl.NArg() == 0 {
		fl.Usage()
		return 2
	}

	rules, err := load(fl.Args(), stderr)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	findings := checks.Run(cfg, rules)
	if err := write(stdout, findings); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	for _, f := range findings {
		if f.Severity >= threshold {
			return 1
		}
	}
	return 0
}

// load reads every path. Named files must parse; files found by walking a
// directory that do not parse are skipped with a note, since a tree can
// hold YAML that is not alert rules (Helm templates, for one).
func load(paths []string, stderr io.Writer) ([]rule.Rule, error) {
	var rules []rule.Rule
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			r, err := rule.LoadFile(p)
			if err != nil {
				return nil, err
			}
			if len(r) == 0 {
				_, _ = fmt.Fprintf(stderr, "%s: no Grafana alert rules found\n", p)
			}
			rules = append(rules, r...)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if path != p && strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".yaml", ".yml", ".json", ".tf":
			default:
				return nil
			}
			r, err := rule.LoadFile(path)
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "skipping %v\n", err)
				return nil
			}
			rules = append(rules, r...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return rules, nil
}

func buildVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}
