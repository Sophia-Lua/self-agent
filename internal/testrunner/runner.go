package testrunner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"autodev/internal/project"
)

// Framework identifies the test framework being used.
type Framework string

const (
	FrameworkGoTest  Framework = "go test"
	FrameworkJest    Framework = "jest"
	FrameworkVitest  Framework = "vitest"
	FrameworkMocha   Framework = "mocha"
	FrameworkPytest  Framework = "pytest"
	FrameworkUnittest Framework = "unittest"
	FrameworkCargo   Framework = "cargo test"
	FrameworkUnknown Framework = "unknown"
)

// Result holds the outcome of a test run.
type Result struct {
	Passed       bool
	Total        int
	PassCount    int
	FailCount    int
	SkipCount    int
	Coverage     float64
	Duration     time.Duration
	Failures     []Failure
	Output       string
	Framework    Framework
}

// Failure represents a single test failure.
type Failure struct {
	Test    string
	File    string
	Line    int
	Message string
	Type    string
}

// Runner executes tests for a given project.
type Runner struct {
	profile *project.Profile
	workDir string
}

// New creates a test runner.
func New(profile *project.Profile) *Runner {
	return &Runner{
		profile: profile,
		workDir: profile.RootPath,
	}
}

// Run executes all tests with coverage.
func (r *Runner) Run(ctx context.Context) (*Result, error) {
	framework := r.detectFramework()

	switch framework {
	case FrameworkGoTest:
		return r.runGoTest(ctx, true)
	case FrameworkJest:
		return r.runJest(ctx, true)
	case FrameworkVitest:
		return r.runVitest(ctx, true)
	case FrameworkPytest:
		return r.runPytest(ctx, true)
	case FrameworkCargo:
		return r.runCargoTest(ctx, true)
	default:
		// Try go test as fallback
		if r.hasFile("go.mod") {
			return r.runGoTest(ctx, true)
		}
		return nil, fmt.Errorf("no supported test framework detected")
	}
}

// RunWithoutCoverage runs tests without coverage.
func (r *Runner) RunWithoutCoverage(ctx context.Context) (*Result, error) {
	framework := r.detectFramework()

	switch framework {
	case FrameworkGoTest:
		return r.runGoTest(ctx, false)
	case FrameworkJest:
		return r.runJest(ctx, false)
	case FrameworkVitest:
		return r.runVitest(ctx, false)
	case FrameworkPytest:
		return r.runPytest(ctx, false)
	case FrameworkCargo:
		return r.runCargoTest(ctx, false)
	default:
		return nil, fmt.Errorf("no supported test framework detected")
	}
}

// RunSpecific runs tests matching a pattern.
func (r *Runner) RunSpecific(ctx context.Context, pattern string) (*Result, error) {
	framework := r.detectFramework()

	switch framework {
	case FrameworkGoTest:
		return r.runGoTestSpecific(ctx, pattern)
	case FrameworkJest, FrameworkVitest:
		return r.runJSTestSpecific(ctx, pattern)
	case FrameworkPytest:
		return r.runPytestSpecific(ctx, pattern)
	default:
		return r.Run(ctx)
	}
}

func (r *Runner) detectFramework() Framework {
	// Go
	if r.hasFile("go.mod") {
		return FrameworkGoTest
	}

	// JavaScript/TypeScript
	if r.hasFile("package.json") {
		if r.hasFile("jest.config.js") || r.hasFile("jest.config.ts") || r.hasDep("jest") {
			return FrameworkJest
		}
		if r.hasDep("vitest") {
			return FrameworkVitest
		}
		if r.hasDep("mocha") {
			return FrameworkMocha
		}
	}

	// Python
	if r.hasFile("pytest.ini") || r.hasFile("pyproject.toml") || r.hasDep("pytest") {
		return FrameworkPytest
	}
	if r.hasFiles("test_*.py") || r.hasFiles("*_test.py") {
		return FrameworkUnittest
	}

	// Rust
	if r.hasFile("Cargo.toml") {
		return FrameworkCargo
	}

	return FrameworkUnknown
}

