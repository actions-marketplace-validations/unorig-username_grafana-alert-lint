package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func runCLI(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestGolden(t *testing.T) {
	t.Chdir("../..")
	old := version
	version = "0.1.0-test"
	t.Cleanup(func() { version = old })

	for _, format := range []string{"text", "github", "json", "sarif"} {
		t.Run(format, func(t *testing.T) {
			code, out, errOut := runCLI("--format", format, "--require-label", "team", "testdata/output/sample.yaml")
			if code != 1 || errOut != "" {
				t.Fatalf("exit %d, stderr %q", code, errOut)
			}
			golden := "testdata/output/sample." + format + ".golden"
			if *update {
				if err := os.WriteFile(golden, []byte(out), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if out != string(want) {
				t.Errorf("output differs from %s (run go test ./cmd/... -update to rewrite):\n%s", golden, out)
			}
			if format == "json" || format == "sarif" {
				var v any
				if err := json.Unmarshal([]byte(out), &v); err != nil {
					t.Errorf("invalid JSON: %v", err)
				}
			}
		})
	}
}

func TestEmptyOutput(t *testing.T) {
	t.Chdir("../..")
	for format, want := range map[string]string{"text": "", "github": "", "json": "[]\n"} {
		if code, out, _ := runCLI("--format", format, "testdata/good"); code != 0 || out != want {
			t.Errorf("%s on clean input: exit %d, output %q", format, code, out)
		}
	}
	_, out, _ := runCLI("--format", "sarif", "testdata/good")
	if !strings.Contains(out, `"results": []`) {
		t.Errorf("sarif on clean input should have empty results:\n%s", out)
	}
}

func TestExitCodes(t *testing.T) {
	t.Chdir("../..")
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.yaml")
	norules := filepath.Join(dir, "norules.yaml")
	hidden := filepath.Join(dir, ".hidden")
	for path, src := range map[string]string{
		broken:                          "groups: [\n",
		norules:                         "apiVersion: 1\n",
		filepath.Join(dir, "notes.txt"): "not yaml: [",
	} {
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(hidden, 0o700); err != nil {
		t.Fatal(err)
	}
	bad, err := os.ReadFile("testdata/bad/GAL001/provisioning.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "rules.yaml"), bad, 0o600); err != nil {
		t.Fatal(err)
	}
	badDir, err := filepath.Abs("testdata/bad")
	if err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(badDir, linked); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		args   []string
		code   int
		stdout string // substring
		stderr string // substring
	}{
		{"clean", []string{"testdata/good"}, 0, "", ""},
		{"findings", []string{"testdata/bad"}, 1, "GAL001", ""},
		{"warnings below fail-on error", []string{"--fail-on", "error", "testdata/bad/GAL002"}, 0, "GAL002", ""},
		{"info below default fail-on", []string{"testdata/bad/GAL009"}, 0, "GAL009", ""},
		{"info at fail-on info", []string{"--fail-on", "info", "testdata/bad/GAL009"}, 1, "GAL009", ""},
		{"disable is case-insensitive", []string{"--disable", "gal001", "testdata/bad/GAL001"}, 0, "", ""},
		{"scrape interval", []string{"--scrape-interval", "15s", "--disable", "GAL005", "testdata/bad/GAL005"}, 0, "", ""},
		{"version", []string{"--version"}, 0, "grafana-alert-lint 0.1.0", ""},
		{"help", []string{"-h"}, 0, "", "Usage: grafana-alert-lint"},
		{"no paths", nil, 2, "", "Usage"},
		{"unknown flag", []string{"--bogus", "x"}, 2, "", "bogus"},
		{"unknown format", []string{"--format", "xml", "x"}, 2, "", "unknown --format"},
		{"unknown fail-on", []string{"--fail-on", "fatal", "x"}, 2, "", "unknown --fail-on"},
		{"missing path", []string{"testdata/missing"}, 2, "", "no such file"},
		{"named file that does not parse", []string{broken}, 2, "", "broken.yaml"},
		{"named file without rules", []string{norules}, 0, "", "no Grafana alert rules found"},
		{"walked file that does not parse is skipped; hidden dirs too", []string{dir}, 0, "", "skipping"},
		{"directory without rules", []string{dir}, 0, "", "no Grafana alert rules found"},
		{"directory behind a symlink", []string{linked}, 1, "GAL001", ""},
	}
	version = "0.1.0"
	t.Cleanup(func() { version = "dev" })
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, out, errOut := runCLI(tt.args...)
			if code != tt.code || !strings.Contains(out, tt.stdout) || !strings.Contains(errOut, tt.stderr) {
				t.Errorf("exit %d, stdout %q, stderr %q; want exit %d, stdout ~%q, stderr ~%q",
					code, out, errOut, tt.code, tt.stdout, tt.stderr)
			}
		})
	}
}

func TestBuildVersion(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "dev"
	if got := buildVersion(); got == "" {
		t.Error("buildVersion is empty")
	}
	version = "1.2.3"
	if got := buildVersion(); got != "1.2.3" {
		t.Errorf("buildVersion = %q", got)
	}
}
