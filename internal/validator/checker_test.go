package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"autodev/internal/project"
)

func TestDefaultThresholds(t *testing.T) {
	th := DefaultThresholds()
	if th.MinLineCoverage != 80.0 {
		t.Errorf("expected line coverage 80.0, got %f", th.MinLineCoverage)
	}
	if th.MinBranchCoverage != 70.0 {
		t.Errorf("expected branch coverage 70.0, got %f", th.MinBranchCoverage)
	}
	if th.MinFunctionCoverage != 80.0 {
		t.Errorf("expected function coverage 80.0, got %f", th.MinFunctionCoverage)
	}
	if th.AllowCritical != false {
		t.Errorf("expected allow critical false, got %t", th.AllowCritical)
	}
	if th.AllowMajor != 0 {
		t.Errorf("expected allow major 0, got %d", th.AllowMajor)
	}
}

func TestCheckCoverage(t *testing.T) {
	th := &Thresholds{
		MinLineCoverage:     80.0,
		MinBranchCoverage:   70.0,
		MinFunctionCoverage: 80.0,
		AllowCritical:       false,
		AllowMajor:          0,
	}
	c := &Checker{thresholds: th}

	tests := []struct {
		name     string
		coverage *CoverageResult
		want     bool
	}{
		{"nil coverage", nil, false},
		{"all pass", &CoverageResult{Line: 90, Branch: 80, Function: 90}, true},
		{"line too low", &CoverageResult{Line: 50, Branch: 80, Function: 90}, false},
		{"branch too low", &CoverageResult{Line: 90, Branch: 60, Function: 90}, false},
		{"function too low", &CoverageResult{Line: 90, Branch: 80, Function: 50}, false},
		{"exact threshold", &CoverageResult{Line: 80, Branch: 70, Function: 80}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.checkCoverage(tt.coverage)
			if got != tt.want {
				t.Errorf("checkCoverage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckCoverageZeroThresholds(t *testing.T) {
	th := &Thresholds{
		MinLineCoverage:     0,
		MinBranchCoverage:   0,
		MinFunctionCoverage: 0,
	}
	c := &Checker{thresholds: th}
	// Zero thresholds means any non-nil coverage passes
	if !c.checkCoverage(&CoverageResult{Line: 10, Branch: 10, Function: 10}) {
		t.Error("expected pass with zero thresholds")
	}
}

func TestCheckLint(t *testing.T) {
	tests := []struct {
		name    string
		th      *Thresholds
		results []LintResult
		want    bool
	}{
		{
			name: "no violations",
			th:   DefaultThresholds(),
			results: []LintResult{
				{Critical: 0, Major: 0},
			},
			want: true,
		},
		{
			name: "critical not allowed",
			th:   DefaultThresholds(),
			results: []LintResult{
				{Critical: 1, Major: 0},
			},
			want: false,
		},
		{
			name: "major exceeds allow",
			th:   &Thresholds{AllowCritical: true, AllowMajor: 1},
			results: []LintResult{
				{Critical: 0, Major: 2},
			},
			want: false,
		},
		{
			name: "major within allow",
			th:   &Thresholds{AllowCritical: true, AllowMajor: 2},
			results: []LintResult{
				{Critical: 0, Major: 2},
			},
			want: true,
		},
		{
			name: "critical allowed but exists",
			th:   &Thresholds{AllowCritical: true, AllowMajor: 0},
			results: []LintResult{
				{Critical: 1, Major: 0},
			},
			want: true,
		},
		{
			name: "multiple results one fails",
			th:   DefaultThresholds(),
			results: []LintResult{
				{Critical: 0, Major: 0},
				{Critical: 1, Major: 0},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Checker{thresholds: tt.th}
			got := c.checkLint(tt.results)
			if got != tt.want {
				t.Errorf("checkLint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCountSeverity(t *testing.T) {
	violations := []Violation{
		{Severity: SeverityCritical},
		{Severity: SeverityMajor},
		{Severity: SeverityMinor},
		{Severity: SeverityCritical},
		{Severity: SeverityInfo},
	}

	if countSeverity(violations, SeverityCritical) != 2 {
		t.Error("expected 2 critical")
	}
	if countSeverity(violations, SeverityMajor) != 1 {
		t.Error("expected 1 major")
	}
	if countSeverity(violations, SeverityMinor) != 1 {
		t.Error("expected 1 minor")
	}
	if countSeverity(violations, SeverityInfo) != 1 {
		t.Error("expected 1 info")
	}
	if countSeverity(violations, "unknown") != 0 {
		t.Error("expected 0 for unknown severity")
	}
	if countSeverity(nil, SeverityCritical) != 0 {
		t.Error("expected 0 for nil slice")
	}
}

func TestParseInt(t *testing.T) {
	if parseInt("42") != 42 {
		t.Error("expected 42")
	}
	if parseInt("0") != 0 {
		t.Error("expected 0")
	}
	if parseInt("-1") != -1 {
		t.Error("expected -1")
	}
	if parseInt("abc") != 0 {
		t.Error("expected 0 for invalid input")
	}
	if parseInt("") != 0 {
		t.Error("expected 0 for empty string")
	}
}

func TestParseCoverage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLine float64
	}{
		{
			name:     "go cover output",
			input:    "total: (statements)\t85.7%",
			wantLine: 85.7,
		},
		{
			name:     "different percent",
			input:    "total: (statements)\t100%",
			wantLine: 100,
		},
		{
			name:     "empty",
			input:    "",
			wantLine: 0,
		},
		{
			name:     "no match",
			input:    "some random output",
			wantLine: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := parseCoverage(tt.input)
			if cr.Line != tt.wantLine {
				t.Errorf("expected line %f, got %f", tt.wantLine, cr.Line)
			}
			// Statement should match Line for Go
			if cr.Statement != cr.Line {
				t.Errorf("expected Statement == Line")
			}
		})
	}
}

func TestParsePythonCoverage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "pytest coverage",
			input: "TOTAL   120    34    72%",
			want:  "total: (statements)\t72%",
		},
		{
			name:  "full pytest output",
			input: "Name    Stmts   Miss  Cover\n-----------------------------------\nTOTAL   120    34    85%",
			want:  "total: (statements)\t85%",
		},
		{
			name:  "no coverage found",
			input: "some random output",
			want:  "",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePythonCoverage(tt.input)
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestParseGoLintJSON(t *testing.T) {
	input := `/src/main.go:10:5: unused variable x
/src/main.go:15:10: error: nil pointer
invalid line without match: some text`

	violations := parseGoLintJSON(input)
	if len(violations) != 2 {
		t.Errorf("expected 2 violations, got %d", len(violations))
	}

	if violations[0].File != "/src/main.go" || violations[0].Line != 10 {
		t.Errorf("first violation parsed incorrectly")
	}

	// Should detect "error" as critical
	if violations[1].Severity != SeverityCritical {
		t.Errorf("expected critical severity for error message, got %s", violations[1].Severity)
	}

	// Empty
	if len(parseGoLintJSON("")) != 0 {
		t.Error("expected 0 violations for empty input")
	}
}

func TestParseGoVetOutput(t *testing.T) {
	input := `autodev/internal/core:types.go:15: struct field tag mismatch
autodev/internal/core:utils.go:22: unreachable code`

	violations := parseGoVetOutput(input)
	if len(violations) != 2 {
		t.Errorf("expected 2 violations, got %d", len(violations))
	}

	if violations[0].File != "types.go" || violations[0].Line != 15 {
		t.Errorf("first violation: got %s:%d, want types.go:15", violations[0].File, violations[0].Line)
	}

	// go vet violations are SeverityMajor
	if violations[0].Severity != SeverityMajor {
		t.Errorf("expected major severity")
	}

	if len(parseGoVetOutput("")) != 0 {
		t.Error("expected 0 for empty input")
	}
}

func TestParseESLintJSON(t *testing.T) {
	input := `/src/app.js:10:15 error Expected semicolon
/src/utils.js:25:5 warning Unused variable`

	violations := parseESLintJSON(input)
	if len(violations) != 2 {
		t.Errorf("expected 2 violations, got %d", len(violations))
	}

	// error -> critical
	if violations[0].Severity != SeverityCritical {
		t.Errorf("expected critical, got %s", violations[0].Severity)
	}

	// warning -> minor
	if violations[1].Severity != SeverityMinor {
		t.Errorf("expected minor, got %s", violations[1].Severity)
	}

	if violations[0].Line != 10 || violations[0].Column != 15 {
		t.Errorf("position mismatch")
	}

	if len(parseESLintJSON("")) != 0 {
		t.Error("expected 0 for empty input")
	}
}

func TestParsePythonLintOutput_Flake8(t *testing.T) {
	input := `/src/app.py:10:1: E302 expected 2 blank lines
/src/utils.py:25:5: W291 trailing whitespace
/src/main.py:30:1: F401 module imported but unused`

	violations := parsePythonLintOutput("flake8", input)
	if len(violations) != 3 {
		t.Errorf("expected 3 violations, got %d", len(violations))
	}

	// E -> Major
	if violations[0].Severity != SeverityMajor {
		t.Errorf("expected major for E, got %s", violations[0].Severity)
	}

	// W -> Minor
	if violations[1].Severity != SeverityMinor {
		t.Errorf("expected minor for W, got %s", violations[1].Severity)
	}

	// F -> Critical
	if violations[2].Severity != SeverityCritical {
		t.Errorf("expected critical for F, got %s", violations[2].Severity)
	}

	if violations[0].Rule != "E302" {
		t.Errorf("expected rule E302, got %s", violations[0].Rule)
	}
}

func TestParsePythonLintOutput_Pylint(t *testing.T) {
	input := `/src/app.py:10:0: error: invalid syntax
/src/utils.py:25:0: warning: unused import
/src/main.py:30:0: info: missing docstring
/src/core.py:5:0: fatal: crash`

	violations := parsePythonLintOutput("pylint", input)
	if len(violations) != 4 {
		t.Errorf("expected 4 violations, got %d", len(violations))
	}

	severityMap := map[string]Severity{
		"error":   SeverityCritical,
		"warning": SeverityMinor,
		"info":    "", // Not handled in current implementation
		"fatal":   SeverityCritical,
	}

	for _, v := range violations {
		expected := severityMap[v.Rule]
		if v.Severity != expected {
			t.Errorf("rule %s: expected %s, got %s", v.Rule, expected, v.Severity)
		}
	}
}

func TestParsePythonLintOutput_Generic(t *testing.T) {
	input := `/src/app.py:10: error something went wrong
/src/utils.py:25: warning minor issue`

	violations := parsePythonLintOutput("unknown", input)
	if len(violations) != 2 {
		t.Errorf("expected 2 violations, got %d", len(violations))
	}

	if violations[0].Severity != SeverityCritical {
		t.Errorf("expected critical for error, got %s", violations[0].Severity)
	}
	if violations[1].Severity != SeverityMajor {
		t.Errorf("expected major for warning, got %s", violations[1].Severity)
	}
}

func TestParsePythonLintOutput_Empty(t *testing.T) {
	if len(parsePythonLintOutput("flake8", "")) != 0 {
		t.Error("expected 0 for empty input")
	}
}

func TestTestOutputCoverage(t *testing.T) {
	t.Run("nil when empty", func(t *testing.T) {
		output := &TestOutput{CoverageRaw: ""}
		if output.Coverage() != nil {
			t.Error("expected nil")
		}
	})

	t.Run("parses when set", func(t *testing.T) {
		output := &TestOutput{CoverageRaw: "total: (statements)\t75.5%"}
		cr := output.Coverage()
		if cr == nil {
			t.Fatal("expected non-nil coverage")
		}
		if cr.Line != 75.5 {
			t.Errorf("expected 75.5, got %f", cr.Line)
		}
	})
}

func TestGenerateSummary(t *testing.T) {
	tests := []struct {
		name     string
		result   *ValidationResult
		contains []string
	}{
		{
			name: "all passed",
			result: &ValidationResult{
				TestPassed:     true,
				CoveragePassed: true,
				LintPassed:     true,
				AllPassed:      true,
				Coverage:       &CoverageResult{Line: 85, Branch: 80},
			},
			contains: []string{"All checks PASSED", "Tests: PASSED", "Coverage:"},
		},
		{
			name: "tests failed",
			result: &ValidationResult{
				TestPassed:     false,
				CoveragePassed: true,
				LintPassed:     true,
				AllPassed:      false,
				Coverage:       &CoverageResult{Line: 85, Branch: 80},
			},
			contains: []string{"Some checks FAILED", "Tests: FAILED"},
		},
		{
			name: "coverage failed",
			result: &ValidationResult{
				TestPassed:     true,
				CoveragePassed: false,
				LintPassed:     true,
				AllPassed:      false,
				Coverage:       &CoverageResult{Line: 60, Branch: 50},
			},
			contains: []string{"Coverage: 60.0%", "FAILED"},
		},
		{
			name: "no coverage data",
			result: &ValidationResult{
				TestPassed:     true,
				CoveragePassed: false,
				LintPassed:     true,
				AllPassed:      true,
				Coverage:       nil,
			},
			contains: []string{"All checks PASSED"},
		},
		{
			name: "lint failed with details",
			result: &ValidationResult{
				TestPassed:     true,
				CoveragePassed: true,
				LintPassed:     false,
				AllPassed:      false,
				Coverage:       &CoverageResult{Line: 90, Branch: 85},
				LintResults: []LintResult{
					{Tool: "golangci-lint", Passed: false, Critical: 1, Major: 2},
				},
			},
			contains: []string{"golangci-lint: FAILED", "1 critical", "2 major"},
		},
		{
			name: "multiple lint results",
			result: &ValidationResult{
				TestPassed:     true,
				CoveragePassed: true,
				LintPassed:     true,
				AllPassed:      true,
				Coverage:       &CoverageResult{Line: 90, Branch: 85},
				LintResults: []LintResult{
					{Tool: "golangci-lint", Passed: true, Critical: 0, Major: 0},
					{Tool: "go vet", Passed: true, Critical: 0, Major: 0},
				},
			},
			contains: []string{"golangci-lint", "go vet"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Checker{}
			summary := c.generateSummary(tt.result)
			for _, expected := range tt.contains {
				if !strings.Contains(summary, expected) {
					t.Errorf("summary missing %q\nGot: %s", expected, summary)
				}
			}
		})
	}
}

func TestViolationStructure(t *testing.T) {
	v := Violation{
		File:     "main.go",
		Line:     42,
		Column:   5,
		Severity: SeverityCritical,
		Rule:     "E101",
		Message:  "unexpected indent",
	}

	if v.File != "main.go" || v.Line != 42 {
		t.Error("violation fields not set correctly")
	}
	if v.Severity != SeverityCritical {
		t.Error("severity not set correctly")
	}
}

func TestLintResultStructure(t *testing.T) {
	lr := LintResult{
		Passed:     false,
		Tool:       "golangci-lint",
		Critical:   2,
		Major:      3,
		Minor:      1,
		Violations: []Violation{{Message: "test"}},
	}

	if lr.Passed != false {
		t.Error("lint result passed not set correctly")
	}
	if lr.Critical != 2 || lr.Major != 3 {
		t.Error("severity counts not set correctly")
	}
}

func TestValidationResultStructure(t *testing.T) {
	vr := ValidationResult{
		TestPassed:     true,
		CoveragePassed: true,
		LintPassed:     true,
		AllPassed:      true,
		Coverage:       &CoverageResult{Line: 90},
		LintResults:    []LintResult{{Tool: "golangci-lint"}},
		Summary:        "All checks PASSED",
	}

	if !vr.AllPassed || !vr.TestPassed {
		t.Error("validation result fields not set correctly")
	}
}

func TestCoverageResultStructure(t *testing.T) {
	cr := CoverageResult{
		Line:      85.5,
		Branch:    70.2,
		Function:  90.0,
		Statement: 85.5,
	}

	if cr.Line != 85.5 || cr.Branch != 70.2 {
		t.Error("coverage result fields not set correctly")
	}
}

func TestSeverityConstants(t *testing.T) {
	if SeverityCritical != "critical" {
		t.Errorf("expected critical='critical', got %s", SeverityCritical)
	}
	if SeverityMajor != "major" {
		t.Errorf("expected major='major', got %s", SeverityMajor)
	}
	if SeverityMinor != "minor" {
		t.Errorf("expected minor='minor', got %s", SeverityMinor)
	}
	if SeverityInfo != "info" {
		t.Errorf("expected info='info', got %s", SeverityInfo)
	}
}

func TestHasNPMScript(t *testing.T) {
	dir := t.TempDir()

	t.Run("valid package.json with scripts", func(t *testing.T) {
		content := `{"scripts": {"test": "jest", "build": "tsc"}}`
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		c := &Checker{profile: &project.Profile{RootPath: dir}}
		if !c.hasNPMScript("test") {
			t.Error("expected to find 'test' script")
		}
		if !c.hasNPMScript("build") {
			t.Error("expected to find 'build' script")
		}
		if c.hasNPMScript("lint") {
			t.Error("expected 'lint' script not to exist")
		}
	})

	t.Run("no scripts key", func(t *testing.T) {
		content := `{"name": "test-app"}`
		file2 := filepath.Join(dir, "package.json")
		if err := os.WriteFile(file2, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		c := &Checker{profile: &project.Profile{RootPath: dir}}
		if c.hasNPMScript("test") {
			t.Error("expected false when no scripts key")
		}
	})

	t.Run("missing package.json", func(t *testing.T) {
		dir2 := t.TempDir()
		c := &Checker{profile: &project.Profile{RootPath: dir2}}
		if c.hasNPMScript("test") {
			t.Error("expected false when package.json missing")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		content := `not valid json`
		dir3 := t.TempDir()
		file := filepath.Join(dir3, "package.json")
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		c := &Checker{profile: &project.Profile{RootPath: dir3}}
		if c.hasNPMScript("test") {
			t.Error("expected false for invalid json")
		}
	})
}

func TestHasPackageDep(t *testing.T) {
	t.Run("node.js dependency", func(t *testing.T) {
		dir := t.TempDir()
		content := `{"dependencies": {"react": "^18.0"}, "devDependencies": {"jest": "^29.0"}}`
		file := filepath.Join(dir, "package.json")
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		c := &Checker{profile: &project.Profile{RootPath: dir}}
		if !c.hasPackageDep("react") {
			t.Error("expected to find 'react'")
		}
		if !c.hasPackageDep("jest") {
			t.Error("expected to find 'jest'")
		}
		if c.hasPackageDep("vue") {
			t.Error("expected 'vue' not to exist")
		}
	})

	t.Run("go.mod dependency", func(t *testing.T) {
		dir := t.TempDir()
		// Uses single-line require format that hasPackageDep can parse
		content := `module example.com/app
go 1.21
require github.com/gin-gonic/gin v1.9.1
require github.com/stretchr/testify v1.8.4`
		file := filepath.Join(dir, "go.mod")
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		c := &Checker{profile: &project.Profile{RootPath: dir}}
		if !c.hasPackageDep("gin-gonic/gin") {
			t.Error("expected to find gin")
		}
		if c.hasPackageDep("non-existent-package") {
			t.Error("expected non-existent package not to exist")
		}
	})

	t.Run("no package files", func(t *testing.T) {
		dir := t.TempDir()
		c := &Checker{profile: &project.Profile{RootPath: dir}}
		if c.hasPackageDep("anything") {
			t.Error("expected false when no package files")
		}
	})
}
