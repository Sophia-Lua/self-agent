package testrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"autodev/internal/project"
)

func TestFrameworkConstants(t *testing.T) {
	if FrameworkGoTest != "go test" {
		t.Errorf("expected 'go test', got %s", FrameworkGoTest)
	}
	if FrameworkJest != "jest" {
		t.Errorf("expected 'jest', got %s", FrameworkJest)
	}
	if FrameworkVitest != "vitest" {
		t.Errorf("expected 'vitest', got %s", FrameworkVitest)
	}
	if FrameworkPytest != "pytest" {
		t.Errorf("expected 'pytest', got %s", FrameworkPytest)
	}
	if FrameworkCargo != "cargo test" {
		t.Errorf("expected 'cargo test', got %s", FrameworkCargo)
	}
	if FrameworkUnknown != "unknown" {
		t.Errorf("expected 'unknown', got %s", FrameworkUnknown)
	}
}

func TestNew(t *testing.T) {
	profile := &project.Profile{RootPath: "/tmp/test"}
	r := New(profile)

	if r.profile != profile {
		t.Error("profile not set correctly")
	}
	if r.workDir != "/tmp/test" {
		t.Errorf("expected workDir '/tmp/test', got %s", r.workDir)
	}
}

func TestResultParseGoOutput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantPass  int
		wantFail  int
		wantSkip  int
		wantTotal int
	}{
		{
			name:      "all pass",
			input:     "--- PASS: TestA (0.00s)\n--- PASS: TestB (0.01s)\n",
			wantPass:  2,
			wantTotal: 2,
		},
		{
			name:      "mixed results",
			input:     "--- PASS: TestA (0.00s)\n--- FAIL: TestB (0.01s)\n--- SKIP: TestC (0.00s)\n",
			wantPass:  1,
			wantFail:  1,
			wantSkip:  1,
			wantTotal: 3,
		},
		{
			name:      "empty",
			input:     "",
			wantTotal: 0,
		},
		{
			name:      "no test output",
			input:     "some random output",
			wantTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Result{}
			r.parseGoOutput(tt.input)

			if r.PassCount != tt.wantPass {
				t.Errorf("PassCount = %d, want %d", r.PassCount, tt.wantPass)
			}
			if r.FailCount != tt.wantFail {
				t.Errorf("FailCount = %d, want %d", r.FailCount, tt.wantFail)
			}
			if r.SkipCount != tt.wantSkip {
				t.Errorf("SkipCount = %d, want %d", r.SkipCount, tt.wantSkip)
			}
			if r.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", r.Total, tt.wantTotal)
			}
		})
	}
}

func TestResultParseGoOutputFailures(t *testing.T) {
	// The regex requires double newline after failure message
	input := `=== RUN   TestA
--- FAIL: TestA (0.01s)
    main_test.go:10: expected 5, got 3

=== RUN   TestB
--- PASS: TestB (0.00s)
`
	r := &Result{}
	r.parseGoOutput(input)

	// Just verify we counted the failure correctly
	if r.FailCount != 1 {
		t.Errorf("expected 1 failure, got %d", r.FailCount)
	}
}

func TestResultParsePytestOutputFailures(t *testing.T) {
	// The regex requires FAILED on same line with test name
	input := `FAILED test_a.py::test_bad - AssertionError\nassert 1 == 2

PASSED test_b.py::test_good
`
	r := &Result{}
	r.parsePytestOutput(input)

	if len(r.Failures) < 1 {
		t.Fatalf("expected at least 1 failure, got %d", len(r.Failures))
	}

	if r.Failures[0].Test != "test_bad" {
		t.Errorf("expected test 'test_bad', got %s", r.Failures[0].Test)
	}
}

