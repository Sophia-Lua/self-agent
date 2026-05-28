package validator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"autodev/internal/project"
	"autodev/internal/sandbox"
)

// Severity represents the severity of a quality issue.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityMajor    Severity = "major"
	SeverityMinor    Severity = "minor"
	SeverityInfo     Severity = "info"
)

// Violation represents a single code quality issue.
type Violation struct {
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Column   int      `json:"column"`
	Severity Severity `json:"severity"`
	Rule     string   `json:"rule"`
	Message  string   `json:"message"`
}

// CoverageResult holds code coverage metrics.
type CoverageResult struct {
	Line      float64 `json:"line"`
	Branch    float64 `json:"branch"`
	Function  float64 `json:"function"`
	Statement float64 `json:"statement"`
}

// LintResult holds static analysis results.
type LintResult struct {
	Passed     bool         `json:"passed"`
	Violations []Violation  `json:"violations,omitempty"`
	Tool       string       `json:"tool"`
	Duration   time.Duration `json:"duration"`
	Critical   int          `json:"critical"`
	Major      int          `json:"major"`
	Minor      int          `json:"minor"`
}

// ValidationResult holds the complete validation outcome.
type ValidationResult struct {
	TestPassed      bool           `json:"test_passed"`
	Coverage        *CoverageResult `json:"coverage,omitempty"`
	CoveragePassed  bool           `json:"coverage_passed"`
	LintResults     []LintResult   `json:"lint_results"`
	LintPassed      bool           `json:"lint_passed"`
	AllPassed       bool           `json:"all_passed"`
	Summary         string         `json:"summary"`
}

// Thresholds defines minimum quality gate requirements.
type Thresholds struct {
	MinLineCoverage      float64 `yaml:"min_line_coverage"`
	MinBranchCoverage    float64 `yaml:"min_branch_coverage"`
	MinFunctionCoverage  float64 `yaml:"min_function_coverage"`
	AllowCritical        bool    `yaml:"allow_critical"`
	AllowMajor           int     `yaml:"allow_major"`
}

// DefaultThresholds returns sensible default thresholds.
func DefaultThresholds() *Thresholds {
	return &Thresholds{
		MinLineCoverage:     80.0,
		MinBranchCoverage:   70.0,
		MinFunctionCoverage: 80.0,
		AllowCritical:       false,
		AllowMajor:          0,
	}
}

// Checker orchestrates all validation checks.
type Checker struct {
	thresholds *Thresholds
	lang       project.Lang
	sandbox    *sandbox.Executor
	profile    *project.Profile
}

// New creates a validation checker.
func New(thresholds *Thresholds, profile *project.Profile) (*Checker, error) {
	if thresholds == nil {
		thresholds = DefaultThresholds()
	}

	sb, err := sandbox.New(nil, "/tmp/autodev-validator")
	if err != nil {
		return nil, err
	}

	return &Checker{
		thresholds: thresholds,
		sandbox:    sb,
		profile:    profile,
	}, nil
}

// Run executes all validation checks (test + coverage + lint).
func (c *Checker) Run(ctx context.Context) (*ValidationResult, error) {
	result := &ValidationResult{AllPassed: true}

	// 1. Run tests with coverage
	testResult, err := c.runTestsWithCoverage(ctx)
	if err != nil {
		result.TestPassed = false
		result.AllPassed = false
		result.Summary = fmt.Sprintf("Tests failed: %v", err)
		return result, nil
	}

	result.TestPassed = testResult.Passed
	result.Coverage = testResult.Coverage()
	result.CoveragePassed = c.checkCoverage(result.Coverage)

	if !result.CoveragePassed {
		result.AllPassed = false
	}

	// 2. Run lint/static analysis
	lintResults, err := c.runLint(ctx)
	if err == nil {
		result.LintResults = lintResults
		result.LintPassed = c.checkLint(lintResults)
	}

	if !result.LintPassed {
		result.AllPassed = false
	}

	result.Summary = c.generateSummary(result)
	return result, nil
}

// TestOutput holds the result of test execution.
type TestOutput struct {
	Passed       bool
	Output       string
	CoverageRaw  string
	Duration     time.Duration
	Error        error
}