func (r *Runner) runGoTest(ctx context.Context, coverage bool) (*Result, error) {
	args := []string{"test", "./...", "-v", "-count=1"}
	if coverage {
		coverFile := filepath.Join(os.TempDir(), "autodev-coverage.out")
		args = append(args, "-coverprofile="+coverFile)
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = r.workDir

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &Result{
		Output:    string(output),
		Duration:  duration,
		Framework: FrameworkGoTest,
	}

	result.Passed = (err == nil)
	result.parseGoOutput(result.Output)

	if coverage {
		result.Coverage = r.parseGoCoverage()
	}

	return result, nil
}

func (r *Runner) runGoTestSpecific(ctx context.Context, pattern string) (*Result, error) {
	args := []string{"test", "./...", "-v", "-count=1", "-run", pattern}

	start := time.Now()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = r.workDir

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &Result{
		Output:    string(output),
		Duration:  duration,
		Framework: FrameworkGoTest,
	}

	result.Passed = (err == nil)
	result.parseGoOutput(result.Output)

	return result, nil
}

func (r *Runner) runJest(ctx context.Context, coverage bool) (*Result, error) {
	args := []string{"jest", "--verbose"}
	if coverage {
		args = append(args, "--coverage")
	}

	return r.runNpx(ctx, args, coverage)
}

func (r *Runner) runVitest(ctx context.Context, coverage bool) (*Result, error) {
	args := []string{"vitest", "run"}
	if coverage {
		args = append(args, "--coverage")
	}

	return r.runNpx(ctx, args, coverage)
}

func (r *Runner) runJSTestSpecific(ctx context.Context, pattern string) (*Result, error) {
	args := []string{"jest", "--verbose", "-t", pattern}
	return r.runNpx(ctx, args, false)
}

func (r *Runner) runPytest(ctx context.Context, coverage bool) (*Result, error) {
	args := []string{"-m", "pytest", "-v"}
	if coverage {
		args = append(args, "--cov=.", "--cov-report=term-missing")
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, "python", args...)
	cmd.Dir = r.workDir

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &Result{
		Output:    string(output),
		Duration:  duration,
		Framework: FrameworkPytest,
	}

	result.Passed = (err == nil)
	result.parsePytestOutput(result.Output)

	if coverage {
		result.Coverage = r.parsePythonCoverage(result.Output)
	}

	return result, nil
}

func (r *Runner) runPytestSpecific(ctx context.Context, pattern string) (*Result, error) {
	args := []string{"-m", "pytest", "-v", "-k", pattern}

	start := time.Now()
	cmd := exec.CommandContext(ctx, "python", args...)
	cmd.Dir = r.workDir

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &Result{
		Output:    string(output),
		Duration:  duration,
		Framework: FrameworkPytest,
	}

	result.Passed = (err == nil)
	result.parsePytestOutput(result.Output)

	return result, nil
}

func (r *Runner) runCargoTest(ctx context.Context, coverage bool) (*Result, error) {
	args := []string{"test", "--verbose"}
	if coverage {
		args = append(args, "--no-fail-fast")
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, "cargo", args...)
	cmd.Dir = r.workDir

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &Result{
		Output:    string(output),
		Duration:  duration,
		Framework: FrameworkCargo,
	}

	result.Passed = (err == nil)
	result.parseCargoOutput(result.Output)

	return result, nil
}

func (r *Runner) runNpx(ctx context.Context, args []string, coverage bool) (*Result, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, "npx", args...)
	cmd.Dir = r.workDir

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &Result{
		Output:    string(output),
		Duration:  duration,
		Framework: FrameworkJest,
	}

	result.Passed = (err == nil)
	result.parseJSOutput(result.Output)

	if coverage {
		result.Coverage = r.parseJSCoverage()
	}

	return result, nil
}