func TestResultParsePytestOutput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantPass  int
		wantFail  int
		wantSkip  int
		wantTotal int
	}{
		{
			name:      "all pass",
			input:     "test_a.py::test_1 PASSED\ntest_b.py::test_2 PASSED\n",
			wantPass:  2,
			wantTotal: 2,
		},
		{
			name:      "mixed results",
			input:     "test_a.py PASSED\ntest_b.py FAILED\ntest_c.py SKIPPED\n",
			wantPass:  1,
			wantFail:  1,
			wantSkip:  1,
			wantTotal: 3,
		},
		{
			name:      "empty",
			input:     "",
			wantTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Result{}
			r.parsePytestOutput(tt.input)

			if r.PassCount != tt.wantPass {
				t.Errorf("PassCount = %d, want %d", r.PassCount, tt.wantPass)
			}
			if r.FailCount != tt.wantFail {
				t.Errorf("FailCount = %d, want %d", r.FailCount, tt.wantFail)
			}
			if r.SkipCount != tt.wantSkip {
				t.Errorf("SkipCount = %d, want %d", r.SkipCount, tt.wantSkip)
			}
			if r.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", r.Total, tt.wantTotal)
			}
		})
	}
}

func TestResultParseJSOutput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantPass  int
		wantFail  int
		wantSkip  int
		wantTotal int
	}{
		{
			name:      "jest summary",
			input:     "Tests:  12 passed, 2 failed, 1 skipped, 15 total",
			wantPass:  12,
			wantFail:  2,
			wantSkip:  1,
			wantTotal: 15,
		},
		{
			name:      "vitest format",
			input:     "5 passed, 1 failed, 0 skipped (6)",
			wantPass:  5,
			wantFail:  1,
			wantSkip:  0,
			wantTotal: 6,
		},
		{
			name:      "empty",
			input:     "",
			wantTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Result{}
			r.parseJSOutput(tt.input)

			if r.PassCount != tt.wantPass {
				t.Errorf("PassCount = %d, want %d", r.PassCount, tt.wantPass)
			}
			if r.FailCount != tt.wantFail {
				t.Errorf("FailCount = %d, want %d", r.FailCount, tt.wantFail)
			}
			if r.SkipCount != tt.wantSkip {
				t.Errorf("SkipCount = %d, want %d", r.SkipCount, tt.wantSkip)
			}
			if r.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", r.Total, tt.wantTotal)
			}
		})
	}
}

func TestResultParseCargoOutput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantPass  int
		wantFail  int
		wantSkip  int
		wantTotal int
	}{
		{
			name:      "cargo ok",
			input:     "test result: ok. 12 passed; 0 failed; 0 ignored",
			wantPass:  12,
			wantFail:  0,
			wantSkip:  0,
			wantTotal: 12,
		},
		{
			name:      "cargo with failures",
			input:     "test result: ok. 8 passed; 2 failed; 1 ignored",
			wantPass:  8,
			wantFail:  2,
			wantSkip:  1,
			wantTotal: 11,
		},
		{
			name:      "empty",
			input:     "",
			wantTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Result{}
			r.parseCargoOutput(tt.input)

			if r.PassCount != tt.wantPass {
				t.Errorf("PassCount = %d, want %d", r.PassCount, tt.wantPass)
			}
			if r.FailCount != tt.wantFail {
				t.Errorf("FailCount = %d, want %d", r.FailCount, tt.wantFail)
			}
		})
	}
}

func TestResultFormatSummary(t *testing.T) {
	tests := []struct {
		name     string
		result   *Result
		contains []string
	}{
		{
			name: "passed with coverage",
			result: &Result{
				Passed:    true,
				Total:     10,
				PassCount: 10,
				FailCount: 0,
				SkipCount: 0,
				Coverage:  85.5,
				Duration:  2 * time.Second,
				Framework: FrameworkGoTest,
			},
			contains: []string{
				"Tests: 10 total, 10 passed, 0 failed, 0 skipped",
				"Coverage: 85.5%",
				"Status: PASSED",
			},
		},
		{
			name: "failed with details",
			result: &Result{
				Passed:    false,
				Total:     5,
				PassCount: 3,
				FailCount: 2,
				SkipCount: 0,
				Coverage:  70.0,
				Duration:  500 * time.Millisecond,
				Framework: FrameworkPytest,
				Failures:  []Failure{{Test: "TestA", Message: "expected 1, got 2"}},
			},
			contains: []string{
				"Tests: 5 total, 3 passed, 2 failed, 0 skipped",
				"Status: FAILED",
				"Failures:",
				"TestA",
			},
		},
		{
			name: "zero coverage",
			result: &Result{
				Passed:   true,
				Coverage: 0,
			},
			contains: []string{"Coverage: 0.0%"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := tt.result.FormatSummary()
			for _, expected := range tt.contains {
				if !strings.Contains(summary, expected) {
					t.Errorf("summary missing %q\nGot: %s", expected, summary)
				}
			}
		})
	}
}