func (t *TestOutput) Coverage() *CoverageResult {
	if t.CoverageRaw == "" {
		return nil
	}
	return parseCoverage(t.CoverageRaw)
}

// runTestsWithCoverage detects the project language and runs appropriate tests.
func (c *Checker) runTestsWithCoverage(ctx context.Context) (*TestOutput, error) {
	if c.profile == nil {
		return nil, fmt.Errorf("no project profile available")
	}

	switch c.profile.Language {
	case project.LangGo:
		return c.runGoTests(ctx)
	case project.LangJS, project.LangTS:
		return c.runJSTests(ctx)
	case project.LangPython:
		return c.runPythonTests(ctx)
	default:
		return nil, fmt.Errorf("unsupported language for test execution: %s", c.profile.Language)
	}
}

func (c *Checker) runGoTests(ctx context.Context) (*TestOutput, error) {
	start := time.Now()

	testCmd := exec.CommandContext(ctx, "go", "test", "./...", "-v", "-coverprofile=/tmp/autodev-validator/coverage.out")
	testCmd.Dir = c.profile.RootPath

	output, err := testCmd.CombinedOutput()
	duration := time.Since(start)

	result := &TestOutput{
		Output:   string(output),
		Duration: duration,
	}

	if err != nil {
		result.Passed = false
		result.Error = err
		return result, err
	}

	result.Passed = true

	// Parse coverage
	coverageCmd := exec.Command("go", "tool", "cover", "-func=/tmp/autodev-validator/coverage.out")
	coverageOutput, err := coverageCmd.CombinedOutput()
	if err == nil {
		result.CoverageRaw = string(coverageOutput)
	}

	return result, nil
}

func (c *Checker) runJSTests(ctx context.Context) (*TestOutput, error) {
	start := time.Now()

	// Detect test framework
	hasJest := c.fileExists("jest.config.js") || c.fileExists("jest.config.ts") ||
		c.hasPackageDep("jest")
	hasVitest := c.hasPackageDep("vitest")

	var cmd *exec.Cmd
	if hasVitest {
		cmd = exec.CommandContext(ctx, "npx", "vitest", "run", "--coverage")
	} else if hasJest {
		cmd = exec.CommandContext(ctx, "npx", "jest", "--coverage")
	} else if c.hasNPMScript("test") {
		cmd = exec.CommandContext(ctx, "npm", "test", "--", "--coverage")
	} else {
		return nil, fmt.Errorf("no test framework detected")
	}

	cmd.Dir = c.profile.RootPath
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &TestOutput{
		Output:   string(output),
		Duration: duration,
	}

	if err != nil {
		result.Passed = false
		result.Error = err
		return result, err
	}

	result.Passed = true
	// Coverage would be in coverage/ directory for JS frameworks
	return result, nil
}

func (c *Checker) runPythonTests(ctx context.Context) (*TestOutput, error) {
	start := time.Now()

	var cmd *exec.Cmd

	if c.fileExists("pytest.ini") || c.fileExists("pyproject.toml") || c.fileExists("setup.cfg") {
		cmd = exec.CommandContext(ctx, "python", "-m", "pytest", "--cov=.", "--cov-report=term-missing")
	} else if c.fileExists("tox.ini") {
		cmd = exec.CommandContext(ctx, "tox")
	} else {
		// Fallback: run unittest
		cmd = exec.CommandContext(ctx, "python", "-m", "unittest", "discover", "-v")
	}

	cmd.Dir = c.profile.RootPath
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &TestOutput{
		Output:   string(output),
		Duration: duration,
	}

	if err != nil {
		result.Passed = false
		result.Error = err
		return result, err
	}

	result.Passed = true
	result.CoverageRaw = parsePythonCoverage(string(output))
	return result, nil
}