// parseGoOutput parses go test output.
func (r *Result) parseGoOutput(output string) {
	// Count tests
	// Pattern: --- PASS: TestName (0.00s)
	passRe := regexp.MustCompile(`--- PASS: (\S+)`)
	failRe := regexp.MustCompile(`--- FAIL: (\S+)`)
	skipRe := regexp.MustCompile(`--- SKIP: (\S+)`)

	r.PassCount = len(passRe.FindAllStringSubmatch(output, -1))
	r.FailCount = len(failRe.FindAllStringSubmatch(output, -1))
	r.SkipCount = len(skipRe.FindAllStringSubmatch(output, -1))
	r.Total = r.PassCount + r.FailCount + r.SkipCount

	// Parse failures
	failDetailRe := regexp.MustCompile(`--- FAIL: (\S+).*?\n(.*?)(?:---|\n\n)`)
	matches := failDetailRe.FindAllStringSubmatch(output, -1)
	for _, m := range matches {
		r.Failures = append(r.Failures, Failure{
			Test:    m[1],
			Message: strings.TrimSpace(m[2]),
		})
	}
}

// parsePytestOutput parses pytest output.
func (r *Result) parsePytestOutput(output string) {
	// Pattern: PASSED, FAILED, SKIPPED
	passRe := regexp.MustCompile(`PASSED`)
	failRe := regexp.MustCompile(`FAILED`)
	skipRe := regexp.MustCompile(`SKIPPED`)

	r.PassCount = len(passRe.FindAllString(output, -1))
	r.FailCount = len(failRe.FindAllString(output, -1))
	r.SkipCount = len(skipRe.FindAllString(output, -1))
	r.Total = r.PassCount + r.FailCount + r.SkipCount

	// Parse failure details
	failDetailRe := regexp.MustCompile(`FAILED (?:\S+::)?(\S+) - (.+?)\n`)
	matches := failDetailRe.FindAllStringSubmatch(output, -1)
	for _, m := range matches {
		r.Failures = append(r.Failures, Failure{
			Test:    m[1],
			Message: m[2],
		})
	}
}

// parseJSOutput parses Jest/Vitest output.
func (r *Result) parseJSOutput(output string) {
	// Jest summary: Tests:  12 passed, 2 failed, 1 skipped, 15 total
	summaryRe := regexp.MustCompile(`Tests:\s+(\d+)\s+passed,\s+(\d+)\s+failed,\s+(\d+)\s+skipped,\s+(\d+)\s+total`)
	matches := summaryRe.FindStringSubmatch(output)
	if len(matches) >= 5 {
		r.PassCount, _ = strconv.Atoi(matches[1])
		r.FailCount, _ = strconv.Atoi(matches[2])
		r.SkipCount, _ = strconv.Atoi(matches[3])
		r.Total, _ = strconv.Atoi(matches[4])
	}

	// Vitest alternative: 12 passed, 2 failed, 1 skipped (15)
	if r.Total == 0 {
		vitestRe := regexp.MustCompile(`(\d+)\s+passed,\s+(\d+)\s+failed,\s+(\d+)\s+skipped.*?\((\d+)\)`)
		matches = vitestRe.FindStringSubmatch(output)
		if len(matches) >= 5 {
			r.PassCount, _ = strconv.Atoi(matches[1])
			r.FailCount, _ = strconv.Atoi(matches[2])
			r.SkipCount, _ = strconv.Atoi(matches[3])
			r.Total, _ = strconv.Atoi(matches[4])
		}
	}
}