func TestGenerateTestScaffold_Go(t *testing.T) {
	result := GenerateTestScaffold(project.LangGo, "MyFunction")

	if !strings.Contains(result, "func TestMyFunction") {
		t.Error("missing TestMyFunction")
	}
	if !strings.Contains(result, `got := MyFunction()`) {
		t.Error("missing MyFunction call")
	}
	if !strings.Contains(result, "testing") {
		t.Error("missing testing import")
	}
}

func TestGenerateTestScaffold_JS(t *testing.T) {
	result := GenerateTestScaffold(project.LangJS, "myFunction")

	if !strings.Contains(result, "describe('myFunction'") {
		t.Error("missing describe block")
	}
	if !strings.Contains(result, "myFunction()") {
		t.Error("missing function call")
	}
	if !strings.Contains(result, "expect") {
		t.Error("missing expect")
	}
}

func TestGenerateTestScaffold_TS(t *testing.T) {
	result := GenerateTestScaffold(project.LangTS, "myType")

	if !strings.Contains(result, "describe('myType'") {
		t.Error("missing describe block for TS")
	}
}

func TestGenerateTestScaffold_Python(t *testing.T) {
	result := GenerateTestScaffold(project.LangPython, "my_function")

	if !strings.Contains(result, "def test_my_function") {
		t.Error("missing test function definition")
	}
	if !strings.Contains(result, "assert") {
		t.Error("missing assertion")
	}
}

func TestGenerateTestScaffold_Unknown(t *testing.T) {
	result := GenerateTestScaffold("cobol", "test")

	if result != "" {
		t.Errorf("expected empty string for unknown language, got %s", result)
	}
}

func TestDetectFramework_Go(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(file, []byte("module test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	r := &Runner{
		profile: &project.Profile{RootPath: dir},
		workDir: dir,
	}

	if r.detectFramework() != FrameworkGoTest {
		t.Errorf("expected FrameworkGoTest, got %s", r.detectFramework())
	}
}

func TestDetectFramework_Jest(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "package.json")
	jestCfg := filepath.Join(dir, "jest.config.js")

	if err := os.WriteFile(pkg, []byte(`{}`), 0644); err != nil {
		t.Fatal(dir)
	}
	if err := os.WriteFile(jestCfg, []byte(`module.exports = {}`), 0644); err != nil {
		t.Fatal(dir)
	}

	r := &Runner{
		profile: &project.Profile{RootPath: dir},
		workDir: dir,
	}

	if r.detectFramework() != FrameworkJest {
		t.Errorf("expected FrameworkJest, got %s", r.detectFramework())
	}
}