// runLint detects and runs the appropriate linter for the project.
func (c *Checker) runLint(ctx context.Context) ([]LintResult, error) {
	var results []LintResult

	switch c.profile.Language {
	case project.LangGo:
		r, err := c.runGoLint(ctx)
		if err == nil {
			results = append(results, r)
		}
	case project.LangJS, project.LangTS:
		r, err := c.runJSLint(ctx)
		if err == nil {
			results = append(results, r)
		}
	case project.LangPython:
		r, err := c.runPythonLint(ctx)
		if err == nil {
			results = append(results, r)
		}
	}

	return results, nil
}

func (c *Checker) runGoLint(ctx context.Context) (LintResult, error) {
	result := LintResult{Tool: "golangci-lint", Passed: true}
	start := time.Now()

	whichCmd := exec.Command("which", "golangci-lint")
	if err := whichCmd.Run(); err != nil {
		// Try go vet as fallback
		return c.runGoVet(ctx)
	}

	cmd := exec.CommandContext(ctx, "golangci-lint", "run", "--out-format=json")
	cmd.Dir = c.profile.RootPath

	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)

	if err != nil {
		// golangci-lint returns non-zero on issues found, which is expected
		result.Passed = false
		result.Violations = parseGoLintJSON(string(output))
	}

	result.Critical = countSeverity(result.Violations, SeverityCritical)
	result.Major = countSeverity(result.Violations, SeverityMajor)
	result.Minor = countSeverity(result.Violations, SeverityMinor)

	return result, nil
}

func (c *Checker) runGoVet(ctx context.Context) (LintResult, error) {
	result := LintResult{Tool: "go vet", Passed: true}
	start := time.Now()

	cmd := exec.CommandContext(ctx, "go", "vet", "./...")
	cmd.Dir = c.profile.RootPath

	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)

	if err != nil {
		result.Passed = false
		result.Violations = parseGoVetOutput(string(output))
		result.Major = len(result.Violations)
	}

	return result, nil
}

func (c *Checker) runJSLint(ctx context.Context) (LintResult, error) {
	result := LintResult{Tool: "eslint", Passed: true}
	start := time.Now()

	if !c.hasPackageDep("eslint") {
		return result, fmt.Errorf("eslint not found")
	}

	cmd := exec.CommandContext(ctx, "npx", "eslint", ".", "--format=json")
	cmd.Dir = c.profile.RootPath

	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)

	if err != nil {
		result.Passed = false
		result.Violations = parseESLintJSON(string(output))
	}

	result.Critical = countSeverity(result.Violations, SeverityCritical)
	result.Major = countSeverity(result.Violations, SeverityMajor)
	result.Minor = countSeverity(result.Violations, SeverityMinor)

	return result, nil
}

func (c *Checker) runPythonLint(ctx context.Context) (LintResult, error) {
	result := LintResult{Tool: "pylint", Passed: true}
	start := time.Now()

	var cmd *exec.Cmd

	if c.fileExists(".flake8") || c.hasPackageDep("flake8") {
		cmd = exec.CommandContext(ctx, "python", "-m", "flake8", ".", "--format=json")
		result.Tool = "flake8"
	} else if c.hasPackageDep("pylint") {
		cmd = exec.CommandContext(ctx, "python", "-m", "pylint", ".")
	} else if c.hasPackageDep("ruff") {
		cmd = exec.CommandContext(ctx, "ruff", "check", ".")
		result.Tool = "ruff"
	} else {
		return result, fmt.Errorf("no Python linter detected")
	}

	cmd.Dir = c.profile.RootPath
	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)

	if err != nil {
		result.Passed = false
		result.Violations = parsePythonLintOutput(result.Tool, string(output))
	}

	result.Critical = countSeverity(result.Violations, SeverityCritical)
	result.Major = countSeverity(result.Violations, SeverityMajor)
	result.Minor = countSeverity(result.Violations, SeverityMinor)

	return result, nil
}

// checkCoverage verifies coverage meets thresholds.
func (c *Checker) checkCoverage(cr *CoverageResult) bool {
	if cr == nil {
		return false
	}

	if c.thresholds.MinLineCoverage > 0 && cr.Line < c.thresholds.MinLineCoverage {
		return false
	}
	if c.thresholds.MinBranchCoverage > 0 && cr.Branch < c.thresholds.MinBranchCoverage {
		return false
	}
	if c.thresholds.MinFunctionCoverage > 0 && cr.Function < c.thresholds.MinFunctionCoverage {
		return false
	}

	return true
}