// parseCargoOutput parses cargo test output.
func (r *Result) parseCargoOutput(output string) {
	// Pattern: test result: ok. 12 passed; 0 failed; 0 ignored
	re := regexp.MustCompile(`test result:\s+ok\.\s+(\d+)\s+passed;\s+(\d+)\s+failed;\s+(\d+)\s+ignored`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 4 {
		r.PassCount, _ = strconv.Atoi(matches[1])
		r.FailCount, _ = strconv.Atoi(matches[2])
		r.SkipCount, _ = strconv.Atoi(matches[3])
		r.Total = r.PassCount + r.FailCount + r.SkipCount
	}
}

func (r *Runner) parseGoCoverage() float64 {
	coverFile := filepath.Join(os.TempDir(), "autodev-coverage.out")
	output, err := exec.Command("go", "tool", "cover", "-func="+coverFile).CombinedOutput()
	if err != nil {
		return 0
	}

	// Last line: total: (statements)  85.7%
	coverageRe := regexp.MustCompile(`total:\s+.*\s+([\d.]+)%`)
	matches := coverageRe.FindStringSubmatch(string(output))
	if len(matches) >= 2 {
		cov, _ := strconv.ParseFloat(matches[1], 64)
		return cov
	}

	return 0
}

func (r *Runner) parsePythonCoverage(output string) float64 {
	// Pytest coverage: TOTAL   120    34    72%
	re := regexp.MustCompile(`TOTAL\s+\d+\s+\d+\s+(\d+)%`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		cov, _ := strconv.ParseFloat(matches[1], 64)
		return cov
	}

	return 0
}

func (r *Runner) parseJSCoverage() float64 {
	// Jest puts coverage in coverage/ directory
	covDir := filepath.Join(r.workDir, "coverage", "coverage-summary.json")
	if _, err := os.Stat(covDir); err != nil {
		return 0
	}

	// Would parse coverage-summary.json
	return 0
}

// Helper methods

func (r *Runner) hasFile(path string) bool {
	_, err := os.Stat(filepath.Join(r.workDir, path))
	return err == nil
}

func (r *Runner) hasFiles(pattern string) bool {
	matches, err := filepath.Glob(filepath.Join(r.workDir, pattern))
	return err == nil && len(matches) > 0
}

func (r *Runner) hasDep(name string) bool {
	pkgFile := filepath.Join(r.workDir, "package.json")
	data, err := os.ReadFile(pkgFile)
	if err != nil {
		return false
	}

	content := string(data)
	return strings.Contains(content, name)
}

// FormatSummary returns a human-readable summary.
func (r *Result) FormatSummary() string {
	status := "PASSED"
	if !r.Passed {
		status = "FAILED"
	}

	parts := []string{
		fmt.Sprintf("Tests: %d total, %d passed, %d failed, %d skipped",
			r.Total, r.PassCount, r.FailCount, r.SkipCount),
		fmt.Sprintf("Coverage: %.1f%%", r.Coverage),
		fmt.Sprintf("Duration: %v", r.Duration.Round(time.Millisecond)),
		fmt.Sprintf("Status: %s", status),
	}

	if len(r.Failures) > 0 {
		parts = append(parts, "\nFailures:")
		for _, f := range r.Failures {
			parts = append(parts, fmt.Sprintf("  - %s: %s", f.Test, f.Message))
		}
	}

	return strings.Join(parts, "\n")
}

// GenerateTestScaffold generates a basic test file for a new function.
func GenerateTestScaffold(lang project.Lang, funcName string) string {
	switch lang {
	case project.LangGo:
		return fmt.Sprintf(`package main

import "testing"

func Test%s(t *testing.T) {
	tests := []struct {
		name    string
		want    string
	}{
		{"basic case", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := %s(); got != tt.want {
				t.Errorf("%s() = %%v, want %%v", got, tt.want)
			}
		})
	}
}
`, funcName, funcName, funcName)

	case project.LangJS, project.LangTS:
		return fmt.Sprintf(`describe('%s', () => {
  it('should work correctly', () => {
    expect(%s()).toBe(expected);
  });
});
`, funcName, funcName)

	case project.LangPython:
		return fmt.Sprintf(`def test_%s():
    """Test %s function."""
    result = %s()
    expected = None
    assert result == expected, f"Expected {{expected}}, got {{result}}"
`, funcName, funcName, funcName)
	}

	return ""
}