func TestDetectFramework_Vitest(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "package.json")
	if err := os.WriteFile(pkg, []byte(`{"devDependencies": {"vitest": "^1.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	r := &Runner{
		profile: &project.Profile{RootPath: dir},
		workDir: dir,
	}

	if r.detectFramework() != FrameworkVitest {
		t.Errorf("expected FrameworkVitest, got %s", r.detectFramework())
	}
}

func TestDetectFramework_Pytest(t *testing.T) {
	dir := t.TempDir()
	pytestIni := filepath.Join(dir, "pytest.ini")
	if err := os.WriteFile(pytestIni, []byte("[pytest]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	r := &Runner{
		profile: &project.Profile{RootPath: dir},
		workDir: dir,
	}

	if r.detectFramework() != FrameworkPytest {
		t.Errorf("expected FrameworkPytest, got %s", r.detectFramework())
	}
}

func TestDetectFramework_Cargo(t *testing.T) {
	dir := t.TempDir()
	cargo := filepath.Join(dir, "Cargo.toml")
	if err := os.WriteFile(cargo, []byte(`[package]`), 0644); err != nil {
		t.Fatal(err)
	}

	r := &Runner{
		profile: &project.Profile{RootPath: dir},
		workDir: dir,
	}

	if r.detectFramework() != FrameworkCargo {
		t.Errorf("expected FrameworkCargo, got %s", r.detectFramework())
	}
}

func TestDetectFramework_Unknown(t *testing.T) {
	dir := t.TempDir()

	r := &Runner{
		profile: &project.Profile{RootPath: dir},
		workDir: dir,
	}

	if r.detectFramework() != FrameworkUnknown {
		t.Errorf("expected FrameworkUnknown, got %s", r.detectFramework())
	}
}

func TestRunnerHelperMethods(t *testing.T) {
	dir := t.TempDir()

	r := &Runner{
		profile: &project.Profile{RootPath: dir},
		workDir: dir,
	}

	t.Run("hasFile existing", func(t *testing.T) {
		file := filepath.Join(dir, "test.txt")
		if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		if !r.hasFile("test.txt") {
			t.Error("expected hasFile to return true")
		}
	})

	t.Run("hasFile missing", func(t *testing.T) {
		if r.hasFile("nonexistent.txt") {
			t.Error("expected hasFile to return false")
		}
	})

	t.Run("hasFiles matching", func(t *testing.T) {
		file := filepath.Join(dir, "test_foo.py")
		if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		if !r.hasFiles("test_*.py") {
			t.Error("expected hasFiles to return true")
		}
	})

	t.Run("hasFiles no match", func(t *testing.T) {
		if r.hasFiles("nonexistent_*.py") {
			t.Error("expected hasFiles to return false")
		}
	})

	t.Run("hasDep present", func(t *testing.T) {
		pkg := filepath.Join(dir, "package.json")
		if err := os.WriteFile(pkg, []byte(`{"dependencies": {"react": "18.0"}}`), 0644); err != nil {
			t.Fatal(err)
		}
		if !r.hasDep("react") {
			t.Error("expected hasDep to return true")
		}
	})

	t.Run("hasDep missing", func(t *testing.T) {
		if r.hasDep("nonexistent-dep") {
			t.Error("expected hasDep to return false")
		}
	})

	t.Run("hasDep no package.json", func(t *testing.T) {
		dir2 := t.TempDir()
		r2 := &Runner{
			profile: &project.Profile{RootPath: dir2},
			workDir: dir2,
		}
		if r2.hasDep("anything") {
			t.Error("expected hasDep to return false when no package.json")
		}
	})
}

func TestResultStructure(t *testing.T) {
	r := Result{
		Passed:    true,
		Total:     10,
		PassCount: 8,
		FailCount: 1,
		SkipCount: 1,
		Coverage:  85.0,
		Duration:  time.Second,
		Framework: FrameworkGoTest,
		Failures:  []Failure{{Test: "X"}},
		Output:    "test output",
	}

	if !r.Passed {
		t.Error("Passed not set correctly")
	}
	if r.Total != 10 {
		t.Error("Total not set correctly")
	}
	if r.Framework != FrameworkGoTest {
		t.Error("Framework not set correctly")
	}
}

func TestFailureStructure(t *testing.T) {
	f := Failure{
		Test:    "TestA",
		File:    "main_test.go",
		Line:    42,
		Message: "expected X to equal Y",
		Type:    "assertion",
	}

	if f.Test != "TestA" || f.Line != 42 {
		t.Error("Failure fields not set correctly")
	}
}