// checkLint verifies lint results meet quality gates.
func (c *Checker) checkLint(results []LintResult) bool {
	for _, r := range results {
		if !c.thresholds.AllowCritical && r.Critical > 0 {
			return false
		}
		if r.Major > c.thresholds.AllowMajor {
			return false
		}
	}
	return true
}

func (c *Checker) generateSummary(result *ValidationResult) string {
	parts := []string{}

	if result.TestPassed {
		parts = append(parts, "Tests: PASSED")
	} else {
		parts = append(parts, "Tests: FAILED")
	}

	if result.Coverage != nil {
		parts = append(parts, fmt.Sprintf("Coverage: %.1f%% line, %.1f%% branch",
			result.Coverage.Line, result.Coverage.Branch))
		if result.CoveragePassed {
			parts[len(parts)-1] += " (PASSED)"
		} else {
			parts[len(parts)-1] += " (FAILED)"
		}
	}

	for _, lr := range result.LintResults {
		status := "PASSED"
		if !lr.Passed {
			status = "FAILED"
		}
		parts = append(parts, fmt.Sprintf("%s: %s (%d critical, %d major)",
			lr.Tool, status, lr.Critical, lr.Major))
	}

	if result.AllPassed {
		return "All checks PASSED: " + strings.Join(parts, ", ")
	}
	return "Some checks FAILED: " + strings.Join(parts, ", ")
}

// Helper methods

func (c *Checker) fileExists(path string) bool {
	_, err := os.Stat(filepath.Join(c.profile.RootPath, path))
	return err == nil
}

func (c *Checker) hasNPMScript(script string) bool {
	packageJSON := filepath.Join(c.profile.RootPath, "package.json")
	data, err := os.ReadFile(packageJSON)
	if err != nil {
		return false
	}

	var pkg map[string]any
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}

	scripts, ok := pkg["scripts"].(map[string]any)
	if !ok {
		return false
	}

	_, exists := scripts[script]
	return exists
}

func (c *Checker) hasPackageDep(name string) bool {
	// Check Node.js package.json dependencies
	packageJSON := filepath.Join(c.profile.RootPath, "package.json")
	if data, err := os.ReadFile(packageJSON); err == nil {
		var pkg map[string]any
		if err := json.Unmarshal(data, &pkg); err == nil {
			for _, depKey := range []string{"dependencies", "devDependencies", "peerDependencies"} {
				if deps, ok := pkg[depKey].(map[string]any); ok {
					if _, exists := deps[name]; exists {
						return true
					}
				}
			}
		}
	}

	// Check Go module dependencies
	goMod := filepath.Join(c.profile.RootPath, "go.mod")
	if data, err := os.ReadFile(goMod); err == nil {
		// Simple string search for the package name in go.mod
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "require") || strings.HasPrefix(line, name) {
				if strings.Contains(line, name) {
					return true
				}
			}
		}
	}

	return false
}

// Coverage parsers

func parseCoverage(raw string) *CoverageResult {
	cr := &CoverageResult{}

	// Parse go tool cover output format: "total: (statements)\tXX.X%"
	lineRe := regexp.MustCompile(`total:\s+.*\s+([\d.]+)%`)
	if match := lineRe.FindStringSubmatch(raw); len(match) > 1 {
		cr.Line, _ = strconv.ParseFloat(match[1], 64)
	}

	// Statement = Line for Go
	cr.Statement = cr.Line
	return cr
}

func parsePythonCoverage(output string) string {
	// Extract pytest coverage summary
	re := regexp.MustCompile(`TOTAL\s+\d+\s+\d+\s+([\d]+)%`)
	if match := re.FindStringSubmatch(output); len(match) > 1 {
		return fmt.Sprintf("total: (statements)\t%s%%", match[1])
	}
	return ""
}

// Lint parsers

func parseGoLintJSON(raw string) []Violation {
	// Would parse golangci-lint JSON output
	var violations []Violation

	// Parse the standard golangci-lint text format
	re := regexp.MustCompile(`^(.+):(\d+):\d+:\s+(.+)$`)
	for _, line := range strings.Split(raw, "\n") {
		if match := re.FindStringSubmatch(line); len(match) >= 4 {
			v := Violation{
				File:    match[1],
				Line:    parseInt(match[2]),
				Message: match[3],
			}
			if strings.Contains(strings.ToLower(match[3]), "error") {
				v.Severity = SeverityCritical
			}
			violations = append(violations, v)
		}
	}

	return violations
}

func parseGoVetOutput(raw string) []Violation {
	var violations []Violation

	// go vet format: "package/path:file.go:line: message"
	re := regexp.MustCompile(`^([^:]+):([^:]+):(\d+):(.+)$`)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if match := re.FindStringSubmatch(line); len(match) >= 4 {
			violations = append(violations, Violation{
				File:    match[2],
				Line:    parseInt(match[3]),
				Message: strings.TrimSpace(match[4]),
				Severity: SeverityMajor,
			})
		}
	}

	return violations
}

func parseESLintJSON(raw string) []Violation {
	// Would parse ESLint JSON output
	var violations []Violation

	// Text format fallback: "file:line:column severity message"
	re := regexp.MustCompile(`^([^:]+):(\d+):(\d+)\s+(error|warning)\s+(.+)$`)
	for _, line := range strings.Split(raw, "\n") {
		if match := re.FindStringSubmatch(line); len(match) >= 5 {
			v := Violation{
				File:   match[1],
				Line:   parseInt(match[2]),
				Column: parseInt(match[3]),
				Message: match[5],
			}

			if match[4] == "error" {
				v.Severity = SeverityCritical
			} else {
				v.Severity = SeverityMinor
			}
			violations = append(violations, v)
		}
	}

	return violations
}

func parsePythonLintOutput(tool, raw string) []Violation {
	var violations []Violation

	switch tool {
	case "flake8":
		re := regexp.MustCompile(`^([^:]+):(\d+):\d+:\s+([A-Z]\d+)\s+(.+)$`)
		for _, line := range strings.Split(raw, "\n") {
			if match := re.FindStringSubmatch(line); len(match) >= 4 {
				v := Violation{
					File:    match[1],
					Line:    parseInt(match[2]),
					Rule:    match[3],
					Message: match[4],
				}
				if strings.HasPrefix(match[3], "E") {
					v.Severity = SeverityMajor
				} else if strings.HasPrefix(match[3], "W") {
					v.Severity = SeverityMinor
				} else if strings.HasPrefix(match[3], "F") {
					v.Severity = SeverityCritical
				}
				violations = append(violations, v)
			}
		}
	case "pylint":
		re := regexp.MustCompile(`^([^:]+):(\d+):\d+: (\w+): (.+)$`)
		for _, line := range strings.Split(raw, "\n") {
			if match := re.FindStringSubmatch(line); len(match) >= 4 {
				v := Violation{
					File:    match[1],
					Line:    parseInt(match[2]),
					Rule:    match[3],
					Message: match[4],
				}
				switch match[3] {
				case "fatal", "error":
					v.Severity = SeverityCritical
				case "warning", "convention":
					v.Severity = SeverityMinor
				case "refactor":
					v.Severity = SeverityInfo
				}
				violations = append(violations, v)
			}
		}
	default:
		// Generic parser
		re := regexp.MustCompile(`([^:]+):(\d+).*?(error|warning|info)`)
		for _, line := range strings.Split(raw, "\n") {
			if match := re.FindStringSubmatch(line); len(match) >= 3 {
				v := Violation{File: match[1], Line: parseInt(match[2]), Message: line}
				switch strings.ToLower(match[3]) {
				case "error", "fatal":
					v.Severity = SeverityCritical
				case "warning":
					v.Severity = SeverityMajor
				default:
					v.Severity = SeverityMinor
				}
				violations = append(violations, v)
			}
		}
	}

	return violations
}

func countSeverity(violations []Violation, sev Severity) int {
	count := 0
	for _, v := range violations {
		if v.Severity == sev {
			count++
		}
	}
	return count
}

func parseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
